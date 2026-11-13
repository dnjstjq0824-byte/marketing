package notify

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerListReadCycle(t *testing.T) {
	st := openNotifyStore(t)
	svc := NewService(st)
	h := NewHandler(svc)
	if _, err := svc.JobFailed(5, "analyze", "boom"); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/notifications?unread=1&limit=10", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list = %d, want 200", rec.Code)
	}
	var body struct {
		Items  []map[string]any `json:"items"`
		Unread int              `json:"unread"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Unread != 1 || len(body.Items) != 1 {
		t.Fatalf("list body = %+v", body)
	}
	readReq := httptest.NewRequest(http.MethodPost, "/api/notifications/1/read", nil)
	readReq.SetPathValue("id", "1")
	readRec := httptest.NewRecorder()
	h.Read(readRec, readReq)
	if readRec.Code != http.StatusOK {
		t.Fatalf("read = %d, want 200", readRec.Code)
	}
	missingReq := httptest.NewRequest(http.MethodPost, "/api/notifications/999/read", nil)
	missingReq.SetPathValue("id", "999")
	missingRec := httptest.NewRecorder()
	h.Read(missingRec, missingReq)
	if missingRec.Code != http.StatusNotFound {
		t.Fatalf("read missing = %d, want 404", missingRec.Code)
	}
	allRec := httptest.NewRecorder()
	h.ReadAll(allRec, httptest.NewRequest(http.MethodPost, "/api/notifications/read-all", nil))
	if allRec.Code != http.StatusOK {
		t.Fatalf("read-all = %d, want 200", allRec.Code)
	}
	var allBody map[string]any
	if err := json.Unmarshal(allRec.Body.Bytes(), &allBody); err != nil {
		t.Fatal(err)
	}
	if allBody["updated"] != float64(0) {
		t.Fatalf("read-all body = %+v, want updated 0", allBody)
	}
	badReq := httptest.NewRequest(http.MethodGet, "/api/notifications?limit=9999", nil)
	badRec := httptest.NewRecorder()
	h.List(badRec, badReq)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("bad limit = %d, want 400", badRec.Code)
	}
}
