package session

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// SafeAccountName must match adapter.browser.safe_name exactly.
func TestSafeAccountName(t *testing.T) {
	cases := map[string]string{
		"insta-1":  "insta-1",
		"../../x":  "x",
		"!!!":      "account",
		"":         "account",
		"a b":      "a_b",
		"a..b":     "a..b",
		"._x_.":    "x",
		"pillot-1": "pillot-1",
	}
	for in, want := range cases {
		if got := SafeAccountName(in); got != want {
			t.Errorf("SafeAccountName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSessionStatePath(t *testing.T) {
	got := SessionStatePath("/data", "../x")
	want := filepath.Join("/data", "profiles", "x", "storage_state.json")
	if got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestValidSessionFile(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.json")
	if err := os.WriteFile(good, []byte(`{"cookies":[],"origins":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if !validSessionFile(good) {
		t.Fatal("valid storage_state rejected")
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte(`{"cookies":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if validSessionFile(bad) {
		t.Fatal("origins-less file accepted")
	}
	garbage := filepath.Join(dir, "garbage.json")
	if err := os.WriteFile(garbage, []byte(`not json`), 0o600); err != nil {
		t.Fatal(err)
	}
	if validSessionFile(garbage) {
		t.Fatal("garbage accepted")
	}
	if validSessionFile(filepath.Join(dir, "missing.json")) {
		t.Fatal("missing file accepted")
	}
}

// stubSpawnLong returns a live process the test kills via CancelLogin.
func stubSpawnLong(python, name, platform, confirmFile string, env []string, log *os.File, realProfile, incognito bool) (*exec.Cmd, error) {
	cmd := exec.Command("ping", "-n", "60", "127.0.0.1")
	cmd.Stdout = nil
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

// stubSpawnDead returns a process that is already gone.
func stubSpawnDead(python, name, platform, confirmFile string, env []string, log *os.File, realProfile, incognito bool) (*exec.Cmd, error) {
	cmd := exec.Command("cmd", "/c", "exit 0")
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func loginTestService(t *testing.T) (*Service, string) {
	t.Helper()
	svc := newTestService(t)
	dataDir := t.TempDir()
	fakePy := filepath.Join(t.TempDir(), "python.exe")
	if err := os.WriteFile(fakePy, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc.ConfigureLogin(dataDir, fakePy)
	return svc, dataDir
}

func mustRegister(t *testing.T, svc *Service, name, platform string) int64 {
	t.Helper()
	a, err := svc.RegisterAccount(name, platform, "me", "worker", "")
	if err != nil {
		t.Fatal(err)
	}
	return a.ID
}

func TestReadLogTail(t *testing.T) {
	if got := readLogTail(""); got != "" {
		t.Fatalf("empty path = %q", got)
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "x.log")
	if err := os.WriteFile(p, []byte("first\nsecond\n\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := readLogTail(p); got != "second" {
		t.Fatalf("tail = %q", got)
	}
	if got := readLogTail(filepath.Join(dir, "missing.log")); got != "" {
		t.Fatalf("missing = %q", got)
	}
}

func TestStartLoginValidation(t *testing.T) {
	svc, _ := loginTestService(t)
	if _, err := svc.StartLogin(9999, false, false); !errors.Is(err, ErrNoAccount) {
		t.Fatalf("unknown account: %v", err)
	}
	empty := mustRegister(t, svc, "noplat", "")
	if _, err := svc.StartLogin(empty, false, false); !errors.Is(err, ErrBadPlatform) {
		t.Fatalf("empty platform: %v", err)
	}
	id := mustRegister(t, svc, "nopy", "meta_ai")
	svc.ConfigureLogin(t.TempDir(), filepath.Join(t.TempDir(), "missing.exe"))
	if _, err := svc.StartLogin(id, false, false); !errors.Is(err, ErrNoAdapter) {
		t.Fatalf("missing venv: %v", err)
	}
}

func TestStartLoginBusyAndCancel(t *testing.T) {
	svc, _ := loginTestService(t)
	svc.logins.spawn = stubSpawnLong
	id := mustRegister(t, svc, "busy", "meta_ai")
	pid, err := svc.StartLogin(id, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if pid <= 0 {
		t.Fatalf("pid = %d", pid)
	}
	if _, err := svc.StartLogin(id, false, false); !errors.Is(err, ErrLoginBusy) {
		t.Fatalf("double start: %v", err)
	}
	svc.CancelLogin(id)
	svc.CancelLogin(id) // idempotent
	svc.logins.mu.Lock()
	_, tracked := svc.logins.procs[id]
	svc.logins.mu.Unlock()
	if tracked {
		t.Fatal("cancelled login still tracked")
	}
}

func TestFinishLoginSuccess(t *testing.T) {
	svc, dataDir := loginTestService(t)
	svc.logins.spawn = stubSpawnLong
	svc.verifyRunner = func(name, platform string) (string, string) {
		if name != "meta-1" || platform != "meta_ai" {
			t.Errorf("verify(%q, %q)", name, platform)
		}
		return verifyOK, "logged in"
	}
	id := mustRegister(t, svc, "meta-1", "meta_ai")
	if _, err := svc.StartLogin(id, false, false); err != nil {
		t.Fatal(err)
	}
	// The headed adapter would save this after the operator confirms.
	statePath := SessionStatePath(dataDir, "meta-1")
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, []byte(`{"cookies":[],"origins":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	a, err := svc.FinishLogin(id)
	if err != nil {
		t.Fatal(err)
	}
	if a.SessionState != StateConnected {
		t.Fatalf("state = %q", a.SessionState)
	}
	svc.logins.mu.Lock()
	_, tracked := svc.logins.procs[id]
	svc.logins.mu.Unlock()
	if tracked {
		t.Fatal("finished login still tracked")
	}
}

func TestFinishLoginFailures(t *testing.T) {
	svc, _ := loginTestService(t)
	if _, err := svc.FinishLogin(9999); !errors.Is(err, ErrNoAccount) {
		t.Fatalf("unknown account: %v", err)
	}
	id := mustRegister(t, svc, "noshow", "meta_ai")
	if _, err := svc.FinishLogin(id); !errors.Is(err, ErrNoLogin) {
		t.Fatalf("no login: %v", err)
	}
	// Browser dying instantly fails fast at start with its log tail.
	svc.logins.spawn = func(python, name, platform, confirmFile string, env []string, log *os.File, realProfile, incognito bool) (*exec.Cmd, error) {
		if _, err := log.WriteString("Chrome is busy\n"); err != nil {
			t.Fatal(err)
		}
		return stubSpawnDead(python, name, platform, confirmFile, env, log, realProfile, incognito)
	}
	dead := mustRegister(t, svc, "dead", "meta_ai")
	_, err := svc.StartLogin(dead, false, false)
	if err == nil || !strings.Contains(err.Error(), "Chrome is busy") {
		t.Fatalf("fast fail: %v", err)
	}
	svc.logins.mu.Lock()
	_, tracked := svc.logins.procs[dead]
	svc.logins.mu.Unlock()
	if tracked {
		t.Fatal("failed start stayed tracked")
	}
	// Exit code 3 (locked real profile) maps to ErrProfileBusy.
	svc.logins.spawn = func(python, name, platform, confirmFile string, env []string, log *os.File, realProfile, incognito bool) (*exec.Cmd, error) {
		cmd := exec.Command("cmd", "/c", "exit 3")
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		return cmd, nil
	}
	busy := mustRegister(t, svc, "busyprof", "meta_ai")
	if _, err := svc.StartLogin(busy, true, false); !errors.Is(err, ErrProfileBusy) {
		t.Fatalf("busy profile: %v", err)
	}
	// Browser dying after tracking fails at finish instead.
	svc.logins.spawn = stubSpawnLong
	late := mustRegister(t, svc, "late", "meta_ai")
	if _, err := svc.StartLogin(late, false, false); err != nil {
		t.Fatal(err)
	}
	svc.logins.mu.Lock()
	p := svc.logins.procs[late]
	svc.logins.mu.Unlock()
	if p == nil || p.cmd.Process == nil {
		t.Fatal("no tracked proc")
	}
	_ = p.cmd.Process.Kill()
	select {
	case <-p.done:
	case <-time.After(10 * time.Second):
		t.Fatal("killed proc never reaped")
	}
	if _, err := svc.FinishLogin(late); !errors.Is(err, ErrLoginFailed) {
		t.Fatalf("dead browser: %v", err)
	}
}

// A structurally valid but logged-out session is rejected and quarantined
// aside (not deleted) so a false-negative verify never destroys evidence.
func TestFinishLoginLoggedOut(t *testing.T) {
	svc, dataDir := loginTestService(t)
	svc.logins.spawn = stubSpawnLong
	svc.verifyRunner = func(name, platform string) (string, string) {
		return verifyLoggedOut, `still logged out (saw 'Log in')`
	}
	id := mustRegister(t, svc, "anon-1", "meta_ai")
	if _, err := svc.StartLogin(id, false, false); err != nil {
		t.Fatal(err)
	}
	statePath := SessionStatePath(dataDir, "anon-1")
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, []byte(`{"cookies":[],"origins":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.FinishLogin(id); !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("logged-out session: %v", err)
	}
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Fatal("bogus session file kept at live path")
	}
	if _, err := os.Stat(statePath + ".invalid"); err != nil {
		t.Fatalf("bogus session not quarantined: %v", err)
	}
	a, err := svc.GetAccount(id)
	if err != nil {
		t.Fatal(err)
	}
	if a.SessionState == StateConnected {
		t.Fatal("logged-out session marked connected")
	}
}

// Incognito and real-profile flags reach the adapter spawn.
func TestStartLoginIncognito(t *testing.T) {
	svc, _ := loginTestService(t)
	var gotReal, gotIncog bool
	svc.logins.spawn = func(python, name, platform, confirmFile string, env []string, log *os.File, realProfile, incognito bool) (*exec.Cmd, error) {
		gotReal, gotIncog = realProfile, incognito
		return stubSpawnLong(python, name, platform, confirmFile, env, log, realProfile, incognito)
	}
	id := mustRegister(t, svc, "incog-1", "meta_ai")
	if _, err := svc.StartLogin(id, false, true); err != nil {
		t.Fatal(err)
	}
	svc.CancelLogin(id)
	if !gotIncog || gotReal {
		t.Fatalf("spawn flags real=%v incognito=%v", gotReal, gotIncog)
	}
}

// loginArgs maps incognito to --incognito (wins over --real-profile).
func TestLoginArgs(t *testing.T) {
	has := func(list []string, want string) bool {
		for _, s := range list {
			if s == want {
				return true
			}
		}
		return false
	}
	incog := loginArgs("n", "p", "c", false, true)
	if !has(incog, "--incognito") || has(incog, "--real-profile") {
		t.Fatalf("incognito args = %v", incog)
	}
	real := loginArgs("n", "p", "c", true, false)
	if !has(real, "--real-profile") || has(real, "--incognito") {
		t.Fatalf("real profile args = %v", real)
	}
	both := loginArgs("n", "p", "c", true, true)
	if has(both, "--real-profile") || !has(both, "--incognito") {
		t.Fatalf("both flags args = %v", both)
	}
	plain := loginArgs("n", "p", "c", false, false)
	if has(plain, "--incognito") || has(plain, "--real-profile") {
		t.Fatalf("plain args = %v", plain)
	}
}

// Terminal-first flow: no tracked browser, but a verified session file
// on disk still connects the account.
func TestFinishLoginUntracked(t *testing.T) {
	svc, dataDir := loginTestService(t)
	svc.verifyRunner = func(name, platform string) (string, string) {
		return verifyOK, "logged in"
	}
	id := mustRegister(t, svc, "term-1", "meta_ai")
	if _, err := svc.FinishLogin(id); !errors.Is(err, ErrNoLogin) {
		t.Fatalf("no session file: %v", err)
	}
	statePath := SessionStatePath(dataDir, "term-1")
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, []byte(`{"cookies":[],"origins":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	a, err := svc.FinishLogin(id)
	if err != nil {
		t.Fatal(err)
	}
	if a.SessionState != StateConnected {
		t.Fatalf("state = %q", a.SessionState)
	}
}
