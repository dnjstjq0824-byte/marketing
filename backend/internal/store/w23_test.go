package store

import (
	"path/filepath"
	"testing"
)

func openW23Test(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "w23.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := Migrate(st.DB()); err != nil {
		t.Fatal(err)
	}
	return st
}

// W23: v30 tables exist.
func TestMigrateV30Tables(t *testing.T) {
	st := openW23Test(t)
	for _, table := range []string{"members", "member_sessions", "consignment_products", "consignment_applications"} {
		var n int
		if err := st.DB().QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("missing table %s", table)
		}
	}
}

// W23: member and session rows round-trip; emails stay unique.
func TestMemberRoundTrip(t *testing.T) {
	st := openW23Test(t)
	m, err := st.CreateMember("a@e.com", "hash", "nick")
	if err != nil || m.ID == 0 {
		t.Fatalf("create = %+v err = %v", m, err)
	}
	if _, err := st.CreateMember("a@e.com", "hash", "nick"); err != ErrDuplicate {
		t.Fatalf("dup err = %v", err)
	}
	got, err := st.GetMemberByEmail("a@e.com")
	if err != nil || got.Nickname != "nick" {
		t.Fatalf("got = %+v err = %v", got, err)
	}
	if err := st.CreateMemberSession("tok", m.ID, "2030-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	sess, err := st.GetMemberSession("tok")
	if err != nil || sess.MemberID != m.ID {
		t.Fatalf("sess = %+v err = %v", sess, err)
	}
	if err := st.TouchMemberSession("tok", "2031-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	sess, _ = st.GetMemberSession("tok")
	if sess.ExpiresAt != "2031-01-01T00:00:00Z" {
		t.Fatalf("sess = %+v", sess)
	}
	if err := st.DeleteMemberSession("tok"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetMemberSession("tok"); err != ErrNotFound {
		t.Fatalf("deleted err = %v", err)
	}
}

// W23: product rows round-trip with status filter and counts.
func TestConsignProductRoundTrip(t *testing.T) {
	st := openW23Test(t)
	p, err := st.CreateConsignmentProduct(&ConsignmentProduct{Name: "nail", FeePerOrder: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != ConsignClosed {
		t.Fatalf("status = %s", p.Status)
	}
	p.Status = ConsignRecruiting
	if err := st.UpdateConsignmentProduct(p); err != nil {
		t.Fatal(err)
	}
	all, err := st.ListConsignmentProducts("")
	if err != nil || len(all) != 1 {
		t.Fatalf("all = %+v err = %v", all, err)
	}
	open, err := st.ListConsignmentProducts(ConsignRecruiting)
	if err != nil || len(open) != 1 {
		t.Fatalf("open = %+v err = %v", open, err)
	}
	shut, err := st.ListConsignmentProducts(ConsignClosed)
	if err != nil || len(shut) != 0 {
		t.Fatalf("shut = %+v err = %v", shut, err)
	}
	if n, _ := st.CountConsignmentProducts(""); n != 1 {
		t.Fatalf("count = %d", n)
	}
}

// W23: applications dedupe per member+product and decide cleanly.
func TestConsignApplicationFlow(t *testing.T) {
	st := openW23Test(t)
	p, err := st.CreateConsignmentProduct(&ConsignmentProduct{Name: "nail", FeePerOrder: 1000})
	if err != nil {
		t.Fatal(err)
	}
	m, err := st.CreateMember("a@e.com", "hash", "nick")
	if err != nil {
		t.Fatal(err)
	}
	a, err := st.CreateApplication(&ConsignmentApplication{ProductID: p.ID, MemberID: m.ID,
		Channels: "blog", Plan: "review", FeeAgreed: 1000})
	if err != nil || a.Status != ApplyPending {
		t.Fatalf("apply = %+v err = %v", a, err)
	}
	if _, err := st.CreateApplication(&ConsignmentApplication{ProductID: p.ID, MemberID: m.ID}); err != ErrDuplicate {
		t.Fatalf("dup err = %v", err)
	}
	mine, err := st.ListMemberApplications(m.ID)
	if err != nil || len(mine) != 1 {
		t.Fatalf("mine = %+v err = %v", mine, err)
	}
	if err := st.DecideApplication(a.ID, ApplyApproved, ""); err != nil {
		t.Fatal(err)
	}
	if n, _ := st.CountApplicationsByStatus(ApplyPending); n != 0 {
		t.Fatalf("pending = %d", n)
	}
	if n, _ := st.CountApprovedPromoters(); n != 1 {
		t.Fatalf("promoters = %d", n)
	}
}
