package queue

import (
	"testing"

	"somun/internal/store"
)

// P0-3: recording a failure fires the notify hook with the failed job.
func TestFailFiresNotifyHook(t *testing.T) {
	st := openTestStore(t)
	pid := seedProduct(t, st, "A")
	j, err := st.CreateJob(JobAnalyze, pid, "{}")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimNextJob(JobAnalyze, "w"); err != nil {
		t.Fatal(err)
	}
	svc := NewService(st)
	var got *store.Job
	svc.OnFail = func(job *store.Job) { got = job }
	if _, err := svc.Fail(j.ID, "nope"); err != nil {
		t.Fatal(err)
	}
	if got == nil || got.ID != j.ID || got.Status != "failed" {
		t.Fatalf("hook got %+v", got)
	}
}
