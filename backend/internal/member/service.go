// Package member serves public consignment users: signup, login,
// product browsing, applications, and the gated private link. Operator
// administration of products and applications lives here too, guarded
// by the operator middleware at the router.
package member

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"somun/internal/store"
)

var (
	ErrNoMember       = errors.New("member not found")
	ErrBadCredentials = errors.New("email or password is wrong")
	ErrWeakPassword   = errors.New("password must be 8+ characters")
	ErrBadEmail       = errors.New("email is invalid")
	ErrBadNickname    = errors.New("nickname is required")
	ErrDuplicate      = errors.New("email already registered")
	ErrBadToken       = errors.New("invalid or expired session")
)

// SessionTTL is the member login lifetime; Validate slides it while active.
const SessionTTL = 30 * 24 * time.Hour

// Service persists members in the store and seals private URLs with the vault.
type Service struct {
	store *store.Store
	vault cipher.AEAD
}

// NewService loads (or creates) the vault key shared with the operator side.
func NewService(st *store.Store, keyPath string) (*Service, error) {
	key, err := loadOrCreateKey(keyPath)
	if err != nil {
		return nil, fmt.Errorf("member vault: %v", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("member vault: %v", err)
	}
	vault, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("member vault: %v", err)
	}
	return &Service{store: st, vault: vault}, nil
}

func loadOrCreateKey(path string) ([]byte, error) {
	if raw, err := os.ReadFile(path); err == nil {
		if len(raw) != 32 {
			return nil, fmt.Errorf("vault key must be 32 bytes")
		}
		return raw, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

func (s *Service) seal(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	nonce := make([]byte, s.vault.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := s.vault.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func (s *Service) open(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	nonceSize := s.vault.NonceSize()
	if len(raw) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	plain, err := s.vault.Open(nil, raw[:nonceSize], raw[nonceSize:], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// MemberView is the safe public shape (never a hash).
type MemberView struct {
	ID        int64  `json:"id"`
	Email     string `json:"email"`
	Nickname  string `json:"nickname"`
	CreatedAt string `json:"createdAt"`
}

func toMemberView(m *store.Member) *MemberView {
	return &MemberView{ID: m.ID, Email: m.Email, Nickname: m.Nickname, CreatedAt: m.CreatedAt}
}

func validEmail(email string) bool {
	email = strings.TrimSpace(email)
	if len(email) < 5 || len(email) > 120 {
		return false
	}
	at := strings.Index(email, "@")
	return at > 0 && at < len(email)-1 && strings.Index(email[at+1:], "@") < 0
}

// Signup registers a member and logs them in immediately.
func (s *Service) Signup(email, password, nickname string) (string, *MemberView, error) {
	email = strings.TrimSpace(email)
	nickname = strings.TrimSpace(nickname)
	if !validEmail(email) {
		return "", nil, ErrBadEmail
	}
	if len(password) < 8 {
		return "", nil, ErrWeakPassword
	}
	if nickname == "" || len(nickname) > 30 {
		return "", nil, ErrBadNickname
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", nil, err
	}
	m, err := s.store.CreateMember(email, string(hash), nickname)
	if err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			return "", nil, ErrDuplicate
		}
		return "", nil, err
	}
	token, err := newToken()
	if err != nil {
		return "", nil, err
	}
	expires := time.Now().Add(SessionTTL).UTC().Format(time.RFC3339)
	if err := s.store.CreateMemberSession(hashToken(token), m.ID, expires); err != nil {
		return "", nil, err
	}
	return token, toMemberView(m), nil
}

// Login verifies credentials and returns a bearer token. Missing emails
// report the same error as wrong passwords (no enumeration).
func (s *Service) Login(email, password string) (string, *MemberView, error) {
	m, err := s.store.GetMemberByEmail(strings.TrimSpace(email))
	if err != nil {
		return "", nil, ErrBadCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(m.PasswordHash), []byte(password)); err != nil {
		return "", nil, ErrBadCredentials
	}
	token, err := newToken()
	if err != nil {
		return "", nil, err
	}
	expires := time.Now().Add(SessionTTL).UTC().Format(time.RFC3339)
	if err := s.store.CreateMemberSession(hashToken(token), m.ID, expires); err != nil {
		return "", nil, err
	}
	return token, toMemberView(m), nil
}

// Validate returns the member id for a token, sliding the expiry while
// the session stays active. Expired rows are removed.
func (s *Service) Validate(token string) (int64, error) {
	hash := hashToken(token)
	sess, err := s.store.GetMemberSession(hash)
	if err != nil {
		return 0, ErrBadToken
	}
	expires, err := time.Parse(time.RFC3339, sess.ExpiresAt)
	if err != nil || time.Now().After(expires) {
		_ = s.store.DeleteMemberSession(hash)
		return 0, ErrBadToken
	}
	if time.Until(expires) < 15*24*time.Hour {
		_ = s.store.TouchMemberSession(hash, time.Now().Add(SessionTTL).UTC().Format(time.RFC3339))
	}
	return sess.MemberID, nil
}

// Logout drops one token.
func (s *Service) Logout(token string) {
	_ = s.store.DeleteMemberSession(hashToken(token))
}

// Me returns one member's public view.
func (s *Service) Me(memberID int64) (*MemberView, error) {
	m, err := s.store.GetMember(memberID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNoMember
		}
		return nil, err
	}
	return toMemberView(m), nil
}

type ctxKey struct{}

func bearerToken(r *http.Request) (string, error) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", ErrBadToken
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") || parts[1] == "" {
		return "", ErrBadToken
	}
	return parts[1], nil
}

// Middleware rejects requests without a valid member token.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := bearerToken(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "member login required")
			return
		}
		id, err := s.Validate(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid or expired session")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, id)))
	})
}

// MemberFrom returns the authenticated member id for a request.
func MemberFrom(r *http.Request) (int64, bool) {
	id, ok := r.Context().Value(ctxKey{}).(int64)
	return id, ok
}
