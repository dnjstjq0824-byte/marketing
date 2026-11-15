package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// AI chat link statuses (W15-4): ok means the probe (or a proven builtin
// seed) confirmed the selectors; unlinked keeps the account logged in but
// out of automatic work and console chat.
const (
	AIChatOK       = "ok"
	AIChatUnlinked = "unlinked"
)

// AIPlatform is one row of the ai_platforms registry (W15-1). Builtin rows
// mirror adapter/platforms.py; custom rows (builtin=0) are served to the
// adapter as JSON.
type AIPlatform struct {
	Key               string `json:"key"`
	Name              string `json:"name"`
	LoginURL          string `json:"loginUrl"`
	ChatURL           string `json:"chatUrl"`
	LoggedOutMarkers  string `json:"loggedOutMarkers"`
	ComposerSelector  string `json:"composerSelector"`
	SendSelector      string `json:"sendSelector"`
	ReplySelector     string `json:"replySelector"`
	ReplyBodySelector string `json:"replyBodySelector"`
	BusySelector      string `json:"busySelector"`
	DoneSelector      string `json:"doneSelector"`
	ThreadURLPattern  string `json:"threadUrlPattern"`
	Icon              string `json:"icon"`
	Color             string `json:"color"`
	Builtin           bool   `json:"builtin"`
	ChatStatus        string `json:"chatStatus"`
}

const aiPlatformCols = `key,name,login_url,chat_url,logged_out_markers,composer_selector,send_selector,` +
	`reply_selector,reply_body_selector,busy_selector,done_selector,thread_url_pattern,icon,color,builtin,chat_status`

func scanAIPlatform(row interface{ Scan(...any) error }) (*AIPlatform, error) {
	var p AIPlatform
	var builtin int
	err := row.Scan(&p.Key, &p.Name, &p.LoginURL, &p.ChatURL, &p.LoggedOutMarkers,
		&p.ComposerSelector, &p.SendSelector, &p.ReplySelector, &p.ReplyBodySelector,
		&p.BusySelector, &p.DoneSelector, &p.ThreadURLPattern, &p.Icon, &p.Color,
		&builtin, &p.ChatStatus)
	if err != nil {
		return nil, err
	}
	p.Builtin = builtin != 0
	return &p, nil
}

// ListAIPlatforms returns the registry, builtin first, then key order.
func (s *Store) ListAIPlatforms() ([]*AIPlatform, error) {
	rows, err := s.ro.Query(`SELECT ` + aiPlatformCols + ` FROM ai_platforms ORDER BY builtin DESC, key ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*AIPlatform
	for rows.Next() {
		p, err := scanAIPlatform(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetAIPlatform fetches one registry row by key.
func (s *Store) GetAIPlatform(key string) (*AIPlatform, error) {
	p, err := scanAIPlatform(s.ro.QueryRow(`SELECT `+aiPlatformCols+` FROM ai_platforms WHERE key=?`, key))
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

// CreateCustomAIPlatform registers a user-defined AI (W15-3). Keys are
// lowercase slugs; login URLs must be https (W15-7).
func (s *Store) CreateCustomAIPlatform(key, name, loginURL string) (*AIPlatform, error) {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" || strings.ContainsAny(key, " \t/\\") {
		return nil, fmt.Errorf("bad platform key %q", key)
	}
	loginURL = strings.TrimSpace(loginURL)
	if !strings.HasPrefix(loginURL, "https://") {
		return nil, fmt.Errorf("login URL must be https")
	}
	now := nowUTC()
	_, err := s.db.Exec(`INSERT INTO ai_platforms(key,name,login_url,chat_url,logged_out_markers,
		composer_selector,send_selector,reply_selector,reply_body_selector,busy_selector,done_selector,
		thread_url_pattern,icon,color,builtin,chat_status,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		key, strings.TrimSpace(name), loginURL, loginURL, `[]`, "", "", "", "", "", "", "", "", "", 0,
		AIChatUnlinked, now, now)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "PRIMARY") {
			return nil, ErrDuplicate
		}
		return nil, err
	}
	return s.GetAIPlatform(key)
}

// UpdateAIPlatform writes selectors, URLs, markers, and chat status. Login
// URL stays https-only; chat status accepts ok/unlinked only.
func (s *Store) UpdateAIPlatform(p *AIPlatform) error {
	if p.LoginURL != "" && !strings.HasPrefix(strings.TrimSpace(p.LoginURL), "https://") {
		return fmt.Errorf("login URL must be https")
	}
	if p.ChatStatus != AIChatOK && p.ChatStatus != AIChatUnlinked {
		return fmt.Errorf("bad chat status %q", p.ChatStatus)
	}
	res, err := s.db.Exec(`UPDATE ai_platforms SET name=?,login_url=?,chat_url=?,logged_out_markers=?,
		composer_selector=?,send_selector=?,reply_selector=?,reply_body_selector=?,busy_selector=?,done_selector=?,
		thread_url_pattern=?,icon=?,color=?,chat_status=?,updated_at=? WHERE key=?`,
		p.Name, p.LoginURL, p.ChatURL, p.LoggedOutMarkers, p.ComposerSelector, p.SendSelector,
		p.ReplySelector, p.ReplyBodySelector, p.BusySelector, p.DoneSelector, p.ThreadURLPattern,
		p.Icon, p.Color, p.ChatStatus, nowUTC(), p.Key)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// AIPlatformJSON materializes a custom AI definition as a temp JSON file
// for the adapter (W15-2, via AI_PLATFORM_JSON). Builtin and unknown
// platforms return "" (the adapter's code registry). Callers remove it.
func (s *Store) AIPlatformJSON(platform string) (string, error) {
	p, err := s.GetAIPlatform(platform)
	if err != nil || p.Builtin {
		return "", nil
	}
	var markers []string
	if err := json.Unmarshal([]byte(p.LoggedOutMarkers), &markers); err != nil {
		markers = []string{}
	}
	doc := map[string]any{
		"name": p.Key, "login_url": p.LoginURL, "chat_url": p.ChatURL,
		"logged_out_markers": markers, "composer_selector": p.ComposerSelector,
		"send_selector": p.SendSelector, "reply_selector": p.ReplySelector,
		"reply_body_selector": p.ReplyBodySelector, "busy_selector": p.BusySelector,
		"done_selector": p.DoneSelector, "thread_url_pattern": p.ThreadURLPattern,
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp("", "somun-aiplat-*.json")
	if err != nil {
		return "", err
	}
	if _, err := f.Write(raw); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	f.Close()
	return f.Name(), nil
}

// DeleteCustomAIPlatform removes a user-defined AI. Builtin rows and rows
// with linked accounts are refused.
func (s *Store) DeleteCustomAIPlatform(key string) error {
	p, err := s.GetAIPlatform(key)
	if err != nil {
		return err
	}
	if p.Builtin {
		return fmt.Errorf("builtin platform %q cannot be deleted", key)
	}
	var n int
	if err := s.ro.QueryRow(`SELECT COUNT(*) FROM channel_accounts WHERE platform=?`, key).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("platform %q still has %d accounts", key, n)
	}
	_, err = s.db.Exec(`DELETE FROM ai_platforms WHERE key=?`, key)
	return err
}
