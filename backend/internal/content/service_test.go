package content

import (
	"path/filepath"
	"testing"

	"somun/internal/store"
)

func openContentStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := store.Migrate(st.DB()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return st
}

func seedQueued(t *testing.T, st *store.Store, scheduled bool) int64 {
	t.Helper()
	p, err := st.CreateProduct("https://example.com/x", "A", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	c, err := st.CreateContent(&store.Content{ProductID: p.ID, Channel: "blog", Body: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateContentStatus(c.ID, "queued"); err != nil {
		t.Fatal(err)
	}
	at := ""
	if scheduled {
		at = "2030-01-01T00:00:00Z"
		if err := st.ScheduleContent(c.ID, "acc", "instagram", "official_api", at); err != nil {
			t.Fatal(err)
		}
	}
	_ = at
	return c.ID
}

// Manual publish recording flips queued to published with a done log.
func TestRecordManualPublish(t *testing.T) {
	st := openContentStore(t)
	svc := NewService(st)
	cid := seedQueued(t, st, true)
	got, err := svc.RecordManualPublish(cid, "https://example.com/post/1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "published" {
		t.Fatalf("status = %s", got.Status)
	}
	rec, err := st.GetPublishLog(cid, "instagram", "acc")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != "done" || rec.ExecutedAt == "" || rec.Note != "https://example.com/post/1" {
		t.Fatalf("log = %+v", rec)
	}
	// Recording twice is rejected: published is terminal.
	if _, err := svc.RecordManualPublish(cid, "https://example.com/post/1"); err == nil {
		t.Fatal("second record accepted")
	}
}

// A plan record (e.g. auto-advanced) completes instead of conflicting.
func TestRecordManualPublishCompletesPlan(t *testing.T) {
	st := openContentStore(t)
	svc := NewService(st)
	cid := seedQueued(t, st, true)
	if _, err := st.CreatePublishLog(cid, "instagram", "acc", "scheduled", "auto-advanced"); err != nil {
		t.Fatal(err)
	}
	got, err := svc.RecordManualPublish(cid, "https://example.com/post/9")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "published" {
		t.Fatalf("status = %s", got.Status)
	}
	rec, _ := st.GetPublishLog(cid, "instagram", "acc")
	if rec.Status != "done" || rec.Note != "https://example.com/post/9" {
		t.Fatalf("log = %+v", rec)
	}
}

// "올라가 있음" completes a queued post; "안 올라감" keeps it queued
// and files a failed log entry instead.
func TestConfirmPublished(t *testing.T) {
	st := openContentStore(t)
	svc := NewService(st)

	yes := seedQueued(t, st, true)
	got, err := svc.ConfirmPublished(yes, true, "https://example.com/post/1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "published" {
		t.Fatalf("status = %s", got.Status)
	}

	no := seedQueued(t, st, true)
	got, err = svc.ConfirmPublished(no, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "queued" {
		t.Fatalf("status = %s, want queued", got.Status)
	}
	rec, err := st.GetPublishLog(no, "instagram", "acc")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != "failed" || rec.Note != "operator: not published" {
		t.Fatalf("log = %+v", rec)
	}

	// An existing plan row is marked failed rather than duplicated.
	plan := seedQueued(t, st, true)
	if _, err := st.CreatePublishLog(plan, "instagram", "acc", "scheduled", "auto"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ConfirmPublished(plan, false, ""); err != nil {
		t.Fatal(err)
	}
	rec, _ = st.GetPublishLog(plan, "instagram", "acc")
	if rec.Status != "failed" || rec.Attempts != 1 {
		t.Fatalf("plan log = %+v", rec)
	}

	// Drafts and unscheduled posts are rejected either way.
	p, _ := st.CreateProduct("https://example.com/z", "A", 0, "")
	c, _ := st.CreateContent(&store.Content{ProductID: p.ID, Channel: "blog", Body: "b"})
	if _, err := svc.ConfirmPublished(c.ID, false, ""); err == nil {
		t.Fatal("draft not-published accepted")
	}
	unsched := seedQueued(t, st, false)
	if _, err := svc.ConfirmPublished(unsched, false, ""); err == nil {
		t.Fatal("unscheduled not-published accepted")
	}
	if _, err := svc.ConfirmPublished(unsched, true, "notaurl"); err == nil {
		t.Fatal("bad URL accepted")
	}
}

// Non-queued, unscheduled, and bad-URL records are rejected.
func TestRecordManualPublishRejected(t *testing.T) {
	st := openContentStore(t)
	svc := NewService(st)
	p, _ := st.CreateProduct("https://example.com/y", "A", 0, "")
	c, _ := st.CreateContent(&store.Content{ProductID: p.ID, Channel: "blog", Body: "b"})
	if _, err := svc.RecordManualPublish(c.ID, "https://example.com/p"); err == nil {
		t.Fatal("draft record accepted")
	}
	queued := seedQueued(t, st, false)
	if _, err := svc.RecordManualPublish(queued, "https://example.com/p"); err == nil {
		t.Fatal("unscheduled record accepted")
	}
	if _, err := svc.RecordManualPublish(queued, "notaurl"); err == nil {
		t.Fatal("bad URL accepted")
	}
}
