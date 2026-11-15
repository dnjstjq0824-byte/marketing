package store

import (
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func openP10Store(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "p10.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := Migrate(st.DB()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return st
}

func TestMigrateV7AgentsAndJobs(t *testing.T) {
	st := openP10Store(t)
	var name string
	if err := st.DB().QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='agents'`).Scan(&name); err != nil {
		t.Fatalf("agents table missing: %v", err)
	}
	var cols int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM pragma_table_info('jobs')
		WHERE name IN ('agent_id','business','priority','attempts','lease_until')`).Scan(&cols); err != nil || cols != 5 {
		t.Fatalf("job cols = %d, %v; want 5", cols, err)
	}
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM pragma_table_info('channel_accounts')
		WHERE name IN ('kind','platform_user_id')`).Scan(&cols); err != nil || cols != 2 {
		t.Fatalf("account cols = %d, %v; want 2", cols, err)
	}
	// Legacy rows inherit routing columns from their products.
	p, err := st.CreateProduct("https://example.com/v7", "A", 7, "biz7")
	if err != nil {
		t.Fatal(err)
	}
	j, err := st.CreateJob("analyze", p.ID, "{}")
	if err != nil {
		t.Fatal(err)
	}
	if j.Business != "biz7" || j.Priority != 7 {
		t.Fatalf("inherited = %q/%d, want biz7/7", j.Business, j.Priority)
	}
}

func TestAgentTokenOnceAndRotate(t *testing.T) {
	st := openP10Store(t)
	a, plaintext, err := st.CreateAgent("w1", "A", "h1", "", "", 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(plaintext) != 64 || a.TokenPrefix != plaintext[:8] {
		t.Fatalf("token shape wrong: len %d prefix %q", len(plaintext), a.TokenPrefix)
	}
	got, err := st.GetAgentByTokenHash(HashAgentToken(plaintext))
	if err != nil || got.ID != a.ID {
		t.Fatalf("token lookup = %+v, %v", got, err)
	}
	if _, _, err := st.CreateAgent("w1", "A", "h1", "", "", 1, nil); err != ErrDuplicate {
		t.Fatalf("dup label err = %v, want ErrDuplicate", err)
	}
	rotated, err := st.RotateAgentToken(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetAgentByTokenHash(HashAgentToken(plaintext)); err != ErrNotFound {
		t.Fatalf("old token still valid: %v", err)
	}
	if _, err := st.GetAgentByTokenHash(HashAgentToken(rotated)); err != nil {
		t.Fatalf("rotated token: %v", err)
	}
}

func seedRoutedProduct(t *testing.T, st *Store, business string, priority int) int64 {
	t.Helper()
	p, err := st.CreateProduct("https://example.com/"+business, "A", priority, business)
	if err != nil {
		t.Fatal(err)
	}
	return p.ID
}

func TestClaimRoutingMatrix(t *testing.T) {
	st := openP10Store(t)
	a, _, err := st.CreateAgent("wA", "A", "", "b1", "", 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	p1 := seedRoutedProduct(t, st, "b1", 5)
	p2 := seedRoutedProduct(t, st, "b2", 9)
	if _, err := st.CreateJob("analyze", p1, "{}"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateJob("analyze", p2, "{}"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateJob("strategy", p1, "{}"); err != nil {
		t.Fatal(err)
	}
	// Business scope beats raw priority: b1 wins over b2's pri-9.
	j, err := st.ClaimRoutedJob(a, []string{"analyze", "strategy"}, 600)
	if err != nil || j == nil {
		t.Fatalf("claim = %+v, %v", j, err)
	}
	if j.RefID != p1 || j.AgentID != a.ID || j.WorkerName != "wA" || j.LeaseUntil == "" {
		t.Fatalf("claimed = %+v", j)
	}
	// Capacity 1 blocks a second claim.
	j2, err := st.ClaimRoutedJob(a, []string{"analyze", "strategy"}, 600)
	if err != nil || j2 != nil {
		t.Fatalf("second claim = %+v, %v; want nil (capacity)", j2, err)
	}
	// A B-laner sees only its own type.
	b, _, err := st.CreateAgent("wB", "B", "", "b1", "", 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateJob("generate", p1, "{}"); err != nil {
		t.Fatal(err)
	}
	got, err := st.ClaimRoutedJob(b, []string{"generate"}, 600)
	if err != nil || got == nil || got.Type != "generate" {
		t.Fatalf("B claim = %+v, %v", got, err)
	}
	// Candidate precheck agrees with the claim outcome.
	has, err := st.HasPendingCandidate(a, []string{"analyze", "strategy"})
	if err != nil || !has {
		t.Fatalf("precheck = %v, %v; want true (strategy waits)", has, err)
	}
}

func TestClaimPublishAccountRule(t *testing.T) {
	st := openP10Store(t)
	acc, err := st.CreateAccountRow(&ChannelAccount{Name: "pa", Platform: "instagram", Role: "worker"})
	if err != nil {
		t.Fatal(err)
	}
	mine, _, err := st.CreateAgent("wP", "P", "", "", "", 1, &acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	other, _, err := st.CreateAgent("wP2", "P", "", "", "", 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateJobFull("publish", 0, `{"account":"pa","content_id":1}`, "", 0, false); err != nil {
		t.Fatal(err)
	}
	if j, err := st.ClaimRoutedJob(other, []string{"publish"}, 600); err != nil || j != nil {
		t.Fatalf("unlinked claim = %+v, %v; want nil", j, err)
	}
	j, err := st.ClaimRoutedJob(mine, []string{"publish"}, 600)
	if err != nil || j == nil {
		t.Fatalf("linked claim = %+v, %v", j, err)
	}
}

func backdateLease(t *testing.T, st *Store, id int64, minutes int) {
	t.Helper()
	past := time.Now().UTC().Add(-time.Duration(minutes) * time.Minute).Format(time.RFC3339)
	if _, err := st.DB().Exec(`UPDATE jobs SET lease_until=? WHERE id=?`, past, id); err != nil {
		t.Fatal(err)
	}
}

// W6: the writing account cannot claim its own drafts for review; other
// linked accounts and unlinked agents claim normally.
func TestClaimReviewExcludesWriter(t *testing.T) {
	st := openP10Store(t)
	wa, err := st.CreateAccountRow(&ChannelAccount{Name: "writer", Platform: "meta_ai", Role: "worker"})
	if err != nil {
		t.Fatal(err)
	}
	ra, err := st.CreateAccountRow(&ChannelAccount{Name: "reviewer", Platform: "claude_app", Role: "worker"})
	if err != nil {
		t.Fatal(err)
	}
	writer, _, err := st.CreateAgent("wB", "B", "", "", "", 1, &wa.ID)
	if err != nil {
		t.Fatal(err)
	}
	reviewer, _, err := st.CreateAgent("wC", "C", "", "", "", 1, &ra.ID)
	if err != nil {
		t.Fatal(err)
	}
	plain, _, err := st.CreateAgent("wC2", "C", "", "", "", 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	p, err := st.CreateProduct("https://example.com/w6", "A", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	c, err := st.CreateContent(&Content{ProductID: p.ID, Channel: "blog", Title: "t", Body: "b",
		GeneratedByAccountID: &wa.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateJob("review", p.ID, `{"content_ids":[`+strconv.FormatInt(c.ID, 10)+`]}`); err != nil {
		t.Fatal(err)
	}
	if j, err := st.ClaimRoutedJob(writer, []string{"review"}, 600); err != nil || j != nil {
		t.Fatalf("writer claim = %+v, %v; want nil", j, err)
	}
	j, err := st.ClaimRoutedJob(reviewer, []string{"review"}, 600)
	if err != nil || j == nil {
		t.Fatalf("reviewer claim = %+v, %v", j, err)
	}
	if _, err := st.DB().Exec(`UPDATE jobs SET status='pending',agent_id=0 WHERE id=?`, j.ID); err != nil {
		t.Fatal(err)
	}
	if j, err := st.ClaimRoutedJob(plain, []string{"review"}, 600); err != nil || j == nil {
		t.Fatalf("unlinked claim = %+v, %v; want job", j, err)
	}
}

func TestReapFirstReturnsOnce(t *testing.T) {
	st := openP10Store(t)
	a, _, err := st.CreateAgent("wR", "A", "", "", "", 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := seedRoutedProduct(t, st, "", 0)
	if _, err := st.CreateJob("analyze", p, "{}"); err != nil {
		t.Fatal(err)
	}
	j, err := st.ClaimRoutedJob(a, []string{"analyze"}, 600)
	if err != nil || j == nil {
		t.Fatal("claim failed")
	}
	backdateLease(t, st, j.ID, 1)
	now := time.Now().UTC().Format(time.RFC3339)
	actions, err := st.ReapExpiredLeases(now)
	if err != nil || len(actions) != 1 || actions[0].Action != "requeued" {
		t.Fatalf("reap = %+v, %v; want one requeued", actions, err)
	}
	got, _ := st.GetJob(j.ID)
	if got.Status != "pending" || got.Attempts != 1 || got.AgentID != 0 || got.LeaseUntil != "" {
		t.Fatalf("after reap = %+v", got)
	}
	// A heartbeat meanwhile saves the claim.
	j2, err := st.ClaimRoutedJob(a, []string{"analyze"}, 600)
	if err != nil || j2 == nil {
		t.Fatal("reclaim failed")
	}
	if _, err := st.ExtendJobLease(j2.ID, a.ID, 600); err != nil {
		t.Fatal(err)
	}
	actions, err = st.ReapExpiredLeases(now)
	if err != nil || len(actions) != 0 {
		t.Fatalf("reap after heartbeat = %+v, %v; want none", actions, err)
	}
}

func TestReapSecondFailsAndNotifies(t *testing.T) {
	st := openP10Store(t)
	a, _, err := st.CreateAgent("wR2", "A", "", "", "", 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := seedRoutedProduct(t, st, "", 0)
	if _, err := st.CreateJob("analyze", p, "{}"); err != nil {
		t.Fatal(err)
	}
	j, _ := st.ClaimRoutedJob(a, []string{"analyze"}, 600)
	backdateLease(t, st, j.ID, 1)
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := st.ReapExpiredLeases(now); err != nil {
		t.Fatal(err)
	}
	j2, _ := st.ClaimRoutedJob(a, []string{"analyze"}, 600)
	backdateLease(t, st, j2.ID, 1)
	actions, err := st.ReapExpiredLeases(time.Now().UTC().Format(time.RFC3339))
	if err != nil || len(actions) != 1 || actions[0].Action != "failed" {
		t.Fatalf("reap = %+v, %v; want one failed", actions, err)
	}
	got, _ := st.GetJob(j2.ID)
	if got.Status != "failed" || got.Error != "lease expired" {
		t.Fatalf("after reap = %+v", got)
	}
	if _, err := st.RequeueJob(j2.ID); err != nil {
		t.Fatalf("requeue: %v", err)
	}
	got, _ = st.GetJob(j2.ID)
	if got.Status != "pending" || got.Attempts != 0 || got.Error == "" {
		t.Fatalf("after requeue = %+v (error text must stay)", got)
	}
	if _, err := st.RequeueJob(j2.ID); err != ErrNotFailed {
		t.Fatalf("requeue pending err = %v, want ErrNotFailed", err)
	}
	if _, err := st.ExtendJobLease(j2.ID, a.ID+999, 600); err != ErrLeaseDenied {
		t.Fatalf("foreign extend err = %v, want ErrLeaseDenied", err)
	}
}

func TestReapPublishHoldsContent(t *testing.T) {
	st := openP10Store(t)
	a, _, err := st.CreateAgent("wP", "P", "", "", "", 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	p, err := st.CreateProduct("https://example.com/pub", "A", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	c, err := st.CreateContent(&Content{ProductID: p.ID, Channel: "blog", Title: "t", Body: "b"})
	if err != nil {
		t.Fatal(err)
	}
	payload := `{"account":"x","content_id":` + strconv.FormatInt(c.ID, 10) + `}`
	j, err := st.CreateJobFull("publish", 0, payload, "", 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`UPDATE jobs SET status='claimed',agent_id=?,lease_until=? WHERE id=?`,
		a.ID, time.Now().UTC().Add(-time.Minute).Format(time.RFC3339), j.ID); err != nil {
		t.Fatal(err)
	}
	actions, err := st.ReapExpiredLeases(time.Now().UTC().Format(time.RFC3339))
	if err != nil || len(actions) != 1 || actions[0].Action != "failed_held" {
		t.Fatalf("reap = %+v, %v; want failed_held", actions, err)
	}
	got, _ := st.GetContent(c.ID)
	if !got.Held {
		t.Fatal("content not held after publish lease death")
	}
}

func TestAgentStatsAndFilters(t *testing.T) {
	st := openP10Store(t)
	if _, _, err := st.CreateAgent("on", "A", "h", "b1", "", 1, nil); err != nil {
		t.Fatal(err)
	}
	off, _, err := st.CreateAgent("off", "B", "h", "", "", 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	paused, _, err := st.CreateAgent("pz", "C", "", "", "", 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	paused.Enabled = false
	if err := st.UpdateAgent(paused); err != nil {
		t.Fatal(err)
	}
	if err := st.TouchAgent(1); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`UPDATE agents SET last_seen_at=? WHERE id=?`,
		time.Now().UTC().Add(-time.Hour).Format(time.RFC3339), off.ID); err != nil {
		t.Fatal(err)
	}
	stats, err := st.StatsForAgents(time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}
	if stats.Online != 1 || stats.Offline != 1 || stats.Paused != 1 {
		t.Fatalf("stats = %+v", stats)
	}
	got, err := st.ListAgents(AgentFilter{Lane: "A"}, 10, 0)
	if err != nil || len(got) != 1 {
		t.Fatalf("lane filter = %d, %v", len(got), err)
	}
	n, err := st.CountAgents(AgentFilter{})
	if err != nil || n != 3 {
		t.Fatalf("count = %d, %v; want 3", n, err)
	}
}
