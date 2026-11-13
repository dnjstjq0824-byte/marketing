package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"somun/internal/store"

	"golang.org/x/crypto/bcrypt"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := store.Migrate(st.DB()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return st
}

func TestLoginLimiterBlocksSixthAttempt(t *testing.T) {
	l := NewLoginLimiter()
	for i := 0; i < LoginMaxFailures; i++ {
		if l.Blocked("admin", "10.0.0.1") {
			t.Fatalf("blocked before reaching the limit (failure %d)", i+1)
		}
		l.Failed("admin", "10.0.0.1")
	}
	if !l.Blocked("admin", "10.0.0.1") {
		t.Fatal("expected block after 5 failures")
	}
	if l.Blocked("admin", "10.0.0.2") {
		t.Fatal("block leaked to another IP")
	}
	if l.Blocked("other", "10.0.0.1") {
		t.Fatal("block leaked to another username")
	}
}

func TestLoginLimiterSuccessClears(t *testing.T) {
	l := NewLoginLimiter()
	for i := 0; i < LoginMaxFailures-1; i++ {
		l.Failed("admin", "10.0.0.1")
	}
	l.Succeeded("admin", "10.0.0.1")
	for i := 0; i < LoginMaxFailures; i++ {
		if l.Blocked("admin", "10.0.0.1") {
			t.Fatal("blocked after a success reset the count")
		}
		l.Failed("admin", "10.0.0.1")
	}
	if !l.Blocked("admin", "10.0.0.1") {
		t.Fatal("expected block after 5 fresh failures")
	}
}

func TestLoginLimiterBlockExpires(t *testing.T) {
	l := NewLoginLimiter()
	now := time.Now()
	l.now = func() time.Time { return now }
	for i := 0; i < LoginMaxFailures; i++ {
		l.Failed("admin", "10.0.0.1")
	}
	if !l.Blocked("admin", "10.0.0.1") {
		t.Fatal("expected block")
	}
	now = now.Add(LoginBlockFor + time.Minute)
	if l.Blocked("admin", "10.0.0.1") {
		t.Fatal("block did not expire")
	}
}

func TestCheckProductionBoot(t *testing.T) {
	goodHash, err := bcrypt.GenerateFromPassword([]byte("a-strong-password-1"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	defHash, err := bcrypt.GenerateFromPassword([]byte("changeme123"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	longToken := strings.Repeat("t", 40)
	cases := []struct {
		name    string
		token   string
		set     bool
		hashes  []string
		wantErr bool
	}{
		{"unset token", "", false, []string{string(goodHash)}, true},
		{"default token", "local-worker-token", true, []string{string(goodHash)}, true},
		{"short token", "only-20-chars-long-000", true, []string{string(goodHash)}, true},
		{"default password", longToken, true, []string{string(defHash)}, true},
		{"clean", longToken, true, []string{string(goodHash)}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckProductionBoot(tc.token, tc.set, "local-worker-token", tc.hashes, "changeme123")
			if tc.wantErr && err == nil {
				t.Fatal("expected refusal, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected boot, got %v", err)
			}
		})
	}
}

func TestLoginHandlerSixthAttempt429(t *testing.T) {
	st := openTestStore(t)
	svc := NewService(st, time.Hour)
	if _, err := svc.CreateOperator("admin", "correct-password-1"); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(svc)
	var lastCode int
	for i := 0; i < 6; i++ {
		body := `{"username":"admin","password":"wrong"}`
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
		req.RemoteAddr = "10.9.9.9:1234"
		rec := httptest.NewRecorder()
		h.Login(rec, req)
		lastCode = rec.Code
	}
	if lastCode != http.StatusTooManyRequests {
		t.Fatalf("6th attempt = %d, want 429", lastCode)
	}
	// Same credentials from another IP still get a plain 401.
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"username":"admin","password":"wrong"}`))
	req.RemoteAddr = "10.9.9.10:1234"
	rec := httptest.NewRecorder()
	h.Login(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("other IP = %d, want 401", rec.Code)
	}
}

func TestLoginHandlerSuccessResetsLimit(t *testing.T) {
	st := openTestStore(t)
	svc := NewService(st, time.Hour)
	if _, err := svc.CreateOperator("admin", "correct-password-1"); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(svc)
	login := func(pass, ip string) int {
		body := fmt.Sprintf(`{"username":"admin","password":%q}`, pass)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
		req.RemoteAddr = ip + ":1234"
		rec := httptest.NewRecorder()
		h.Login(rec, req)
		return rec.Code
	}
	for i := 0; i < 4; i++ {
		if code := login("wrong", "10.9.9.9"); code != http.StatusUnauthorized {
			t.Fatalf("failure %d = %d, want 401", i+1, code)
		}
	}
	if code := login("correct-password-1", "10.9.9.9"); code != http.StatusOK {
		t.Fatalf("good login = %d, want 200", code)
	}
	for i := 0; i < 4; i++ {
		if code := login("wrong", "10.9.9.9"); code != http.StatusUnauthorized {
			t.Fatalf("post-reset failure %d = %d, want 401", i+1, code)
		}
	}
}
