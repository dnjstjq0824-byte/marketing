// Package auth handles operator login and bearer-token sessions.
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"somun/internal/store"

	"golang.org/x/crypto/bcrypt"
)

// Errors for login failures.
var (
	ErrBadCredentials = errors.New("invalid username or password")
	ErrNoToken        = errors.New("missing bearer token")
	ErrBadToken       = errors.New("invalid or expired token")
	ErrWeakPassword   = errors.New("password must be at least 8 characters")
	ErrDeleteSelf     = errors.New("cannot delete your own account")
	ErrDeleteLast     = errors.New("cannot delete the last operator")
)

// MinPasswordLength is the shortest accepted operator password.
const MinPasswordLength = 8

// Service issues and validates operator sessions.
type Service struct {
	store *store.Store
	ttl   time.Duration

	mu       sync.Mutex
	sessions map[string]session
}

type session struct {
	userID   int64
	username string
	expires  time.Time
}

// NewService builds an auth service with the given token TTL.
func NewService(st *store.Store, ttl time.Duration) *Service {
	return &Service{store: st, ttl: ttl, sessions: map[string]session{}}
}

// EnsureAdmin seeds the first admin user from env-provided credentials.
func (s *Service) EnsureAdmin(username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.store.EnsureAdmin(username, string(hash))
	return err
}

// Login verifies credentials and returns a bearer token.
func (s *Service) Login(username, password string) (string, *store.User, error) {
	u, err := s.store.GetUserByUsername(username)
	if err != nil {
		return "", nil, ErrBadCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return "", nil, ErrBadCredentials
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, err
	}
	token := hex.EncodeToString(buf)
	s.mu.Lock()
	s.sessions[token] = session{userID: u.ID, username: u.Username, expires: time.Now().Add(s.ttl)}
	s.mu.Unlock()
	return token, u, nil
}

// Validate returns the session user for a token.
func (s *Service) Validate(token string) (int64, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[token]
	if !ok || time.Now().After(sess.expires) {
		delete(s.sessions, token)
		return 0, "", ErrBadToken
	}
	return sess.userID, sess.username, nil
}

// Logout drops one token.
func (s *Service) Logout(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}

// ListUsers returns operators (hashes are json:"-" and never serialize).
func (s *Service) ListUsers() ([]*store.User, error) {
	return s.store.ListUsers()
}

// CreateOperator adds an operator account.
func (s *Service) CreateOperator(username, password string) (*store.User, error) {
	if len(password) < MinPasswordLength {
		return nil, ErrWeakPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	return s.store.CreateUser(username, string(hash), "operator")
}

// ChangeOwnPassword verifies the current password and sets a new one.
func (s *Service) ChangeOwnPassword(userID int64, username, current, next string) error {
	if len(next) < MinPasswordLength {
		return ErrWeakPassword
	}
	u, err := s.store.GetUserByUsername(username)
	if err != nil || u.ID != userID {
		return ErrBadCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(current)); err != nil {
		return ErrBadCredentials
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.store.UpdateUserPassword(userID, string(hash))
}

// DeleteUserGuarded removes an operator unless it is the caller or the last one.
func (s *Service) DeleteUserGuarded(callerID, targetID int64) error {
	if callerID == targetID {
		return ErrDeleteSelf
	}
	n, err := s.store.CountUsers()
	if err != nil {
		return err
	}
	if n <= 1 {
		return ErrDeleteLast
	}
	return s.store.DeleteUser(targetID)
}

// BearerToken extracts the Authorization: Bearer token from a request.
func BearerToken(r *http.Request) (string, error) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", ErrNoToken
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") || parts[1] == "" {
		return "", ErrNoToken
	}
	return parts[1], nil
}

// Middleware rejects requests without a valid operator token.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := BearerToken(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "operator login required")
			return
		}
		if _, _, err := s.Validate(token); err != nil {
			writeError(w, http.StatusUnauthorized, "invalid or expired session")
			return
		}
		next.ServeHTTP(w, r)
	})
}
