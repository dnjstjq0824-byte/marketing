// Package media implements chunked uploads (S2/S4): init → parts →
// complete, with type caps, forgery rejection, and hash dedupe.
package media

import (
	"archive/zip"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"somun/internal/store"
)

const (
	// W14-1 initial caps: images 20MB, video 500MB, documents 20MB.
	MaxImageBytes = 20 << 20
	MaxVideoBytes = 500 << 20
	MaxDocBytes   = 20 << 20
	// ChunkBytes is the advisory client part size; the server accepts any
	// part size but requires strictly sequential part numbers.
	ChunkBytes = 5 << 20
)

var (
	ErrBadUpload  = errors.New("올릴 수 없는 파일입니다")
	ErrNoSession  = errors.New("업로드 세션이 없습니다")
	ErrOutOfOrder = errors.New("순서대로 올리세요")
	ErrIncomplete = errors.New("빠진 조각이 있습니다. 다시 시도하세요")
	ErrForgery    = errors.New("위조된 파일은 올리지 못했습니다")
)

var extMajor = map[string]string{
	".jpg": "image", ".jpeg": "image", ".png": "image", ".webp": "image", ".gif": "image", ".heic": "image",
	".mp4": "video", ".webm": "video", ".mov": "video",
	".pdf": "doc", ".txt": "doc", ".md": "doc", ".docx": "doc",
}

var majorCap = map[string]int64{
	"image": MaxImageBytes,
	"video": MaxVideoBytes,
	"doc":   MaxDocBytes,
}

// Service stages chunked uploads under uploadDir/tmp and finalizes them.
type Service struct {
	store     *store.Store
	uploadDir string
	// FileKey signs worker download URLs (W5-3). Empty refuses all.
	FileKey []byte
	// dataDir/python drive AI-session generation (W14-2/3, like chat).
	dataDir string
	python  string
	// genRunner replaces the adapter.genmedia subprocess in tests.
	genRunner genRunner
}

// NewService builds the media service.
func NewService(st *store.Store, uploadDir string) *Service {
	return &Service{store: st, uploadDir: uploadDir}
}

type sessionMeta struct {
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	Mime  string `json:"mime"`
	Major string `json:"major"`
	Ext   string `json:"ext"`
	Total int    `json:"total"`
}

// Init validates and opens one upload session.
func (s *Service) Init(name string, size int64, mime string) (id string, total int, err error) {
	ext := strings.ToLower(filepath.Ext(name))
	major, ok := extMajor[ext]
	if !ok {
		return "", 0, ErrBadUpload
	}
	if size <= 0 || size > majorCap[major] {
		return "", 0, ErrBadUpload
	}
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", 0, err
	}
	id = hex.EncodeToString(raw[:])
	total = int((size + ChunkBytes - 1) / ChunkBytes)
	dir := s.sessionDir(id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", 0, err
	}
	meta, _ := json.Marshal(sessionMeta{Name: name, Size: size, Mime: mime, Major: major, Ext: ext, Total: total})
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), meta, 0o600); err != nil {
		return "", 0, err
	}
	return id, total, nil
}

func (s *Service) sessionDir(id string) string {
	return filepath.Join(s.uploadDir, "tmp", id)
}

func (s *Service) loadMeta(id string) (*sessionMeta, error) {
	if strings.ContainsAny(id, `/\.`) || id == "" {
		return nil, ErrNoSession
	}
	raw, err := os.ReadFile(filepath.Join(s.sessionDir(id), "meta.json"))
	if err != nil {
		return nil, ErrNoSession
	}
	var m sessionMeta
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, ErrNoSession
	}
	return &m, nil
}

func (s *Service) doneParts(id string) int {
	done := 0
	for n := 1; n <= 200000; n++ {
		if _, err := os.Stat(filepath.Join(s.sessionDir(id), fmt.Sprintf("part.%d", n))); err != nil {
			break
		}
		done = n
	}
	return done
}

// PutPart appends part n (1-based, strictly sequential). Any part size up
// to one chunk plus slack is accepted; the total is verified at complete.
func (s *Service) PutPart(id string, n int, body io.Reader) (done, total int, err error) {
	m, err := s.loadMeta(id)
	if err != nil {
		return 0, 0, err
	}
	if n < 1 || n != s.doneParts(id)+1 {
		return 0, 0, ErrOutOfOrder
	}
	f, err := os.Create(filepath.Join(s.sessionDir(id), fmt.Sprintf("part.%d", n)))
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	if _, err := io.Copy(f, io.LimitReader(body, ChunkBytes+(1<<20))); err != nil {
		return 0, 0, err
	}
	return n, m.Total, nil
}

// Complete assembles the parts, verifies type and size, dedupes by hash,
// and records the media row.
func (s *Service) Complete(id string) (*store.Media, error) {
	return s.CompleteFull(id, "", "", 0)
}

// CompleteFull is Complete with W14 generation tracking: workers pass the
// made kind (card/gen_image/gen_video), the prompt behind it, and their AI
// account. Empty kind records a plain upload.
func (s *Service) CompleteFull(id, kind, prompt string, accountID int64) (*store.Media, error) {
	m, err := s.loadMeta(id)
	if err != nil {
		return nil, err
	}
	present := s.doneParts(id)
	if present == 0 {
		return nil, ErrIncomplete
	}
	final, err := os.CreateTemp(s.uploadDir, "asm-*")
	if err != nil {
		return nil, err
	}
	asmName := final.Name()
	head := []byte{}
	hash := sha256.New()
	var total int64
	for n := 1; n <= present; n++ {
		part, err := os.Open(filepath.Join(s.sessionDir(id), fmt.Sprintf("part.%d", n)))
		if err != nil {
			final.Close()
			os.Remove(asmName)
			return nil, ErrIncomplete
		}
		b, err := io.ReadAll(part)
		part.Close()
		if err != nil {
			final.Close()
			os.Remove(asmName)
			return nil, err
		}
		if len(head) < 512 {
			head = append(head, b...)
			if len(head) > 512 {
				head = head[:512]
			}
		}
		hash.Write(b)
		if _, err := final.Write(b); err != nil {
			final.Close()
			os.Remove(asmName)
			return nil, err
		}
		total += int64(len(b))
	}
	final.Close()
	if total != m.Size {
		os.Remove(asmName)
		return nil, ErrIncomplete
	}
	sniffed, err := sniffMime(m.Ext, head, asmName)
	if err != nil {
		os.Remove(asmName)
		return nil, err
	}
	sum := hex.EncodeToString(hash.Sum(nil))
	if dup, err := s.store.MediaByHash(sum); err == nil {
		os.Remove(asmName)
		os.RemoveAll(s.sessionDir(id))
		return dup, nil
	}
	name := filepath.Join("lib", sum[:12]+"-"+safeName(m.Name))
	dest := filepath.Join(s.uploadDir, name)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		os.Remove(asmName)
		return nil, err
	}
	if err := os.Rename(asmName, dest); err != nil {
		os.Remove(asmName)
		return nil, err
	}
	os.RemoveAll(s.sessionDir(id))
	rel := filepath.ToSlash(name)
	if kind == "" {
		kind = store.MediaKindUpload
	}
	source := store.MediaSourceOriginal
	if kind != store.MediaKindUpload {
		source = store.MediaSourceGenerated
	}
	attrs := store.MediaAttrs{
		Kind: kind, Path: rel, Bytes: total,
		Mime: sniffed, SHA256: sum, Prompt: prompt, Source: source,
	}
	if accountID > 0 {
		if _, err := s.store.GetAccountRow(accountID); err == nil {
			v := accountID
			attrs.AIAccountID = &v
		}
	}
	if strings.HasPrefix(sniffed, "image/") {
		thumb, w, h, _ := makeThumb(s.uploadDir, rel, sum)
		attrs.ThumbPath, attrs.Width, attrs.Height = thumb, w, h
	}
	return s.store.CreateMediaFull(attrs)
}

// MediaDetail is one library row with display joins: the business, the
// post title, the AI account name, and the W14-5 download name.
type MediaDetail struct {
	store.Media
	Business     string `json:"business"`
	Channel      string `json:"channel,omitempty"`
	ContentTitle string `json:"contentTitle,omitempty"`
	AccountName  string `json:"accountName,omitempty"`
	Download     string `json:"download"`
}

// List returns newest-first library rows.
func (s *Service) List(kind string, productID int64, business, from, to string, page, size int) ([]*store.Media, error) {
	if page < 1 {
		page = 1
	}
	return s.store.ListMediaQuery(kind, productID, business, from, to, size, (page-1)*size)
}

// Detail loads one row with its business.
func (s *Service) Detail(id int64) (*MediaDetail, error) {
	m, err := s.store.GetMedia(id)
	if err != nil {
		return nil, err
	}
	d := &MediaDetail{Media: *m}
	if m.ProductID != nil {
		if p, err := s.store.GetProduct(*m.ProductID); err == nil {
			d.Business = p.Business
		}
	}
	d.Channel = s.contentChannel(m)
	d.Download = DownloadName(m, d.Channel)
	if m.ContentID != nil {
		if c, err := s.store.GetContent(*m.ContentID); err == nil {
			d.ContentTitle = c.Title
		}
	}
	if m.AIAccountID != nil {
		if a, err := s.store.GetAccountRow(*m.AIAccountID); err == nil {
			d.AccountName = a.Name
		}
	}
	return d, nil
}

// Open returns one row with its file handle for serving.
func (s *Service) Open(id int64) (*store.Media, *os.File, error) {
	m, err := s.store.GetMedia(id)
	if err != nil {
		return nil, nil, err
	}
	f, err := os.Open(filepath.Join(s.uploadDir, m.Path))
	if err != nil {
		return nil, nil, store.ErrNotFound
	}
	return m, f, nil
}

// OpenThumb returns one row's thumbnail for serving. Rows without a stored
// thumbnail report ErrNotFound so callers can fall back to the original.
func (s *Service) OpenThumb(id int64) (*store.Media, *os.File, error) {
	m, err := s.store.GetMedia(id)
	if err != nil {
		return nil, nil, err
	}
	if m.ThumbPath == "" {
		return nil, nil, store.ErrNotFound
	}
	f, err := os.Open(filepath.Join(s.uploadDir, m.ThumbPath))
	if err != nil {
		return nil, nil, store.ErrNotFound
	}
	return m, f, nil
}

// OpenLegacy serves one legacy product image (pre-library uploads).
func (s *Service) OpenLegacy(id int64) (*store.ProductImage, *os.File, error) {
	im, err := s.store.GetProductImage(id)
	if err != nil {
		return nil, nil, err
	}
	f, err := os.Open(filepath.Join(s.uploadDir, im.Path))
	if err != nil {
		return nil, nil, store.ErrNotFound
	}
	return im, f, nil
}

// Zip streams originals into w under W14-5 download names.
func (s *Service) Zip(w io.Writer, ids []int64) error {
	zw := zip.NewWriter(w)
	defer zw.Close()
	for _, id := range ids {
		m, f, err := s.Open(id)
		if err != nil {
			return err
		}
		entry, err := zw.Create(DownloadName(m, s.contentChannel(m)))
		if err != nil {
			f.Close()
			return err
		}
		_, err = io.Copy(entry, f)
		f.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// Delete removes one unattached file (row + bytes + thumbnail).
func (s *Service) Delete(id int64) error {
	m, err := s.store.DeleteMedia(id)
	if err != nil {
		return err
	}
	os.Remove(filepath.Join(s.uploadDir, m.Path))
	if m.ThumbPath != "" {
		os.Remove(filepath.Join(s.uploadDir, m.ThumbPath))
	}
	return nil
}

func safeName(name string) string {
	base := filepath.Base(name)
	var b strings.Builder
	for _, r := range base {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
			r == '.' || r == '-' || r == '_' || (r >= 0xAC00 && r <= 0xD7A3) {
			b.WriteRune(r)
		}
	}
	out := strings.Trim(b.String(), ".")
	if out == "" {
		out = "file"
	}
	if len(out) > 80 {
		out = out[:80]
	}
	return out
}
