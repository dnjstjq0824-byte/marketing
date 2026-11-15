// Package chat sends console/worker messages through saved AI sessions and
// records the turns. Delivery runs adapter.chat (headless) per message.
package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"somun/internal/session"
	"somun/internal/store"
)

// Errors for chat failures.
var (
	ErrNoAccount      = errors.New("account not found")
	ErrNotConnected   = errors.New("연결되지 않은 계정입니다")
	ErrEmptyMessage   = errors.New("message is required")
	ErrMessageTooBig  = errors.New("message exceeds 4000 characters")
	ErrChatFailed     = errors.New("AI가 답하지 않았습니다")
	ErrChatNoSession = errors.New("저장된 세션이 없습니다. 다시 로그인하세요")
	ErrUnavailable   = errors.New("이 플랫폼의 실채팅은 아직 준비 중입니다")
	ErrBadThread     = errors.New("invalid thread")
)

// MaxMessageRunes bounds one chat message.
const MaxMessageRunes = 4000

// bgTimeoutSec bounds one background adapter run.
const bgTimeoutSec = 300

// graceSec is how long Send waits before answering 202; the run continues
// in the background and the caller polls Pending. Must stay comfortably
// under the HTTP WriteTimeout (60s default).
const graceSec = 45

// pendingTTL bounds how long finished runs stay pollable.
const pendingTTL = 10 * time.Minute

// pendingRun tracks one background delivery.
type pendingRun struct {
	doneAt time.Time
	done   chan struct{}
	reply  string
	err    error
}

// Service sends chats and keeps history.
type Service struct {
	store   *store.Store
	dataDir string
	python  string
	// sessionSvc marks accounts expired when their session file is gone,
	// and decrypts API keys for the key-based path.
	sessionSvc *session.Service
	// sendRunner replaces the adapter.chat subprocess in tests.
	// It returns the reply or a typed error.
	sendRunner func(account, platform, message, thread string, files []string) (string, error)

	mu      sync.Mutex
	pending map[int64]*pendingRun
	grace   time.Duration
	// Per-thread send locks (W13-5): overlapping sends to one thread
	// tangle replies; the second waiter goes after the first finishes.
	lockMu sync.Mutex
	locks  map[string]*sync.Mutex
}

func (s *Service) lockFor(id int64, thread string) *sync.Mutex {
	key := threadLockKey(id, thread)
	s.lockMu.Lock()
	defer s.lockMu.Unlock()
	if s.locks == nil {
		s.locks = map[string]*sync.Mutex{}
	}
	m, ok := s.locks[key]
	if !ok {
		m = &sync.Mutex{}
		s.locks[key] = m
	}
	return m
}

func threadLockKey(id int64, thread string) string {
	return fmt.Sprintf("%d\x00%s", id, thread)
}

// sanitizeThreadKey keeps adapter file names safe. Empty means default.
func sanitizeThreadKey(thread string) (string, error) {
	if thread == "" {
		return "", nil
	}
	if len(thread) > 64 {
		return "", ErrBadThread
	}
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			return r
		}
		return -1
	}, thread)
	if safe == "" || safe != thread {
		return "", ErrBadThread
	}
	return safe, nil
}

// NewService builds the chat service.
func NewService(st *store.Store) *Service {
	return &Service{store: st, pending: map[int64]*pendingRun{}, grace: graceSec * time.Second}
}

// SendResult is either an immediate reply or a pending handle.
type SendResult struct {
	Reply       string
	Pending     bool
	UserMessage int64
}

// Send records the user turn and delivers it. Fast runs return the reply;
// slow runs continue in the background and return Pending (poll Pending).
// Empty thread means the account default thread; workers pass per-product keys.
// Attachments are server-side file paths the AI should see (W2-2). Every
// path must resolve inside the data dir; anything else is rejected before
// the adapter ever runs.
func (s *Service) Send(accountID int64, message, thread string, attachments ...string) (SendResult, error) {
	row, err := s.store.GetAccountRow(accountID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return SendResult{}, ErrNoAccount
		}
		return SendResult{}, err
	}
	if row.SessionState != session.StateConnected {
		return SendResult{}, ErrNotConnected
	}
	files, err := s.checkAttachments(attachments)
	if err != nil {
		return SendResult{}, err
	}
	// W15-5: automatic work and console chat run only on chat-linked AI.
	// Channel accounts (no registry row) are unaffected.
	if p, err := s.store.GetAIPlatform(strings.TrimSpace(row.Platform)); err == nil {
		if p.ChatStatus != store.AIChatOK {
			return SendResult{}, fmt.Errorf("%w: 대화 미연결 — 연결 화면에서 대화 연결 테스트를 먼저 하세요", ErrUnavailable)
		}
	}
	text := strings.TrimSpace(message)
	if text == "" {
		return SendResult{}, ErrEmptyMessage
	}
	if len([]rune(text)) > MaxMessageRunes {
		return SendResult{}, ErrMessageTooBig
	}
	thread, err = sanitizeThreadKey(strings.TrimSpace(thread))
	if err != nil {
		return SendResult{}, err
	}
	userMsg, err := s.store.AppendChatMessage(accountID, "user", text)
	if err != nil {
		return SendResult{}, err
	}
	run := &pendingRun{done: make(chan struct{})}
	s.mu.Lock()
	s.pending[userMsg.ID] = run
	s.mu.Unlock()
	go s.deliver(accountID, userMsg.ID, row.Name, strings.TrimSpace(row.Platform), text, thread, files, run)
	select {
	case <-run.done:
		return s.collect(userMsg.ID, run)
	case <-time.After(s.gracePeriod()):
		return SendResult{Pending: true, UserMessage: userMsg.ID}, nil
	}
}

func (s *Service) gracePeriod() time.Duration {
	if s.grace > 0 {
		return s.grace
	}
	return graceSec * time.Second
}

func (s *Service) deliver(accountID, userMsgID int64, name, platform, text, thread string, files []string, run *pendingRun) {
	defer close(run.done)
	mu := s.lockFor(accountID, thread)
	mu.Lock()
	defer mu.Unlock()
	reply, err := s.runAdapter(name, platform, text, thread, files)
	if err == nil {
		_, err = s.store.AppendChatMessage(accountID, "assistant", reply)
	}
	if err == nil {
		// Mirror write is best-effort: observability must not fail a chat.
		_ = s.store.UpsertChatThread(accountID, thread)
	}
	run.reply, run.err = reply, err
	run.doneAt = time.Now()
}

// collect resolves a finished run: reply or typed error.
func (s *Service) collect(userMsgID int64, run *pendingRun) (SendResult, error) {
	s.mu.Lock()
	delete(s.pending, userMsgID)
	s.mu.Unlock()
	if run.err != nil {
		return SendResult{}, run.err
	}
	return SendResult{Reply: run.reply}, nil
}

// Pending reports one background run: reply when done, failed when the run
// errored or is unknown (e.g. server restarted mid-run).
func (s *Service) Pending(userMsgID int64) (reply string, done, failed bool, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for id, run := range s.pending {
		select {
		case <-run.done:
			if now.Sub(run.doneAt) > pendingTTL {
				delete(s.pending, id)
			}
		default:
		}
	}
	run, ok := s.pending[userMsgID]
	if !ok {
		return "", true, true, "delivery lost (server restarted?)"
	}
	select {
	case <-run.done:
		if run.err != nil {
			return "", true, true, run.err.Error()
		}
		return run.reply, true, false, ""
	default:
		return "", false, false, ""
	}
}

// Configure sets the data dir and adapter venv python.
func (s *Service) Configure(dataDir, pythonPath string) {
	if abs, err := filepath.Abs(dataDir); err == nil {
		dataDir = abs
	}
	s.dataDir = dataDir
	s.python = pythonPath
}

// AttachSession enables session-loss marking on missing session files.
func (s *Service) AttachSession(svc *session.Service) { s.sessionSvc = svc }

// History returns recorded turns, oldest first.
// ResetThread deletes one account's saved thread URL(s). Empty thread drops
// the default thread; "*" drops all named threads of the account.
func (s *Service) ResetThread(accountID int64, thread string) error {
	row, err := s.store.GetAccountRow(accountID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrNoAccount
		}
		return err
	}
	dir := filepath.Join(s.dataDir, "profiles", row.Name)
	names := []string{"thread.json"}
	if thread == "*" {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil
		}
		names = names[:0]
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "thread") && strings.HasSuffix(e.Name(), ".json") {
				names = append(names, e.Name())
			}
		}
	} else if thread != "" {
		safe, err := sanitizeThreadKey(thread)
		if err != nil {
			return err
		}
		names = []string{"thread_" + safe + ".json"}
	}
	for _, name := range names {
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	_ = s.store.DeleteChatThreads(accountID, thread)
	return nil
}

func (s *Service) History(accountID int64, limit int) ([]*store.ChatMessage, error) {
	if _, err := s.store.GetAccountRow(accountID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNoAccount
		}
		return nil, err
	}
	return s.store.ListChatHistory(accountID, limit)
}

// ErrBadAttachment marks paths outside the data dir or unreadable files.
var ErrBadAttachment = errors.New("attachment must be a readable file inside the data dir")

// checkAttachments resolves every path against the data dir. Symlink and
// traversal escapes are rejected; the adapter receives absolute paths.
func (s *Service) checkAttachments(paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	root, err := filepath.Abs(s.dataDir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, p := range paths {
		abs := p
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(root, abs)
		}
		abs, err = filepath.Abs(abs)
		if err != nil {
			return nil, ErrBadAttachment
		}
		if rel, err := filepath.Rel(root, abs); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return nil, ErrBadAttachment
		}
		fi, err := os.Stat(abs)
		if err != nil || fi.IsDir() {
			return nil, ErrBadAttachment
		}
		out = append(out, abs)
	}
	return out, nil
}

func (s *Service) runAdapter(name, platform, message, thread string, files []string) (string, error) {
	if s.sendRunner != nil {
		return s.sendRunner(name, platform, message, thread, files)
	}
	if s.python == "" {
		return "", fmt.Errorf("%w: adapter venv 없음", ErrUnavailable)
	}
	ctx, cancel := context.WithTimeout(context.Background(), bgTimeoutSec*time.Second)
	defer cancel()
	args := []string{"-m", "adapter.chat", name, platform}
	if thread != "" {
		args = append(args, "--thread", thread)
	}
	for _, f := range files {
		args = append(args, "--attach", f)
	}
	args = append(args, message)
	cmd := exec.CommandContext(ctx, s.python, args...)
	root := s.python
	for i := 0; i < 4; i++ {
		root = filepath.Dir(root)
	}
	cmd.Dir = root
	profiles, _ := filepath.Abs(filepath.Join(s.dataDir, "profiles"))
	env := append(os.Environ(), "PROFILES_DIR="+profiles, "PYTHONUTF8=1")
	if jsonPath, jerr := s.store.AIPlatformJSON(platform); jerr == nil && jsonPath != "" {
		defer os.Remove(jsonPath)
		env = append(env, "AI_PLATFORM_JSON="+jsonPath)
	}
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))
	if ctx.Err() == context.DeadlineExceeded {
		return "", ErrChatFailed
	}
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			switch exit.ExitCode() {
			case 2:
				return "", s.mapExit2(name, output)
			}
		}
		if output != "" {
			return "", fmt.Errorf("%w: %s", ErrChatFailed, output)
		}
		return "", ErrChatFailed
	}
	var decoded struct {
		Reply string `json:"reply"`
	}
	lines := strings.Split(output, "\n")
	// The reply JSON is the last line; earlier lines are progress notes.
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &decoded); err != nil {
		return "", fmt.Errorf("%w: bad adapter output", ErrChatFailed)
	}
	if strings.TrimSpace(decoded.Reply) == "" {
		return "", fmt.Errorf("%w: empty reply", ErrChatFailed)
	}
	return decoded.Reply, nil
}

// markSessionLost flips a connected account to expired when its session file
// vanished, so the relogin flow picks it up.
// mapExit2 maps adapter exit-2 output to an error. The adapter prints the
// fixed NO_SESSION code when the saved session is gone (B18); anything else
// with exit 2 is an unavailable platform or a usage error.
func (s *Service) mapExit2(name, output string) error {
	if strings.Contains(output, "not pinned") {
		return ErrUnavailable
	}
	if strings.Contains(output, "NO_SESSION") {
		s.markSessionLost(name)
		return ErrChatNoSession
	}
	return fmt.Errorf("%w: %s", ErrUnavailable, output)
}

func (s *Service) markSessionLost(name string) {
	if s.sessionSvc == nil {
		return
	}
	row, err := s.store.GetAccountRowByName(name)
	if err != nil {
		return
	}
	_, _ = s.sessionSvc.SetSessionState(row.ID, session.StateExpired, "session file lost")
}
