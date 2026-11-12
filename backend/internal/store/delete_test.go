package store

import (
	"errors"
	"path/filepath"
	"testing"
)

func openDeleteStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "d.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := Migrate(st.DB()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return st
}

func seedFullProduct(t *testing.T, st *Store) int64 {
	t.Helper()
	p, err := st.CreateProduct("https://example.com/del", "B", 0, "testbiz")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddImage(p.ID, "a.png", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertReading(&Reading{ProductID: p.ID, ProductName: "n"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateAffiliateLink(p.ID, "partner", "https://example.com/go"); err != nil {
		t.Fatal(err)
	}
	c, err := st.CreateContent(&Content{ProductID: p.ID, Channel: "blog", Title: "t", Body: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreatePublishLog(c.ID, "blog", "acc", "scheduled", ""); err != nil {
		t.Fatal(err)
	}
	exp := int64(10)
	if _, err := st.CreateMetric(&Metric{ContentID: &c.ID, Exposures: &exp}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateJob("analyze", p.ID, "{}"); err != nil {
		t.Fatal(err)
	}
	return p.ID
}

// Deleting a product removes its reading, images, contents, affiliate links,
// jobs, and (via contents) publish logs and metrics.
func TestDeleteProductCascade(t *testing.T) {
	st := openDeleteStore(t)
	pid := seedFullProduct(t, st)
	if err := st.DeleteProduct(pid); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetProduct(pid); !errors.Is(err, ErrNotFound) {
		t.Fatalf("product: err=%v", err)
	}
	if _, err := st.GetReadingByProduct(pid); !errors.Is(err, ErrNotFound) {
		t.Fatalf("reading: err=%v", err)
	}
	if imgs, _ := st.ListImages(pid); len(imgs) != 0 {
		t.Fatalf("images left: %d", len(imgs))
	}
	if links, _ := st.ListAffiliateLinks(pid); len(links) != 0 {
		t.Fatalf("affiliates left: %d", len(links))
	}
	if jobs, _ := st.ListJobsByRef(pid); len(jobs) != 0 {
		t.Fatalf("jobs left: %d", len(jobs))
	}
	if items, _ := st.ListContents("", "", 100); len(items) != 0 {
		t.Fatalf("contents left: %d", len(items))
	}
	if logs, _ := st.ListPublishLogs(100); len(logs) != 0 {
		t.Fatalf("publish logs left: %d", len(logs))
	}
	if ms, _ := st.ListMetrics(100); len(ms) != 0 {
		t.Fatalf("metrics left: %d", len(ms))
	}
	// Second delete reports not-found.
	if err := st.DeleteProduct(pid); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete: err=%v", err)
	}
	if err := st.DeleteProduct(999999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing delete: err=%v", err)
	}
}

// Deleting a content removes its publish logs and metrics, nothing else.
func TestDeleteContentCascade(t *testing.T) {
	st := openDeleteStore(t)
	pid := seedFullProduct(t, st)
	items, _ := st.ListContents("", "", 10)
	if len(items) != 1 {
		t.Fatalf("seeded contents = %d", len(items))
	}
	if err := st.DeleteContent(items[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetContent(items[0].ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("content: err=%v", err)
	}
	if logs, _ := st.ListPublishLogs(100); len(logs) != 0 {
		t.Fatalf("publish logs left: %d", len(logs))
	}
	if ms, _ := st.ListMetrics(100); len(ms) != 0 {
		t.Fatalf("metrics left: %d", len(ms))
	}
	// The product and its other rows survive.
	if _, err := st.GetProduct(pid); err != nil {
		t.Fatalf("product gone: %v", err)
	}
	if err := st.DeleteContent(items[0].ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete: err=%v", err)
	}
}

// Products update in place; missing ids report not-found.
func TestUpdateProduct(t *testing.T) {
	st := openDeleteStore(t)
	pid := seedFullProduct(t, st)
	if err := st.UpdateProduct(pid, "new name", "https://example.com/new", "A", 3, "biz2"); err != nil {
		t.Fatal(err)
	}
	p, _ := st.GetProduct(pid)
	if p.Name != "new name" || p.SourceURL != "https://example.com/new" || p.ExitType != "A" || p.Priority != 3 || p.Business != "biz2" {
		t.Fatalf("updated product = %+v", p)
	}
	if err := st.UpdateProduct(999999, "x", "https://example.com/x", "A", 0, ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing update: err=%v", err)
	}
}

// Images delete one row at a time; siblings survive.
func TestDeleteProductImage(t *testing.T) {
	st := openDeleteStore(t)
	pid := seedFullProduct(t, st)
	second, err := st.AddImage(pid, "b.png", 1)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := st.ListImages(pid)
	if len(first) != 2 {
		t.Fatalf("seeded images = %d", len(first))
	}
	if err := st.DeleteProductImage(first[0].ID); err != nil {
		t.Fatal(err)
	}
	rest, _ := st.ListImages(pid)
	if len(rest) != 1 || rest[0].ID != second.ID {
		t.Fatalf("remaining images = %+v", rest)
	}
	if _, err := st.GetProductImage(first[0].ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted image: err=%v", err)
	}
}

// Readings update field by field for operator fixes.
func TestUpdateReading(t *testing.T) {
	st := openDeleteStore(t)
	pid := seedFullProduct(t, st)
	before, _ := st.GetReadingByProduct(pid)
	before.PriceSummary = "24,900원"
	before.Unknowns = ""
	if err := st.UpdateReadingByProduct(before); err != nil {
		t.Fatal(err)
	}
	after, _ := st.GetReadingByProduct(pid)
	if after.PriceSummary != "24,900원" || after.Unknowns != "" || after.ProductName != "n" {
		t.Fatalf("updated reading = %+v", after)
	}
	if err := st.UpdateReadingByProduct(&Reading{ProductID: 999999}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing update: err=%v", err)
	}
}

// Publish logs update note/status and delete individually.
func TestPublishLogUpdateDelete(t *testing.T) {
	st := openDeleteStore(t)
	pid := seedFullProduct(t, st)
	items, _ := st.ListContents("", "", 10)
	rec, err := st.CreatePublishLog(items[0].ID, "instagram", "acc", "scheduled", "first")
	if err != nil {
		t.Fatal(err)
	}
	_ = pid
	if err := st.UpdatePublishLog(rec.ID, "held", "waiting"); err != nil {
		t.Fatal(err)
	}
	got, _ := st.GetPublishLogByID(rec.ID)
	if got.Status != "held" || got.Note != "waiting" {
		t.Fatalf("updated log = %+v", got)
	}
	if err := st.DeletePublishLog(rec.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetPublishLogByID(rec.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted log: err=%v", err)
	}
}

// Metrics and jobs delete individually.
func TestMetricAndJobDelete(t *testing.T) {
	st := openDeleteStore(t)
	pid := seedFullProduct(t, st)
	ms, _ := st.ListMetrics(10)
	if err := st.DeleteMetric(ms[0].ID); err != nil {
		t.Fatal(err)
	}
	if rest, _ := st.ListMetrics(10); len(rest) != 0 {
		t.Fatalf("metrics left: %d", len(rest))
	}
	jobs, _ := st.ListJobsByRef(pid)
	if err := st.DeleteJob(jobs[0].ID); err != nil {
		t.Fatal(err)
	}
	if rest, _ := st.ListJobsByRef(pid); len(rest) != 0 {
		t.Fatalf("jobs left: %d", len(rest))
	}
	if err := st.DeleteMetric(999999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing metric: err=%v", err)
	}
}

// Affiliate links update partner and URL.
func TestUpdateAffiliateLink(t *testing.T) {
	st := openDeleteStore(t)
	pid := seedFullProduct(t, st)
	links, _ := st.ListAffiliateLinks(pid)
	if err := st.UpdateAffiliateLink(links[0].ID, "new partner", "https://example.com/new"); err != nil {
		t.Fatal(err)
	}
	got, _ := st.GetAffiliateLink(links[0].ID)
	if got.PartnerName != "new partner" || got.URL != "https://example.com/new" {
		t.Fatalf("updated link = %+v", got)
	}
}

// Users list, count, and delete; passwords update by hash.
func TestUserCRUD(t *testing.T) {
	st := openDeleteStore(t)
	u, err := st.CreateUser("op1", "hash1", "operator")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateUser("op2", "hash2", "operator"); err != nil {
		t.Fatal(err)
	}
	if n, _ := st.CountUsers(); n != 2 {
		t.Fatalf("count = %d", n)
	}
	if err := st.UpdateUserPassword(u.ID, "hash-new"); err != nil {
		t.Fatal(err)
	}
	got, _ := st.GetUserByUsername("op1")
	if got.PasswordHash != "hash-new" {
		t.Fatal("password hash not updated")
	}
	if err := st.DeleteUser(u.ID); err != nil {
		t.Fatal(err)
	}
	if n, _ := st.CountUsers(); n != 1 {
		t.Fatalf("count after delete = %d", n)
	}
}

// Business filters isolate one line's products and contents.
func TestBusinessFilter(t *testing.T) {
	st := openDeleteStore(t)
	pa, err := st.CreateProduct("https://example.com/a", "A", 0, "bizA")
	if err != nil {
		t.Fatal(err)
	}
	pb, err := st.CreateProduct("https://example.com/b", "A", 0, "bizB")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateContent(&Content{ProductID: pa.ID, Channel: "blog", Body: "a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateContent(&Content{ProductID: pb.ID, Channel: "blog", Body: "b"}); err != nil {
		t.Fatal(err)
	}
	prods, _ := st.ListProducts(10, "bizA")
	if len(prods) != 1 || prods[0].ID != pa.ID {
		t.Fatalf("products = %+v", prods)
	}
	items, _ := st.ListContents("", "bizB", 10)
	if len(items) != 1 || items[0].ProductID != pb.ID {
		t.Fatalf("contents = %+v", items)
	}
	all, _ := st.ListContents("", "", 10)
	if len(all) != 2 {
		t.Fatalf("unfiltered = %d", len(all))
	}
}

// Pre-v2 databases gain the business column with data intact.
func TestMigrateV2AddsBusiness(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "v1.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	v1 := `CREATE TABLE products (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL DEFAULT '',
		source_url TEXT NOT NULL DEFAULT '',
		exit_type TEXT NOT NULL DEFAULT 'A',
		priority INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);`
	if _, err := st.DB().Exec(v1); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`INSERT INTO products(name,source_url,created_at,updated_at)
		VALUES('old','https://example.com/o','t','t')`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(st.DB()); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM pragma_table_info('products') WHERE name='business'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("business column missing: n=%d err=%v", n, err)
	}
	p, err := st.GetProduct(1)
	if err != nil || p.Name != "old" || p.Business != "" {
		t.Fatalf("migrated product = %+v %v", p, err)
	}
}

// Dashboard aggregates per business line.
func TestDashboardByBusiness(t *testing.T) {
	st := openDeleteStore(t)
	pa, _ := st.CreateProduct("https://example.com/a", "A", 0, "bizA")
	pb, _ := st.CreateProduct("https://example.com/b", "A", 0, "bizB")
	ca, _ := st.CreateContent(&Content{ProductID: pa.ID, Channel: "blog", Body: "a"})
	cb, _ := st.CreateContent(&Content{ProductID: pb.ID, Channel: "blog", Body: "b"})
	if err := st.UpdateContentStatus(ca.ID, "queued"); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateContentStatus(cb.ID, "review"); err != nil {
		t.Fatal(err)
	}
	rows, err := st.DashboardByBusiness()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]*BusinessCounts{}
	for _, r := range rows {
		got[r.Business] = r
	}
	if got["bizA"] == nil || got["bizA"].Products != 1 || got["bizA"].Queued != 1 || got["bizA"].Review != 0 {
		t.Fatalf("bizA = %+v", got["bizA"])
	}
	if got["bizB"] == nil || got["bizB"].Products != 1 || got["bizB"].Queued != 0 || got["bizB"].Review != 1 {
		t.Fatalf("bizB = %+v", got["bizB"])
	}
}

// Affiliate links delete individually; siblings survive.
func TestDeleteAffiliateLink(t *testing.T) {
	st := openDeleteStore(t)
	pid := seedFullProduct(t, st)
	second, err := st.CreateAffiliateLink(pid, "p2", "https://example.com/go2")
	if err != nil {
		t.Fatal(err)
	}
	first, err := st.ListAffiliateLinks(pid)
	if err != nil || len(first) != 2 {
		t.Fatalf("seeded links = %d", len(first))
	}
	got, err := st.GetAffiliateLink(second.ID)
	if err != nil || got.ProductID != pid {
		t.Fatalf("get link: %+v %v", got, err)
	}
	if err := st.DeleteAffiliateLink(first[0].ID); err != nil {
		t.Fatal(err)
	}
	rest, _ := st.ListAffiliateLinks(pid)
	if len(rest) != 1 || rest[0].ID != second.ID {
		t.Fatalf("remaining links = %+v", rest)
	}
	if err := st.DeleteAffiliateLink(first[0].ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete: err=%v", err)
	}
}
