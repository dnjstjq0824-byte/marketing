package notify

import (
	"path/filepath"
	"testing"
	"time"

	"somun/internal/store"
)

func openNotifyStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "n.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := store.Migrate(st.DB()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return st
}

func TestNotifyDedupsRepeat(t *testing.T) {
	st := openNotifyStore(t)
	svc := NewService(st)
	made, err := svc.JobFailed(3, "generate", "boom")
	if err != nil || !made {
		t.Fatalf("first = %v, %v; want true, nil", made, err)
	}
	made, err = svc.JobFailed(3, "generate", "boom again")
	if err != nil || made {
		t.Fatalf("repeat = %v, %v; want false, nil", made, err)
	}
	items, unread, err := svc.List(true, 10, 0)
	if err != nil || unread != 1 || len(items) != 1 {
		t.Fatalf("list = %d items unread %d, %v; want 1/1", len(items), unread, err)
	}
	if items[0].Severity != "error" || items[0].RefType != "job" || items[0].RefID != 3 {
		t.Fatalf("row = %+v", items[0])
	}
}

func TestSweepStuckDedups(t *testing.T) {
	st := openNotifyStore(t)
	p, err := st.CreateProduct("https://example.com/s", "A", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	j, err := st.CreateJob("generate", p.ID, "{}")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimNextJob("generate", "w"); err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().Add(-40 * time.Minute).Format(time.RFC3339)
	if _, err := st.DB().Exec(`UPDATE jobs SET claimed_at=? WHERE id=?`, old, j.ID); err != nil {
		t.Fatal(err)
	}
	svc := NewService(st)
	stuck, _, err := svc.Sweep()
	if err != nil || stuck != 1 {
		t.Fatalf("first sweep stuck = %d, %v; want 1", stuck, err)
	}
	stuck, _, err = svc.Sweep()
	if err != nil || stuck != 0 {
		t.Fatalf("second sweep stuck = %d, %v; want 0 (dedup)", stuck, err)
	}
	_, unread, err := svc.List(true, 10, 0)
	if err != nil || unread != 1 {
		t.Fatalf("unread = %d, %v; want 1", unread, err)
	}
}

func TestSessionStateOnlyFiresOnBadMoves(t *testing.T) {
	st := openNotifyStore(t)
	svc := NewService(st)
	made, err := svc.SessionState(9, "acc", "connected", "expired")
	if err != nil || !made {
		t.Fatalf("expired = %v, %v; want true, nil", made, err)
	}
	made, err = svc.SessionState(9, "acc", "connected", "connected")
	if err != nil || made {
		t.Fatalf("same-state = %v, %v; want false, nil", made, err)
	}
	_, unread, err := svc.List(true, 10, 0)
	if err != nil || unread != 1 {
		t.Fatalf("unread = %d, %v; want 1", unread, err)
	}
}

func TestReadAndReadAll(t *testing.T) {
	st := openNotifyStore(t)
	svc := NewService(st)
	if _, err := svc.BackupFailed(errTestBackup); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ReviewHuman(11, "needs a person"); err != nil {
		t.Fatal(err)
	}
	ok, err := svc.Read(1)
	if err != nil || !ok {
		t.Fatalf("read 1 = %v, %v; want true, nil", ok, err)
	}
	updated, err := svc.ReadAll()
	if err != nil || updated != 1 {
		t.Fatalf("read-all = %d, %v; want 1", updated, err)
	}
	_, unread, err := svc.List(true, 10, 0)
	if err != nil || unread != 0 {
		t.Fatalf("unread = %d, %v; want 0", unread, err)
	}
}

type testBackupError struct{}

func (testBackupError) Error() string { return "disk gone" }

var errTestBackup error = testBackupError{}
