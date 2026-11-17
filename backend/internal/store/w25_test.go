package store

import (
	"path/filepath"
	"testing"
)

func openW25Test(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "w25.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := Migrate(st.DB()); err != nil {
		t.Fatal(err)
	}
	return st
}

// W25: v31 work tracking round-trips.
func TestWorkTrackingRoundTrip(t *testing.T) {
	st := openW25Test(t)
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
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DecideApplication(a.ID, ApplyApproved, ""); err != nil {
		t.Fatal(err)
	}
	post, err := st.AddConsignmentPost(a.ID, "https://blog.example.com/1")
	if err != nil || post.URL == "" {
		t.Fatalf("post = %+v err = %v", post, err)
	}
	posts, err := st.ListConsignmentPosts(a.ID)
	if err != nil || len(posts) != 1 {
		t.Fatalf("posts = %+v err = %v", posts, err)
	}
	if err := st.SetConfirmedOrders(a.ID, 3); err != nil {
		t.Fatal(err)
	}
	if err := st.SetConfirmedOrders(9999, 1); err != ErrNotFound {
		t.Fatalf("missing err = %v", err)
	}
	orders, total, err := st.MemberEarnings(m.ID)
	if err != nil || orders != 3 || total != 3000 {
		t.Fatalf("earnings = %d/%d err = %v", orders, total, err)
	}
	payable, err := st.PayableTotal()
	if err != nil || payable != 3000 {
		t.Fatalf("payable = %d err = %v", payable, err)
	}
}
