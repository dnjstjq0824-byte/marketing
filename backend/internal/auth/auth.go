// Package auth handles operator login and bearer-token sessions.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
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

// Service issues and validates operator sessions. Sessions persist in the
// store (hashed), so logins survive restarts and refreshes without new logins.
type Service struct {
	store   *store.Store
	ttl     time.Duration
	limiter *LoginLimiter
}

// NewService builds an auth service with the given token TTL.
func NewService(st *store.Store, ttl time.Duration) *Service {
	return &Service{store: st, ttl: ttl, limiter: NewLoginLimiter()}
}

// hashToken identifies a session row without storing the bearer itself.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// LoginBlocked reports whether this username+IP is rate limited (D31).
func (s *Service) LoginBlocked(username, ip string) bool {
	return s.limiter.Blocked(username, ip)
}

// LoginFailed records a failed attempt for rate limiting.
func (s *Service) LoginFailed(username, ip string) {
	s.limiter.Failed(username, ip)
}

// LoginSucceeded clears rate-limit state after a good login.
func (s *Service) LoginSucceeded(username, ip string) {
	s.limiter.Succeeded(username, ip)
}

// ClientIP returns the caller address for rate limiting. Behind a tunnel or
// proxy the edge address wins (CF-Connecting-IP, then X-Forwarded-For);
// direct connections fall back to the remote address.
func ClientIP(r *http.Request) string {
	if ip := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); ip != "" {
		return ip
	}
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if ip := strings.TrimSpace(strings.SplitN(fwd, ",", 2)[0]); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// DefaultPasswordInUse reports whether any stored hash matches the default
// password. Expensive by design (bcrypt); call only at boot (D31).
func DefaultPasswordInUse(hashes []string, defaultPassword string) bool {
	for _, h := range hashes {
		if bcrypt.CompareHashAndPassword([]byte(h), []byte(defaultPassword)) == nil {
			return true
		}
	}
	return false
}

// MinProductionTokenLength is the shortest accepted WORKER_TOKEN in production.
const MinProductionTokenLength = 32

// CheckProductionBoot refuses production boot on default secrets (D31).
func CheckProductionBoot(workerToken string, workerTokenSet bool, defaultToken string, passwordHashes []string, defaultPassword string) error {
	if !workerTokenSet || workerToken == "" || workerToken == defaultToken {
		return fmt.Errorf("production refuses to boot: set WORKER_TOKEN explicitly")
	}
	if len(workerToken) < MinProductionTokenLength {
		return fmt.Errorf("production refuses to boot: WORKER_TOKEN must be %d+ characters", MinProductionTokenLength)
	}
	if DefaultPasswordInUse(passwordHashes, defaultPassword) {
		return fmt.Errorf("production refuses to boot: an operator still uses the default password")
	}
	return nil
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
	expires := time.Now().Add(s.ttl).UTC().Format(time.RFC3339)
	if err := s.store.CreateOperatorSession(hashToken(token), u.ID, u.Username, expires); err != nil {
		return "", nil, err
	}
	return token, u, nil
}

// Validate returns the session user for a token. Expired rows are removed.
func (s *Service) Validate(token string) (int64, string, error) {
	hash := hashToken(token)
	sess, err := s.store.GetOperatorSession(hash)
	if err != nil {
		return 0, "", ErrBadToken
	}
	expires, err := time.Parse(time.RFC3339, sess.ExpiresAt)
	if err != nil || time.Now().After(expires) {
		_ = s.store.DeleteOperatorSession(hash)
		return 0, "", ErrBadToken
	}
	return sess.UserID, sess.Username, nil
}

// Logout drops one token.
func (s *Service) Logout(token string) {
	_ = s.store.DeleteOperatorSession(hashToken(token))
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

// Middleware rejects requests without a valid operator token. Validated
// requests carry the operator identity for the W20-2 audit log.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := BearerToken(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "operator login required")
			return
		}
		userID, username, err := s.Validate(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid or expired session")
			return
		}
		next.ServeHTTP(w, r.WithContext(WithOperator(r.Context(), userID, username)))
	})
}
