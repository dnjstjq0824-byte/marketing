package store

import (
	"path/filepath"
	"testing"
)

// W0: stopped automation claims nothing; re-enabling resumes.
func TestClaimRespectsAutomationSwitch(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "gate.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := Migrate(st.DB()); err != nil {
		t.Fatal(err)
	}
	p, err := st.CreateProduct("https://example.com/g", "A", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateJob("analyze", p.ID, "{}"); err != nil {
		t.Fatal(err)
	}
	if _, err := SetAutomationEnabled(st.DB(), false); err != nil {
		t.Fatal(err)
	}
	got, err := st.ClaimNextJob("analyze", "w")
	if err != nil || got != nil {
		t.Fatalf("claim while off = %+v err = %v", got, err)
	}
	if _, err := SetAutomationEnabled(st.DB(), true); err != nil {
		t.Fatal(err)
	}
	got, err = st.ClaimNextJob("analyze", "w")
	if err != nil || got == nil {
		t.Fatalf("claim after on = %+v err = %v", got, err)
	}
}
