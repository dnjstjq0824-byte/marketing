package member

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"somun/internal/store"
)

// Public promo stats count recruiting listings, promoters, and posts.
func TestPromoStatsEndpoint(t *testing.T) {
	svc := newTestService(t)
	_, m, err := svc.Signup("s@t.com", "password-1", "nick")
	if err != nil {
		t.Fatal(err)
	}
	pid := seedProduct(t, svc, 1000, "https://priv.test/x")
	if _, err := svc.SetProductStatus(pid, store.ConsignRecruiting); err != nil {
		t.Fatal(err)
	}
	a, err := svc.Apply(m.ID, pid, "blog", "plan", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Decide(a.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RegisterPost(m.ID, a.ID, "https://blog.test/p1"); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(svc)
	rec := httptest.NewRecorder()
	h.Stats(rec, httptest.NewRequest("GET", "/api/promo/stats", nil))
	if rec.Code != 200 {
		t.Fatalf("code = %d", rec.Code)
	}
	var body map[string]int
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["recruiting"] != 1 || body["promoters"] != 1 || body["posts"] != 1 {
		t.Fatalf("stats = %v", body)
	}
}

// Empty stores report zeros, never nulls or errors.
func TestPromoStatsEmpty(t *testing.T) {
	svc := newTestService(t)
	rec := httptest.NewRecorder()
	NewHandler(svc).Stats(rec, httptest.NewRequest("GET", "/api/promo/stats", nil))
	var body map[string]int
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["recruiting"] != 0 || body["promoters"] != 0 || body["posts"] != 0 {
		t.Fatalf("stats = %v", body)
	}
}
