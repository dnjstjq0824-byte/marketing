package session

import (
	"errors"
	"path/filepath"
	"testing"

	"somun/internal/store"
)

func openSessionStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := store.Migrate(st.DB()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return st
}

// Account fields update; bad roles are rejected.
func TestUpdateAccountFields(t *testing.T) {
	svc := NewService(openSessionStore(t))
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

// Posting mode defaults to manual; only auto/manual are accepted.
func TestPostingMode(t *testing.T) {
	svc := NewService(openSessionStore(t))
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

// Unregister is refused while reservations are held, allowed after resume.
func TestUnregisterHeldGuard(t *testing.T) {
	st := openSessionStore(t)
	svc := NewService(st)
	a, err := svc.RegisterAccount("acc", "instagram", "me", "worker", "")
	if err != nil {
		t.Fatal(err)
	}
	p, _ := st.CreateProduct("https://example.com/x", "A", 0, "")
	c, _ := st.CreateContent(&store.Content{ProductID: p.ID, Channel: "blog", Body: "b"})
	_ = st.UpdateContentStatus(c.ID, "queued")
	_ = st.ScheduleContent(c.ID, "acc", "instagram", "official_api", "")
	if _, err := svc.SetSessionState(a.ID, StateExpired, ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.UnregisterAccount(a.ID); !errors.Is(err, ErrHeldItems) {
		t.Fatalf("held unregister: err=%v", err)
	}
	if _, err := svc.SetSessionState(a.ID, StateConnected, ""); err != nil {
		t.Fatal(err)
	}
	if err := svc.UnregisterAccount(a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetAccount(a.ID); !errors.Is(err, ErrNoAccount) {
		t.Fatalf("unregistered: err=%v", err)
	}
}

// Slots update label/agent with uniqueness, then delete.
func TestSlotUpdateDelete(t *testing.T) {
	svc := NewService(openSessionStore(t))
	slots := svc.ListSlots()
	if len(slots) != 3 {
		t.Fatalf("seeded slots = %d", len(slots))
	}
	name := "agent-x"
	upd, err := svc.UpdateSlot(slots[0].ID, nil, &name)
	if err != nil {
		t.Fatal(err)
	}
	if upd.AgentName != "agent-x" || upd.Label != "A" {
		t.Fatalf("updated = %+v", upd)
	}
	dup := "B"
	if _, err := svc.UpdateSlot(slots[0].ID, &dup, nil); !errors.Is(err, ErrSlotTaken) {
		t.Fatalf("dup label: err=%v", err)
	}
	if err := svc.DeleteSlot(slots[0].ID); err != nil {
		t.Fatal(err)
	}
	if len(svc.ListSlots()) != 2 {
		t.Fatal("slot not deleted")
	}
	if err := svc.DeleteSlot(slots[0].ID); !errors.Is(err, ErrNoSlot) {
		t.Fatalf("second delete: err=%v", err)
	}
}
