package auth

import (
	"sync"
	"time"
)

// Login rate limit (D31): 5 failures per username+IP within 15 minutes
// blocks further attempts for 15 minutes (HTTP 429).
const (
	LoginMaxFailures = 5
	LoginWindow      = 15 * time.Minute
	LoginBlockFor    = 15 * time.Minute
)

type loginBucket struct {
	failures int
	window   time.Time
	blocked  time.Time // non-zero while blocked
}

// LoginLimiter tracks failed operator logins per username+IP.
type LoginLimiter struct {
	mu      sync.Mutex
	buckets map[string]*loginBucket
	now     func() time.Time
}

// NewLoginLimiter builds a limiter using the wall clock.
func NewLoginLimiter() *LoginLimiter {
	return &LoginLimiter{buckets: map[string]*loginBucket{}, now: time.Now}
}

// Blocked reports whether an attempt must be refused with 429 right now.
func (l *LoginLimiter) Blocked(username, ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[username+"\x00"+ip]
	if !ok {
		return false
	}
	if !b.blocked.IsZero() {
		if l.now().Before(b.blocked) {
			return true
		}
		b.blocked = time.Time{}
		b.failures = 0
	}
	return false
}

// Failed records one failed attempt. The failure that reaches the limit arms
// the block; that attempt itself still returns 401, the next gets 429.
func (l *LoginLimiter) Failed(username, ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	key := username + "\x00" + ip
	b, ok := l.buckets[key]
	now := l.now()
	if !ok {
		b = &loginBucket{}
		l.buckets[key] = b
	}
	if now.Sub(b.window) >= LoginWindow {
		b.window = now
		b.failures = 0
	}
	b.failures++
	if b.failures >= LoginMaxFailures && b.blocked.IsZero() {
		b.blocked = now.Add(LoginBlockFor)
	}
}

// Succeeded clears the bucket after a good login.
func (l *LoginLimiter) Succeeded(username, ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.buckets, username+"\x00"+ip)
}
