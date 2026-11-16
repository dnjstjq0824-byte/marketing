package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"somun/internal/store"
)

// AI generation errors.
var (
	ErrGenBadKind      = errors.New("kind must be gen_image or gen_video")
	ErrGenBadPrompt    = errors.New("prompt is required (max 2000 characters)")
	ErrGenNoAccount    = errors.New("account not found")
	ErrGenNotConnected = errors.New("연결되지 않은 계정입니다")
	ErrGenNoCap        = errors.New("이 계정은 생성 기능이 없습니다")
	ErrGenUnavailable  = errors.New("이 플랫폼의 실생성은 아직 확인되지 않았습니다")
	ErrGenFailed       = errors.New("AI가 만들지 못했습니다")
	ErrGenNoPython     = errors.New("adapter venv 없음")
)

// genTimeoutSec bounds one adapter generation run (video renders slowly).
const genTimeoutSec = 900

// MaxPromptRunes bounds one generation prompt.
const MaxPromptRunes = 2000

// genRatios are the accepted aspect hints; the adapter passes them to the
// session UI when the platform supports it.
var genRatios = map[string]bool{"1:1": true, "4:5": true, "9:16": true, "16:9": true, "3:4": true}

// Configure sets the data dir and adapter venv python for AI generation.
func (s *Service) Configure(dataDir, pythonPath string) {
	if abs, err := filepath.Abs(dataDir); err == nil {
		dataDir = abs
	}
	s.dataDir = dataDir
	s.python = pythonPath
}

// genRunner replaces the adapter.genmedia subprocess in tests. It writes
// the generated file into outDir and returns its absolute path plus the
// observed dimensions (duration for video).
type genRunner func(account, platform, kind, prompt, ratio, outDir string) (genFile, error)

type genFile struct {
	Path        string
	Width       int
	Height      int
	DurationSec float64
}

// GenViaAI generates one image or video through a connected AI account's
// session and stores it as a tracked media row. Unverified platforms
// refuse with ErrGenUnavailable: only live-session-confirmed generation
// from adapter.genmedia ever lands here.
func (s *Service) GenViaAI(accountID int64, kind, prompt, ratio string, contentID, productID int64) (*store.Media, error) {
	var needCap string
	switch kind {
	case store.MediaKindGenImage:
		needCap = store.MediaCapImage
	case store.MediaKindGenVideo:
		needCap = store.MediaCapVideo
	default:
		return nil, ErrGenBadKind
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" || len([]rune(prompt)) > MaxPromptRunes {
		return nil, ErrGenBadPrompt
	}
	if !genRatios[ratio] {
		ratio = "1:1"
		if kind == store.MediaKindGenVideo {
			ratio = "9:16"
		}
	}
	row, err := s.store.GetAccountRow(accountID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrGenNoAccount
		}
		return nil, err
	}
	if row.SessionState != "connected" {
		return nil, ErrGenNotConnected
	}
	if !store.AccountHasCap(row.Capabilities, needCap) {
		return nil, ErrGenNoCap
	}
	outDir, err := os.MkdirTemp("", "somun-gen-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(outDir)
	got, err := s.runGen(row.Name, strings.TrimSpace(row.Platform), kind, prompt, ratio, outDir)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(got.Path)
	if err != nil || len(raw) == 0 {
		return nil, ErrGenFailed
	}
	sum := sha256.Sum256(raw)
	sha := hex.EncodeToString(sum[:])
	if dup, err := s.store.MediaByHash(sha); err == nil {
		return dup, nil
	}
	mime := http.DetectContentType(head512(raw))
	ext := mimeExt(mime, got.Path)
	name := filepath.Join("lib", "gen-"+sha[:12]+ext)
	dest := filepath.Join(s.uploadDir, name)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(dest, raw, 0o644); err != nil {
		return nil, err
	}
	rel := filepath.ToSlash(name)
	attrs := store.MediaAttrs{
		Kind: store.MediaKindGenImage, Path: rel, Bytes: int64(len(raw)),
		Mime: mime, SHA256: sha, Prompt: prompt, Source: store.MediaSourceGenerated,
		Width: got.Width, Height: got.Height, DurationSec: got.DurationSec,
	}
	if kind == store.MediaKindGenVideo {
		attrs.Kind = store.MediaKindGenVideo
	}
	if contentID > 0 {
		v := contentID
		attrs.ContentID = &v
	}
	if productID > 0 {
		v := productID
		attrs.ProductID = &v
	}
	attrs.AIAccountID = &row.ID
	if kind == store.MediaKindGenImage {
		thumb, w, h, _ := makeThumb(s.uploadDir, rel, sha)
		attrs.ThumbPath = thumb
		if attrs.Width == 0 {
			attrs.Width, attrs.Height = w, h
		}
	}
	return s.store.CreateMediaFull(attrs)
}

// head512 caps sniff input at the detector's window.
func head512(raw []byte) []byte {
	if len(raw) > 512 {
		return raw[:512]
	}
	return raw
}

func (s *Service) runGen(account, platform, kind, prompt, ratio, outDir string) (genFile, error) {
	if s.genRunner != nil {
		return s.genRunner(account, platform, kind, prompt, ratio, outDir)
	}
	if s.python == "" {
		return genFile{}, ErrGenNoPython
	}
	ctx, cancel := context.WithTimeout(context.Background(), genTimeoutSec*time.Second)
	defer cancel()
	args := []string{"-m", "adapter.genmedia", account, platform,
		"--kind", kind, "--ratio", ratio, "--out", outDir, prompt}
	cmd := exec.CommandContext(ctx, s.python, args...)
	root := s.python
	for i := 0; i < 4; i++ {
		root = filepath.Dir(root)
	}
	cmd.Dir = root
	profiles, _ := filepath.Abs(filepath.Join(s.dataDir, "profiles"))
	cmd.Env = append(os.Environ(), "PROFILES_DIR="+profiles, "PYTHONUTF8=1")
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if ctx.Err() == context.DeadlineExceeded {
		return genFile{}, ErrGenFailed
	}
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 2 {
			return genFile{}, fmt.Errorf("%w: %s", ErrGenUnavailable, output)
		}
		if output != "" {
			return genFile{}, fmt.Errorf("%w: %s", ErrGenFailed, output)
		}
		return genFile{}, ErrGenFailed
	}
	var decoded struct {
		File        string  `json:"file"`
		Width       int     `json:"width"`
		Height      int     `json:"height"`
		DurationSec float64 `json:"duration_sec"`
	}
	lines := strings.Split(output, "\n")
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &decoded); err != nil {
		return genFile{}, fmt.Errorf("%w: bad adapter output", ErrGenFailed)
	}
	if decoded.File == "" {
		return genFile{}, fmt.Errorf("%w: empty file", ErrGenFailed)
	}
	if !filepath.IsAbs(decoded.File) {
		decoded.File = filepath.Join(outDir, decoded.File)
	}
	if rel, err := filepath.Rel(outDir, decoded.File); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return genFile{}, fmt.Errorf("%w: bad adapter output", ErrGenFailed)
	}
	return genFile{Path: decoded.File, Width: decoded.Width, Height: decoded.Height, DurationSec: decoded.DurationSec}, nil
}
