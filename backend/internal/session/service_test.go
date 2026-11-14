package session

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"somun/internal/store"
)

func openSessionStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "s.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := store.Migrate(st.DB()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return st
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	st := openSessionStore(t)
	svc, err := NewService(st, filepath.Join(t.TempDir(), ".vault.key"))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	return svc
}

// Account fields update; bad roles are rejected.
func TestUpdateAccountFields(t *testing.T) {
	svc := newTestService(t)
	a, err := svc.RegisterAccount("acc", "instagram", "me", "worker", "biz")
	if err != nil {
		t.Fatal(err)
	}
	owner, business := "you", "biz2"
	upd, err := svc.UpdateAccountFields(a.ID, &owner, nil, &business)
	if err != nil {
		t.Fatal(err)
	}
	if upd.Owner != "you" || upd.Business != "biz2" || upd.Role != "worker" {
		t.Fatalf("updated = %+v", upd)
	}
	bad := "ceo"
	if _, err := svc.UpdateAccountFields(a.ID, nil, &bad, nil); !errors.Is(err, ErrBadRole) {
		t.Fatalf("bad role: err=%v", err)
	}
}

// Bulk registration creates prefix01..NN; duplicates are skipped so a
// half-done batch can be re-run.
func TestRegisterAccounts(t *testing.T) {
	svc := newTestService(t)
	got, failures, err := svc.RegisterAccounts("luno_meta_", "meta_ai", "me", "worker", "", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || len(failures) != 0 {
		t.Fatalf("created=%d failures=%v", len(got), failures)
	}
	for i, want := range []string{"luno_meta_01", "luno_meta_02", "luno_meta_03"} {
		if got[i].Name != want {
			t.Fatalf("name[%d] = %q, want %q", i, got[i].Name, want)
		}
		if got[i].SessionState != StatePending || got[i].PostingMode != PostingManual {
			t.Fatalf("account[%d] = %+v", i, got[i])
		}
	}
	// Re-run: all duplicates, nothing created, nothing aborted.
	got, failures, err = svc.RegisterAccounts("luno_meta_", "meta_ai", "me", "worker", "", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 || len(failures) != 3 {
		t.Fatalf("rerun created=%d failures=%v", len(got), failures)
	}
	if _, _, err := svc.RegisterAccounts("", "meta_ai", "me", "", "", 2); !errors.Is(err, ErrNoPrefix) {
		t.Fatalf("empty prefix: %v", err)
	}
	if _, _, err := svc.RegisterAccounts("x_", "meta_ai", "me", "", "", 0); !errors.Is(err, ErrBadCount) {
		t.Fatalf("count 0: %v", err)
	}
	if _, _, err := svc.RegisterAccounts("x_", "meta_ai", "me", "", "", 51); !errors.Is(err, ErrBadCount) {
		t.Fatalf("count 51: %v", err)
	}
	if _, _, err := svc.RegisterAccounts("x_", "meta_ai", "me", "ceo", "", 2); !errors.Is(err, ErrBadRole) {
		t.Fatalf("bad role: %v", err)
	}
}

func TestNextLogin(t *testing.T) {
	svc := newTestService(t)
	if next, err := svc.NextLogin(""); err != nil || next != nil {
		t.Fatalf("empty = %+v %v", next, err)
	}
	ok := mustRegister(t, svc, "fine", "meta_ai")
	a, err := svc.SetSessionState(ok, StateConnected, "")
	_ = a
	if err != nil {
		t.Fatal(err)
	}
	mustRegister(t, svc, "wait-meta", "meta_ai")
	mustRegister(t, svc, "wait-gem", "gemini")
	next, err := svc.NextLogin("gemini")
	if err != nil || next == nil || next.Name != "wait-gem" {
		t.Fatalf("prefer = %+v %v", next, err)
	}
	next, err = svc.NextLogin("muse")
	if err != nil || next == nil || next.Name != "wait-meta" {
		t.Fatalf("fallback = %+v %v", next, err)
	}
}

// Posting mode defaults to manual; only auto/manual are accepted.
func TestPostingMode(t *testing.T) {
	svc := newTestService(t)
	a, err := svc.RegisterAccount("acc", "instagram", "me", "worker", "")
	if err != nil {
		t.Fatal(err)
	}
	if a.PostingMode != PostingManual {
		t.Fatalf("default mode = %q", a.PostingMode)
	}
	upd, err := svc.SetPostingMode(a.ID, PostingAuto)
	if err != nil || upd.PostingMode != PostingAuto {
		t.Fatalf("set auto: %+v %v", upd, err)
	}
	if _, err := svc.SetPostingMode(a.ID, "turbo"); !errors.Is(err, ErrBadMode) {
		t.Fatalf("bad mode: err=%v", err)
	}
	if _, err := svc.SetPostingMode(999999, PostingAuto); !errors.Is(err, ErrNoAccount) {
		t.Fatalf("missing account: err=%v", err)
	}
}

// D20: unregister is refused while ANY item is queued for the account —
// held or not — and allowed once the queue no longer references it.
func TestUnregisterHeldGuard(t *testing.T) {
	st := openSessionStore(t)
	svc, err := NewService(st, filepath.Join(t.TempDir(), ".vault.key"))
	if err != nil {
		t.Fatal(err)
	}
	a, err := svc.RegisterAccount("acc", "instagram", "me", "worker", "")
	if err != nil {
		t.Fatal(err)
	}
	p, _ := st.CreateProduct("https://example.com/x", "A", 0, "")
	c, _ := st.CreateContent(&store.Content{ProductID: p.ID, Channel: "blog", Body: "b"})
	_ = st.UpdateContentStatus(c.ID, "queued")
	_ = st.ScheduleContent(c.ID, "acc", "instagram", "official_api", "")
	if err := svc.UnregisterAccount(a.ID); !errors.Is(err, ErrHeldItems) {
		t.Fatalf("unheld queued unregister: err=%v", err)
	}
	if _, err := svc.SetSessionState(a.ID, StateExpired, ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.UnregisterAccount(a.ID); !errors.Is(err, ErrHeldItems) {
		t.Fatalf("held unregister: err=%v", err)
	}
	if _, err := svc.SetSessionState(a.ID, StateConnected, ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.UnregisterAccount(a.ID); !errors.Is(err, ErrHeldItems) {
		t.Fatalf("resumed queued unregister: err=%v", err)
	}
	if err := st.DeleteContent(c.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.UnregisterAccount(a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetAccount(a.ID); !errors.Is(err, ErrNoAccount) {
		t.Fatalf("unregistered: err=%v", err)
	}
}

// Credentials encrypt at rest, round-trip internally, and never leak to views.
func TestCredentialsVault(t *testing.T) {
	svc := newTestService(t)
	a, err := svc.RegisterAccount("acc", "instagram", "me", "worker", "")
	if err != nil {
		t.Fatal(err)
	}
	if a.HasCredentials {
		t.Fatal("fresh account reports credentials")
	}
	upd, err := svc.SetCredentials(a.ID, "user1", "pw1")
	if err != nil || !upd.HasCredentials {
		t.Fatalf("set credentials: %+v %v", upd, err)
	}
	loginID, password, err := svc.Credentials(a.ID)
	if err != nil || loginID != "user1" || password != "pw1" {
		t.Fatalf("roundtrip = %q/%q err=%v", loginID, password, err)
	}
	if _, err := svc.SetCredentials(a.ID, "", ""); !errors.Is(err, ErrCredentials) {
		t.Fatalf("empty credentials: err=%v", err)
	}
	if _, err := svc.RegisterAccount("acc", "x", "", "", ""); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("duplicate name: err=%v", err)
	}
}

// PublishPlan judges every queued item without mutating anything.
func TestPublishPlan(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := store.Migrate(st.DB()); err != nil {
		t.Fatal(err)
	}
	svc, err := NewService(st, filepath.Join(dir, ".vault.key"))
	if err != nil {
		t.Fatal(err)
	}
	auto, err := svc.RegisterAccount("auto1", "instagram", "me", "worker", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetPostingMode(auto.ID, PostingAuto); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetSessionState(auto.ID, StateConnected, ""); err != nil {
		t.Fatal(err)
	}
	man, err := svc.RegisterAccount("man1", "instagram", "me", "worker", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetSessionState(man.ID, StateConnected, ""); err != nil {
		t.Fatal(err)
	}
	mk := func(account string) int64 {
		p, _ := st.CreateProduct("https://example.com/x", "A", 0, "")
		c, _ := st.CreateContent(&store.Content{ProductID: p.ID, Channel: "blog", Body: "b"})
		_ = st.UpdateContentStatus(c.ID, "queued")
		_ = st.ScheduleContent(c.ID, account, "instagram", "official_api", "")
		return c.ID
	}
	dueID := mk("auto1")
	mk("man1")
	mk("ghost")
	items := svc.PublishPlan(time.Now())
	byID := map[int64]*PlanItem{}
	for _, it := range items {
		byID[it.ContentID] = it
	}
	if len(byID) != 3 {
		t.Fatalf("plan items = %d", len(byID))
	}
	var dueVerdict, manVerdict, ghostVerdict string
	for _, it := range items {
		switch it.Account {
		case "auto1":
			dueVerdict = it.Verdict
		case "man1":
			manVerdict = it.Verdict
		case "ghost":
			ghostVerdict = it.Verdict
		}
	}
	if dueVerdict != VerdictReady || manVerdict != VerdictManual || ghostVerdict != VerdictUnknownAccount {
		t.Fatalf("verdicts = %s/%s/%s", dueVerdict, manVerdict, ghostVerdict)
	}
	// Planning mutates nothing.
	if c, _ := st.GetContent(dueID); c.Status != "queued" {
		t.Fatalf("plan mutated status to %s", c.Status)
	}
}

// Accounts survive a service restart against the same DB and vault key.
func TestAccountsPersistAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := store.Migrate(st.DB()); err != nil {
		t.Fatal(err)
	}
	keyPath := filepath.Join(dir, ".vault.key")
	first, err := NewService(st, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	a, err := first.RegisterAccount("acc", "instagram", "me", "worker", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.SetCredentials(a.ID, "user1", "pw1"); err != nil {
		t.Fatal(err)
	}
	second, err := NewService(st, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	got, err := second.GetAccount(a.ID)
	if err != nil || !got.HasCredentials {
		t.Fatalf("persisted account: %+v %v", got, err)
	}
	if _, _, err := second.Credentials(a.ID); err != nil {
		t.Fatalf("decrypt after restart: %v", err)
	}
}
