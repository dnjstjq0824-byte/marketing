package media

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"somun/internal/store"
)

func openW14Media(t *testing.T) (*store.Store, *Service, string) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "w14m.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := store.Migrate(st.DB()); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	return st, NewService(st, dir), dir
}

func roundTrip(t *testing.T, svc *Service, name string, body []byte) *store.Media {
	t.Helper()
	id, _, err := svc.Init(name, int64(len(body)), "x")
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
	return m
}

func TestSniffMatrix(t *testing.T) {
	ftyp := func(brand string) []byte {
		b := []byte("\x00\x00\x00\x18ftyp" + brand)
		return append(b, make([]byte, 64)...)
	}
	webm := append([]byte{0x1a, 0x45, 0xdf, 0xa3}, make([]byte, 64)...)
	cases := []struct {
		ext  string
		head []byte
		mime string
	}{
		{".png", append([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}, make([]byte, 64)...), "image/png"},
		{".jpg", append([]byte{0xff, 0xd8, 0xff, 0xe0}, make([]byte, 64)...), "image/jpeg"},
		{".gif", append([]byte("GIF89a"), make([]byte, 64)...), "image/gif"},
		{".webp", append([]byte("RIFF....WEBPVP8 "), make([]byte, 64)...), "image/webp"},
		{".heic", ftyp("heic"), "image/heic"},
		{".mp4", ftyp("isom"), "video/mp4"},
		{".mov", ftyp("qt  "), "video/quicktime"},
		{".webm", webm, "video/webm"},
		{".pdf", append([]byte("%PDF-1.7\n"), make([]byte, 64)...), "application/pdf"},
		{".txt", []byte("plain ascii text for the reading, only printable bytes here"), "text/plain; charset=utf-8"},
		{".md", []byte("# title\n\nplain markdown text here"), "text/plain; charset=utf-8"},
	}
	for _, c := range cases {
		mime, err := sniffMime(c.ext, c.head, "")
		if err != nil || mime != c.mime {
			t.Fatalf("%s: mime=%q err=%v", c.ext, mime, err)
		}
	}
	// Forgeries: text as image, pdf as text, zip without word/ as docx.
	text := bytes.Repeat([]byte("hello world, this is text. "), 10)
	if _, err := sniffMime(".jpg", text, ""); !errors.Is(err, ErrForgery) {
		t.Fatalf("text-as-jpg: %v", err)
	}
	if _, err := sniffMime(".txt", append([]byte("%PDF-1.4\n"), make([]byte, 64)...), ""); !errors.Is(err, ErrForgery) {
		t.Fatalf("pdf-as-txt: %v", err)
	}
	zwPath := filepath.Join(t.TempDir(), "z.zip")
	zf, err := os.Create(zwPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(zf)
	w, _ := zw.Create("readme.txt")
	_, _ = w.Write([]byte("hi"))
	_ = zw.Close()
	_ = zf.Close()
	raw, _ := os.ReadFile(zwPath)
	if _, err := sniffMime(".docx", raw[:512], zwPath); !errors.Is(err, ErrForgery) {
		t.Fatalf("zip-as-docx: %v", err)
	}
	// A real docx (zip + word/document.xml) passes.
	dxPath := filepath.Join(t.TempDir(), "d.docx")
	df, err := os.Create(dxPath)
	if err != nil {
		t.Fatal(err)
	}
	dx := zip.NewWriter(df)
	for _, name := range []string{"[Content_Types].xml", "word/document.xml"} {
		w, _ := dx.Create(name)
		_, _ = w.Write([]byte("<x/>"))
	}
	_ = dx.Close()
	_ = df.Close()
	draw, _ := os.ReadFile(dxPath)
	mime, err := sniffMime(".docx", draw, dxPath)
	if err != nil || mime != "application/vnd.openxmlformats-officedocument.wordprocessingml.document" {
		t.Fatalf("docx: %q %v", mime, err)
	}
	// Short/empty heads fail closed, unknown exts are bad uploads.
	if _, err := sniffMime(".mp4", []byte("tiny"), ""); !errors.Is(err, ErrForgery) {
		t.Fatalf("short: %v", err)
	}
	if _, err := sniffMime(".exe", make([]byte, 64), ""); !errors.Is(err, ErrBadUpload) {
		t.Fatalf("exe: %v", err)
	}
}

func TestW14RoundTrips(t *testing.T) {
	_, svc, _ := openW14Media(t)
	mp4 := append([]byte("\x00\x00\x00\x18ftypisom"), make([]byte, 200)...)
	m := roundTrip(t, svc, "clip.mp4", mp4)
	if m.Mime != "video/mp4" || m.Source != store.MediaSourceOriginal {
		t.Fatalf("mp4 = %+v", m)
	}
	txt := []byte("reading notes: plain text only, nothing binary at all here")
	m = roundTrip(t, svc, "notes.txt", txt)
	if m.Mime != "text/plain; charset=utf-8" {
		t.Fatalf("txt = %+v", m)
	}
	// Worker-marked generations record kind + prompt + account.
	p, _ := svc.store.CreateProduct("https://x.test/", "A", 0, "")
	acc, _ := svc.store.CreateAccountRow(&store.ChannelAccount{Name: "w14w", Platform: "meta_ai"})
	id, _, err := svc.Init("card.png", int64(len(pngBytes(100))), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.PutPart(id, 1, bytes.NewReader(pngBytes(100))); err != nil {
		t.Fatal(err)
	}
	g, err := svc.CompleteFull(id, "card", "local card", acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if g.Kind != "card" || g.Prompt != "local card" || g.Source != store.MediaSourceGenerated {
		t.Fatalf("gen = %+v", g)
	}
	if g.AIAccountID == nil || *g.AIAccountID != acc.ID {
		t.Fatalf("account = %+v", g)
	}
	_ = p
}

func TestW14Caps(t *testing.T) {
	_, svc, _ := openW14Media(t)
	if _, _, err := svc.Init("big.png", MaxImageBytes+1, "image/png"); !errors.Is(err, ErrBadUpload) {
		t.Fatalf("image cap: %v", err)
	}
	if _, _, err := svc.Init("big.mp4", MaxVideoBytes+1, "video/mp4"); !errors.Is(err, ErrBadUpload) {
		t.Fatalf("video cap: %v", err)
	}
	if _, _, err := svc.Init("big.pdf", MaxDocBytes+1, "application/pdf"); !errors.Is(err, ErrBadUpload) {
		t.Fatalf("doc cap: %v", err)
	}
	if _, _, err := svc.Init("ok.heic", 100, "image/heic"); err != nil {
		t.Fatalf("heic: %v", err)
	}
	if _, _, err := svc.Init("ok.docx", 100, "application/vnd.openxmlformats-officedocument.wordprocessingml.document"); err != nil {
		t.Fatalf("docx: %v", err)
	}
	if _, _, err := svc.Init("ok.md", 100, "text/markdown"); err != nil {
		t.Fatalf("md: %v", err)
	}
}

func TestW14Thumb(t *testing.T) {
	_, svc, dir := openW14Media(t)
	// A real 1x1 PNG decodes: dims recorded, JPEG thumb served.
	one := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x02,
		0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
		0x54, 0x08, 0xd7, 0x63, 0xf8, 0xff, 0xff, 0x3f, 0x00, 0x05, 0xfe, 0x02, 0xfe, 0xdc,
		0xcc, 0x59, 0xe7, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82}
	m := roundTrip(t, svc, "one.png", one)
	if m.Width != 1 || m.Height != 1 || m.ThumbPath == "" {
		t.Fatalf("thumb meta = %+v", m)
	}
	if _, err := os.Stat(filepath.Join(dir, m.ThumbPath)); err != nil {
		t.Fatalf("thumb file: %v", err)
	}
	if _, f, err := svc.OpenThumb(m.ID); err != nil {
		t.Fatalf("open thumb: %v", err)
	} else {
		f.Close()
	}
	// Fake PNG bytes (header + zeros) still store; the thumb fails soft.
	fake := roundTrip(t, svc, "fake.png", pngBytes(100))
	if fake.ThumbPath != "" || fake.Width != 0 {
		t.Fatalf("fake thumb = %+v", fake)
	}
	if _, _, err := svc.OpenThumb(fake.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("fake open: %v", err)
	}
}

func TestW14DownloadName(t *testing.T) {
	st, svc, _ := openW14Media(t)
	p, _ := st.CreateProduct("https://x.test/", "A", 0, "")
	c, _ := st.CreateContent(&store.Content{ProductID: p.ID, Channel: "blog", Title: "t", Body: "b"})
	m := roundTrip(t, svc, "a.png", pngBytes(64))
	if n := DownloadName(m, ""); n != "0_post_upload_1.png" {
		t.Fatalf("name = %q", n)
	}
	if err := st.LinkMediaContent(m.ID, c.ID, p.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := st.GetMedia(m.ID)
	if n := DownloadName(got, svc.contentChannel(got)); n != "1_blog_upload_1.png" {
		t.Fatalf("linked name = %q", n)
	}
	var buf bytes.Buffer
	if err := svc.Zip(&buf, []int64{m.ID}); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(zr.File) != 1 || zr.File[0].Name != "1_blog_upload_1.png" {
		t.Fatalf("zip = %+v", zr.File)
	}
}

func TestW14GenViaAI(t *testing.T) {
	st, svc, _ := openW14Media(t)
	acc, err := st.CreateAccountRow(&store.ChannelAccount{Name: "gen1", Platform: "meta_ai", Role: "worker", SessionState: "connected"})
	if err != nil {
		t.Fatal(err)
	}
	acc.Capabilities = `["text","image"]`
	if err := st.UpdateAccountRow(acc); err != nil {
		t.Fatal(err)
	}
	p, _ := st.CreateProduct("https://x.test/", "A", 0, "")
	c, _ := st.CreateContent(&store.Content{ProductID: p.ID, Channel: "instagram", Title: "t", Body: "b"})
	png := append([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}, bytes.Repeat([]byte{1}, 100)...)
	svc.genRunner = func(account, platform, kind, prompt, ratio, outDir string) (genFile, error) {
		if account != "gen1" || platform != "meta_ai" || kind != "gen_image" {
			t.Fatalf("runner args = %q %q %q", account, platform, kind)
		}
		dst := filepath.Join(outDir, "gen.png")
		if err := os.WriteFile(dst, png, 0o644); err != nil {
			t.Fatal(err)
		}
		return genFile{Path: dst, Width: 1024, Height: 1024}, nil
	}
	m, err := svc.GenViaAI(acc.ID, "gen_image", "a cat", "1:1", c.ID, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if m.Kind != "gen_image" || m.Prompt != "a cat" || m.Source != store.MediaSourceGenerated {
		t.Fatalf("gen = %+v", m)
	}
	if m.ContentID == nil || *m.ContentID != c.ID || m.AIAccountID == nil || *m.AIAccountID != acc.ID {
		t.Fatalf("links = %+v", m)
	}
	if m.Width != 1024 || m.Mime != "image/png" {
		t.Fatalf("meta = %+v", m)
	}
	d, err := svc.Detail(m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if d.AccountName != "gen1" || d.ContentTitle != "t" || d.Channel != "instagram" || d.Download == "" {
		t.Fatalf("detail = %+v", d)
	}
	// Refusals: bad kind/prompt, dead account, missing capability.
	if _, err := svc.GenViaAI(acc.ID, "nope", "x", "", 0, 0); !errors.Is(err, ErrGenBadKind) {
		t.Fatalf("kind: %v", err)
	}
	if _, err := svc.GenViaAI(acc.ID, "gen_image", "", "", 0, 0); !errors.Is(err, ErrGenBadPrompt) {
		t.Fatalf("prompt: %v", err)
	}
	if _, err := svc.GenViaAI(9999, "gen_image", "x", "", 0, 0); !errors.Is(err, ErrGenNoAccount) {
		t.Fatalf("account: %v", err)
	}
	if _, err := svc.GenViaAI(acc.ID, "gen_video", "x", "", 0, 0); !errors.Is(err, ErrGenNoCap) {
		t.Fatalf("cap: %v", err)
	}
	dead, _ := st.CreateAccountRow(&store.ChannelAccount{Name: "dead", Platform: "meta_ai", SessionState: "expired"})
	if _, err := svc.GenViaAI(dead.ID, "gen_image", "x", "", 0, 0); !errors.Is(err, ErrGenNotConnected) {
		t.Fatalf("connected: %v", err)
	}
}

func TestW14Sweep(t *testing.T) {
	st, svc, dir := openW14Media(t)
	old := time.Now().UTC().AddDate(0, 0, -40).Format(time.RFC3339)
	stale, err := st.CreateMediaFull(store.MediaAttrs{Kind: "gen_image", Path: "lib/old.png", ThumbPath: "thumbs/old.jpg", Bytes: 3, Mime: "image/png", SHA256: "s-old"})
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"lib/old.png", "thumbs/old.jpg"} {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.DB().Exec(`UPDATE media SET created_at=? WHERE id=?`, old, stale.ID); err != nil {
		t.Fatal(err)
	}
	n, err := svc.SweepStale(30, time.Now())
	if err != nil || n != 1 {
		t.Fatalf("sweep = %d %v", n, err)
	}
	if _, err := st.GetMedia(stale.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("row left: %v", err)
	}
	for _, rel := range []string{"lib/old.png", "thumbs/old.jpg"} {
		if _, err := os.Stat(filepath.Join(dir, rel)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("bytes left: %s", rel)
		}
	}
}
