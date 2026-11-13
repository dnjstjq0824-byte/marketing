// Package publisher is the official-API contract (D35): one Publisher per
// platform with a fixed request/response/error shape. P1-1 ships stubs;
// P1-2 wires real credentials and HTTP behind the same interface.
package publisher

import (
	"context"
	"errors"
)

// Typed execution outcomes. The executor maps them to account/log moves.
var (
	// ErrNotConfigured marks a platform without credentials yet.
	ErrNotConfigured = errors.New("publisher not configured")
	// ErrSessionLimited marks a mid-flight session limit: the executor fails
	// the item, flips the account to limited, and holds its queue.
	ErrSessionLimited = errors.New("session limited mid-flight")
	// ErrRateLimited marks a soft throttle: retry on the next eligibility.
	ErrRateLimited = errors.New("rate limited")
	// ErrRejected marks a platform-side content rejection.
	ErrRejected = errors.New("platform rejected the content")
)

// Request is one official-API post.
type Request struct {
	ContentID int64
	Channel   string
	Title     string
	Body      string
	Account   string
	LoginID   string
	Password  string
}

// Result is a successful post.
type Result struct {
	URL string
}

// Publisher posts one request.
type Publisher interface {
	Platform() string
	Publish(ctx context.Context, req Request) (Result, error)
}

type stub struct{ platform string }

// Platform names the stubbed platform.
func (s stub) Platform() string { return s.platform }

// Publish always reports unconfigured: P1-2 plugs the real client in.
func (s stub) Publish(ctx context.Context, req Request) (Result, error) {
	return Result{}, ErrNotConfigured
}

// Registry returns the stub for a platform, or nil for unknown platforms.
func Registry() map[string]Publisher {
	out := map[string]Publisher{}
	for _, p := range []string{"instagram", "facebook", "threads", "tiktok", "youtube", "blogger"} {
		out[p] = stub{platform: p}
	}
	return out
}
