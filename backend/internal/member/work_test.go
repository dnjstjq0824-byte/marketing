package member

import (
	"errors"
	"testing"
)

// W25: post registration is owner-and-approved only with URL shape check.
func TestRegisterPostRules(t *testing.T) {
	svc := newTestService(t)
	_, m1, _ := svc.Signup("a@e.com", "password-1", "n1")
	_, m2, _ := svc.Signup("b@e.com", "password-1", "n2")
	pid := seedProduct(t, svc, 1000, "https://x.example.com/1")
	if _, err := svc.SetProductStatus(pid, "recruiting"); err != nil {
		t.Fatal(err)
	}
	a1, _ := svc.Apply(m1.ID, pid, "blog", "review", true)
	if _, err := svc.RegisterPost(m1.ID, a1.ID, "notaurl"); !errors.Is(err, ErrBadURL) {
		t.Fatalf("url err = %v", err)
	}
	if _, err := svc.RegisterPost(m1.ID, a1.ID, "https://blog.example.com/1"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("pending err = %v", err)
	}
	if _, err := svc.Decide(a1.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RegisterPost(m2.ID, a1.ID, "https://blog.example.com/1"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("other err = %v", err)
	}
	post, err := svc.RegisterPost(m1.ID, a1.ID, "https://blog.example.com/1")
	if err != nil || post.URL == "" {
		t.Fatalf("post = %+v err = %v", post, err)
	}
}

// W25: workspace shows approved items with materials, posts, earnings.
func TestMyWorkView(t *testing.T) {
	svc := newTestService(t)
	_, m, _ := svc.Signup("a@e.com", "password-1", "n1")
	pid := seedProduct(t, svc, 1000, "https://x.example.com/1")
	if _, err := svc.SetProductStatus(pid, "recruiting"); err != nil {
		t.Fatal(err)
	}
	a, _ := svc.Apply(m.ID, pid, "blog", "review", true)
	empty := svc.MyWork(m.ID)
	if len(empty.Items) != 0 || empty.Total != 0 {
		t.Fatalf("pending work = %+v", empty)
	}
	if _, err := svc.Decide(a.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RegisterPost(m.ID, a.ID, "https://blog.example.com/1"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ConfirmOrders(a.ID, 4); err != nil {
		t.Fatal(err)
	}
	work := svc.MyWork(m.ID)
	if len(work.Items) != 1 || len(work.Items[0].Posts) != 1 {
		t.Fatalf("work = %+v", work)
	}
	if work.Orders != 4 || work.Total != 4000 {
		t.Fatalf("earnings = %d/%d", work.Orders, work.Total)
	}
	ov := svc.Overview()
	if ov["payableTotal"] != 4000 {
		t.Fatalf("overview = %+v", ov)
	}
}

// W25: order confirmation validates target and count.
func TestConfirmOrdersRules(t *testing.T) {
	svc := newTestService(t)
	_, m, _ := svc.Signup("a@e.com", "password-1", "n1")
	pid := seedProduct(t, svc, 1000, "https://x.example.com/1")
	if _, err := svc.SetProductStatus(pid, "recruiting"); err != nil {
		t.Fatal(err)
	}
	a, _ := svc.Apply(m.ID, pid, "blog", "review", true)
	if err := svc.ConfirmOrders(a.ID, 2); !errors.Is(err, ErrBadDecision) {
		t.Fatalf("pending err = %v", err)
	}
	if err := svc.ConfirmOrders(9999, 2); !errors.Is(err, ErrNoApplication) {
		t.Fatalf("missing err = %v", err)
	}
	if _, err := svc.Decide(a.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.ConfirmOrders(a.ID, -1); !errors.Is(err, ErrBadCount) {
		t.Fatalf("negative err = %v", err)
	}
	if err := svc.ConfirmOrders(a.ID, 2); err != nil {
		t.Fatal(err)
	}
}
