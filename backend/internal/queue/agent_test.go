package queue

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"somun/internal/agent"
	"somun/internal/store"
)

func agentHandler(t *testing.T) (*Handler, *store.Store, *agent.Service, string) {
	t.Helper()
	st := openHandlerStore(t)
	agentSvc := agent.NewService(st, agent.NewHub(), 600, 25)
	h := NewHandler(NewService(st), agentSvc, testWorkerToken, true)
	a, token, err := agentSvc.Register("wA", "A", "", "", "", 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = a
	return h, st, agentSvc, token
}

func agentAuthed(req *http.Request, token string) {
	req.Header.Set("Authorization", "Bearer "+token)
}

func seedAnalyzeJob(t *testing.T, st *store.Store) int64 {
	t.Helper()
	p, err := st.CreateProduct("https://example.com/x", "A", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	j, err := st.CreateJob(JobAnalyze, p.ID, "{}")
	if err != nil {
		t.Fatal(err)
	}
	return j.ID
}

// Agent claims route by lane and stamp the agent label, not the header.
func TestAgentClaimRouted(t *testing.T) {
	h, st, _, token := agentHandler(t)
	jid := seedAnalyzeJob(t, st)
	req := httptest.NewRequest("GET", "/api/worker/jobs/next", nil)
	agentAuthed(req, token)
	req.Header.Set("X-Worker-Name", "spoofed")
	rec := httptest.NewRecorder()
	h.Auth(http.HandlerFunc(h.Next)).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("claim = %d", rec.Code)
	}
	got, _ := st.GetJob(jid)
	if got.Status != "claimed" || got.WorkerName != "wA" || got.LeaseUntil == "" {
		t.Fatalf("claimed = %+v", got)
	}
	// Lane violation is forbidden.
	bad := httptest.NewRequest("GET", "/api/worker/jobs/next?type=generate", nil)
	agentAuthed(bad, token)
	badRec := httptest.NewRecorder()
	h.Auth(http.HandlerFunc(h.Next)).ServeHTTP(badRec, bad)
	if badRec.Code != http.StatusForbidden {
		t.Fatalf("lane violation = %d, want 403", badRec.Code)
	}
}

// Agents touch only their own claims; paused agents are refused outright.
func TestAgentOwnershipAndPause(t *testing.T) {
	h, st, agentSvc, token := agentHandler(t)
	jid := seedAnalyzeJob(t, st)
	claim := httptest.NewRequest("GET", "/api/worker/jobs/next", nil)
	agentAuthed(claim, token)
	h.Auth(http.HandlerFunc(h.Next)).ServeHTTP(httptest.NewRecorder(), claim)

	_, token2, err := agentSvc.Register("wB", "A", "", "", "", 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	foreign := httptest.NewRequest("POST", "/api/worker/jobs/x/result", strings.NewReader(`{}`))
	foreign.SetPathValue("id", strconv.FormatInt(jid, 10))
	agentAuthed(foreign, token2)
	foreignRec := httptest.NewRecorder()
	h.Auth(http.HandlerFunc(h.Result)).ServeHTTP(foreignRec, foreign)
	if foreignRec.Code != http.StatusConflict {
		t.Fatalf("foreign submit = %d, want 409", foreignRec.Code)
	}
	if _, err := agentSvc.Bulk([]int64{1}, "pause", ""); err != nil {
		t.Fatal(err)
	}
	paused := httptest.NewRequest("GET", "/api/worker/jobs/next", nil)
	agentAuthed(paused, token)
	pausedRec := httptest.NewRecorder()
	h.Auth(http.HandlerFunc(h.Next)).ServeHTTP(pausedRec, paused)
	if pausedRec.Code != http.StatusForbidden {
		t.Fatalf("paused claim = %d, want 403", pausedRec.Code)
	}
}
