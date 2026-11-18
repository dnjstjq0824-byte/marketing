package store

import (
	"path/filepath"
	"testing"
)

func openW22Test(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "w22.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := Migrate(st.DB()); err != nil {
		t.Fatal(err)
	}
	return st
}

// W22: v29 adds executor columns and seeds the backend registry.
func TestMigrateV29BackendColumns(t *testing.T) {
	st := openW22Test(t)
	for _, col := range []string{"executor_backend", "relay_account"} {
		var n int
		if err := st.DB().QueryRow(
			`SELECT COUNT(*) FROM pragma_table_info('channel_accounts') WHERE name=?`, col).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("missing column %s", col)
		}
	}
	got, err := st.ListPublishBackends()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "direct" || got[1].Name != "muse" {
		t.Fatalf("seeds = %+v", got)
	}
	if !got[0].Ready || !got[1].Ready || got[1].AIPlatform != "muse" {
		t.Fatalf("seeds = %+v", got)
	}
	if v, err := CurrentSchemaVersion(st.DB()); err != nil || v != "33" {
		t.Fatalf("schema_version = %q err = %v", v, err)
	}
}

// W22: re-migrating keeps seeds single and data intact.
func TestMigrateV29Idempotent(t *testing.T) {
	st := openW22Test(t)
	a, err := st.CreateAccountRow(&ChannelAccount{Name: "nb1", Platform: "naver_blog"})
	if err != nil {
		t.Fatal(err)
	}
	if a.ExecutorBackend != "direct" || a.RelayAccount != "" {
		t.Fatalf("defaults = %+v", a)
	}
	if err := Migrate(st.DB()); err != nil {
		t.Fatal(err)
	}
	backs, err := st.ListPublishBackends()
	if err != nil || len(backs) != 2 {
		t.Fatalf("backs = %+v err = %v", backs, err)
	}
	kept, err := st.GetAccountRow(a.ID)
	if err != nil || kept.Name != "nb1" || kept.ExecutorBackend != "direct" {
		t.Fatalf("kept = %+v err = %v", kept, err)
	}
}

// W22: executor selection round-trips through create/update/get.
func TestAccountBackendRoundTrip(t *testing.T) {
	st := openW22Test(t)
	a, err := st.CreateAccountRow(&ChannelAccount{Name: "nb1", Platform: "naver_blog"})
	if err != nil {
		t.Fatal(err)
	}
	a.ExecutorBackend, a.RelayAccount = "muse", "cap1"
	if err := st.UpdateAccountRow(a); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetAccountRowByName("nb1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ExecutorBackend != "muse" || got.RelayAccount != "cap1" {
		t.Fatalf("got = %+v", got)
	}
	rows, err := st.ListAccountRows()
	if err != nil || len(rows) != 1 || rows[0].RelayAccount != "cap1" {
		t.Fatalf("rows = %+v err = %v", rows, err)
	}
}

// W22: registry get/flip/unknown paths.
func TestPublishBackendRegistry(t *testing.T) {
	st := openW22Test(t)
	be, err := st.GetPublishBackend("muse")
	if err != nil || !be.Ready || be.AIPlatform != "muse" {
		t.Fatalf("be = %+v err = %v", be, err)
	}
	if _, err := st.GetPublishBackend("nope"); err != ErrNotFound {
		t.Fatalf("unknown err = %v", err)
	}
	if err := st.SetPublishBackendReady("muse", false); err != nil {
		t.Fatal(err)
	}
	be, _ = st.GetPublishBackend("muse")
	if be.Ready {
		t.Fatal("still ready")
	}
	if err := st.SetPublishBackendReady("muse", true); err != nil {
		t.Fatal(err)
	}
	if err := st.SetPublishBackendReady("nope", true); err != ErrNotFound {
		t.Fatalf("unknown flip err = %v", err)
	}
}
