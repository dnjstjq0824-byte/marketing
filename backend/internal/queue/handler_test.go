package queue

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"somun/internal/agent"
	"somun/internal/store"
)

const testWorkerToken = "test-token"

func openHandlerStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := store.Migrate(st.DB()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return st
}

func testHandler(st *store.Store) *Handler {
	agentSvc := agent.NewService(st, agent.NewHub(), 600, 25)
	return NewHandler(NewService(st), agentSvc, testWorkerToken, true)
}

func authed(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+testWorkerToken)
	req.Header.Set("X-Worker-Name", "test-worker")
}

// W9: production refuses fake- workers on claim and result paths.
func TestFakeRefusedInProduction(t *testing.T) {
	st := openHandlerStore(t)
	h := testHandler(st)
	h.Production = true
	req := httptest.NewRequest("GET", "/api/worker/jobs/next?type=analyze", nil)
	req.Header.Set("Authorization", "Bearer "+testWorkerToken)
	req.Header.Set("X-Worker-Name", "fake-probe")
	rec := httptest.NewRecorder()
	h.Auth(http.HandlerFunc(h.Next)).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("fake claim: got %d, want 403", rec.Code)
	}
	req = httptest.NewRequest("POST", "/api/worker/jobs/1/result", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+testWorkerToken)
	req.Header.Set("X-Worker-Name", "fake-probe")
	req.SetPathValue("id", "1")
	rec = httptest.NewRecorder()
	h.Auth(http.HandlerFunc(h.Result)).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("fake result: got %d, want 403", rec.Code)
	}
	// Same fake worker is allowed outside production.
	h.Production = false
	req = httptest.NewRequest("GET", "/api/worker/jobs/next?type=analyze", nil)
	req.Header.Set("Authorization", "Bearer "+testWorkerToken)
	req.Header.Set("X-Worker-Name", "fake-probe")
	rec = httptest.NewRecorder()
	h.Auth(http.HandlerFunc(h.Next)).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("dev fake claim: got %d, want 200", rec.Code)
	}
}

func TestWorkerAuthRequired(t *testing.T) {
	st := openHandlerStore(t)
	h := testHandler(st)
	req := httptest.NewRequest("GET", "/api/worker/jobs", nil)
	rec := httptest.NewRecorder()
	h.Auth(http.HandlerFunc(h.List)).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token: got %d, want 401", rec.Code)
	}
	req = httptest.NewRequest("GET", "/api/worker/jobs", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rec = httptest.NewRecorder()
	h.Auth(http.HandlerFunc(h.List)).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad token: got %d, want 401", rec.Code)
	}
}

func TestNextEmptyMessage(t *testing.T) {
	st := openHandlerStore(t)
	h := testHandler(st)
	req := httptest.NewRequest("GET", "/api/worker/jobs/next?type=analyze", nil)
	authed(req)
	rec := httptest.NewRecorder()
	h.Next(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["job"] != nil {
		t.Fatalf("job = %v, want null", body["job"])
	}
	if body["message"] != EmptyQueueMessage {
		t.Fatalf("message = %q", body["message"])
	}
}

func TestNextBadType(t *testing.T) {
	st := openHandlerStore(t)
	h := testHandler(st)
	req := httptest.NewRequest("GET", "/api/worker/jobs/next?type=check", nil)
	authed(req)
	rec := httptest.NewRecorder()
	h.Next(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400", rec.Code)
	}
}

func TestResultDoneTwice400(t *testing.T) {
	st := openHandlerStore(t)
	p, _ := st.CreateProduct("https://example.com/x", "A", 0, "")
	j, _ := st.CreateJob(JobAnalyze, p.ID, "{}")
	if _, err := st.ClaimNextJob(JobAnalyze, "w"); err != nil {
		t.Fatal(err)
	}
	h := testHandler(st)
	first := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/api/worker/jobs/x/result",
			strings.NewReader(`{"product_name":"n"}`))
		authed(req)
		req.SetPathValue("id", itoa(j.ID))
		rec := httptest.NewRecorder()
		h.Result(rec, req)
		return rec
	}
	if rec := first(); rec.Code != http.StatusOK {
		t.Fatalf("first result: got %d (%s)", rec.Code, rec.Body.String())
	}
	if rec := first(); rec.Code != http.StatusBadRequest {
		t.Fatalf("second result: got %d, want 400", rec.Code)
	}
}

func TestFailRequiresReason(t *testing.T) {
	st := openHandlerStore(t)
	p, _ := st.CreateProduct("https://example.com/x", "A", 0, "")
	j, _ := st.CreateJob(JobAnalyze, p.ID, "{}")
	h := testHandler(st)
	req := httptest.NewRequest("POST", "/api/worker/jobs/x/fail", strings.NewReader(`{"error":""}`))
	authed(req)
	req.SetPathValue("id", itoa(j.ID))
	rec := httptest.NewRecorder()
	h.Fail(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400", rec.Code)
	}
}

func TestDefaultWorkerName(t *testing.T) {
	st := openHandlerStore(t)
	p, _ := st.CreateProduct("https://example.com/x", "A", 0, "")
	if _, err := st.CreateJob(JobAnalyze, p.ID, "{}"); err != nil {
		t.Fatal(err)
	}
	h := testHandler(st)
	req := httptest.NewRequest("GET", "/api/worker/jobs/next?type=analyze", nil)
	req.Header.Set("Authorization", "Bearer "+testWorkerToken)
	// No X-Worker-Name header.
	rec := httptest.NewRecorder()
	h.Next(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
	var body struct {
		Job *JobView `json:"job"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Job == nil || body.Job.WorkerName != DefaultWorkerName {
		t.Fatalf("worker = %+v, want %q", body.Job, DefaultWorkerName)
	}
}

func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}
