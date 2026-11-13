package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"somun/internal/store"
	"strings"
	"sync"
	"time"
)

// Console-driven login errors.
var (
	ErrNoAdapter    = errors.New("adapter venv 없음: adapter/README.md 설치 절차를 먼저 수행하세요")
	ErrLoginBusy    = errors.New("이미 진행 중인 로그인이 있습니다")
	ErrNoLogin      = errors.New("진행 중인 로그인이 없습니다")
	ErrLoginFailed  = errors.New("브라우저가 닫혔거나 로그인이 완료되지 않았습니다")
	ErrProfileBusy  = errors.New("Chrome이 실행 중입니다. 완전히 종료 후 다시 시도하세요")
	ErrNotLoggedIn  = errors.New("브라우저에서 로그인이 끝나지 않았습니다. 로그인 후 다시 완료 확인을 누르세요")
	ErrUnverifiable = errors.New("로그인 확인 불가")
	ErrBadPlatform  = errors.New("로그인할 플랫폼이 지정되지 않았습니다")
)

// SafeAccountName mirrors adapter.browser.safe_name: runs of anything outside
// [A-Za-z0-9._-] collapse to "_", leading/trailing dots collapse away.
func SafeAccountName(name string) string {
	var b strings.Builder
	underscore := false
	for _, r := range name {
		ok := r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' ||
			r == '.' || r == '_' || r == '-'
		if ok {
			b.WriteRune(r)
			underscore = false
			continue
		}
		if !underscore {
			b.WriteByte('_')
			underscore = true
		}
	}
	out := strings.Trim(b.String(), "._")
	if out == "" {
		return "account"
	}
	return out
}

// SessionStatePath is where adapter.login persists the account session.
func SessionStatePath(dataDir, name string) string {
	return filepath.Join(dataDir, "profiles", SafeAccountName(name), "storage_state.json")
}

type loginProc struct {
	cmd         *exec.Cmd
	confirmFile string
	logPath     string
	done        chan struct{}
	exitCode    int
}

// adapterBusyExit is adapter.login's exit code for a locked real profile.
const adapterBusyExit = 3

func waitLogin(cmd *exec.Cmd, p *loginProc) {
	err := cmd.Wait()
	if exit, ok := err.(*exec.ExitError); ok {
		p.exitCode = exit.ExitCode()
	} else if err == nil {
		p.exitCode = 0
	}
	close(p.done)
}

// LoginManager tracks one headed adapter.login per account. The console opens
// the browser (StartLogin), the operator logs in on the real screen, and the
// console confirm button ends the wait (FinishLogin).
type LoginManager struct {
	mu    sync.Mutex
	procs map[int64]*loginProc
	spawn func(python, name, platform, confirmFile string, env []string, log *os.File, realProfile bool) (*exec.Cmd, error)
}

// repoRootFromPython derives the repo root from the venv python path
// (<root>/adapter/.venv/Scripts/python.exe).
func repoRootFromPython(python string) string {
	root := python
	for i := 0; i < 4; i++ {
		root = filepath.Dir(root)
	}
	return root
}

func defaultSpawn(python, name, platform, confirmFile string, env []string, log *os.File, realProfile bool) (*exec.Cmd, error) {
	args := []string{"-m", "adapter.login", name, platform, "--confirm-file", confirmFile}
	if realProfile {
		args = append(args, "--real-profile")
	}
	cmd := exec.Command(python, args...)
	// adapter.login is a repo-root module: run it from the repo root.
	cmd.Dir = repoRootFromPython(python)
	env = append(env, "PYTHONUTF8=1")
	cmd.Env = append(os.Environ(), env...)
	// Headed-launch failures must be diagnosable instead of silent.
	cmd.Stdout = log
	cmd.Stderr = log
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

// ConfigureLogin sets the data dir (for session verification) and the
// adapter venv python. It must be called before the login endpoints serve.
func (s *Service) ConfigureLogin(dataDir, pythonPath string) {
	if abs, err := filepath.Abs(dataDir); err == nil {
		dataDir = abs
	}
	s.loginDataDir = dataDir
	s.loginPython = pythonPath
	if s.logins == nil {
		s.logins = &LoginManager{procs: map[int64]*loginProc{}, spawn: defaultSpawn}
	}
}

// startFailWait bounds the fast-fail probe: a child that dies this quickly
// never showed a browser, so report its log tail instead of tracking it.
const startFailWait = 8 * time.Second

// StartLogin opens the headed login browser for one account. With
// realProfile the operator's own Chrome profile is used (Chrome must be
// fully closed or the start fails fast with the reason).
func (s *Service) StartLogin(id int64, realProfile bool) (int, error) {
	row, err := s.store.GetAccountRow(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return 0, ErrNoAccount
		}
		return 0, err
	}
	platform := strings.TrimSpace(row.Platform)
	if platform == "" {
		return 0, ErrBadPlatform
	}
	if s.logins == nil {
		s.ConfigureLogin(".", "")
	}
	if s.loginPython == "" {
		return 0, ErrNoAdapter
	}
	if _, err := os.Stat(s.loginPython); err != nil {
		return 0, ErrNoAdapter
	}
	s.logins.mu.Lock()
	defer s.logins.mu.Unlock()
	if p, ok := s.logins.procs[id]; ok {
		select {
		case <-p.done:
			delete(s.logins.procs, id)
		default:
			return 0, ErrLoginBusy
		}
	}
	confirm, err := loginConfirmFile()
	if err != nil {
		return 0, err
	}
	logFile, err := os.CreateTemp("", "somun-login-*.log")
	if err != nil {
		os.Remove(confirm)
		return 0, err
	}
	logPath := logFile.Name()
	profiles, _ := filepath.Abs(filepath.Join(s.loginDataDir, "profiles"))
	evidence, _ := filepath.Abs(filepath.Join(s.loginDataDir, "uploads", "evidence"))
	cmd, err := s.logins.spawn(s.loginPython, row.Name, platform, confirm,
		[]string{"PROFILES_DIR=" + profiles, "EVIDENCE_DIR=" + evidence}, logFile, realProfile)
	logFile.Close() // the child keeps its own descriptor
	if err != nil {
		os.Remove(confirm)
		os.Remove(logPath)
		return 0, fmt.Errorf("login browser start: %w", err)
	}
	p := &loginProc{cmd: cmd, confirmFile: confirm, logPath: logPath, done: make(chan struct{}), exitCode: -1}
	go waitLogin(cmd, p)
	select {
	case <-p.done:
		// Died before any browser could matter: report why, track nothing.
		tail := readLogTail(p.logPath)
		os.Remove(confirm)
		os.Remove(logPath)
		if p.exitCode == adapterBusyExit {
			if tail != "" && !strings.Contains(tail, "Chrome이 실행 중") {
				return 0, fmt.Errorf("%w: %s", ErrProfileBusy, tail)
			}
			return 0, ErrProfileBusy
		}
		if tail != "" {
			return 0, fmt.Errorf("login browser failed: %s", tail)
		}
		return 0, ErrLoginFailed
	case <-time.After(startFailWait):
	}
	s.logins.procs[id] = p
	pid := 0
	if cmd.Process != nil {
		pid = cmd.Process.Pid
	}
	return pid, nil
}

// Verify verdicts from adapter.verify.
const (
	verifyOK        = "ok"
	verifyLoggedOut = "loggedout"
	verifyUnknown   = "unknown"
)

// FinishLogin confirms the headed login, verifies the saved session is
// really logged in, and marks the account connected. Without a tracked
// browser (terminal-first flow), it verifies the existing session file.
func (s *Service) FinishLogin(id int64) (*Account, error) {
	row, err := s.store.GetAccountRow(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNoAccount
		}
		return nil, err
	}
	var p *loginProc
	if s.logins != nil {
		s.logins.mu.Lock()
		p = s.logins.procs[id]
		s.logins.mu.Unlock()
	}
	statePath := SessionStatePath(s.loginDataDir, row.Name)
	if p == nil {
		if !validSessionFile(statePath) {
			return nil, ErrNoLogin
		}
		return s.concludeLogin(id, row, nil)
	}
	// Ends the adapter wait; the adapter then saves the session and exits.
	_ = os.WriteFile(p.confirmFile, []byte("confirmed\n"), 0o600)
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		if validSessionFile(statePath) {
			return s.concludeLogin(id, row, p)
		}
		select {
		case <-p.done:
			if validSessionFile(statePath) {
				return s.concludeLogin(id, row, p)
			}
			tail := readLogTail(p.logPath)
			s.stopLogin(id, p)
			if tail != "" {
				return nil, fmt.Errorf("%w: %s", ErrLoginFailed, tail)
			}
			return nil, ErrLoginFailed
		default:
		}
		time.Sleep(500 * time.Millisecond)
	}
	if validSessionFile(statePath) {
		return s.concludeLogin(id, row, p)
	}
	s.stopLogin(id, p)
	return nil, fmt.Errorf("세션 파일이 저장되지 않았습니다: %s", statePath)
}

// concludeLogin runs the login-state check on a structurally valid session.
// A file that fails the check is deleted so retries start clean.
func (s *Service) concludeLogin(id int64, row *store.ChannelAccount, p *loginProc) (*Account, error) {
	verdict, message := s.runVerify(row.Name, strings.TrimSpace(row.Platform))
	if p != nil {
		s.stopLogin(id, p)
	}
	statePath := SessionStatePath(s.loginDataDir, row.Name)
	switch verdict {
	case verifyOK:
		return s.SetSessionState(id, StateConnected, "")
	case verifyLoggedOut:
		os.Remove(statePath)
		if message != "" {
			return nil, fmt.Errorf("%w (%s)", ErrNotLoggedIn, message)
		}
		return nil, ErrNotLoggedIn
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnverifiable, message)
	}
}

// runVerify executes adapter.verify (headless) with a timeout. Tests replace
// verifyRunner with a stub.
func (s *Service) runVerify(name, platform string) (string, string) {
	if s.verifyRunner != nil {
		return s.verifyRunner(name, platform)
	}
	if s.loginPython == "" {
		return verifyUnknown, ErrNoAdapter.Error()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.loginPython, "-m", "adapter.verify", name, platform)
	cmd.Dir = repoRootFromPython(s.loginPython)
	profiles, _ := filepath.Abs(filepath.Join(s.loginDataDir, "profiles"))
	cmd.Env = append(os.Environ(), "PROFILES_DIR="+profiles, "PYTHONUTF8=1")
	out, err := cmd.CombinedOutput()
	message := strings.TrimSpace(string(out))
	if err == nil {
		return verifyOK, message
	}
	if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
		return verifyLoggedOut, message
	}
	if ctx.Err() == context.DeadlineExceeded {
		return verifyUnknown, "verify timed out"
	}
	if message == "" {
		message = err.Error()
	}
	return verifyUnknown, message
}

// CancelLogin kills an in-flight headed login. It is idempotent.
func (s *Service) CancelLogin(id int64) {
	if s.logins == nil {
		return
	}
	s.logins.mu.Lock()
	p, ok := s.logins.procs[id]
	s.logins.mu.Unlock()
	if !ok {
		return
	}
	s.stopLogin(id, p)
}

func (s *Service) stopLogin(id int64, p *loginProc) {
	if p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	os.Remove(p.confirmFile)
	os.Remove(p.logPath)
	if s.logins != nil {
		s.logins.mu.Lock()
		if cur, ok := s.logins.procs[id]; ok && cur == p {
			delete(s.logins.procs, id)
		}
		s.logins.mu.Unlock()
	}
}

// loginConfirmFile reserves a unique handshake path that must NOT exist yet:
// login.py ends its wait the moment the file appears.
func loginConfirmFile() (string, error) {
	f, err := os.CreateTemp("", "somun-login-*.confirm")
	if err != nil {
		return "", err
	}
	path := f.Name()
	f.Close()
	os.Remove(path)
	return path, nil
}

// readLogTail returns the last line of a login child log for error messages.
func readLogTail(path string) string {
	if path == "" {
		return ""
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			if len(line) > 300 {
				line = line[:300] + "…"
			}
			return line
		}
	}
	return ""
}

// validSessionFile reports a plausible Playwright storage_state.
func validSessionFile(path string) bool {
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 {
		return false
	}
	var state struct {
		Cookies []any `json:"cookies"`
		Origins []any `json:"origins"`
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return false
	}
	return state.Cookies != nil && state.Origins != nil
}
