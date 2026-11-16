package queue

import (
	"encoding/json"
	"errors"
	"testing"

	"somun/internal/content/policy"
	"somun/internal/store"
)

func seedW7Content(t *testing.T, st *store.Store, pid int64) int64 {
	t.Helper()
	c, err := st.CreateContent(&store.Content{ProductID: pid, Channel: "blog", Title: "t", Body: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateContentStatus(c.ID, "queued"); err != nil {
		t.Fatal(err)
	}
	if err := st.MarkContentPublished(c.ID); err != nil {
		t.Fatal(err)
	}
	return c.ID
}

func claimCollect(t *testing.T, st *store.Store, cid int64, window string) int64 {
	t.Helper()
	payload, _ := json.Marshal(map[string]any{
		"content_id": cid, "account": "w7acct", "platform": "naver_blog",
		"url": "https://x.test/post/1", "window": window,
	})
	j, err := st.CreateJobFull(JobCollect, cid, string(payload), "", 0, false)
	if err != nil {
		t.Fatal(err)
	}
	cl, err := st.ClaimNextJob(JobCollect, "w7w")
	if err != nil || cl == nil || cl.ID != j.ID {
		t.Fatalf("claim collect = %+v, %v", cl, err)
	}
	return j.ID
}

func w7i64(v int64) *int64 { return &v }

func TestSubmitCollectStoresRow(t *testing.T) {
	st := openTestStore(t)
	pid := seedProduct(t, st, "A")
	cid := seedW7Content(t, st, pid)
	svc := NewService(st)
	jid := claimCollect(t, st, cid, "24h")
	done, err := svc.SubmitCollect(jid, CollectResult{
		Views: w7i64(120), Likes: w7i64(7), Comments: w7i64(3), Note: "stub",
	})
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != StatusDone {
		t.Fatalf("status = %s", done.Status)
	}
	rows, err := st.ListMetrics(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("metrics = %d rows", len(rows))
	}
	m := rows[0]
	if m.ContentID == nil || *m.ContentID != cid || m.Origin != "collect:24h" {
		t.Fatalf("metric = %+v", m)
	}
	if m.Views == nil || *m.Views != 120 || m.CauseClass != "" {
		t.Fatalf("figures/cause = %+v", m)
	}
}

func TestSubmitCollectMissingKeepsUnclassified(t *testing.T) {
	st := openTestStore(t)
	pid := seedProduct(t, st, "A")
	cid := seedW7Content(t, st, pid)
	svc := NewService(st)
	jid := claimCollect(t, st, cid, "72h")
	if _, err := svc.SubmitCollect(jid, CollectResult{}); err != nil {
		t.Fatal(err)
	}
	rows, err := st.ListMetrics(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].CauseClass != policy.UnclassifiedCause {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].Views != nil || rows[0].Origin != "collect:72h" {
		t.Fatalf("empty row = %+v", rows[0])
	}
	// Zero figures are equally unclassified, not a success story.
	cid2 := seedW7Content(t, st, pid)
	jid2 := claimCollect(t, st, cid2, "24h")
	if _, err := svc.SubmitCollect(jid2, CollectResult{Views: w7i64(0)}); err != nil {
		t.Fatal(err)
	}
	rows, err = st.ListMetrics(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].CauseClass != policy.UnclassifiedCause {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestSubmitCollectRejects(t *testing.T) {
	st := openTestStore(t)
	pid := seedProduct(t, st, "A")
	cid := seedW7Content(t, st, pid)
	svc := NewService(st)
	// Wrong job type.
	aj, err := st.CreateJob(JobAnalyze, pid, "{}")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimNextJob(JobAnalyze, "w"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SubmitCollect(aj.ID, CollectResult{}); !errors.Is(err, ErrJobNotCollect) {
		t.Fatalf("wrong type err = %v", err)
	}
	// Unclaimed job.
	payload, _ := json.Marshal(map[string]any{"content_id": cid, "window": "24h"})
	uj, err := st.CreateJobFull(JobCollect, cid, string(payload), "", 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SubmitCollect(uj.ID, CollectResult{}); !errors.Is(err, store.ErrNotClaimed) {
		t.Fatalf("unclaimed err = %v", err)
	}
	// Negative figures (claim the same job: oldest-pending order).
	if _, err := st.ClaimNextJob(JobCollect, "w"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SubmitCollect(uj.ID, CollectResult{Likes: w7i64(-1)}); !errors.Is(err, ErrBadCollect) {
		t.Fatalf("negative err = %v", err)
	}
	// Unknown content.
	bad, _ := json.Marshal(map[string]any{"content_id": 99999, "window": "24h"})
	bj, err := st.CreateJobFull(JobCollect, 99999, string(bad), "", 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimNextJob(JobCollect, "w"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SubmitCollect(bj.ID, CollectResult{Views: w7i64(1)}); !errors.Is(err, ErrBadCollect) {
		t.Fatalf("unknown content err = %v", err)
	}
}

func TestFailCollectReleasesWindow(t *testing.T) {
	st := openTestStore(t)
	pid := seedProduct(t, st, "A")
	cid := seedW7Content(t, st, pid)
	svc := NewService(st)
	jid := claimCollect(t, st, cid, "24h")
	if _, err := st.EnsureCollectWindow(cid, "24h", "w7acct", jid); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Fail(jid, "selector missing"); err != nil {
		t.Fatal(err)
	}
	has, err := st.HasCollectWindow(cid, "24h", "w7acct")
	if err != nil || has {
		t.Fatalf("after fail has = %v,%v, want released", has, err)
	}
}

func TestStrategyPayloadCarriesPastTop(t *testing.T) {
	st := openTestStore(t)
	p, err := st.CreateProduct("https://example.com/w7past", "A", 0, "w7biz")
	if err != nil {
		t.Fatal(err)
	}
	c, err := st.CreateContent(&store.Content{ProductID: p.ID, Channel: "blog", Title: "winner", Body: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateMetric(&store.Metric{ContentID: &c.ID, Channel: "blog",
		Views: w7i64(900), Likes: w7i64(30), Origin: "collect:24h"}); err != nil {
		t.Fatal(err)
	}
	svc := NewService(st)
	aj, err := st.CreateJob(JobAnalyze, p.ID, "{}")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimNextJob(JobAnalyze, "w"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SubmitAnalyze(aj.ID, AnalyzeResult{ProductName: "w7"}); err != nil {
		t.Fatal(err)
	}
	sj, err := st.ClaimNextJob(JobStrategy, "w")
	if err != nil || sj == nil {
		t.Fatalf("strategy job = %+v, %v", sj, err)
	}
	var payload struct {
		PastTop []store.TopMetric `json:"past_top"`
	}
	if err := json.Unmarshal([]byte(sj.Payload), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.PastTop) != 1 || payload.PastTop[0].Title != "winner" || payload.PastTop[0].Views != 900 {
		t.Fatalf("past_top = %+v", payload.PastTop)
	}
}

func TestValidateCollectType(t *testing.T) {
	if err := ValidateJobType(JobCollect); err != nil {
		t.Fatalf("collect rejected: %v", err)
	}
	if err := ValidateJobType("nope"); !errors.Is(err, ErrBadJobType) {
		t.Fatalf("unknown err = %v", err)
	}
}
