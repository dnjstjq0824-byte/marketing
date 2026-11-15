package media

import (
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"somun/internal/store"
)

func openMediaStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := store.Migrate(st.DB()); err != nil {
		t.Fatal(err)
	}
	return st
}

func pngBytes(n int) []byte {
	head := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	out := make([]byte, 0, len(head)+n)
	out = append(out, head...)
	for len(out) < len(head)+n {
		out = append(out, 0x00)
	}
	return out
}

// W5-3: signed URLs verify, tampered and expired ones fail closed.
func TestSignedURL(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	future := time.Now().Add(time.Hour).Unix()
	u := SignFileURL(key, 7, future)
	if !strings.HasPrefix(u, "/api/media/7/signed?exp=") || !strings.Contains(u, "&sig=") {
		t.Fatalf("url = %q", u)
	}
	parts := strings.Split(u[strings.Index(u, "?")+1:], "&")
	q := map[string]string{}
	for _, p := range parts {
		kv := strings.SplitN(p, "=", 2)
		q[kv[0]] = kv[1]
	}
	if !VerifyFileURL(key, 7, q["exp"], q["sig"]) {
		t.Fatal("valid signature rejected")
	}
	if VerifyFileURL(key, 7, q["exp"], q["sig"]+"0") {
		t.Fatal("tampered signature accepted")
	}
	if VerifyFileURL(key, 8, q["exp"], q["sig"]) {
		t.Fatal("swapped id accepted")
	}
	past := time.Now().Add(-time.Hour).Unix()
	old := SignFileURL(key, 7, past)
	oldsig := old[strings.Index(old, "sig=")+4:]
	if VerifyFileURL(key, 7, strconv.FormatInt(past, 10), oldsig) {
		t.Fatal("expired signature accepted")
	}
	if VerifyFileURL(nil, 7, q["exp"], q["sig"]) {
		t.Fatal("empty key accepted")
	}
}

func TestUploadRoundTrip(t *testing.T) {
	st := openMediaStore(t)
	svc := NewService(st, t.TempDir())
	body := pngBytes(100)
	id, total, err := svc.Init("a.png", int64(len(body)), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Fatalf("total = %d", total)
	}
	if _, _, err := svc.PutPart(id, 1, bytes.NewReader(body)); err != nil {
		t.Fatal(err)
	}
	m, err := svc.Complete(id)
	if err != nil {
		t.Fatal(err)
	}
	if m.Bytes != int64(len(body)) || m.Mime != "image/png" || m.Kind != "upload" {
		t.Fatalf("media = %+v", m)
	}
	if m.SHA256 == "" || m.Path == "" {
		t.Fatalf("media = %+v", m)
	}
	// Duplicate content returns the same row.
	id2, _, err := svc.Init("b.png", int64(len(body)), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.PutPart(id2, 1, bytes.NewReader(body)); err != nil {
		t.Fatal(err)
	}
	dup, err := svc.Complete(id2)
	if err != nil {
		t.Fatal(err)
	}
	if dup.ID != m.ID {
		t.Fatalf("dedupe = %+v, want %+v", dup, m)
	}
	// Attach links to a product; double attach refuses.
	p, err := st.CreateProduct("https://x.test/", "A", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AttachMedia([]int64{m.ID}, p.ID); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetMedia(m.ID)
	if err != nil || got.ProductID == nil || *got.ProductID != p.ID {
		t.Fatalf("attached = %+v %v", got, err)
	}
	if err := st.AttachMedia([]int64{m.ID}, p.ID); !errors.Is(err, store.ErrDuplicate) {
		t.Fatalf("double attach = %v", err)
	}
}

func TestLibraryCRUD(t *testing.T) {
	st := openMediaStore(t)
	dir := t.TempDir()
	svc := NewService(st, dir)
	body := pngBytes(64)
	id, _, err := svc.Init("a.png", int64(len(body)), "image/png")
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
	items, err := svc.List("", 0, "", 1, 48)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != m.ID {
		t.Fatalf("list = %+v", items)
	}
	items, err = svc.List("gen_image", 0, "", 1, 48)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("kind filter = %+v", items)
	}
	d, err := svc.Detail(m.ID)
	if err != nil || d.Business != "" {
		t.Fatalf("detail = %+v %v", d, err)
	}
	got, f, err := svc.Open(m.ID)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, body) || got.SHA256 != m.SHA256 {
		t.Fatal("bytes mismatch")
	}
	var buf bytes.Buffer
	if err := svc.Zip(&buf, []int64{m.ID}); err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 || buf.Bytes()[0] != 'P' || buf.Bytes()[1] != 'K' {
		t.Fatal("not a zip")
	}
	if err := svc.Delete(m.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Detail(m.ID); err == nil {
		t.Fatal("still there")
	}
	// Attached files refuse deletion.
	m2id, _, err := svc.Init("b.png", int64(len(body)), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.PutPart(m2id, 1, bytes.NewReader(body)); err != nil {
		t.Fatal(err)
	}
	m2, err := svc.Complete(m2id)
	if err != nil {
		t.Fatal(err)
	}
	p, err := st.CreateProduct("https://x.test/", "A", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AttachMedia([]int64{m2.ID}, p.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(m2.ID); !errors.Is(err, store.ErrAttached) {
		t.Fatalf("attached delete = %v", err)
	}
}

func TestUploadGuards(t *testing.T) {
	st := openMediaStore(t)
	svc := NewService(st, t.TempDir())
	if _, _, err := svc.Init("x.exe", 10, "application/x-ms"); !errors.Is(err, ErrBadUpload) {
		t.Fatalf("bad ext: %v", err)
	}
	if _, _, err := svc.Init("big.png", MaxImageBytes+1, "image/png"); !errors.Is(err, ErrBadUpload) {
		t.Fatalf("oversize: %v", err)
	}
	id, _, err := svc.Init("ok.png", 100, "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.PutPart(id, 2, bytes.NewReader(make([]byte, 10))); !errors.Is(err, ErrOutOfOrder) {
		t.Fatalf("order: %v", err)
	}
	if _, _, err := svc.PutPart("nope", 1, bytes.NewReader(make([]byte, 10))); !errors.Is(err, ErrNoSession) {
		t.Fatalf("session: %v", err)
	}
	if _, err := svc.Complete(id); !errors.Is(err, ErrIncomplete) {
		t.Fatalf("incomplete: %v", err)
	}
	// Forged content (text named .jpg) is rejected.
	fake := bytes.Repeat([]byte("hello world, this is text. "), 10)
	fid, _, err := svc.Init("fake.jpg", int64(len(fake)), "image/jpeg")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.PutPart(fid, 1, bytes.NewReader(fake)); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Complete(fid); !errors.Is(err, ErrForgery) {
		t.Fatalf("forgery: %v", err)
	}
}
