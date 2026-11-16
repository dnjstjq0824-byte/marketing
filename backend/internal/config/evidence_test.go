package config

import (
	"os"
	"path/filepath"
	"testing"
)

// W9-2: the legacy uploads/evidence folder moves to data/evidence once;
// missing source or existing target is a no-op, and nothing overwrites.
func TestMigrateEvidenceDir(t *testing.T) {
	mkshot := func(t *testing.T, dir string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(dir, "uploads", "evidence"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "uploads", "evidence", "s.png"), []byte("shot"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	moved := t.TempDir()
	mkshot(t, moved)
	MigrateEvidenceDir(moved)
	if _, err := os.Stat(filepath.Join(moved, "evidence", "s.png")); err != nil {
		t.Fatalf("moved shot: %v", err)
	}
	if _, err := os.Stat(filepath.Join(moved, "uploads", "evidence")); !os.IsNotExist(err) {
		t.Fatal("legacy folder should be gone after the move")
	}
	// Idempotent: a second boot changes nothing.
	MigrateEvidenceDir(moved)
	if _, err := os.Stat(filepath.Join(moved, "evidence", "s.png")); err != nil {
		t.Fatalf("second run: %v", err)
	}

	fresh := t.TempDir()
	MigrateEvidenceDir(fresh) // no source: no-op, no error
	if _, err := os.Stat(filepath.Join(fresh, "evidence")); !os.IsNotExist(err) {
		t.Fatal("fresh dir must not gain an evidence folder")
	}

	both := t.TempDir()
	mkshot(t, both)
	if err := os.MkdirAll(filepath.Join(both, "evidence"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(both, "evidence", "n.png"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	MigrateEvidenceDir(both) // both present: operator merges, nothing lost
	if _, err := os.Stat(filepath.Join(both, "evidence", "n.png")); err != nil {
		t.Fatalf("target kept: %v", err)
	}
	if _, err := os.Stat(filepath.Join(both, "uploads", "evidence", "s.png")); err != nil {
		t.Fatalf("source kept: %v", err)
	}
}
