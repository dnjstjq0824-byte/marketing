package queue

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"somun/internal/content/policy"
	"somun/internal/store"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := store.Migrate(st.DB()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return st
}

func seedProduct(t *testing.T, st *store.Store, exit string) int64 {
	t.Helper()
	p, err := st.CreateProduct("https://example.com/item", exit, 0, "")
	if err != nil {
		t.Fatalf("seed product: %v", err)
	}
	return p.ID
}

// Parallel claims of one pending job must assign it exactly once.
func TestClaimConcurrentSingleWinner(t *testing.T) {
	st := openTestStore(t)
	pid := seedProduct(t, st, "A")
	if _, err := st.CreateJob(JobAnalyze, pid, "{}"); err != nil {
		t.Fatal(err)
	}
	const racers = 8
	var wg sync.WaitGroup
	winners := make(chan int64, racers)
	errs := make(chan error, racers)
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			j, err := st.ClaimNextJob(JobAnalyze, fmt.Sprintf("w%d", i))
			if err != nil {
				errs <- err
				return
			}
			if j != nil {
				winners <- j.ID
			}
		}(i)
	}
	wg.Wait()
	close(winners)
	close(errs)
	for err := range errs {
		t.Fatalf("claim error: %v", err)
	}
	var got []int64
	for id := range winners {
		got = append(got, id)
	}
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 winner, got %d", len(got))
	}
	j, _ := st.GetJob(got[0])
	if j.Status != StatusClaimed {
		t.Fatalf("winner status = %s, want claimed", j.Status)
	}
	if j.WorkerName == "" {
		t.Fatal("winner has no worker name")
	}
}

// Oldest pending job is claimed first.
func TestClaimOldestFirst(t *testing.T) {
	st := openTestStore(t)
	pid := seedProduct(t, st, "A")
	var ids []int64
	for i := 0; i < 3; i++ {
		j, err := st.CreateJob(JobAnalyze, pid, "{}")
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, j.ID)
	}
	svc := NewService(st)
	for _, want := range ids {
		j, err := svc.Next(JobAnalyze, "w")
		if err != nil || j == nil {
			t.Fatalf("next: job=%v err=%v", j, err)
		}
		if j.ID != want {
			t.Fatalf("claimed %d, want oldest %d", j.ID, want)
		}
		if _, err := st.SetJobResult(j.ID, "{}"); err != nil {
			t.Fatal(err)
		}
	}
	none, err := svc.Next(JobAnalyze, "w")
	if err != nil || none != nil {
		t.Fatalf("expected no job, got %v err %v", none, err)
	}
}

// Analyze result stores the reading and auto-creates exactly one strategy job.
func TestAnalyzeResultChain(t *testing.T) {
	st := openTestStore(t)
	pid := seedProduct(t, st, "A")
	j, _ := st.CreateJob(JobAnalyze, pid, "{}")
	if _, err := st.ClaimNextJob(JobAnalyze, "resident-claude"); err != nil {
		t.Fatal(err)
	}
	svc := NewService(st)
	done, err := svc.SubmitAnalyze(j.ID, AnalyzeResult{
		ProductName: "글로우사인 네임스티커",
		Unknowns:    "배송비 미확인",
	})
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != StatusDone {
		t.Fatalf("status = %s", done.Status)
	}
	rd, err := st.GetReadingByProduct(pid)
	if err != nil {
		t.Fatal(err)
	}
	if rd.ProductName != "글로우사인 네임스티커" || rd.PriceSummary != "" || rd.Unknowns != "배송비 미확인" {
		t.Fatalf("reading not stored as-is: %+v", rd)
	}
	if n, _ := st.CountOpenJobs(JobStrategy, pid); n != 1 {
		t.Fatalf("strategy jobs = %d, want 1", n)
	}
	if n, _ := st.CountOpenJobs(JobGenerate, pid); n != 0 {
		t.Fatalf("generate jobs too early = %d, want 0", n)
	}
	p, _ := st.GetProduct(pid)
	if p.Name != "글로우사인 네임스티커" {
		t.Fatalf("product name = %q", p.Name)
	}
	// Re-registering on a done job fails (non-idempotent).
	if _, err := svc.SubmitAnalyze(j.ID, AnalyzeResult{}); !errors.Is(err, store.ErrAlreadyDone) {
		t.Fatalf("second submit err = %v, want ErrAlreadyDone", err)
	}
	// ...and no second strategy job appears.
	if n, _ := st.CountOpenJobs(JobStrategy, pid); n != 1 {
		t.Fatalf("strategy jobs after retry = %d, want 1", n)
	}
}

// Strategy result stores the strategy and auto-creates exactly one generate job.
func TestStrategyResultChain(t *testing.T) {
	st := openTestStore(t)
	pid := seedProduct(t, st, "A")
	j, _ := st.CreateJob(JobStrategy, pid, "{}")
	if _, err := st.ClaimNextJob(JobStrategy, "agent-a"); err != nil {
		t.Fatal(err)
	}
	svc := NewService(st)
	if _, err := svc.SubmitStrategy(j.ID, StrategyResult{}); !errors.Is(err, ErrEmptyAngle) {
		t.Fatalf("empty angle err = %v, want ErrEmptyAngle", err)
	}
	done, err := svc.SubmitStrategy(j.ID, StrategyResult{
		Angle: "gift angle", Channels: "instagram,blog", KeyMessages: "k", ScheduleNote: "",
	})
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != StatusDone {
		t.Fatalf("status = %s", done.Status)
	}
	strat, err := st.GetStrategyByProduct(pid)
	if err != nil {
		t.Fatal(err)
	}
	if strat.Angle != "gift angle" || strat.ScheduleNote != "" {
		t.Fatalf("strategy not stored as-is: %+v", strat)
	}
	if n, _ := st.CountOpenJobs(JobGenerate, pid); n != 1 {
		t.Fatalf("generate jobs = %d, want 1", n)
	}
	if _, err := svc.SubmitStrategy(j.ID, StrategyResult{Angle: "x"}); !errors.Is(err, store.ErrAlreadyDone) {
		t.Fatalf("second submit err = %v, want ErrAlreadyDone", err)
	}
}

// Generate result stores all valid items as drafts; bad channels are dropped.
func TestGenerateResultDrafts(t *testing.T) {
	st := openTestStore(t)
	pid := seedProduct(t, st, "B") // sponsored product
	j, _ := st.CreateJob(JobGenerate, pid, "{}")
	if _, err := st.ClaimNextJob(JobGenerate, "w"); err != nil {
		t.Fatal(err)
	}
	svc := NewService(st)
	done, kept, err := svc.SubmitGenerate(j.ID, GenerateResult{Contents: []GenerateItem{
		{Channel: "instagram", Title: "t1", Body: "b1"},
		{Channel: "carrier-pigeon", Title: "t2", Body: "b2"},
		{Channel: "blog", Title: "t3", Body: ""},
		{Channel: "shortform", Title: "t4", Body: "b4", VirtualPersona: true},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if kept != 2 || done.Status != StatusDone {
		t.Fatalf("kept=%d status=%s", kept, done.Status)
	}
	drafts, err := st.ListContents("draft", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(drafts) != 2 {
		t.Fatalf("drafts = %d, want 2", len(drafts))
	}
	for _, d := range drafts {
		if d.Disclosure == "" {
			t.Fatal("sponsored draft misses disclosure")
		}
	}
	var persona *store.Content
	for _, d := range drafts {
		if d.Channel == "shortform" {
			persona = d
		}
	}
	if persona == nil || !persona.IsVirtualPersona || persona.AILabel == "" {
		t.Fatal("virtual-persona label missing")
	}
}

// Empty contents is rejected.
func TestGenerateEmptyRejected(t *testing.T) {
	st := openTestStore(t)
	pid := seedProduct(t, st, "A")
	j, _ := st.CreateJob(JobGenerate, pid, "{}")
	if _, err := st.ClaimNextJob(JobGenerate, "w"); err != nil {
		t.Fatal(err)
	}
	svc := NewService(st)
	if _, _, err := svc.SubmitGenerate(j.ID, GenerateResult{Contents: nil}); !errors.Is(err, ErrEmptyContents) {
		t.Fatalf("err = %v, want ErrEmptyContents", err)
	}
	if _, _, err := svc.SubmitGenerate(j.ID, GenerateResult{Contents: []GenerateItem{{Channel: "nope", Body: "x"}}}); !errors.Is(err, ErrNoValidItems) {
		t.Fatalf("err = %v, want ErrNoValidItems", err)
	}
	// Job stays claimed for the worker to retry correctly.
	cur, _ := st.GetJob(j.ID)
	if cur.Status != StatusClaimed {
		t.Fatalf("status = %s, want claimed", cur.Status)
	}
}

// Failed jobs never come back to the queue by themselves.
func TestFailedNeverRequeues(t *testing.T) {
	st := openTestStore(t)
	pid := seedProduct(t, st, "A")
	j, _ := st.CreateJob(JobAnalyze, pid, "{}")
	svc := NewService(st)
	if _, err := svc.Fail(j.ID, "page blocked"); err != nil {
		t.Fatal(err)
	}
	next, err := svc.Next(JobAnalyze, "w")
	if err != nil {
		t.Fatal(err)
	}
	if next != nil {
		t.Fatalf("failed job requeued as #%d", next.ID)
	}
	// Result registration on a failed job is rejected like on done jobs.
	if _, err := svc.SubmitAnalyze(j.ID, AnalyzeResult{}); !errors.Is(err, store.ErrAlreadyDone) {
		t.Fatalf("submit on failed err = %v, want ErrAlreadyDone", err)
	}
}

// Results require a claim first.
func TestResultRequiresClaim(t *testing.T) {
	st := openTestStore(t)
	pid := seedProduct(t, st, "A")
	j, _ := st.CreateJob(JobAnalyze, pid, "{}")
	svc := NewService(st)
	if _, err := svc.SubmitAnalyze(j.ID, AnalyzeResult{}); !errors.Is(err, store.ErrNotClaimed) {
		t.Fatalf("err = %v, want ErrNotClaimed", err)
	}
}

// Only the five specified types are accepted (publish joined in P1-0).
func TestJobTypeValidation(t *testing.T) {
	for _, bad := range []string{"", "check", "metrics"} {
		if err := ValidateJobType(bad); !errors.Is(err, ErrBadJobType) {
			t.Fatalf("type %q err = %v, want ErrBadJobType", bad, err)
		}
	}
	for _, good := range []string{JobAnalyze, JobStrategy, JobGenerate, JobReview, JobPublish} {
		if err := ValidateJobType(good); err != nil {
			t.Fatalf("type %q err = %v", good, err)
		}
	}
}

// Chief review applies judgments: targeted approvals schedule, bare approvals
// advance, rejections return to review. Every judgment is recorded.
func TestSubmitReview(t *testing.T) {
	st := openTestStore(t)
	pid := seedProduct(t, st, "A")
	if _, err := st.CreateAccountRow(&store.ChannelAccount{Name: "racc", Platform: "instagram",
		SessionState: "connected", PostingMode: "auto"}); err != nil {
		t.Fatal(err)
	}
	mkDraft := func() int64 {
		c, err := st.CreateContent(&store.Content{ProductID: pid, Channel: "blog", Body: "b"})
		if err != nil {
			t.Fatal(err)
		}
		return c.ID
	}
	newReviewJob := func() int64 {
		j, err := st.CreateJob(JobReview, pid, "{}")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.ClaimNextJob(JobReview, "muse-ai"); err != nil {
			t.Fatal(err)
		}
		return j.ID
	}
	svc := NewService(st)

	// Approve with target -> queued + scheduled + chief note.
	d1 := mkDraft()
	if _, err := svc.SubmitReview(newReviewJob(), ReviewResult{Decisions: []ReviewDecisionItem{
		{ContentID: d1, Approve: true, Reason: "good", Account: "racc",
			Platform: "instagram", ScheduledAt: "2030-01-01T00:00:00Z"},
	}}); err != nil {
		t.Fatal(err)
	}
	c1, _ := st.GetContent(d1)
	if c1.Status != policy.StatusQueued || c1.TargetAccount != "racc" {
		t.Fatalf("forwarded = %+v", c1)
	}

	// Approve without target -> approved.
	d2 := mkDraft()
	if _, err := svc.SubmitReview(newReviewJob(), ReviewResult{Decisions: []ReviewDecisionItem{
		{ContentID: d2, Approve: true, Reason: "ok"},
	}}); err != nil {
		t.Fatal(err)
	}
	c2, _ := st.GetContent(d2)
	if c2.Status != policy.StatusApproved {
		t.Fatalf("status = %s", c2.Status)
	}

	// Reject -> review with recorded reason.
	d3 := mkDraft()
	if _, err := svc.SubmitReview(newReviewJob(), ReviewResult{Decisions: []ReviewDecisionItem{
		{ContentID: d3, Approve: false, Reason: "weak hook"},
	}}); err != nil {
		t.Fatal(err)
	}
	c3, _ := st.GetContent(d3)
	if c3.Status != policy.StatusReview {
		t.Fatalf("status = %s", c3.Status)
	}
	revs, _ := st.ListReviewDecisions(d3)
	if len(revs) != 1 || revs[0].Decision != "rejected" || revs[0].Decider != "muse-ai" {
		t.Fatalf("reviews = %+v", revs)
	}

	// Bad target fails the whole submission; the job stays claimed.
	d4 := mkDraft()
	jid := newReviewJob()
	if _, err := svc.SubmitReview(jid, ReviewResult{Decisions: []ReviewDecisionItem{
		{ContentID: d4, Approve: true, Account: "ghost", Platform: "instagram"},
	}}); !errors.Is(err, ErrBadDecision) {
		t.Fatalf("err = %v, want ErrBadDecision", err)
	}
	j, _ := st.GetJob(jid)
	if j.Status != StatusClaimed {
		t.Fatalf("job status = %s, want claimed", j.Status)
	}

	// Non-idempotent like every result registration.
	if _, err := svc.SubmitReview(newReviewJob(), ReviewResult{}); !errors.Is(err, ErrEmptyDecisions) {
		t.Fatalf("err = %v, want ErrEmptyDecisions", err)
	}
}
