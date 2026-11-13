package agent

import (
	"context"

	"somun/internal/store"
)

type ctxKey struct{}

// identity carries the authenticated caller: an agent, or nil for the
// shared dev token. Presence of the value means authentication ran.
type identity struct {
	agent *store.Agent
}

// WithAgent stores the authenticated agent.
func WithAgent(ctx context.Context, a *store.Agent) context.Context {
	return context.WithValue(ctx, ctxKey{}, identity{agent: a})
}

// WithShared marks a shared-token (identity-less) authentication.
func WithShared(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKey{}, identity{agent: nil})
}

// AgentFrom reads the authenticated caller back: (agent, true), (nil, true)
// for the shared token, or (nil, false) when no auth ran.
func AgentFrom(ctx context.Context) (*store.Agent, bool) {
	id, ok := ctx.Value(ctxKey{}).(identity)
	if !ok {
		return nil, false
	}
	return id.agent, true
}
