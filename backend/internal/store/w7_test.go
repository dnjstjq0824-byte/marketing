package store

import (
	"path/filepath"
	"testing"
	"time"
)

func openW7Store(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "w7.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := Migrate(st.DB()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return st
}

// seedW7Published builds business/contents Fluss: one product with one
// published post whose done log executed at execAgo with the given note.
func seedW7Published(t *testing.T, st *Store, business, note string, execAgo time.Duration) (prodID, contentID int64) {
	t.Helper()
	p, err := st.CreateProduct("https://example.com/w7", "A", 0, business)
	if err != nil {
		t.Fatal(err)
	}
	c, err := st.CreateContent(&Content{ProductID: p.ID, Channel: "blog", Title: "t", Body: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateContentStatus(c.ID, "queued"); err != nil {
		t.Fatal(err)
	}
	if err := st.MarkContentPublished(c.ID); err != nil {
		t.Fatal(err)
	}
	rec, err := st.CreatePublishLog(c.ID, "naver_blog", "w7acct", "scheduled", "")
	if err != nil {
		t.Fatal(err)
	}
	exec := time.Now().UTC().Add(-execAgo).Format(time.RFC3339)
	if err := st.MarkLogDone(rec.ID, exec, note); err != nil {
		t.Fatal(err)
	}
	return p.ID, c.ID
}

func TestCollectURL(t *testing.T) {
	cases := map[string]string{
		"http://127.0.0.1:18399/post/1":                      "http://127.0.0.1:18399/post/1",
		"https://blog.naver.com/x/1 | evidence: shots/a.png": "https://blog.naver.com/x/1",
		"  https://x.test/p  ":                               "https://x.test/p",
		"":                                                   "",
		"scheduled":                                          "",
		"ftp://x.test/p":                                     "",
		"no url here | evidence: shots/a.png":                "",
	}
	for in, want := range cases {
		if got := CollectURL(in); got != want {
			t.Errorf("CollectURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDueCollectsWindows(t *testing.T) {
	st := openW7Store(t)
	_, cid := seedW7Published(t, st, "w7biz", "https://x.test/post/1", 25*time.Hour)
	due, err := st.DueCollects(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 1 || due[0].ContentID != cid || due[0].Window != "24h" {
		t.Fatalf("due after 25h = %+v, want single 24h", due)
	}
	if due[0].URL != "https://x.test/post/1" || due[0].Account != "w7acct" || due[0].Business != "w7biz" {
		t.Fatalf("due fields = %+v", due[0])
	}
	// 72h crosses the second window too.
	if _, err := st.EnsureCollectWindow(cid, "24h", "w7acct", 1); err != nil {
		t.Fatal(err)
	}
	due, err = st.DueCollects(time.Now().Add(48 * time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 1 || due[0].Window != "72h" {
		t.Fatalf("due after 73h = %+v, want single 72h", due)
	}
	// Ledgered windows never repeat.
	if _, err := st.EnsureCollectWindow(cid, "72h", "w7acct", 2); err != nil {
		t.Fatal(err)
	}
	if _, err := st.EnsureCollectWindow(cid, "7d", "w7acct", 3); err != nil {
		t.Fatal(err)
	}
	due, err = st.DueCollects(time.Now().Add(30 * 24 * time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 0 {
		t.Fatalf("due after full ledger = %+v, want none", due)
	}
}

func TestDueCollectsSkips(t *testing.T) {
	st := openW7Store(t)
	// Too fresh: nothing due.
	seedW7Published(t, st, "w7biz", "https://x.test/post/9", time.Hour)
	// URL-less log: skipped even when old.
	seedW7Published(t, st, "w7biz", "scheduled", 200*time.Hour)
	due, err := st.DueCollects(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(due) != 0 {
		t.Fatalf("due = %+v, want none", due)
	}
}

func TestCollectWindowLedger(t *testing.T) {
	st := openW7Store(t)
	_, cid := seedW7Published(t, st, "w7biz", "https://x.test/post/7", 25*time.Hour)
	fresh, err := st.EnsureCollectWindow(cid, "24h", "a", 11)
	if err != nil || !fresh {
		t.Fatalf("first ensure = %v,%v, want true,nil", fresh, err)
	}
	fresh, err = st.EnsureCollectWindow(cid, "24h", "a", 12)
	if err != nil || fresh {
		t.Fatalf("replay ensure = %v,%v, want false,nil", fresh, err)
	}
	has, err := st.HasCollectWindow(cid, "24h", "a")
	if err != nil || !has {
		t.Fatalf("has = %v,%v, want true,nil", has, err)
	}
	if err := st.ReleaseCollectWindow(cid, "24h", "a"); err != nil {
		t.Fatal(err)
	}
	has, err = st.HasCollectWindow(cid, "24h", "a")
	if err != nil || has {
		t.Fatalf("after release has = %v,%v, want false,nil", has, err)
	}
	// Releasing a missing row is a no-op, not an error.
	if err := st.ReleaseCollectWindow(cid, "24h", "a"); err != nil {
		t.Fatal(err)
	}
}

func i64(v int64) *int64 { return &v }

func TestMetricPublicFieldsRoundtrip(t *testing.T) {
	st := openW7Store(t)
	pid, cid := seedW7Published(t, st, "w7biz", "https://x.test/post/2", 25*time.Hour)
	m, err := st.CreateMetric(&Metric{ContentID: &cid, Channel: "blog",
		Views: i64(120), Likes: i64(7), Comments: i64(3),
		Origin: CollectOrigin("24h"), CauseClass: "", Note: "stub"})
	if err != nil {
		t.Fatal(err)
	}
	if m.Views == nil || *m.Views != 120 || m.Likes == nil || *m.Likes != 7 ||
		m.Comments == nil || *m.Comments != 3 || m.Origin != "collect:24h" {
		t.Fatalf("metric = %+v", m)
	}
	got, err := st.GetMetric(m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Origin != "collect:24h" || got.Views == nil || *got.Views != 120 {
		t.Fatalf("reloaded = %+v", got)
	}
	// Card projection sums collected figures.
	rows, err := st.WorkPosts(pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Views != 120 || rows[0].Likes != 7 || rows[0].Comments != 3 {
		t.Fatalf("work posts = %+v", rows)
	}
	if rows[0].URL != "https://x.test/post/2" {
		t.Fatalf("work post url = %q", rows[0].URL)
	}
}

func TestTopMetricsByBusiness(t *testing.T) {
	st := openW7Store(t)
	mk := func(biz, title string, views, likes, comments int64) {
		p, err := st.CreateProduct("https://example.com/"+title, "A", 0, biz)
		if err != nil {
			t.Fatal(err)
		}
		c, err := st.CreateContent(&Content{ProductID: p.ID, Channel: "blog", Title: title, Body: "b"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.CreateMetric(&Metric{ContentID: &c.ID, Channel: "blog",
			Views: i64(views), Likes: i64(likes), Comments: i64(comments),
			Origin: "collect:24h"}); err != nil {
			t.Fatal(err)
		}
	}
	mk("w7biz", "low", 10, 1, 0)
	mk("w7biz", "high", 500, 40, 9)
	mk("w7biz", "mid", 100, 5, 2)
	mk("w7biz", "mid2", 90, 4, 1)
	mk("other", "rich", 9999, 999, 99)
	top, err := st.TopMetricsByBusiness("w7biz", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(top) != 3 {
		t.Fatalf("top = %+v, want 3", top)
	}
	if top[0].Title != "high" || top[1].Title != "mid" || top[2].Title != "mid2" {
		t.Fatalf("order = %s,%s,%s", top[0].Title, top[1].Title, top[2].Title)
	}
	if top[0].Views != 500 || top[0].Collected == "" {
		t.Fatalf("top[0] = %+v", top[0])
	}
	// Empty business and unknown business yield nothing, never an error.
	for _, biz := range []string{"", "nope"} {
		got, err := st.TopMetricsByBusiness(biz, 3)
		if err != nil || len(got) != 0 {
			t.Fatalf("business %q = %+v,%v", biz, got, err)
		}
	}
}

func TestTopMetricsLatestRoundPerPost(t *testing.T) {
	st := openW7Store(t)
	p, err := st.CreateProduct("https://example.com/r", "A", 0, "w7biz")
	if err != nil {
		t.Fatal(err)
	}
	c, err := st.CreateContent(&Content{ProductID: p.ID, Channel: "blog", Title: "r", Body: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateMetric(&Metric{ContentID: &c.ID, Channel: "blog", Views: i64(10), Origin: "collect:24h"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateMetric(&Metric{ContentID: &c.ID, Channel: "blog", Views: i64(400), Origin: "collect:72h"}); err != nil {
		t.Fatal(err)
	}
	top, err := st.TopMetricsByBusiness("w7biz", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(top) != 1 || top[0].Views != 400 {
		t.Fatalf("top = %+v, want single latest round", top)
	}
}
