package agent

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"somun/internal/session"
	"somun/internal/store"
)

func openAgentStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "ag.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := store.Migrate(st.DB()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return st
}

func newTestService(t *testing.T) (*Service, *store.Store) {
	t.Helper()
	st := openAgentStore(t)
	svc := NewService(st, NewHub(), 600, 25)
	sess, err := session.NewService(st, filepath.Join(t.TempDir(), ".vault.key"))
	if err != nil {
		t.Fatal(err)
	}
	svc.Session = sess
	return svc, st
}

// W6: one account cannot bind both the B (generate) and C (review)
// lanes, at register or at update time. Same-lane twins stay legal.
func TestLaneSplitGuard(t *testing.T) {
	svc, st := newTestService(t)
	acc, err := st.CreateAccountRow(&store.ChannelAccount{Name: "a1", Platform: "meta_ai", Role: "worker"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Register("wB", "B", "", "", "", 1, &acc.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Register("wC", "C", "", "", "", 1, &acc.ID); err == nil {
		t.Fatal("B+C split accepted at register")
	}
	if _, _, err := svc.Register("wB2", "B", "", "", "", 1, &acc.ID); err != nil {
		t.Fatalf("B+B twin refused: %v", err)
	}
	a, _, err := svc.Register("wA", "A", "", "", "", 1, &acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Update(a.ID, "C", "", "", "", 1, true, &acc.ID); err == nil {
		t.Fatal("B+C split accepted at update")
	}
}

// W16-7: the team executor (lane T) claims the whole AI chain.
func TestLaneTClaimsChain(t *testing.T) {
	svc, st := newTestService(t)
	if !store.ValidLane("T") {
		t.Fatal("lane T invalid")
	}
	stt := store.DefaultAutomationSettings()
	stt.MaxConcurrent = 10
	if err := store.SetAutomationSettings(st.DB(), stt); err != nil {
		t.Fatal(err)
	}
	a, _, err := svc.Register("wt", "T", "", "", "", 4, nil)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := st.CreateProduct("https://example.com/wt", "A", 0, "")
	for _, typ := range []string{"analyze", "strategy", "generate", "review"} {
		if _, err := st.CreateJob(typ, p.ID, "{}"); err != nil {
			t.Fatal(err)
		}
	}
	for _, typ := range []string{"analyze", "strategy", "generate", "review"} {
		fresh, _ := st.GetAgent(a.ID)
		j, err := svc.Claim(fresh, typ, 0)
		if err != nil || j == nil || j.Type != typ {
			t.Fatalf("T claim %s = %+v, %v", typ, j, err)
		}
	}
}

func TestClaimLaneMismatch(t *testing.T) {
	svc, _ := newTestService(t)
	a, _, err := svc.Register("w", "A", "", "", "", 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Claim(a, "generate", 0); !errors.Is(err, ErrLaneMismatch) {
		t.Fatalf("err = %v, want ErrLaneMismatch", err)
	}
	if _, err := svc.Claim(a, "", 0); err != nil {
		t.Fatalf("lane claim: %v", err)
	}
}

func TestBulkAndDeleteGuard(t *testing.T) {
	svc, st := newTestService(t)
	a1, _, _ := svc.Register("w1", "A", "", "", "", 1, nil)
	a2, _, _ := svc.Register("w2", "B", "", "", "", 1, nil)
	n, err := svc.Bulk([]int64{a1.ID, a2.ID, 9999}, "pause", "")
	if err != nil || n != 2 {
		t.Fatalf("bulk pause = %d, %v; want 2", n, err)
	}
	if _, err := svc.Authenticate("bogus"); !errors.Is(err, ErrUnknownToken) {
		t.Fatalf("bogus token err = %v", err)
	}
	// Live claim blocks deletion (a1 still lanes A here).
	p, _ := st.CreateProduct("https://example.com/x", "A", 0, "")
	if _, err := st.CreateJob("analyze", p.ID, "{}"); err != nil {
		t.Fatal(err)
	}
	fresh, _ := st.GetAgent(a1.ID)
	if j, err := svc.Claim(fresh, "", 0); err != nil || j == nil {
		t.Fatalf("claim = %+v, %v", j, err)
	}
	if err := svc.Delete(a1.ID); err == nil {
		t.Fatal("delete with live claim accepted")
	}
	n, err = svc.Bulk([]int64{a1.ID}, "lane", "C")
	if err != nil || n != 1 {
		t.Fatalf("bulk lane = %d, %v", n, err)
	}
	if _, err := svc.Bulk([]int64{a1.ID}, "lane", "Z"); err == nil {
		t.Fatal("bad lane accepted")
	}
	if err := svc.Delete(a2.ID); err != nil {
		t.Fatalf("delete idle: %v", err)
	}
}

func TestReapHookOnlyOnFailed(t *testing.T) {
	svc, st := newTestService(t)
	a, _, _ := svc.Register("w", "A", "", "", "", 2, nil)
	p, _ := st.CreateProduct("https://example.com/x", "A", 0, "")
	st.CreateJob("analyze", p.ID, "{}")
	st.CreateJob("analyze", p.ID, "{}")
	j1, _ := svc.Claim(a, "", 0)
	j2, _ := svc.Claim(a, "", 0)
	past := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)
	st.DB().Exec(`UPDATE jobs SET lease_until=? WHERE id IN (?,?)`, past, j1.ID, j2.ID)
	var fired []int64
	svc.OnLeaseExpired = func(action store.ReapAction) { fired = append(fired, action.JobID) }
	// First reap requeues silently.
	if _, err := svc.ReapTick(time.Now()); err != nil {
		t.Fatal(err)
	}
	if len(fired) != 0 {
		t.Fatalf("hook fired on requeue: %v", fired)
	}
	// Second reap fails with notification.
	j3, _ := svc.Claim(a, "", 0)
	j4, _ := svc.Claim(a, "", 0)
	st.DB().Exec(`UPDATE jobs SET lease_until=? WHERE id IN (?,?)`, past, j3.ID, j4.ID)
	if _, err := svc.ReapTick(time.Now()); err != nil {
		t.Fatal(err)
	}
	if len(fired) != 2 {
		t.Fatalf("hook fired %d times, want 2", len(fired))
	}
}

func TestHeartbeatSessionReport(t *testing.T) {
	svc, _ := newTestService(t)
	acc, err := svc.Session.RegisterAccount("hb", "instagram", "me", "worker", "")
	if err != nil {
		t.Fatal(err)
	}
	a, _, err := svc.Register("w", "A", "", "", "", 1, &acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	enabled, err := svc.Heartbeat(a, "limited")
	if err != nil || !enabled {
		t.Fatalf("heartbeat = %v, %v", enabled, err)
	}
	got, _ := svc.Session.GetAccount(acc.ID)
	if got.SessionState != "limited" {
		t.Fatalf("account state = %q, want limited", got.SessionState)
	}
	if _, err := svc.Heartbeat(a, "bogus"); err == nil {
		t.Fatal("bogus session state accepted")
	}
}

func TestHubWakeUnblocksWait(t *testing.T) {
	h := NewHub()
	done := make(chan struct{})
	go func() {
		h.Wait(30 * time.Second)
		close(done)
	}()
	time.Sleep(50 * time.Millisecond)
	h.Notify()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("waiter not woken")
	}
}
