package scheduler

import (
	"encoding/json"
	"testing"

	"somun/internal/notify"
	"somun/internal/store"
)

func setRelay(t *testing.T, st *store.Store, name, backend, relay string) {
	t.Helper()
	row, err := st.GetAccountRowByName(name)
	if err != nil {
		t.Fatal(err)
	}
	row.ExecutorBackend, row.RelayAccount = backend, relay
	if err := st.UpdateAccountRow(row); err != nil {
		t.Fatal(err)
	}
}

func setSession(t *testing.T, st *store.Store, name, state string) {
	t.Helper()
	row, err := st.GetAccountRowByName(name)
	if err != nil {
		t.Fatal(err)
	}
	row.SessionState = state
	if err := st.UpdateAccountRow(row); err != nil {
		t.Fatal(err)
	}
}

// W22: a relay-backend channel qualifies on the relay AI session, even
// when its own channel session is expired.
func TestAssignRelayUsesRelaySession(t *testing.T) {
	st := openTestStore(t)
	svc := NewService(st, notify.NewService(st))
	seedAccount(t, st, "nb1", "naver_blog", "expired", "auto")
	seedAIAccount(t, st, "cap1", "muse")
	setRelay(t, st, "nb1", "muse", "cap1")
	cid := seedApproved(t, st, "LUNO", blogBrief, "blog")
	res, err := svc.Assign(cid)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Assigned || res.Account != "nb1" {
		t.Fatalf("result = %+v", res)
	}
	// The relay session dropping out disqualifies the channel again.
	setSession(t, st, "cap1", "expired")
	cid2 := seedApproved(t, st, "LUNO", blogBrief, "blog")
	res2, err := svc.Assign(cid2)
	if err != nil {
		t.Fatal(err)
	}
	if res2.Assigned {
		t.Fatalf("assigned despite dead relay: %+v", res2)
	}
}

// W22: publish payloads carry the backend routing the worker executes.
func TestPublishPayloadCarriesBackend(t *testing.T) {
	st := openTestStore(t)
	svc := NewService(st, notify.NewService(st))
	seedAccount(t, st, "nb1", "naver_blog", "expired", "auto")
	seedAIAccount(t, st, "cap1", "muse")
	setRelay(t, st, "nb1", "muse", "cap1")
	cid := seedApproved(t, st, "LUNO", blogBrief, "blog")
	if _, err := svc.Assign(cid); err != nil {
		t.Fatal(err)
	}
	jobs, err := st.ListJobsByRef(cid)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("jobs = %+v err = %v", jobs, err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(jobs[0].Payload), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["backend"] != "muse" || payload["relay_account"] != "cap1" {
		t.Fatalf("payload = %s", jobs[0].Payload)
	}
	// Direct accounts keep routing explicitly without a relay name.
	seedAccount(t, st, "nb2", "naver_blog", "connected", "auto")
	cid2 := seedApproved(t, st, "LUNO", blogBrief, "blog")
	res, err := svc.Assign(cid2)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Assigned {
		t.Fatalf("direct not assigned: %+v", res)
	}
	jobs2, err := st.ListJobsByRef(cid2)
	if err != nil || len(jobs2) != 1 {
		t.Fatalf("jobs = %+v err = %v", jobs2, err)
	}
	var payload2 map[string]any
	if err := json.Unmarshal([]byte(jobs2[0].Payload), &payload2); err != nil {
		t.Fatal(err)
	}
	if payload2["backend"] != "direct" {
		t.Fatalf("payload = %s", jobs2[0].Payload)
	}
	if _, ok := payload2["relay_account"]; ok {
		t.Fatalf("direct carries relay: %s", jobs2[0].Payload)
	}
}
