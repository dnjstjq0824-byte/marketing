package store

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func openP03Store(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "p03.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := Migrate(st.DB()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return st
}

func TestMigrateV6Notifications(t *testing.T) {
	st := openP03Store(t)
	var name string
	if err := st.DB().QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='notifications'`).Scan(&name); err != nil {
		t.Fatalf("notifications table missing: %v", err)
	}
	var idx string
	if err := st.DB().QueryRow(`SELECT name FROM sqlite_master WHERE type='index' AND name='idx_notifications_dedup_unread'`).Scan(&idx); err != nil {
		t.Fatalf("dedup index missing: %v", err)
	}
	if _, err := st.DB().Exec(`INSERT INTO notifications(kind,severity,ref_type,ref_id,dedup_key,message,created_at,read_at)
		VALUES('nope','warn','job',1,'k','m','2026-01-01T00:00:00Z','')`); err == nil {
		t.Fatal("bad kind accepted, want CHECK rejection")
	}
	v, err := CurrentSchemaVersion(st.DB())
	if err != nil || v != SchemaVersion {
		t.Fatalf("schema version = %q, %v; want %s", v, err, SchemaVersion)
	}
}

func TestNotificationDedup(t *testing.T) {
	st := openP03Store(t)
	made, err := st.CreateNotificationDedup("job_failed", "error", "job", 7, "job_failed:job:7", "boom")
	if err != nil || !made {
		t.Fatalf("first create = %v, %v; want true, nil", made, err)
	}
	made, err = st.CreateNotificationDedup("job_failed", "error", "job", 7, "job_failed:job:7", "boom again")
	if err != nil || made {
		t.Fatalf("repeat create = %v, %v; want false, nil", made, err)
	}
	n, err := st.CountUnreadNotifications()
	if err != nil || n != 1 {
		t.Fatalf("unread = %d, %v; want 1", n, err)
	}
	ok, err := st.MarkNotificationRead(1)
	if err != nil || !ok {
		t.Fatalf("mark read = %v, %v; want true, nil", ok, err)
	}
	made, err = st.CreateNotificationDedup("job_failed", "error", "job", 7, "job_failed:job:7", "after read")
	if err != nil || !made {
		t.Fatalf("create after read = %v, %v; want true, nil", made, err)
	}
	ok, err = st.MarkNotificationRead(9999)
	if err != nil || ok {
		t.Fatalf("mark unknown = %v, %v; want false, nil", ok, err)
	}
}

func TestNotificationListReadAll(t *testing.T) {
	st := openP03Store(t)
	for i := 0; i < 3; i++ {
		key := "job_failed:job:" + string(rune('a'+i))
		if _, err := st.CreateNotificationDedup("job_failed", "error", "job", int64(i), key, "m"); err != nil {
			t.Fatal(err)
		}
	}
	items, err := st.ListNotifications(true, 2, 0)
	if err != nil || len(items) != 2 {
		t.Fatalf("paged unread = %d, %v; want 2", len(items), err)
	}
	if items[0].ID < items[1].ID {
		t.Fatal("list is not newest-first")
	}
	updated, err := st.MarkAllNotificationsRead()
	if err != nil || updated != 3 {
		t.Fatalf("read-all updated = %d, %v; want 3", updated, err)
	}
	n, _ := st.CountUnreadNotifications()
	if n != 0 {
		t.Fatalf("unread after read-all = %d, want 0", n)
	}
	all, err := st.ListNotifications(false, 0, 0)
	if err != nil || len(all) != 3 {
		t.Fatalf("full list = %d, %v; want 3", len(all), err)
	}
	empty, err := st.ListNotifications(true, 10, 0)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty unread list must be [], got %+v, %v", empty, err)
	}
}

func TestNotificationPurgeKeepsUnread(t *testing.T) {
	st := openP03Store(t)
	old := time.Now().UTC().AddDate(0, 0, -100).Format(time.RFC3339)
	if _, err := st.db.Exec(`INSERT INTO notifications(kind,severity,ref_type,ref_id,dedup_key,message,created_at,read_at)
		VALUES('job_failed','error','job',1,'a','m',?,?)`, old, old); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`INSERT INTO notifications(kind,severity,ref_type,ref_id,dedup_key,message,created_at,read_at)
		VALUES('job_failed','error','job',2,'b','m',?,?)`, old, ""); err != nil {
		t.Fatal(err)
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -90).Format(time.RFC3339)
	n, err := st.DeleteReadNotificationsBefore(cutoff)
	if err != nil || n != 1 {
		t.Fatalf("purged = %d, %v; want 1", n, err)
	}
	var left int
	if err := st.db.QueryRow(`SELECT COUNT(*) FROM notifications`).Scan(&left); err != nil || left != 1 {
		t.Fatalf("remaining = %d, %v; want 1 (unread)", left, err)
	}
}

func TestOverdueHolds(t *testing.T) {
	st := openP03Store(t)
	p, err := st.CreateProduct("https://example.com/x", "A", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	mk := func(held bool, ageDays int) int64 {
		c, err := st.CreateContent(&Content{ProductID: p.ID, Channel: "blog", Title: "t", Body: "b"})
		if err != nil {
			t.Fatal(err)
		}
		ts := time.Now().UTC().AddDate(0, 0, -ageDays).Format(time.RFC3339)
		hv := 0
		if held {
			hv = 1
		}
		if _, err := st.db.Exec(`UPDATE contents SET status='queued', held=?, updated_at=? WHERE id=?`, hv, ts, c.ID); err != nil {
			t.Fatal(err)
		}
		return c.ID
	}
	oldHeld := mk(true, 2)
	mk(true, 0)
	mk(false, 2)
	cutoff := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	got, err := st.ListOverdueHolds(cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != oldHeld {
		t.Fatalf("overdue = %+v, want only %d", got, oldHeld)
	}
}

func seedBackupData(t *testing.T, st *Store, tag string) {
	t.Helper()
	p, err := st.CreateProduct("https://example.com/"+tag, "A", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateJob("analyze", p.ID, "{}"); err != nil {
		t.Fatal(err)
	}
}

func openSnapshot(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestBackupNowVacuumIntegrity(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "b.db")
	st, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := Migrate(st.DB()); err != nil {
		t.Fatal(err)
	}
	seedBackupData(t, st, "one")
	uploads := filepath.Join(dir, "uploads")
	if err := os.MkdirAll(uploads, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(uploads, "a.png"), []byte("img"), 0o644); err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(dir, ".vault.key")
	if err := os.WriteFile(keyPath, make([]byte, 32), 0o600); err != nil {
		t.Fatal(err)
	}
	backups := filepath.Join(dir, "backups")
	// Write load during the snapshot: backup must still be consistent.
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 30; i++ {
			select {
			case <-stop:
				return
			default:
			}
			_, _ = st.CreateProduct("https://example.com/load", "A", 0, "")
			time.Sleep(5 * time.Millisecond)
		}
	}()
	dest, err := BackupNow(st.DB(), dbPath, uploads, backups, keyPath, 7, "")
	close(stop)
	<-done
	if err != nil {
		t.Fatalf("backup: %v", err)
	}
	snap := openSnapshot(t, filepath.Join(dest, "b.db"))
	var verdict string
	if err := snap.QueryRow(`PRAGMA integrity_check`).Scan(&verdict); err != nil || verdict != "ok" {
		t.Fatalf("snapshot integrity = %q, %v", verdict, err)
	}
	var products, jobs int
	if err := snap.QueryRow(`SELECT COUNT(*) FROM products`).Scan(&products); err != nil || products < 1 {
		t.Fatalf("snapshot products = %d, %v", products, err)
	}
	if err := snap.QueryRow(`SELECT COUNT(*) FROM jobs`).Scan(&jobs); err != nil || jobs < 1 {
		t.Fatalf("snapshot jobs = %d, %v", jobs, err)
	}
	if _, err := os.Stat(filepath.Join(dest, "uploads", "a.png")); err != nil {
		t.Fatalf("uploads not bundled: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, ".vault.key")); err != nil {
		t.Fatalf("vault key not bundled: %v", err)
	}
}

func TestBackupNowSuffixAndMirror(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "b.db")
	st, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := Migrate(st.DB()); err != nil {
		t.Fatal(err)
	}
	uploads := filepath.Join(dir, "uploads")
	backups := filepath.Join(dir, "backups")
	mirror := filepath.Join(dir, "mirror")
	if err := os.MkdirAll(backups, 0o755); err != nil {
		t.Fatal(err)
	}
	// Occupy the next few seconds so the collision (and -2 suffix) is
	// deterministic regardless of second boundaries.
	base := time.Now()
	for i := 0; i < 4; i++ {
		name := base.Add(time.Duration(i) * time.Second).Format("20060102-150405")
		if err := os.MkdirAll(filepath.Join(backups, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	dest, err := BackupNow(st.DB(), dbPath, uploads, backups, "", 7, mirror)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(dest, "-2") {
		t.Fatalf("colliding generation = %q, want -2 suffix", dest)
	}
	if _, err := os.Stat(filepath.Join(mirror, filepath.Base(dest), "b.db")); err != nil {
		t.Fatalf("mirror lacks latest snapshot: %v", err)
	}
}

func TestRestoreRoundtrip(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dataDir, "r.db")
	st, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := Migrate(st.DB()); err != nil {
		t.Fatal(err)
	}
	seedBackupData(t, st, "kept")
	uploads := filepath.Join(dataDir, "uploads")
	if err := os.MkdirAll(uploads, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(uploads, "k.png"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	backups := filepath.Join(dataDir, "backups")
	dest, err := BackupNow(st.DB(), dbPath, uploads, backups, "", 7, "")
	if err != nil {
		t.Fatal(err)
	}
	gen := filepath.Base(dest)
	// Live data moves on after the backup.
	seedBackupData(t, st, "live-only")
	st.Close()
	neverRunning := func() error { return nil }
	if err := RestoreGeneration(dbPath, dataDir, backups, gen, neverRunning); err != nil {
		t.Fatalf("restore: %v", err)
	}
	restored, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { restored.Close() })
	var n int
	if err := restored.DB().QueryRow(`SELECT COUNT(*) FROM products`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("restored products = %d, %v; want snapshot state (1)", n, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	parked := 0
	for _, e := range entries {
		if strings.Contains(e.Name(), "pre-restore-") {
			parked++
		}
	}
	if parked != 1 {
		t.Fatalf("parked siblings = %d, want 1", parked)
	}
}

func TestRestoreRefusesRunningAndCorrupt(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dataDir, "r.db")
	st, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := Migrate(st.DB()); err != nil {
		t.Fatal(err)
	}
	seedBackupData(t, st, "kept")
	backups := filepath.Join(dataDir, "backups")
	dest, err := BackupNow(st.DB(), dbPath, filepath.Join(dataDir, "uploads"), backups, "", 7, "")
	if err != nil {
		t.Fatal(err)
	}
	gen := filepath.Base(dest)
	st.Close()
	running := func() error { return errors.New("server running") }
	if err := RestoreGeneration(dbPath, dataDir, backups, gen, running); err == nil {
		t.Fatal("restore ran while the server was up")
	}
	// Corrupt the generation snapshot; live data must survive the rollback.
	if err := os.WriteFile(filepath.Join(dest, "r.db"), []byte("not a database"), 0o644); err != nil {
		t.Fatal(err)
	}
	neverRunning := func() error { return nil }
	if err := RestoreGeneration(dbPath, dataDir, backups, gen, neverRunning); err == nil {
		t.Fatal("corrupt generation restored without error")
	}
	restored, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { restored.Close() })
	var n int
	if err := restored.DB().QueryRow(`SELECT COUNT(*) FROM products`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("live products after failed restore = %d, %v; want 1", n, err)
	}
}
