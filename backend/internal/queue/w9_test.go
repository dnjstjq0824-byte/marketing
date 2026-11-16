package queue

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"somun/internal/store"
)

// W9-4 (B20): SubmitReview's doc comment was split in two by SubmitPublish
// sitting between the comment lines and the function. The pipeline order is
// review -> publish, each with its own adjacent doc comment.
func TestW9SubmitReviewOrder(t *testing.T) {
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller unavailable")
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(self), "service.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	reviewFn := strings.Index(src, "func (s *Service) SubmitReview(")
	publishFn := strings.Index(src, "func (s *Service) SubmitPublish(")
	if reviewFn < 0 || publishFn < 0 {
		t.Fatalf("funcs found: review=%d publish=%d", reviewFn, publishFn)
	}
	if reviewFn > publishFn {
		t.Fatal("SubmitReview must precede SubmitPublish (pipeline order)")
	}
	// Each doc comment sits directly above its own func with no other
	// top-level declaration wedged between.
	reviewDoc := strings.Index(src, "// SubmitReview applies chief judgments")
	if reviewDoc < 0 || reviewDoc > reviewFn {
		t.Fatal("SubmitReview doc comment missing above the func")
	}
	if mid := src[reviewDoc:reviewFn]; strings.Contains(mid, "\nfunc ") {
		t.Fatal("declaration wedged between SubmitReview comment and func")
	}
	publishDoc := strings.Index(src, "// SubmitPublish records an adapter session post")
	if publishDoc < 0 || publishDoc < reviewFn || publishDoc > publishFn {
		t.Fatal("SubmitPublish doc comment missing above its own func")
	}
	// The two rejoined halves read as one comment (B20 regression).
	if !strings.Contains(src, "// scheduling when a target rides along), rejections return to review and") {
		t.Fatal("SubmitReview comment second half missing")
	}
}

func TestW9ClassifyPublishFailure(t *testing.T) {
	cases := []struct {
		reason string
		want   bool
	}{
		{"PrePublishFailed: PRE_PUBLISH selector not clickable: x", true},
		{"RuntimeError: PRE_PUBLISH account a: save a login session first (adapter.login) -- run: ...", true},
		{"PostFailed: selector not fillable: input (None)", true},        // legacy
		{"PostFailed: platform x selectors not pinned yet (P0-1)", true}, // legacy
		{"PostFailed: unknown platform: x", true},                        // legacy
		{"UncertainPostFailed: UNCERTAIN_POST landing /x lacks /post/", false},
		{"TimeoutError: UNCERTAIN_POST Timeout 30000ms exceeded", false},
		{"boom", false}, // unrecognized: fail closed toward uncertain
		{"", false},
		// Uncertain wins when both markers appear (e.g. a timeout message
		// quoting an early selector probe).
		{"UncertainPostFailed: UNCERTAIN_POST click ok, then PRE_PUBLISH probe echoed", false},
	}
	for _, tc := range cases {
		if got := classifyPublishFailure(tc.reason); got != tc.want {
			t.Errorf("classifyPublishFailure(%q) = %v, want %v", tc.reason, got, tc.want)
		}
	}
}

func seedPublishJob(t *testing.T, st *store.Store) (cid, jid int64) {
	t.Helper()
	pid := seedProduct(t, st, "A")
	c, err := st.CreateContent(&store.Content{ProductID: pid, Channel: "blog", Title: "t", Body: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`UPDATE contents SET status='queued',target_account='a',target_platform='naver_blog' WHERE id=?`, c.ID); err != nil {
		t.Fatal(err)
	}
	j, err := st.CreateJobFull(JobPublish, c.ID, `{"content_id":`+itoa(c.ID)+`,"account":"a","platform":"naver_blog"}`, "", 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimNextJob(JobPublish, "w"); err != nil {
		t.Fatal(err)
	}
	return c.ID, j.ID
}

// W9-5: a pre-publish failure (login entry, selector miss) never posted, so
// the first one returns the job to pending for an automatic retry on the
// next poll; the second closes it. Neither holds the content.
func TestW9PublishPrePublishRetryOnce(t *testing.T) {
	st := openTestStore(t)
	cid, jid := seedPublishJob(t, st)
	svc := NewService(st)
	fails := 0
	svc.OnFail = func(job *store.Job) { fails++ }

	got, err := svc.Fail(jid, "PrePublishFailed: PRE_PUBLISH selector not clickable: input.submit")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusPending {
		t.Fatalf("first fail status = %s, want pending", got.Status)
	}
	if got.Attempts != 1 {
		t.Fatalf("attempts = %d, want 1", got.Attempts)
	}
	if !strings.Contains(got.Error, "PRE_PUBLISH") {
		t.Fatalf("retry record = %q, want reason kept", got.Error)
	}
	if c, _ := st.GetContent(cid); c.Held {
		t.Fatal("pre-publish retry must not hold the content")
	}
	if fails != 0 {
		t.Fatalf("OnFail fired %d times on transient retry, want 0", fails)
	}
	// The retried job is claimable again on the next poll.
	next, err := st.ClaimNextJob(JobPublish, "w2")
	if err != nil || next == nil || next.ID != jid {
		t.Fatalf("reclaim = %+v, %v", next, err)
	}
	second, err := svc.Fail(jid, "PrePublishFailed: PRE_PUBLISH selector not clickable: input.submit")
	if err != nil {
		t.Fatal(err)
	}
	if second.Status != StatusFailed {
		t.Fatalf("second fail status = %s, want failed", second.Status)
	}
	if c, _ := st.GetContent(cid); c.Held {
		t.Fatal("failed pre-publish retry must not hold the content")
	}
	if fails != 1 {
		t.Fatalf("OnFail fired %d times, want 1 (terminal only)", fails)
	}
}

// W9-5: an uncertain failure may have posted, so the job closes and the
// content is held — the scheduler never reposts it (duplicate prevention)
// until the operator resolves it with confirm-published.
func TestW9PublishUncertainHolds(t *testing.T) {
	st := openTestStore(t)
	cid, jid := seedPublishJob(t, st)
	svc := NewService(st)
	fails := 0
	svc.OnFail = func(job *store.Job) { fails++ }

	got, err := svc.Fail(jid, "UncertainPostFailed: UNCERTAIN_POST landing /x lacks /post/")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusFailed {
		t.Fatalf("status = %s, want failed", got.Status)
	}
	c, _ := st.GetContent(cid)
	if !c.Held || c.HoldReason == "" {
		t.Fatalf("held=%v reason=%q, want held with reason", c.Held, c.HoldReason)
	}
	if fails != 1 {
		t.Fatalf("OnFail fired %d times, want 1", fails)
	}
	if next, err := svc.Next(JobPublish, "w"); err != nil || next != nil {
		t.Fatalf("held publish requeued as %+v (%v)", next, err)
	}
}

// W9-5: failures without a code fail closed — they hold like uncertain
// ones instead of risking a duplicate post.
func TestW9PublishUnclassifiedHolds(t *testing.T) {
	st := openTestStore(t)
	cid, jid := seedPublishJob(t, st)
	svc := NewService(st)

	got, err := svc.Fail(jid, "boom: something odd happened")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusFailed {
		t.Fatalf("status = %s, want failed", got.Status)
	}
	if c, _ := st.GetContent(cid); !c.Held {
		t.Fatal("unclassified publish failure must hold the content")
	}
}
