package content

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"somun/internal/store"
)

// Trend endpoint serves day buckets; bad days values refuse with 400.
func TestPublishTrendEndpoint(t *testing.T) {
	st := openContentStore(t)
	p, err := st.CreateProduct("https://example.com/t", "A", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	c, err := st.CreateContent(&store.Content{ProductID: p.ID, Channel: "blog"})
	if err != nil {
		t.Fatal(err)
	}
	exec := time.Now().UTC().Format(time.RFC3339)
	if _, err := st.CreatePublishDone(c.ID, "blog", "acc", exec, "ok"); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(NewService(st), st)
	rec := httptest.NewRecorder()
	h.PublishTrend(rec, httptest.NewRequest("GET", "/api/publish-trend?days=2", nil))
	if rec.Code != 200 {
		t.Fatalf("code = %d", rec.Code)
	}
	var body struct {
		Days []store.PublishTrendDay `json:"days"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Days) != 2 || body.Days[1].Published != 1 {
		t.Fatalf("days = %+v", body.Days)
	}
	for _, q := range []string{"?days=0", "?days=91", "?days=abc"} {
		rec := httptest.NewRecorder()
		h.PublishTrend(rec, httptest.NewRequest("GET", "/api/publish-trend"+q, nil))
		if rec.Code != 400 {
			t.Fatalf("%s code = %d", q, rec.Code)
		}
	}
}
