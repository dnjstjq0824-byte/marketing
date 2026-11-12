// Package session is the Tranche-1 skeleton for agent slots, channel-account
// sessions, and reservation holds (§9). State is in-memory by design: session
// liveness is operational, not part of the 9-entity data model. No account
// credentials are ever stored here — only session state and timestamps.
package session

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"somun/internal/content"
	"somun/internal/store"
)

// Session states: pending/connected/expired/limited/inactive_hold.
const (
	StatePending      = "pending"
	StateConnected    = "connected"
	StateExpired      = "expired"
	StateLimited      = "limited"
	StateInactiveHold = "inactive_hold"
)

var validStates = map[string]bool{
	StatePending: true, StateConnected: true, StateExpired: true,
	StateLimited: true, StateInactiveHold: true,
}

// Roles for agent accounts.
const (
	RoleChief  = "chief"
	RoleWorker = "worker"
)

// Errors.
var (
	ErrNoAccount     = errors.New("account not found")
	ErrNoSlot        = errors.New("slot not found")
	ErrBadState      = errors.New("unknown session state")
	ErrBadRole       = errors.New("role must be chief or worker")
	ErrNoName        = errors.New("name is required")
	ErrNoSlotLabel   = errors.New("slot label is required")
	ErrSlotTaken     = errors.New("slot label already exists")
	ErrFinalDisposal = errors.New("release/transfer is a PO decision, not implemented")
)

// Account is one agent/channel account with session liveness.
type Account struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Platform      string `json:"platform"`
	Owner         string `json:"owner"`
	Role          string `json:"role"`
	Business      string `json:"business"`
	SessionState  string `json:"sessionState"`
	LastCheckedAt string `json:"lastCheckedAt"`
	LastWakeAt    string `json:"lastWakeAt"`
	Usage         string `json:"usage"`
	LastJobAt     string `json:"lastJobAt"`
}

// Slot is one neutral agent slot. Labels A/B/C are examples; any label works.
type Slot struct {
	ID        int64  `json:"id"`
	Label     string `json:"label"`
	AgentName string `json:"agentName"`
	AccountID *int64 `json:"accountId"`
}

// ReloginItem is one account needing its owner to log back in.
type ReloginItem struct {
	AccountID    int64  `json:"accountId"`
	AccountName  string `json:"accountName"`
	Owner        string `json:"owner"`
	SessionState string `json:"sessionState"`
	HeldCount    int    `json:"heldCount"`
}

// Service holds slots and account sessions in memory.
type Service struct {
	store   *store.Store
	content *content.Service

	mu       sync.Mutex
	accounts map[int64]*Account
	slots    map[int64]*Slot
	nextID   int64
}

// NewService builds the skeleton with neutral example slots A/B/C.
func NewService(st *store.Store) *Service {
	s := &Service{
		store:    st,
		content:  content.NewService(st),
		accounts: map[int64]*Account{},
		slots:    map[int64]*Slot{},
		nextID:   1,
	}
	for _, label := range []string{"A", "B", "C"} {
		s.slots[s.nextID] = &Slot{ID: s.nextID, Label: label}
		s.nextID++
	}
	return s
}

func nowUTC() string { return time.Now().UTC().Format(time.RFC3339) }

// RegisterAccount adds an account in pending state.
func (s *Service) RegisterAccount(name, platform, owner, role, business string) (*Account, error) {
	if name == "" {
		return nil, ErrNoName
	}
	if role == "" {
		role = RoleWorker
	}
	if role != RoleChief && role != RoleWorker {
		return nil, ErrBadRole
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	a := &Account{ID: s.nextID, Name: name, Platform: platform, Owner: owner,
		Role: role, Business: business, SessionState: StatePending}
	s.accounts[s.nextID] = a
	s.nextID++
	return copyAccount(a), nil
}

// ListAccounts returns all accounts by id.
func (s *Service) ListAccounts() []*Account {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*Account
	for id := int64(1); id < s.nextID; id++ {
		if a, ok := s.accounts[id]; ok {
			out = append(out, copyAccount(a))
		}
	}
	return out
}

// GetAccount returns one account.
func (s *Service) GetAccount(id int64) (*Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[id]
	if !ok {
		return nil, ErrNoAccount
	}
	return copyAccount(a), nil
}

// SetSessionState moves an account's session state machine with side effects:
// expired/limited/inactive_hold holds only that account's queued items
// (reservations kept, never deleted); connected resumes holds except items
// overdue by a day or more, which stay as report targets.
func (s *Service) SetSessionState(id int64, state, usage string) (*Account, error) {
	if !validStates[state] {
		return nil, ErrBadState
	}
	s.mu.Lock()
	a, ok := s.accounts[id]
	if !ok {
		s.mu.Unlock()
		return nil, ErrNoAccount
	}
	prev := a.SessionState
	a.SessionState = state
	if usage != "" {
		a.Usage = usage
	}
	name := a.Name
	s.mu.Unlock()

	switch state {
	case StateExpired, StateLimited, StateInactiveHold:
		reason := fmt.Sprintf("세션 %s — 발행 정지, 예약 보류", stateLabel(state))
		if _, err := s.store.SetHeldByAccount(name, reason); err != nil {
			return nil, err
		}
	case StateConnected:
		if prev == StateExpired || prev == StateLimited {
			if _, _, err := s.content.ResumeAccount(name); err != nil {
				return nil, err
			}
		}
		// Leaving inactive_hold resumes nothing automatically: contact-lost
		// disposal is a PO decision. Holds stay until reviewed.
	}
	return s.GetAccount(id)
}

// CheckNow runs the light periodic inspection: timestamps only, no wake-ups.
func (s *Service) CheckNow() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, a := range s.accounts {
		a.LastCheckedAt = nowUTC()
		n++
	}
	return n
}

// Wake marks one assigned session as woken. There is no wake-all: only
// sessions with assigned work are ever woken.
func (s *Service) Wake(id int64) (*Account, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.accounts[id]
	if !ok {
		return nil, ErrNoAccount
	}
	a.LastWakeAt = nowUTC()
	a.LastCheckedAt = a.LastWakeAt
	return copyAccount(a), nil
}

// ReloginQueue gathers expired/limited accounts with their held counts.
func (s *Service) ReloginQueue() []*ReloginItem {
	s.mu.Lock()
	accts := make([]*Account, 0, len(s.accounts))
	for _, a := range s.accounts {
		accts = append(accts, copyAccount(a))
	}
	s.mu.Unlock()
	var out []*ReloginItem
	for _, a := range accts {
		if a.SessionState != StateExpired && a.SessionState != StateLimited {
			continue
		}
		held, _ := s.store.ListHeldByAccount(a.Name)
		out = append(out, &ReloginItem{AccountID: a.ID, AccountName: a.Name,
			Owner: a.Owner, SessionState: a.SessionState, HeldCount: len(held)})
	}
	return out
}

// ListSlots returns all slots by id.
func (s *Service) ListSlots() []*Slot {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*Slot
	for id := int64(1); id < s.nextID; id++ {
		if sl, ok := s.slots[id]; ok {
			cp := *sl
			out = append(out, &cp)
		}
	}
	return out
}

// AddSlot appends a neutral slot with a unique label.
func (s *Service) AddSlot(label, agentName string) (*Slot, error) {
	if label == "" {
		return nil, ErrNoSlotLabel
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sl := range s.slots {
		if sl.Label == label {
			return nil, ErrSlotTaken
		}
	}
	sl := &Slot{ID: s.nextID, Label: label, AgentName: agentName}
	s.slots[s.nextID] = sl
	s.nextID++
	cp := *sl
	return &cp, nil
}

func copyAccount(a *Account) *Account {
	cp := *a
	return &cp
}

func stateLabel(s string) string {
	switch s {
	case StateExpired:
		return "만료"
	case StateLimited:
		return "한도"
	case StateInactiveHold:
		return "비활성 보류"
	case StateConnected:
		return "연결"
	default:
		return s
	}
}
