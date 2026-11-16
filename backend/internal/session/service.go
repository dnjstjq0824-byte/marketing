// Package session manages channel-account sessions (§9).
// Accounts persist in channel_accounts; login credentials persist only as
// AES-256-GCM ciphertext under a machine-local vault key. Plaintext passwords
// never touch the database, logs, or API responses.
package session

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
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

// Posting modes: auto executes on schedule, manual prepares the queue.
const (
	PostingAuto   = "auto"
	PostingManual = "manual"
)

// Errors.
var (
	ErrNoAccount     = errors.New("account not found")
	ErrBadState      = errors.New("unknown session state")
	ErrBadRole       = errors.New("role must be chief or worker")
	ErrNoName        = errors.New("name is required")
	ErrFinalDisposal = errors.New("release/transfer is a PO decision, not implemented")
	ErrHeldItems     = errors.New("account still has queued items")
	ErrBadMode       = errors.New("posting mode must be auto or manual")
	ErrBadDailyCap   = errors.New("daily cap must be 0..999")
	ErrBadKind       = errors.New("kind must be channel or agent_service")
	ErrCredentials   = errors.New("login ID and password are both required")
	ErrDuplicate     = errors.New("account name already exists")
	ErrVault = errors.New("credential vault unavailable")
	ErrBadCapabilities = errors.New("capabilities must be a JSON array of text/image/video/file")
)

// Account is one agent/channel account with session liveness.
// It never carries secrets; HasCredentials only reports their presence.
type Account struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Platform       string `json:"platform"`
	Owner          string `json:"owner"`
	Role           string `json:"role"`
	Business       string `json:"business"`
	SessionState   string `json:"sessionState"`
	PostingMode    string `json:"postingMode"`
	DailyCap       int    `json:"dailyCap"`
	HasCredentials bool   `json:"hasCredentials"`
	LastCheckedAt  string `json:"lastCheckedAt"`
	LastWakeAt     string `json:"lastWakeAt"`
	Usage          string `json:"usage"`
	LastJobAt      string `json:"lastJobAt"`
	Kind           string `json:"kind"`
	PlatformUserID string `json:"platformUserId"`
	PlanTier       string `json:"planTier"`
	Capabilities   string `json:"capabilities"`
	ProfileMode    string `json:"profileMode"`
	ChatStatus     string `json:"chatStatus"`
}

// ReloginItem is one account needing its owner to log back in.
type ReloginItem struct {
	AccountID    int64  `json:"accountId"`
	AccountName  string `json:"accountName"`
	Owner        string `json:"owner"`
	SessionState string `json:"sessionState"`
	HeldCount    int    `json:"heldCount"`
}

// Service persists channel accounts in the store.
type Service struct {
	store   *store.Store
	content *content.Service
	vault   cipher.AEAD
	signKey []byte
	// OnSessionState, when set, fires after a session move (notify hook).
	OnSessionState func(id int64, name, prev, next string)
	// Console-driven login (login.go).
	loginDataDir string
	loginPython  string
	logins       *LoginManager
	// verifyRunner replaces the adapter.verify subprocess in tests.
	verifyRunner func(name, platform string) (string, string)
	// probeRunner replaces the adapter.chat --probe subprocess in tests.
	// It returns the probe JSON document.
	probeRunner func(name, platform string) (map[string]any, error)
}

// NewService loads (or creates) the vault key.
func NewService(st *store.Store, keyPath string) (*Service, error) {
	key, err := loadOrCreateKey(keyPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrVault, err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrVault, err)
	}
	vault, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrVault, err)
	}
	return &Service{store: st, content: content.NewService(st), vault: vault, signKey: key}, nil
}

// FileKey returns a copy of the vault key bytes for HMAC-signing worker
// download URLs (W5-3). Callers must not log or persist it.
func (s *Service) FileKey() []byte {
	out := make([]byte, len(s.signKey))
	copy(out, s.signKey)
	return out
}

// loadOrCreateKey reads the 32-byte vault key, generating one (0600) on first boot.
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

func nowUTC() string { return time.Now().UTC().Format(time.RFC3339) }

// CredentialsFor decrypts one account's login pair for internal publishers.
// It never serves API responses; callers must not log the values.
func (s *Service) CredentialsFor(id int64) (string, string, error) {
	row, err := s.store.GetAccountRow(id)
	if err != nil {
		return "", "", err
	}
	loginID, err := s.open(row.EncID)
	if err != nil {
		return "", "", err
	}
	password, err := s.open(row.EncPW)
	if err != nil {
		return "", "", err
	}
	if loginID == "" || password == "" {
		return "", "", ErrCredentials
	}
	return loginID, password, nil
}

func toView(row *store.ChannelAccount) *Account {
	return &Account{
		ID: row.ID, Name: row.Name, Platform: row.Platform, Owner: row.Owner,
		Role: row.Role, Business: row.Business, SessionState: row.SessionState,
		PostingMode: row.PostingMode, DailyCap: row.DailyCap, HasCredentials: row.EncPW != "",
		LastCheckedAt: row.LastCheckedAt, LastWakeAt: row.LastWakeAt,
		Usage: row.Usage, LastJobAt: row.LastJobAt,
		Kind: row.Kind, PlatformUserID: row.PlatformUser,
		PlanTier: row.PlanTier, Capabilities: row.Capabilities,
		ProfileMode: row.ProfileMode,
	}
}

// withChatStatus fills the AI chat link state from the registry. Channel
// accounts keep it empty (not applicable).
func (s *Service) withChatStatus(a *Account) *Account {
	if a == nil {
		return nil
	}
	if p, err := s.store.GetAIPlatform(a.Platform); err == nil {
		a.ChatStatus = p.ChatStatus
	}
	return a
}

// RegisterAccount adds an account in pending state. Channel platforms default
// to auto posting (W0); AI and unknown platforms stay manual.
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
	row, err := s.store.CreateAccountRow(&store.ChannelAccount{
		Name: name, Platform: platform, Owner: owner, Role: role, Business: business,
		SessionState: StatePending, PostingMode: s.defaultMode(platform),
	})
	if err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			return nil, ErrDuplicate
		}
		return nil, err
	}
	return toView(row), nil
}

// MaxBulkCount bounds one multi-account registration (UI_DESIGN S5).
const MaxBulkCount = 50

// defaultMode returns the W0 posting default: auto for registered channel
// platforms, manual for AI service accounts and anything unknown.
func (s *Service) defaultMode(platform string) string {
	if _, err := s.store.GetChannelPlatform(platform); err == nil {
		return PostingAuto
	}
	return PostingManual
}

var (
	ErrNoPrefix = errors.New("prefix is required")
	ErrBadCount = errors.New("count must be 1..50")
)

// RegisterAccounts creates count accounts named <prefix>01..<prefix>NN.
// Duplicates are skipped (reported in failures) so a half-done batch can
// simply be re-run; other errors abort the batch.
func (s *Service) RegisterAccounts(prefix, platform, owner, role, business string, count int) ([]*Account, []string, error) {
	if prefix == "" {
		return nil, nil, ErrNoPrefix
	}
	if count < 1 || count > MaxBulkCount {
		return nil, nil, ErrBadCount
	}
	if role == "" {
		role = RoleWorker
	}
	if role != RoleChief && role != RoleWorker {
		return nil, nil, ErrBadRole
	}
	var out []*Account
	var failures []string
	for i := 1; i <= count; i++ {
		name := fmt.Sprintf("%s%02d", prefix, i)
		row, err := s.store.CreateAccountRow(&store.ChannelAccount{
			Name: name, Platform: platform, Owner: owner, Role: role, Business: business,
			SessionState: StatePending, PostingMode: s.defaultMode(platform),
		})
		if err != nil {
			if errors.Is(err, store.ErrDuplicate) {
				failures = append(failures, name+": already exists")
				continue
			}
			return out, failures, err
		}
		out = append(out, toView(row))
	}
	return out, failures, nil
}

// SetCredentials encrypts and stores an account's login ID and password.
func (s *Service) SetCredentials(id int64, loginID, password string) (*Account, error) {
	if loginID == "" || password == "" {
		return nil, ErrCredentials
	}
	row, err := s.store.GetAccountRow(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNoAccount
		}
		return nil, err
	}
	encID, err := s.seal(loginID)
	if err != nil {
		return nil, err
	}
	encPW, err := s.seal(password)
	if err != nil {
		return nil, err
	}
	row.EncID, row.EncPW = encID, encPW
	if err := s.store.UpdateAccountRow(row); err != nil {
		return nil, err
	}
	return s.GetAccount(id)
}

// NextLogin returns the next account needing login for the S5 queue
// banner, preferring the same platform. Nil when the queue is empty.
func (s *Service) NextLogin(platform string) (*Account, error) {
	rows, err := s.store.ListAccountRows()
	if err != nil {
		return nil, err
	}
	var fallback *store.ChannelAccount
	for _, row := range rows {
		if row.SessionState != StatePending && row.SessionState != StateExpired && row.SessionState != StateLimited {
			continue
		}
		if platform != "" && row.Platform == platform {
			return toView(row), nil
		}
		if fallback == nil {
			fallback = row
		}
	}
	if fallback == nil {
		return nil, nil
	}
	return toView(fallback), nil
}

// Credentials decrypts an account's login pair for internal executor use only.
// It is never exposed through the handler layer.
func (s *Service) Credentials(id int64) (string, string, error) {
	row, err := s.store.GetAccountRow(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return "", "", ErrNoAccount
		}
		return "", "", err
	}
	loginID, err := s.open(row.EncID)
	if err != nil {
		return "", "", err
	}
	password, err := s.open(row.EncPW)
	if err != nil {
		return "", "", err
	}
	return loginID, password, nil
}

// ListAccounts returns all accounts by id.
func (s *Service) ListAccounts() []*Account {
	rows, err := s.store.ListAccountRows()
	if err != nil {
		return []*Account{}
	}
	out := make([]*Account, 0, len(rows))
	for _, row := range rows {
		out = append(out, s.withChatStatus(toView(row)))
	}
	return out
}

// GetAccount returns one account.
func (s *Service) GetAccount(id int64) (*Account, error) {
	row, err := s.store.GetAccountRow(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNoAccount
		}
		return nil, err
	}
	return s.withChatStatus(toView(row)), nil
}

// SetSessionState moves an account's session state machine with side effects:
// expired/limited/inactive_hold holds only that account's queued items
// (reservations kept, never deleted); connected resumes holds except items
// overdue by a day or more, which stay as report targets.
func (s *Service) SetSessionState(id int64, state, usage string) (*Account, error) {
	if !validStates[state] {
		return nil, ErrBadState
	}
	row, err := s.store.GetAccountRow(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNoAccount
		}
		return nil, err
	}
	prev := row.SessionState
	row.SessionState = state
	if usage != "" {
		row.Usage = usage
	}
	if err := s.store.UpdateAccountRow(row); err != nil {
		return nil, err
	}
	switch state {
	case StateExpired, StateLimited, StateInactiveHold:
		reason := fmt.Sprintf("세션 %s — 발행 정지, 예약 보류", stateLabel(state))
		if _, err := s.store.SetHeldByAccount(row.Name, reason); err != nil {
			return nil, err
		}
	case StateConnected:
		if prev == StateExpired || prev == StateLimited {
			if _, _, err := s.content.ResumeAccount(row.Name); err != nil {
				return nil, err
			}
		}
		// Leaving inactive_hold resumes nothing automatically: contact-lost
		// disposal is a PO decision. Holds stay until reviewed.
	}
	if s.OnSessionState != nil {
		s.OnSessionState(id, row.Name, prev, state)
	}
	return s.GetAccount(id)
}

// UpdateAccountFields edits owner/role/business. The name is the stable key
// that queued items reference, so it is immutable through this API.
func (s *Service) UpdateAccountFields(id int64, owner, role, business *string) (*Account, error) {
	row, err := s.store.GetAccountRow(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNoAccount
		}
		return nil, err
	}
	if owner != nil {
		row.Owner = *owner
	}
	if role != nil {
		if *role != RoleChief && *role != RoleWorker {
			return nil, ErrBadRole
		}
		row.Role = *role
	}
	if business != nil {
		row.Business = *business
	}
	if err := s.store.UpdateAccountRow(row); err != nil {
		return nil, err
	}
	return s.GetAccount(id)
}

// SetPostingMode switches an account between auto and manual posting.
func (s *Service) SetPostingMode(id int64, mode string) (*Account, error) {
	if mode != PostingAuto && mode != PostingManual {
		return nil, ErrBadMode
	}
	row, err := s.store.GetAccountRow(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNoAccount
		}
		return nil, err
	}
	row.PostingMode = mode
	if err := s.store.UpdateAccountRow(row); err != nil {
		return nil, err
	}
	return s.GetAccount(id)
}

// SetDailyCap overrides one account's daily publish cap (W0).
// 0 means "follow the global settings cap".
func (s *Service) SetDailyCap(id int64, dailyCap int) (*Account, error) {
	if dailyCap < 0 || dailyCap > 999 {
		return nil, ErrBadDailyCap
	}
	row, err := s.store.GetAccountRow(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNoAccount
		}
		return nil, err
	}
	row.DailyCap = dailyCap
	if err := s.store.UpdateAccountRow(row); err != nil {
		return nil, err
	}
	return s.GetAccount(id)
}

// SetKindFields tags an account (channel vs agent_service) with an optional
// platform user id. Nil pointers leave the field untouched.
func (s *Service) SetKindFields(id int64, kind, platformUserID *string) (*Account, error) {
	if kind != nil && *kind != "channel" && *kind != "agent_service" {
		return nil, ErrBadKind
	}
	row, err := s.store.GetAccountRow(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNoAccount
		}
		return nil, err
	}
	if kind != nil {
		row.Kind = *kind
	}
	if platformUserID != nil {
		row.PlatformUser = *platformUserID
	}
	if err := s.store.UpdateAccountRow(row); err != nil {
		return nil, err
	}
	return s.GetAccount(id)
}

// validCapabilities is the W15-9 feature vocabulary.
var validCapabilities = map[string]bool{"text": true, "image": true, "video": true, "file": true}

// normalizeCapabilities parses and canonicalizes a capabilities JSON array.
func normalizeCapabilities(raw string) (string, error) {
	var items []string
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return "", ErrBadCapabilities
	}
	seen := map[string]bool{}
	out := []string{}
	for _, it := range items {
		it = strings.ToLower(strings.TrimSpace(it))
		if !validCapabilities[it] || seen[it] {
			if !validCapabilities[it] {
				return "", ErrBadCapabilities
			}
			continue
		}
		seen[it] = true
		out = append(out, it)
	}
	if out == nil {
		out = []string{}
	}
	enc, _ := json.Marshal(out)
	return string(enc), nil
}

// SetPlanFields tags an account with its plan tier label and proven
// capabilities (W15-9). Nil pointers leave the field untouched.
func (s *Service) SetPlanFields(id int64, planTier, capabilities *string) (*Account, error) {
	row, err := s.store.GetAccountRow(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNoAccount
		}
		return nil, err
	}
	if planTier != nil {
		row.PlanTier = strings.TrimSpace(*planTier)
	}
	if capabilities != nil {
		norm, err := normalizeCapabilities(strings.TrimSpace(*capabilities))
		if err != nil {
			return nil, err
		}
		row.Capabilities = norm
	}
	if err := s.store.UpdateAccountRow(row); err != nil {
		return nil, err
	}
	return s.GetAccount(id)
}

// ProbeResult is one chat-probe outcome (W15-4).
type ProbeResult struct {
	OK       bool   `json:"ok"`
	Reason   string `json:"reason,omitempty"`
	ChatURL  string `json:"chatUrl,omitempty"`
	Composer string `json:"composerSelector,omitempty"`
	Send     string `json:"sendSelector,omitempty"`
	Reply    string `json:"replySelector,omitempty"`
}

// ChatProbe runs adapter.chat --probe for one account and stores the
// outcome: success writes selectors + chat_status=ok, anything else marks
// the platform unlinked. Proven text/file features merge into the
// account's capabilities (W15-9).
func (s *Service) ChatProbe(id int64) (*ProbeResult, error) {
	row, err := s.store.GetAccountRow(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNoAccount
		}
		return nil, err
	}
	platform := strings.TrimSpace(row.Platform)
	p, err := s.store.GetAIPlatform(platform)
	if err != nil {
		return nil, ErrBadPlatform
	}
	doc, err := s.runProbe(row.Name, platform)
	if err != nil {
		p.ChatStatus = store.AIChatUnlinked
		_ = s.store.UpdateAIPlatform(p)
		return &ProbeResult{Reason: err.Error()}, nil
	}
	res := &ProbeResult{
		OK:       doc["ok"] == true,
		Reason:   strField(doc, "reason"),
		ChatURL:  strField(doc, "chat_url"),
		Composer: strField(doc, "composer_selector"),
		Send:     strField(doc, "send_selector"),
		Reply:    strField(doc, "reply_selector"),
	}
	if !res.OK {
		p.ChatStatus = store.AIChatUnlinked
		_ = s.store.UpdateAIPlatform(p)
		return res, nil
	}
	p.ComposerSelector = res.Composer
	p.SendSelector = res.Send
	p.ReplySelector = res.Reply
	if res.ChatURL != "" {
		p.ChatURL = res.ChatURL
	}
	if pat := strField(doc, "thread_url_pattern"); pat != "" {
		p.ThreadURLPattern = pat
	}
	p.ChatStatus = store.AIChatOK
	if err := s.store.UpdateAIPlatform(p); err != nil {
		return nil, err
	}
	features := []string{"text"}
	if doc["has_file_input"] == true {
		features = append(features, "file")
	}
	row.Capabilities = mergeCapabilities(row.Capabilities, features)
	_ = s.store.UpdateAccountRow(row)
	return res, nil
}

func strField(doc map[string]any, key string) string {
	if v, ok := doc[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// mergeCapabilities unions proven features into the stored JSON array.
func mergeCapabilities(stored string, proven []string) string {
	seen := map[string]bool{}
	out := []string{}
	var cur []string
	if err := json.Unmarshal([]byte(stored), &cur); err == nil {
		for _, c := range cur {
			c = strings.ToLower(strings.TrimSpace(c))
			if c != "" && !seen[c] {
				seen[c] = true
				out = append(out, c)
			}
		}
	}
	for _, c := range proven {
		if validCapabilities[c] && !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	if out == nil {
		out = []string{}
	}
	enc, _ := json.Marshal(out)
	return string(enc)
}

// runProbe executes adapter.chat --probe. Tests replace probeRunner.
// The real subprocess lives in login.go beside the other adapter calls.
func (s *Service) runProbe(name, platform string) (map[string]any, error) {
	if s.probeRunner != nil {
		return s.probeRunner(name, platform)
	}
	return s.execProbe(name, platform)
}

// UnregisterAccount removes an account. Accounts with any queued items are
// refused so queued items never lose their owner silently; move or delete
// the items first.
func (s *Service) UnregisterAccount(id int64) error {
	row, err := s.store.GetAccountRow(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrNoAccount
		}
		return err
	}
	n, err := s.store.CountQueuedByAccount(row.Name)
	if err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("%w (%d queued)", ErrHeldItems, n)
	}
	if err := s.store.DeleteAccountRow(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrNoAccount
		}
		return err
	}
	return nil
}

// Audit records one operator action in the W20-2 audit log. The session
// handler owns no store handle, so the service exposes this narrow call.
func (s *Service) Audit(actor, action, refType string, refID, workID int64, detail string) {
	_, _ = s.store.AppendAudit(actor, action, refType, refID, workID, detail)
}

// ReconcileProfiles marks connected accounts expired when their session
// files are gone (W20-3: after a restore without sessions, or a deleted
// profiles dir). Expired accounts show "login required" and appear in
// the relogin queue automatically. It returns the moved count.
func (s *Service) ReconcileProfiles() int {
	if s.loginDataDir == "" {
		return 0
	}
	rows, err := s.store.ListAccountRows()
	if err != nil {
		return 0
	}
	moved := 0
	for _, row := range rows {
		if row.SessionState != StateConnected {
			continue
		}
		if _, err := os.Stat(SessionStatePath(s.loginDataDir, row.Name)); err == nil {
			continue
		}
		if _, err := s.SetSessionState(row.ID, StateExpired, row.Usage); err == nil {
			moved++
		}
	}
	return moved
}

// CheckNow runs the light periodic inspection: timestamps only, no wake-ups.
func (s *Service) CheckNow() int {
	rows, err := s.store.ListAccountRows()
	if err != nil {
		return 0
	}
	now := nowUTC()
	n := 0
	for _, row := range rows {
		row.LastCheckedAt = now
		if s.store.UpdateAccountRow(row) == nil {
			n++
		}
	}
	return n
}

// Wake marks one assigned session as woken. There is no wake-all: only
// sessions with assigned work are ever woken.
func (s *Service) Wake(id int64) (*Account, error) {
	row, err := s.store.GetAccountRow(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNoAccount
		}
		return nil, err
	}
	now := nowUTC()
	row.LastWakeAt, row.LastCheckedAt = now, now
	if err := s.store.UpdateAccountRow(row); err != nil {
		return nil, err
	}
	return s.GetAccount(id)
}

// Plan verdicts for queued items.
const (
	VerdictReady           = "ready"
	VerdictManual          = "manual"
	VerdictHeld            = "held"
	VerdictBlockedSession  = "blocked_session"
	VerdictUnknownAccount  = "unknown_account"
	VerdictScheduledFuture = "scheduled_future"
)

// PlanItem is one queued item's execution readiness. Read-only by design:
// it never mutates queue state.
type PlanItem struct {
	ContentID   int64  `json:"contentId"`
	Title       string `json:"title"`
	Account     string `json:"account"`
	Platform    string `json:"platform"`
	Group       string `json:"group"`
	ScheduledAt string `json:"scheduledAt"`
	Session     string `json:"session"`
	Mode        string `json:"mode"`
	Verdict     string `json:"verdict"`
	Reason      string `json:"reason"`
}

// PublishPlan evaluates every queued item for execution readiness.
func (s *Service) PublishPlan(now time.Time) []*PlanItem {
	rows, err := s.store.ListAccountRows()
	if err != nil {
		return []*PlanItem{}
	}
	byName := map[string]*store.ChannelAccount{}
	for _, row := range rows {
		byName[row.Name] = row
	}
	var items []*store.Content
	for _, group := range []string{"official_api", "manual"} {
		got, err := s.store.ListQueuedByGroup(group)
		if err != nil {
			return []*PlanItem{}
		}
		items = append(items, got...)
	}
	out := []*PlanItem{}
	for _, c := range items {
		it := &PlanItem{
			ContentID: c.ID, Title: c.Title, Account: c.TargetAccount,
			Platform: c.TargetPlatform, Group: c.ChannelGroup, ScheduledAt: c.ScheduledAt,
		}
		acc, ok := byName[c.TargetAccount]
		switch {
		case c.Held:
			it.Verdict, it.Reason = VerdictHeld, c.HoldReason
		case !ok:
			it.Verdict, it.Reason = VerdictUnknownAccount, "account is not registered"
		default:
			it.Session, it.Mode = acc.SessionState, acc.PostingMode
			switch {
			case acc.SessionState != StateConnected:
				it.Verdict, it.Reason = VerdictBlockedSession, "session is "+acc.SessionState
			case acc.PostingMode != PostingAuto:
				it.Verdict, it.Reason = VerdictManual, "manual mode: human publishes"
			case c.ScheduledAt == "":
				it.Verdict, it.Reason = VerdictReady, "unscheduled queue entry"
			default:
				t, err := time.Parse(time.RFC3339, c.ScheduledAt)
				if err != nil {
					it.Verdict, it.Reason = VerdictHeld, "bad scheduled time"
				} else if t.After(now) {
					it.Verdict, it.Reason = VerdictScheduledFuture, "not due yet"
				} else {
					it.Verdict, it.Reason = VerdictReady, "due"
				}
			}
		}
		out = append(out, it)
	}
	return out
}

// ReloginQueue gathers expired/limited accounts with their held counts.
func (s *Service) ReloginQueue() []*ReloginItem {
	rows, err := s.store.ListAccountRows()
	if err != nil {
		return []*ReloginItem{}
	}
	var out []*ReloginItem
	for _, row := range rows {
		if row.SessionState != StateExpired && row.SessionState != StateLimited {
			continue
		}
		held, _ := s.store.ListHeldByAccount(row.Name)
		out = append(out, &ReloginItem{AccountID: row.ID, AccountName: row.Name,
			Owner: row.Owner, SessionState: row.SessionState, HeldCount: len(held)})
	}
	return out
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
