package store

import (
	"path/filepath"
	"testing"
)

func openSettingsTest(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := Migrate(st.DB()); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestAutomationSettingsDefaults(t *testing.T) {
	st := openSettingsTest(t)
	got, err := GetAutomationSettings(st.DB())
	if err != nil {
		t.Fatal(err)
	}
	want := DefaultAutomationSettings()
	if got != want {
		t.Fatalf("defaults = %+v, want %+v", got, want)
	}
}

func TestAutomationSettingsRoundTrip(t *testing.T) {
	st := openSettingsTest(t)
	want := DefaultAutomationSettings()
	want.Enabled = false
	want.PublishIntervalMin = 45
	want.DailyCap = 5
	want.WindowStart = "08:30"
	want.WindowEnd = "23:00"
	want.BlogMinLen = 800
	want.Outputs = AutomationOutputs{Text: true, Image: false, Video: true}
	want.MediaRetentionDays = 7
	want.BackupIncludeVideo = true
	if err := SetAutomationSettings(st.DB(), want); err != nil {
		t.Fatal(err)
	}
	got, err := GetAutomationSettings(st.DB())
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("round trip = %+v, want %+v", got, want)
	}
}

func TestAutomationSettingsValidation(t *testing.T) {
	st := openSettingsTest(t)
	base := DefaultAutomationSettings()
	cases := []struct {
		name   string
		mutate func(*AutomationSettings)
	}{
		{"interval-negative", func(s *AutomationSettings) { s.PublishIntervalMin = -1 }},
		{"interval-too-big", func(s *AutomationSettings) { s.PublishIntervalMin = 1441 }},
		{"cap-negative", func(s *AutomationSettings) { s.DailyCap = -1 }},
		{"window-bad-format", func(s *AutomationSettings) { s.WindowStart = "9:00" }},
		{"window-equal", func(s *AutomationSettings) { s.WindowStart = s.WindowEnd }},
		{"blog-negative", func(s *AutomationSettings) { s.BlogMinLen = -1 }},
		{"retention-zero", func(s *AutomationSettings) { s.MediaRetentionDays = 0 }},
		{"concurrent-zero", func(s *AutomationSettings) { s.MaxConcurrent = 0 }},
		{"concurrent-too-big", func(s *AutomationSettings) { s.MaxConcurrent = 51 }},
	}
	for _, tc := range cases {
		s := base
		tc.mutate(&s)
		if err := SetAutomationSettings(st.DB(), s); err == nil {
			t.Fatalf("%s: expected validation error", tc.name)
		}
	}
	// Overnight windows (start later than end) are allowed.
	overnight := base
	overnight.WindowStart, overnight.WindowEnd = "22:00", "06:00"
	if err := SetAutomationSettings(st.DB(), overnight); err != nil {
		t.Fatalf("overnight window: %v", err)
	}
}

func TestAutomationSettingsCorruptFallsBack(t *testing.T) {
	st := openSettingsTest(t)
	if _, err := st.DB().Exec(`INSERT INTO app_meta(k,v) VALUES(?,?)`,
		KeyAutomationSettings, "{not json"); err != nil {
		t.Fatal(err)
	}
	got, err := GetAutomationSettings(st.DB())
	if err != nil {
		t.Fatal(err)
	}
	if got != DefaultAutomationSettings() {
		t.Fatalf("corrupt value = %+v, want defaults", got)
	}
}

func TestSetAutomationEnabledKeepsRest(t *testing.T) {
	st := openSettingsTest(t)
	custom := DefaultAutomationSettings()
	custom.DailyCap = 9
	custom.BlogMinLen = 500
	if err := SetAutomationSettings(st.DB(), custom); err != nil {
		t.Fatal(err)
	}
	got, err := SetAutomationEnabled(st.DB(), false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Enabled || got.DailyCap != 9 || got.BlogMinLen != 500 {
		t.Fatalf("flip = %+v, want disabled with fields kept", got)
	}
}
