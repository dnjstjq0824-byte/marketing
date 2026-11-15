package media

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"somun/internal/store"
)

func openHandlerStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "mh.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := store.Migrate(st.DB()); err != nil {
		t.Fatal(err)
	}
	return st
}

func seedFile(t *testing.T, svc *Service) int64 {
	t.Helper()
	body := pngBytes(64)
	id, _, err := svc.Init("s.png", int64(len(body)), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.PutPart(id, 1, bytes.NewReader(body)); err != nil {
		t.Fatal(err)
	}
	m, err := svc.Complete(id)
	if err != nil {
		t.Fatal(err)
	}
	return m.ID
}

// W5-3: signed downloads serve bytes; bad signatures get 403.
func TestSignedFile(t *testing.T) {
	st := openHandlerStore(t)
	svc := NewService(st, t.TempDir())
	svc.FileKey = []byte("0123456789abcdef0123456789abcdef")
	h := NewHandler(svc)
	mid := seedFile(t, svc)
	exp := time.Now().Add(time.Hour).Unix()
	u := SignFileURL(svc.FileKey, mid, exp)

	midStr := pathIDOf(t, u)
	get := func(target string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", target, nil)
		req.SetPathValue("id", midStr)
		rec := httptest.NewRecorder()
		h.SignedFile(rec, req)
		return rec
	}
	rec := get(u)
	if rec.Code != http.StatusOK {
		t.Fatalf("valid: code=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("X-SHA256") == "" {
		t.Fatal("missing hash header")
	}
	bad := get(u + "0")
	if bad.Code != http.StatusForbidden {
		t.Fatalf("tampered: code=%d", bad.Code)
	}
	past := SignFileURL(svc.FileKey, mid, time.Now().Add(-time.Hour).Unix())
	if rec := get(past); rec.Code != http.StatusForbidden {
		t.Fatalf("expired: code=%d", rec.Code)
	}
	svc.FileKey = nil
	if rec := get(u); rec.Code != http.StatusForbidden {
		t.Fatalf("no key: code=%d", rec.Code)
	}
}

func pathIDOf(t *testing.T, target string) string {
	t.Helper()
	// target looks like /api/media/<id>/signed?...; extract <id>.
	var id string
	rest := target[len("/api/media/"):]
	for i := 0; i < len(rest); i++ {
		if rest[i] < '0' || rest[i] > '9' {
			break
		}
		id += string(rest[i])
	}
	if id == "" {
		t.Fatalf("no id in %q", target)
	}
	return id
}
