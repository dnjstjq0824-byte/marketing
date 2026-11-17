package member

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"somun/internal/store"
)

func openMemberStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "member.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := store.Migrate(st.DB()); err != nil {
		t.Fatal(err)
	}
	return st
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	svc, err := NewService(openMemberStore(t), filepath.Join(t.TempDir(), ".vault.key"))
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func seedProduct(t *testing.T, svc *Service, fee int, link string) int64 {
	t.Helper()
	p, err := svc.CreateProduct(ProductInput{Name: "nail", Store: "glow", FeePerOrder: fee,
		PrivateURL: link})
	if err != nil {
		t.Fatal(err)
	}
	return p.ID
}

// W23: signup logs in; login validates; logout kills; expiry refuses.
func TestMemberAuthFlow(t *testing.T) {
	svc := newTestService(t)
	token, m, err := svc.Signup("a@e.com", "password-1", "nick")
	if err != nil || token == "" || m.Nickname != "nick" {
		t.Fatalf("signup = %+v err = %v", m, err)
	}
	if id, err := svc.Validate(token); err != nil || id != m.ID {
		t.Fatalf("validate = %d err = %v", id, err)
	}
	if _, _, err := svc.Signup("a@e.com", "password-1", "nick"); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("dup err = %v", err)
	}
	if _, _, err := svc.Signup("bad", "password-1", "nick"); !errors.Is(err, ErrBadEmail) {
		t.Fatalf("email err = %v", err)
	}
	if _, _, err := svc.Signup("b@e.com", "short", "nick"); !errors.Is(err, ErrWeakPassword) {
		t.Fatalf("pw err = %v", err)
	}
	token2, _, err := svc.Login("a@e.com", "password-1")
	if err != nil || token2 == "" {
		t.Fatalf("login err = %v", err)
	}
	if _, _, err := svc.Login("a@e.com", "wrong-pass"); !errors.Is(err, ErrBadCredentials) {
		t.Fatalf("wrong pw err = %v", err)
	}
	if _, _, err := svc.Login("ghost@e.com", "password-1"); !errors.Is(err, ErrBadCredentials) {
		t.Fatalf("ghost err = %v", err)
	}
	svc.Logout(token2)
	if _, err := svc.Validate(token2); !errors.Is(err, ErrBadToken) {
		t.Fatalf("logged-out err = %v", err)
	}
}

// W23: recruiting opens only with fee and link; publics never see the link.
func TestProductRecruitGate(t *testing.T) {
	svc := newTestService(t)
	bare := seedProduct(t, svc, 0, "")
	if _, err := svc.SetProductStatus(bare, store.ConsignRecruiting); !errors.Is(err, ErrRecruitGate) {
		t.Fatalf("fee-less err = %v", err)
	}
	p, err := svc.SetProductStatus(bare, store.ConsignClosed)
	if err != nil || p.Status != store.ConsignClosed {
		t.Fatalf("close = %+v err = %v", p, err)
	}
	ready := seedProduct(t, svc, 1000, "https://smartstore.example.com/secret/1")
	open, err := svc.SetProductStatus(ready, store.ConsignRecruiting)
	if err != nil || open.Status != store.ConsignRecruiting {
		t.Fatalf("open = %+v err = %v", open, err)
	}
	if !strings.HasPrefix(open.LinkMasked, "https://smartstore") || strings.Contains(open.LinkMasked, "secret/1") ||
		!strings.HasSuffix(open.LinkMasked, "•••") {
		t.Fatalf("mask = %q", open.LinkMasked)
	}
	list := svc.ListProducts()
	if len(list) != 2 {
		t.Fatalf("list = %d", len(list))
	}
	detail, err := svc.ProductDetail(ready)
	if err != nil || detail.FeePerOrder != 1000 {
		t.Fatalf("detail = %+v err = %v", detail, err)
	}
}

// W23: apply validates, dedupes, and snapshots the agreed fee.
func TestApplyFlow(t *testing.T) {
	svc := newTestService(t)
	_, m, err := svc.Signup("a@e.com", "password-1", "nick")
	if err != nil {
		t.Fatal(err)
	}
	pid := seedProduct(t, svc, 1000, "https://x.example.com/1")
	if _, err := svc.Apply(m.ID, pid, "blog", "review", true); !errors.Is(err, ErrRecruitClosed) {
		t.Fatalf("closed err = %v", err)
	}
	if _, err := svc.SetProductStatus(pid, store.ConsignRecruiting); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Apply(m.ID, pid, "", "review", true); !errors.Is(err, ErrBadApply) {
		t.Fatalf("channels err = %v", err)
	}
	if _, err := svc.Apply(m.ID, pid, "blog", "review", false); !errors.Is(err, ErrBadApply) {
		t.Fatalf("consent err = %v", err)
	}
	a, err := svc.Apply(m.ID, pid, "blog", "review", true)
	if err != nil || a.Status != store.ApplyPending || a.FeeAgreed != 1000 {
		t.Fatalf("apply = %+v err = %v", a, err)
	}
	if _, err := svc.Apply(m.ID, pid, "blog", "again", true); !errors.Is(err, ErrDuplicateApply) {
		t.Fatalf("dup err = %v", err)
	}
	mine := svc.MyApplications(m.ID)
	if len(mine) != 1 || mine[0].Product != "nail" {
		t.Fatalf("mine = %+v", mine)
	}
}

// W23: the private link opens for the approved owner only (4 denials).
func TestPrivateLinkGating(t *testing.T) {
	svc := newTestService(t)
	_, m1, _ := svc.Signup("a@e.com", "password-1", "n1")
	_, m2, _ := svc.Signup("b@e.com", "password-1", "n2")
	pid := seedProduct(t, svc, 1000, "https://x.example.com/secret")
	if _, err := svc.SetProductStatus(pid, store.ConsignRecruiting); err != nil {
		t.Fatal(err)
	}
	a1, _ := svc.Apply(m1.ID, pid, "blog", "review", true)
	if _, err := svc.PrivateLink(m1.ID, a1.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("pending err = %v", err)
	}
	a2, _ := svc.Apply(m2.ID, pid, "blog", "review", true)
	if _, err := svc.Decide(a2.ID, false, "off-topic"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PrivateLink(m2.ID, a2.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("rejected err = %v", err)
	}
	if _, err := svc.PrivateLink(m2.ID, a1.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("other err = %v", err)
	}
	if _, err := svc.Decide(a1.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	url, err := svc.PrivateLink(m1.ID, a1.ID)
	if err != nil || url != "https://x.example.com/secret" {
		t.Fatalf("url = %q err = %v", url, err)
	}
	if _, err := svc.PrivateLink(m1.ID, 9999); !errors.Is(err, ErrNoApplication) {
		t.Fatalf("missing err = %v", err)
	}
}

// W23: decisions move pending only; rejections need a reason.
func TestDecideRules(t *testing.T) {
	svc := newTestService(t)
	_, m, _ := svc.Signup("a@e.com", "password-1", "n1")
	pid := seedProduct(t, svc, 1000, "https://x.example.com/1")
	if _, err := svc.SetProductStatus(pid, store.ConsignRecruiting); err != nil {
		t.Fatal(err)
	}
	a, _ := svc.Apply(m.ID, pid, "blog", "review", true)
	if _, err := svc.Decide(a.ID, false, ""); !errors.Is(err, ErrBadDecision) {
		t.Fatalf("reasonless err = %v", err)
	}
	if _, err := svc.Decide(a.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Decide(a.ID, false, "late"); !errors.Is(err, ErrBadDecision) {
		t.Fatalf("re-decide err = %v", err)
	}
	got := svc.ListApplicationsAdmin(store.ApplyApproved)
	if len(got) != 1 || got[0].MemberEmail != "a@e.com" {
		t.Fatalf("admin list = %+v", got)
	}
	ov := svc.Overview()
	if ov["products"] != 1 || ov["pending"] != 0 || ov["promoters"] != 1 {
		t.Fatalf("overview = %+v", ov)
	}
}

// W23: sliding sessions refresh near expiry and die past it.
func TestSessionSliding(t *testing.T) {
	svc := newTestService(t)
	token, m, err := svc.Signup("a@e.com", "password-1", "nick")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Validate("bogus"); !errors.Is(err, ErrBadToken) {
		t.Fatalf("bogus err = %v", err)
	}
	hash := hashToken(token)
	near := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	if err := svc.store.TouchMemberSession(hash, near); err != nil {
		t.Fatal(err)
	}
	if id, err := svc.Validate(token); err != nil || id != m.ID {
		t.Fatalf("sliding = %d err = %v", id, err)
	}
	sess, err := svc.store.GetMemberSession(hash)
	if err != nil {
		t.Fatal(err)
	}
	expires, _ := time.Parse(time.RFC3339, sess.ExpiresAt)
	if time.Until(expires) < 20*24*time.Hour {
		t.Fatalf("not extended: %s", sess.ExpiresAt)
	}
	past := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	if err := svc.store.TouchMemberSession(hash, past); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Validate(token); !errors.Is(err, ErrBadToken) {
		t.Fatalf("expired err = %v", err)
	}
}
