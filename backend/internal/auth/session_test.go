package auth

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"somun/internal/store"
)

func openAuthStore(t *testing.T) *store.Store {
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

func seedOperator(t *testing.T, svc *Service) {
	t.Helper()
	if err := svc.EnsureAdmin("admin", "testpass123"); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// Sessions survive service restarts (the refresh/reboot regression);
// logout and expiry still invalidate.
func TestSessionPersistence(t *testing.T) {
	st := openAuthStore(t)
	svc := NewService(st, time.Hour)
	seedOperator(t, svc)
	token, _, err := svc.Login("admin", "testpass123")
	if err != nil {
		t.Fatal(err)
	}
	// Restart: a new service over the same store honors the token.
	svc2 := NewService(st, time.Hour)
	uid, name, err := svc2.Validate(token)
	if err != nil {
		t.Fatalf("validate after restart: %v", err)
	}
	if uid <= 0 || name != "admin" {
		t.Fatalf("session = (%d, %q)", uid, name)
	}
	svc2.Logout(token)
	if _, _, err := svc.Validate(token); !errors.Is(err, ErrBadToken) {
		t.Fatalf("logged-out token: %v", err)
	}
	if _, _, err := svc.Validate("nope"); !errors.Is(err, ErrBadToken) {
		t.Fatalf("unknown token: %v", err)
	}
}

// Expired sessions are rejected and their rows removed.
func TestSessionExpiry(t *testing.T) {
	st := openAuthStore(t)
	svc := NewService(st, -time.Hour)
	seedOperator(t, svc)
	token, _, err := svc.Login("admin", "testpass123")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Validate(token); !errors.Is(err, ErrBadToken) {
		t.Fatalf("expired token: %v", err)
	}
	if _, err := st.GetOperatorSession(hashToken(token)); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expired row kept: %v", err)
	}
}
