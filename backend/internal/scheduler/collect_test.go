package scheduler

import (
	"encoding/json"
	"testing"
	"time"

	"somun/internal/notify"
	"somun/internal/store"
)

func seedW7Published(t *testing.T, st *store.Store, execAgo time.Duration) (pid, cid int64) {
	t.Helper()
	p, err := st.CreateProduct("https://example.com/w7", "A", 3, "w7biz")
	if err != nil {
		t.Fatal(err)
	}
	c, err := st.CreateContent(&store.Content{ProductID: p.ID, Channel: "blog", Title: "t", Body: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateContentStatus(c.ID, "queued"); err != nil {
		t.Fatal(err)
	}
	if err := st.MarkContentPublished(c.ID); err != nil {
		t.Fatal(err)
	}
	rec, err := st.CreatePublishLog(c.ID, "naver_blog", "w7acct", "scheduled", "")
	if err != nil {
		t.Fatal(err)
	}
	exec := time.Now().UTC().Add(-execAgo).Format(time.RFC3339)
	if err := st.MarkLogDone(rec.ID, exec, "https://x.test/post/1 | evidence: shots/a.png"); err != nil {
		t.Fatal(err)
	}
	return p.ID, c.ID
}

func TestCollectDueEnqueues(t *testing.T) {
	st := openTestStore(t)
	svc := NewService(st, notify.NewService(st))
	_, cid := seedW7Published(t, st, 25*time.Hour)
	n, err := svc.CollectDue(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("enqueued = %d, want 1", n)
	}
	j, err := st.ClaimNextJob("collect", "w7w")
	if err != nil || j == nil {
		t.Fatalf("collect job = %+v, %v", j, err)
	}
	var payload struct {
		ContentID int64  `json:"content_id"`
		Account   string `json:"account"`
		Platform  string `json:"platform"`
		URL       string `json:"url"`
		Window    string `json:"window"`
	}
	if err := json.Unmarshal([]byte(j.Payload), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ContentID != cid || payload.Account != "w7acct" || payload.Platform != "naver_blog" ||
		payload.URL != "https://x.test/post/1" || payload.Window != "24h" {
		t.Fatalf("payload = %+v", payload)
	}
	if j.Business != "w7biz" || j.Priority != 3 {
		t.Fatalf("routing = %+v", j)
	}
	// The sweep is idempotent: the ledgered window never re-enqueues.
	n, err = svc.CollectDue(time.Now())
	if err != nil || n != 0 {
		t.Fatalf("replay = %d,%v, want 0,nil", n, err)
	}
	if again, err := st.ClaimNextJob("collect", "w"); err != nil || again != nil {
		t.Fatalf("extra collect job = %+v, %v", again, err)
	}
}

func TestCollectDueFreshAndOff(t *testing.T) {
	st := openTestStore(t)
	svc := NewService(st, notify.NewService(st))
	seedW7Published(t, st, time.Hour)
	n, err := svc.CollectDue(time.Now())
	if err != nil || n != 0 {
		t.Fatalf("fresh = %d,%v, want 0,nil", n, err)
	}
	cfg := store.DefaultAutomationSettings()
	cfg.Enabled = false
	if err := store.SetAutomationSettings(st.DB(), cfg); err != nil {
		t.Fatal(err)
	}
	n, err = svc.CollectDue(time.Now().Add(30 * 24 * time.Hour))
	if err != nil || n != 0 {
		t.Fatalf("automation-off = %d,%v, want 0,nil", n, err)
	}
}
