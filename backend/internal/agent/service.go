// Package agent owns the resident registry (D22), lane-routed claims (D24),
// leases (D25) and the long-poll hub (D26).
package agent

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"somun/internal/session"
	"somun/internal/store"
)

// Auth errors for agent tokens.
var (
	ErrUnknownToken = errors.New("unknown agent token")
	ErrAgentPaused  = errors.New("agent is paused")
	ErrLaneMismatch = errors.New("job type is outside the agent lane")
)

// Service implements agent registry and routed claiming.
type Service struct {
	store       *store.Store
	hub         *Hub
	leaseSec    int
	longpollMax int
	// OnLeaseExpired fires per failed closing (failed/failed_held only).
	OnLeaseExpired func(action store.ReapAction)
	// Session applies agent-observed session moves (wired by main; nil skips).
	Session *session.Service
}

// NewService builds the agent service.
func NewService(st *store.Store, hub *Hub, leaseSec, longpollMax int) *Service {
	return &Service{store: st, hub: hub, leaseSec: leaseSec, longpollMax: longpollMax}
}

// Authenticate resolves a bearer token to an enabled agent.
func (s *Service) Authenticate(plaintext string) (*store.Agent, error) {
	a, err := s.store.GetAgentByTokenHash(store.HashAgentToken(plaintext))
	if err != nil {
		return nil, ErrUnknownToken
	}
	if !a.Enabled {
		return nil, ErrAgentPaused
	}
	return a, nil
}

// Claim assigns the best fitting job, optionally long-polling up to waitSec.
// wantType empty means the agent's whole lane. Polling proves liveness, so a
// successful claim path refreshes last_seen_at.
func (s *Service) Claim(agent *store.Agent, wantType string, waitSec int) (*store.Job, error) {
	types := store.LaneTypes[agent.Lane]
	if wantType != "" {
		wantType = strings.ToLower(strings.TrimSpace(wantType))
		ok := false
		for _, t := range types {
			if t == wantType {
				ok = true
				break
			}
		}
		if !ok {
			return nil, ErrLaneMismatch
		}
		types = []string{wantType}
	}
	if waitSec > 0 {
		if waitSec > s.longpollMax {
			waitSec = s.longpollMax
		}
		if has, err := s.store.HasPendingCandidate(agent, types); err != nil {
			return nil, err
		} else if !has {
			s.hub.Wait(time.Duration(waitSec) * time.Second)
		}
	}
	job, err := s.store.ClaimRoutedJob(agent, types, s.leaseSec)
	if err != nil {
		return nil, err
	}
	if job != nil {
		_ = s.store.TouchAgent(agent.ID)
	}
	return job, nil
}

// Wait sleeps for a claim signal, capped at the configured long-poll max.
func (s *Service) Wait(sec int) {
	if sec > s.longpollMax {
		sec = s.longpollMax
	}
	s.hub.Wait(time.Duration(sec) * time.Second)
}

// Heartbeat records liveness and applies an observed session move for the
// agent's linked account, if any.
func (s *Service) Heartbeat(agent *store.Agent, sessionState string) (bool, error) {
	if err := s.store.TouchAgent(agent.ID); err != nil {
		return false, err
	}
	if sessionState != "" && agent.AccountID != nil && s.Session != nil {
		if _, err := s.Session.SetSessionState(*agent.AccountID, sessionState, ""); err != nil {
			return true, err
		}
	}
	fresh, err := s.store.GetAgent(agent.ID)
	if err != nil {
		return false, err
	}
	return fresh.Enabled, nil
}

// JobHeartbeat extends one claim's lease for its owning agent.
func (s *Service) JobHeartbeat(agent *store.Agent, jobID int64) (string, error) {
	return s.store.ExtendJobLease(jobID, agent.ID, s.leaseSec)
}

// Requeue returns a failed job to pending (operator action).
func (s *Service) Requeue(jobID int64) (*store.Job, error) {
	return s.store.RequeueJob(jobID)
}

// ReapTick expires stale leases and reports failed closings via the hook.
func (s *Service) ReapTick(now time.Time) ([]store.ReapAction, error) {
	actions, err := s.store.ReapExpiredLeases(now.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	if s.OnLeaseExpired != nil {
		for _, a := range actions {
			if a.Action != "requeued" {
				s.OnLeaseExpired(a)
			}
		}
	}
	return actions, nil
}

// Register adds an agent and returns it with a one-time plaintext token.
func (s *Service) Register(label, lane, host, businesses, platforms string, capacity int, accountID *int64) (*store.Agent, string, error) {
	if strings.TrimSpace(label) == "" {
		return nil, "", fmt.Errorf("label is required")
	}
	if accountID != nil {
		if _, err := s.store.GetAccountRow(*accountID); err != nil {
			return nil, "", fmt.Errorf("unknown account id %d", *accountID)
		}
		if err := s.checkLaneSplit(0, lane, *accountID); err != nil {
			return nil, "", err
		}
	}
	return s.store.CreateAgent(strings.TrimSpace(label), lane, strings.TrimSpace(host),
		strings.TrimSpace(businesses), strings.TrimSpace(platforms), capacity, accountID)
}

// checkLaneSplit refuses to bind one account across both the B (generate)
// and C (review) lanes: a writer must never review its own drafts (W6).
// selfID excludes the agent being updated from the bound lanes.
func (s *Service) checkLaneSplit(selfID int64, lane string, accountID int64) error {
	lanes, err := s.store.LanesForAccountExcept(accountID, selfID)
	if err != nil {
		return err
	}
	has := map[string]bool{strings.ToUpper(strings.TrimSpace(lane)): true}
	for _, l := range lanes {
		has[strings.ToUpper(strings.TrimSpace(l))] = true
	}
	if has["B"] && has["C"] {
		return fmt.Errorf("account %d spans lanes B+C: one account cannot write and review", accountID)
	}
	return nil
}

// Rotate replaces an agent token, returning the one-time plaintext.
func (s *Service) Rotate(id int64) (string, error) {
	return s.store.RotateAgentToken(id)
}

// List returns a filtered page plus the total.
func (s *Service) List(f store.AgentFilter, limit, offset int) ([]*store.Agent, int, error) {
	items, err := s.store.ListAgents(f, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.store.CountAgents(f)
	if err != nil {
		return nil, 0, err
	}
	if err := s.fillActivity(items); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Get returns one agent with its recent jobs.
func (s *Service) Get(id int64) (*store.Agent, []*store.Job, error) {
	a, err := s.store.GetAgent(id)
	if err != nil {
		return nil, nil, err
	}
	jobs, err := s.store.ListJobsByAgent(id, 20)
	if err != nil {
		return nil, nil, err
	}
	return a, jobs, nil
}

// Update writes mutable fields.
func (s *Service) Update(id int64, lane, host, businesses, platforms string, capacity int, enabled bool, accountID *int64) (*store.Agent, error) {
	a, err := s.store.GetAgent(id)
	if err != nil {
		return nil, err
	}
	if accountID != nil {
		if _, err := s.store.GetAccountRow(*accountID); err != nil {
			return nil, fmt.Errorf("unknown account id %d", *accountID)
		}
		if err := s.checkLaneSplit(id, lane, *accountID); err != nil {
			return nil, err
		}
	}
	a.Lane, a.Host, a.Businesses, a.Platforms = lane, host, businesses, platforms
	a.Capacity, a.Enabled, a.AccountID = capacity, enabled, accountID
	if err := s.store.UpdateAgent(a); err != nil {
		return nil, err
	}
	return s.store.GetAgent(id)
}

// Delete removes an agent unless it holds live claims.
func (s *Service) Delete(id int64) error {
	if _, err := s.store.GetAgent(id); err != nil {
		return err
	}
	n, err := s.store.ClaimedCountByAgent(id)
	if err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("agent holds %d claimed jobs", n)
	}
	return s.store.DeleteAgent(id)
}

// Bulk pauses, resumes, or relanes agents, reporting how many changed.
func (s *Service) Bulk(ids []int64, action, lane string) (int, error) {
	switch action {
	case "pause", "resume", "lane":
	default:
		return 0, fmt.Errorf("action must be pause, resume or lane")
	}
	if action == "lane" && !store.ValidLane(lane) {
		return 0, fmt.Errorf("lane must be A, B, C or P")
	}
	updated := 0
	for _, id := range ids {
		a, err := s.store.GetAgent(id)
		if err != nil {
			continue
		}
		switch action {
		case "pause":
			a.Enabled = false
		case "resume":
			a.Enabled = true
		case "lane":
			a.Lane = lane
		}
		if err := s.store.UpdateAgent(a); err == nil {
			updated++
		}
	}
	return updated, nil
}

// Stats returns the dashboard agent panel.
func (s *Service) Stats(now time.Time) (*store.AgentStats, error) {
	return s.store.StatsForAgents(now.UTC().Format(time.RFC3339))
}

func (s *Service) fillActivity(items []*store.Agent) error {
	act, err := s.store.AgentActivity()
	if err != nil {
		return err
	}
	for _, a := range items {
		if id, ok := act.Claimed[a.ID]; ok {
			v := id
			a.CurrentJobID = &v
		}
		a.Done24h = act.Done[a.ID]
		a.Failed24h = act.Failed[a.ID]
	}
	return nil
}
