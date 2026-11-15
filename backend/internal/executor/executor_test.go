package executor

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"somun/internal/config"
	"somun/internal/content/policy"
	"somun/internal/publisher"
	"somun/internal/session"
	"somun/internal/store"
)

type fakePub struct {
	platform string
	calls    int
	err      error
	url      string
	lastBody string
}

func (f *fakePub) Platform() string { return f.platform }
func (f *fakePub) Publish(ctx context.Context, req publisher.Request) (publisher.Result, error) {
	f.calls++
	f.lastBody = req.Body
	if f.err != nil {
		return publisher.Result{}, f.err
	}
	return publisher.Result{URL: f.url}, nil
}

func openExecStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "ex.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := store.Migrate(st.DB()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return st
}

func newExec(t *testing.T, pubs map[string]publisher.Publisher) (*Service, *store.Store, *session.Service) {
	t.Helper()
	st := openExecStore(t)
	sess, err := session.NewService(st, filepath.Join(t.TempDir(), ".vault.key"))
	if err != nil {
		t.Fatal(err)
	}
	defs := store.ExecutorConfig{HeartbeatMin: 1, MinIntervalMin: 0, MaxActive: 4}
	return NewService(st, sess, pubs, defs), st, sess
}

func seedAccount(t *testing.T, sess *session.Service, name string) int64 {
	t.Helper()
	a, err := sess.RegisterAccount(name, "instagram", "me", "worker", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sess.SetCredentials(a.ID, "u", "p"); err != nil {
		t.Fatal(err)
	}
	return a.ID
}

func seedQueued(t *testing.T, st *store.Store, pid int64, account string) int64 {
	t.Helper()
	c, err := st.CreateContent(&store.Content{ProductID: pid, Channel: "instagram", Title: "t", Body: "b"})
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)
	if _, err := st.DB().Exec(`UPDATE contents SET status='queued',channel_group='official_api',target_account=?,target_platform='instagram',scheduled_at=? WHERE id=?`,
		account, past, c.ID); err != nil {
		t.Fatal(err)
	}
	return c.ID
}

// W5-2: the executor sends the assembled body (disclosure + buy CTA).
func TestTickAssemblesBody(t *testing.T) {
	fake := &fakePub{platform: "instagram", url: "https://e.com/p/1"}
	svc, st, sess := newExec(t, map[string]publisher.Publisher{"instagram": fake})
	p, _ := st.CreateProduct("https://example.com/x", "A", 0, "")
	if _, err := st.CreateAffiliateLink(p.ID, "shop", "https://shop.test/1"); err != nil {
		t.Fatal(err)
	}
	acc := seedAccount(t, sess, "ready")
	sess.SetSessionState(acc, session.StateConnected, "")
	sess.SetPostingMode(acc, session.PostingAuto)
	cid := seedQueued(t, st, p.ID, "ready")
	if _, err := st.DB().Exec(`UPDATE contents SET is_sponsored=1 WHERE id=?`, cid); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Tick(time.Now()); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(fake.lastBody, policy.DisclosureText) {
		t.Fatalf("body = %q", fake.lastBody)
	}
	if !strings.Contains(fake.lastBody, "https://shop.test/1") {
		t.Fatalf("no buy link: %q", fake.lastBody)
	}
}

// W0: fresh installs run the executor (heartbeat default > 0).
func TestTickDefaultOn(t *testing.T) {
	fake := &fakePub{platform: "instagram", url: "https://e.com/p/1"}
	st := openExecStore(t)
	sess, err := session.NewService(st, filepath.Join(t.TempDir(), ".vault.key"))
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(st, sess, map[string]publisher.Publisher{"instagram": fake},
		store.ExecutorConfig{HeartbeatMin: config.DefaultExecutorHeartbeatMin, MinIntervalMin: 0, MaxActive: 4})
	p, _ := st.CreateProduct("https://example.com/x", "A", 0, "")
	acc := seedAccount(t, sess, "ready")
	sess.SetSessionState(acc, session.StateConnected, "")
	sess.SetPostingMode(acc, session.PostingAuto)
	seedQueued(t, st, p.ID, "ready")
	n, err := svc.Tick(time.Now())
	if err != nil || n != 1 || fake.calls != 1 {
		t.Fatalf("fresh tick: n=%d calls=%d err=%v", n, fake.calls, err)
	}
}

// W0: stopped automation publishes nothing, even when items are due.
func TestTickRespectsAutomationSwitch(t *testing.T) {
	fake := &fakePub{platform: "instagram", url: "https://e.com/p/1"}
	svc, st, sess := newExec(t, map[string]publisher.Publisher{"instagram": fake})
	p, _ := st.CreateProduct("https://example.com/x", "A", 0, "")
	acc := seedAccount(t, sess, "ready")
	sess.SetSessionState(acc, session.StateConnected, "")
	sess.SetPostingMode(acc, session.PostingAuto)
	seedQueued(t, st, p.ID, "ready")
	if _, err := store.SetAutomationEnabled(st.DB(), false); err != nil {
		t.Fatal(err)
	}
	n, err := svc.Tick(time.Now())
	if err != nil || n != 0 || fake.calls != 0 {
		t.Fatalf("tick while off: n=%d calls=%d err=%v", n, fake.calls, err)
	}
	if _, err := store.SetAutomationEnabled(st.DB(), true); err != nil {
		t.Fatal(err)
	}
	n, err = svc.Tick(time.Now())
	if err != nil || n != 1 || fake.calls != 1 {
		t.Fatalf("tick after on: n=%d calls=%d err=%v", n, fake.calls, err)
	}
}

// Verdict x execution matrix: only ready+official runs.
func TestTickVerdictMatrix(t *testing.T) {
	fake := &fakePub{platform: "instagram", url: "https://e.com/p/1"}
	svc, st, sess := newExec(t, map[string]publisher.Publisher{"instagram": fake})
	p, _ := st.CreateProduct("https://example.com/x", "A", 0, "")

	readyAcc := seedAccount(t, sess, "ready")
	sess.SetSessionState(readyAcc, session.StateConnected, "")
	sess.SetPostingMode(readyAcc, session.PostingAuto)
	readyCID := seedQueued(t, st, p.ID, "ready")

	heldAcc := seedAccount(t, sess, "held")
	sess.SetSessionState(heldAcc, session.StateConnected, "")
	sess.SetPostingMode(heldAcc, session.PostingAuto)
	heldCID := seedQueued(t, st, p.ID, "held")
	st.HoldContent(heldCID, "manual hold")

	manAcc := seedAccount(t, sess, "man")
	sess.SetSessionState(manAcc, session.StateConnected, "")
	sess.SetPostingMode(manAcc, session.PostingManual)
	seedQueued(t, st, p.ID, "man") // posting explicitly manual

	discAcc := seedAccount(t, sess, "disc")
	sess.SetPostingMode(discAcc, session.PostingAuto)
	seedQueued(t, st, p.ID, "disc") // session stays pending

	seedQueued(t, st, p.ID, "ghost") // unknown account

	futAcc := seedAccount(t, sess, "fut")
	sess.SetSessionState(futAcc, session.StateConnected, "")
	sess.SetPostingMode(futAcc, session.PostingAuto)
	futCID := seedQueued(t, st, p.ID, "fut")
	future := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)
	st.DB().Exec(`UPDATE contents SET scheduled_at=? WHERE id=?`, future, futCID)

	n, err := svc.Tick(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || fake.calls != 1 {
		t.Fatalf("tick ran %d calls=%d, want 1/1", n, fake.calls)
	}
	got, _ := st.GetContent(readyCID)
	if got.Status != "published" {
		t.Fatalf("ready status = %s, want published", got.Status)
	}
	rec, _ := st.GetPublishLog(readyCID, "instagram", "ready")
	if rec.Status != "done" || rec.Note != "https://e.com/p/1" {
		t.Fatalf("log = %+v", rec)
	}
	held, _ := st.GetContent(heldCID)
	if held.Status != "queued" {
		t.Fatalf("held ran: %s", held.Status)
	}
}

func TestTickOffWhenHeartbeatZero(t *testing.T) {
	fake := &fakePub{platform: "instagram", url: "https://e.com/p/1"}
	svc, st, sess := newExec(t, map[string]publisher.Publisher{"instagram": fake})
	if _, err := svc.SetConfig(store.ExecutorConfig{HeartbeatMin: 0, MinIntervalMin: 0, MaxActive: 4}); err != nil {
		t.Fatal(err)
	}
	p, _ := st.CreateProduct("https://example.com/x", "A", 0, "")
	acc := seedAccount(t, sess, "r")
	sess.SetSessionState(acc, session.StateConnected, "")
	sess.SetPostingMode(acc, session.PostingAuto)
	seedQueued(t, st, p.ID, "r")
	n, err := svc.Tick(time.Now())
	if err != nil || n != 0 || fake.calls != 0 {
		t.Fatalf("tick = %d calls=%d %v; want 0/0", n, fake.calls, err)
	}
}

func TestSetConfigClampsInterval(t *testing.T) {
	svc, _, _ := newExec(t, map[string]publisher.Publisher{})
	cfg, err := svc.SetConfig(store.ExecutorConfig{HeartbeatMin: 1, MinIntervalMin: 9999, MaxActive: 2})
	if err != nil || cfg.MinIntervalMin != 1440 {
		t.Fatalf("clamp = %+v, %v; want 1440", cfg, err)
	}
	if _, err := svc.SetConfig(store.ExecutorConfig{HeartbeatMin: 1, MinIntervalMin: 0, MaxActive: 0}); err == nil {
		t.Fatal("maxActive 0 accepted")
	}
	got, err := svc.Config()
	if err != nil || got.MinIntervalMin != 1440 {
		t.Fatalf("persisted = %+v, %v", got, err)
	}
}

func TestMinIntervalGate(t *testing.T) {
	fake := &fakePub{platform: "instagram", url: "https://e.com/p/1"}
	svc, st, sess := newExec(t, map[string]publisher.Publisher{"instagram": fake})
	if _, err := svc.SetConfig(store.ExecutorConfig{HeartbeatMin: 1, MinIntervalMin: 60, MaxActive: 4}); err != nil {
		t.Fatal(err)
	}
	p, _ := st.CreateProduct("https://example.com/x", "A", 0, "")
	acc := seedAccount(t, sess, "g")
	sess.SetSessionState(acc, session.StateConnected, "")
	sess.SetPostingMode(acc, session.PostingAuto)
	seedQueued(t, st, p.ID, "g")
	if _, err := svc.Tick(time.Now()); err != nil {
		t.Fatal(err)
	}
	if fake.calls != 1 {
		t.Fatalf("first tick calls = %d, want 1", fake.calls)
	}
	seedQueued(t, st, p.ID, "g")
	n, err := svc.Tick(time.Now())
	if err != nil || n != 1 || fake.calls != 1 {
		t.Fatalf("gated tick = %d calls=%d, want n=1 calls=1 (skipped, not run)", n, fake.calls)
	}
}

func TestAttemptsCapAndLimitedFlip(t *testing.T) {
	lim := &fakePub{platform: "instagram", err: publisher.ErrSessionLimited}
	svc, st, sess := newExec(t, map[string]publisher.Publisher{"instagram": lim})
	p, _ := st.CreateProduct("https://example.com/x", "A", 0, "")
	acc := seedAccount(t, sess, "l")
	sess.SetSessionState(acc, session.StateConnected, "")
	sess.SetPostingMode(acc, session.PostingAuto)
	cid := seedQueued(t, st, p.ID, "l")
	var failed []int64
	svc.OnPublishFailed = func(id int64, msg string) { failed = append(failed, id) }
	if _, err := svc.Tick(time.Now()); err != nil {
		t.Fatal(err)
	}
	got, _ := st.GetAccountRow(acc)
	if got.SessionState != session.StateLimited {
		t.Fatalf("account = %s, want limited", got.SessionState)
	}
	c, _ := st.GetContent(cid)
	if !c.Held {
		t.Fatal("queue not held after limited flip")
	}
	if len(failed) != 1 {
		t.Fatalf("notify calls = %d, want 1", len(failed))
	}

	// Plain failures count attempts, then close at five.
	flaky := &fakePub{platform: "instagram", err: publisher.ErrRateLimited}
	svc2, st2, sess2 := newExec(t, map[string]publisher.Publisher{"instagram": flaky})
	p2, _ := st2.CreateProduct("https://example.com/y", "A", 0, "")
	acc2 := seedAccount(t, sess2, "f")
	sess2.SetSessionState(acc2, session.StateConnected, "")
	sess2.SetPostingMode(acc2, session.PostingAuto)
	cid2 := seedQueued(t, st2, p2.ID, "f")
	for i := 0; i < 5; i++ {
		if _, err := svc2.Tick(time.Now().Add(time.Duration(i) * time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	rec, _ := st2.GetPublishLog(cid2, "instagram", "f")
	if rec.Status != "failed" || rec.Attempts != 5 {
		t.Fatalf("log = %+v, want failed/5", rec)
	}
}
