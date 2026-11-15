// S6 automation settings (UI_DESIGN S6), stored as one JSON value in
// app_meta. W8 persists and serves these; enforcement (claim gating,
// daily caps, publish windows) is W0/W4 territory.
package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
)

// KeyAutomationSettings is the app_meta key holding AutomationSettings JSON.
const KeyAutomationSettings = "settings.automation"

// AutomationOutputs mirrors the S6 "만들 것 기본값" checkboxes.
type AutomationOutputs struct {
	Text  bool `json:"text"`
	Image bool `json:"image"`
	Video bool `json:"video"`
}

// AutomationSettings is the whole S6 form state.
type AutomationSettings struct {
	Enabled            bool              `json:"enabled"`
	PublishIntervalMin int               `json:"publishIntervalMin"`
	DailyCap           int               `json:"dailyCap"`
	WindowStart        string            `json:"windowStart"` // "HH:MM"
	WindowEnd          string            `json:"windowEnd"`   // "HH:MM"
	BlogMinLen         int               `json:"blogMinLen"`
	Outputs            AutomationOutputs `json:"outputs"`
	MediaRetentionDays int               `json:"mediaRetentionDays"`
	BackupIncludeVideo bool              `json:"backupIncludeVideo"`
	MaxConcurrent      int               `json:"maxConcurrent"`
}

// DefaultAutomationSettings returns the S6 wireframe defaults.
// Enabled defaults to true (W0: 실행기 기본 가동).
func DefaultAutomationSettings() AutomationSettings {
	return AutomationSettings{
		Enabled:            true,
		PublishIntervalMin: 120,
		DailyCap:           3,
		WindowStart:        "09:00",
		WindowEnd:          "22:00",
		BlogMinLen:         1500,
		Outputs:            AutomationOutputs{Text: true, Image: true, Video: true},
		MediaRetentionDays: 30,
		MaxConcurrent:      3,
	}
}

var hhmm = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

// Validate reports whether the settings are storable.
func (s AutomationSettings) Validate() error {
	if s.PublishIntervalMin < 0 || s.PublishIntervalMin > 1440 {
		return errors.New("publishIntervalMin must be 0..1440")
	}
	if s.DailyCap < 0 || s.DailyCap > 999 {
		return errors.New("dailyCap must be 0..999")
	}
	if !hhmm.MatchString(s.WindowStart) || !hhmm.MatchString(s.WindowEnd) {
		return errors.New("windowStart/windowEnd must be HH:MM")
	}
	if s.WindowStart == s.WindowEnd {
		return errors.New("windowStart and windowEnd must differ")
	}
	if s.BlogMinLen < 0 || s.BlogMinLen > 20000 {
		return errors.New("blogMinLen must be 0..20000")
	}
	if s.MediaRetentionDays < 1 || s.MediaRetentionDays > 365 {
		return errors.New("mediaRetentionDays must be 1..365")
	}
	if s.MaxConcurrent < 1 || s.MaxConcurrent > 50 {
		return errors.New("maxConcurrent must be 1..50")
	}
	return nil
}

// GetAutomationSettings reads the settings, returning defaults when the key
// is missing or unreadable. A corrupt value never breaks the settings screen.
func GetAutomationSettings(db *sql.DB) (AutomationSettings, error) {
	def := DefaultAutomationSettings()
	var raw string
	err := db.QueryRow(`SELECT v FROM app_meta WHERE k=?`, KeyAutomationSettings).Scan(&raw)
	if err == sql.ErrNoRows {
		return def, nil
	}
	if err != nil {
		return def, err
	}
	var s AutomationSettings
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return def, nil
	}
	// Pre-W16 rows lack the field: zero means "never set", not "uncapped".
	if s.MaxConcurrent <= 0 {
		s.MaxConcurrent = def.MaxConcurrent
	}
	if err := s.Validate(); err != nil {
		return def, nil
	}
	return s, nil
}

// SetAutomationSettings validates and persists the settings.
func SetAutomationSettings(db *sql.DB, s AutomationSettings) error {
	if err := s.Validate(); err != nil {
		return err
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO app_meta(k,v) VALUES(?,?)
		ON CONFLICT(k) DO UPDATE SET v=excluded.v`, KeyAutomationSettings, string(raw))
	return err
}

// AutomationEnabled reports the master switch for claim/executor gating (W0).
// Missing settings mean enabled (fresh DB runs). Read errors fail open so a
// transient app_meta hiccup cannot brick every worker; the switch itself is
// the only OFF signal.
func (s *Store) AutomationEnabled() bool {
	cfg, err := GetAutomationSettings(s.db)
	if err != nil {
		return true
	}
	return cfg.Enabled
}

// SetAutomationEnabled flips only the master switch, keeping every other
// field. It backs PUT /api/automation (the shell chip).
func SetAutomationEnabled(db *sql.DB, enabled bool) (AutomationSettings, error) {
	s, err := GetAutomationSettings(db)
	if err != nil {
		return s, fmt.Errorf("read settings: %w", err)
	}
	s.Enabled = enabled
	if err := SetAutomationSettings(db, s); err != nil {
		return s, err
	}
	return s, nil
}
