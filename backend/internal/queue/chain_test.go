package queue

import (
	"encoding/json"
	"testing"
)

// D21+W6: three rejects on one revision chain create exactly two
// regenerations, then dispose the chain to archived with one info notice
// instead of looping forever. Superseded originals archive the moment
// their revision is born. The flow mirrors production: generate drafts
// link onto the chain, each batch auto-creates its review job, and
// rejects walk the newest draft.
func TestRejectCapCountsWholeChain(t *testing.T) {
	st := openTestStore(t)
	pid := seedProduct(t, st, "A")
	svc := NewService(st)
	var humanCalls []int64
	svc.OnReviewHuman = func(contentID int64, message string) {
		humanCalls = append(humanCalls, contentID)
	}

	claim := func(jobType string) int64 {
		t.Helper()
		j, err := st.ClaimNextJob(jobType, "w")
		if err != nil {
			t.Fatal(err)
		}
		if j == nil {
			t.Fatalf("no pending %s job", jobType)
		}
		return j.ID
	}
	generateOne := func() int64 {
		t.Helper()
		gid := claim(JobGenerate)
		res := GenerateResult{Contents: []GenerateItem{{Channel: "blog", Title: "t", Body: "b"}}}
		if _, _, err := svc.SubmitGenerate(gid, res); err != nil {
			t.Fatal(err)
		}
		// The auto-created review job carries the new draft ID.
		jobs, err := st.ListRecentJobs()
		if err != nil {
			t.Fatal(err)
		}
		for _, j := range jobs {
			if j.Type != JobReview || j.Status != StatusPending {
				continue
			}
			var payload struct {
				ContentIDs []int64 `json:"content_ids"`
			}
			if err := json.Unmarshal([]byte(j.Payload), &payload); err == nil && len(payload.ContentIDs) == 1 {
				return payload.ContentIDs[0]
			}
		}
		t.Fatal("auto review job not found")
		return 0
	}
	reject := func(contentID int64) {
		t.Helper()
		rid := claim(JobReview)
		res := ReviewResult{Decisions: []ReviewDecisionItem{{ContentID: contentID, Approve: false, Reason: "tone"}}}
		if _, err := svc.SubmitReview(rid, res); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := st.CreateJob(JobGenerate, pid, "{}"); err != nil {
		t.Fatal(err)
	}
	first := generateOne()
	got, err := st.GetContent(first)
	if err != nil {
		t.Fatal(err)
	}
	if got.RevisionOf != 0 || got.RevisionRootID != got.ID {
		t.Fatalf("first draft chain = of %d root %d, want of 0 root self", got.RevisionOf, got.RevisionRootID)
	}

	reject(first)
	second := generateOne()
	got, err = st.GetContent(second)
	if err != nil {
		t.Fatal(err)
	}
	if got.RevisionOf != first || got.RevisionRootID != first {
		t.Fatalf("second draft chain = of %d root %d, want of %d root %d", got.RevisionOf, got.RevisionRootID, first, first)
	}

	reject(second)
	third := generateOne()
	got, err = st.GetContent(third)
	if err != nil {
		t.Fatal(err)
	}
	if got.RevisionRootID != first {
		t.Fatalf("third draft root = %d, want %d", got.RevisionRootID, first)
	}

	// Third reject: no new generate job, one info notice, chain disposed.
	reject(third)
	if j, err := st.ClaimNextJob(JobGenerate, "w"); err != nil {
		t.Fatal(err)
	} else if j != nil {
		t.Fatalf("regen job #%d created after the cap", j.ID)
	}
	if len(humanCalls) != 1 || humanCalls[0] != third {
		t.Fatalf("human calls = %v, want [%d]", humanCalls, third)
	}
	n, err := st.CountRejectedChain(first)
	if err != nil || n != 3 {
		t.Fatalf("chain rejects = %d, %v; want 3", n, err)
	}
	revs, err := st.ListRevisionIDs(first)
	if err != nil || len(revs) != 2 || revs[0] != second || revs[1] != third {
		t.Fatalf("revisions = %v, %v; want [%d %d]", revs, err, second, third)
	}
	// W6: superseded originals and the cap-hit chain rest in archived.
	for _, id := range []int64{first, second, third} {
		got, err := st.GetContent(id)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != "archived" {
			t.Fatalf("content %d status = %s, want archived", id, got.Status)
		}
	}
}
