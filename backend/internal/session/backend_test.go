package session

import (
	"errors"
	"testing"
)

func strptr(s string) *string { return &s }

func seedChannelAndAI(t *testing.T, svc *Service) (ch, ai *Account) {
	t.Helper()
	var err error
	ch, err = svc.RegisterAccount("ch1", "naver_blog", "me", "worker", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetKindFields(ch.ID, strptr("channel"), nil); err != nil {
		t.Fatal(err)
	}
	ai, err = svc.RegisterAccount("cap1", "muse", "me", "chief", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetKindFields(ai.ID, strptr("agent_service"), nil); err != nil {
		t.Fatal(err)
	}
	return svc.mustGet(t, ch.ID), svc.mustGet(t, ai.ID)
}

func (s *Service) mustGet(t *testing.T, id int64) *Account {
	t.Helper()
	a, err := s.GetAccount(id)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// W22: relay backend happy path; direct clears the relay name.
func TestSetExecutorBackendHappy(t *testing.T) {
	svc := newTestService(t)
	ch, ai := seedChannelAndAI(t, svc)
	got, err := svc.SetExecutorBackend(ch.ID, strptr("muse"), strptr(ai.Name))
	if err != nil {
		t.Fatal(err)
	}
	if got.ExecutorBackend != "muse" || got.RelayAccount != ai.Name {
		t.Fatalf("got = %+v", got)
	}
	back, err := svc.SetExecutorBackend(ch.ID, strptr("direct"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if back.ExecutorBackend != "direct" || back.RelayAccount != "" {
		t.Fatalf("back = %+v", back)
	}
}

// W22: partial relay-only update on an already-relay account.
func TestSetExecutorBackendPartialRelay(t *testing.T) {
	svc := newTestService(t)
	ch, ai := seedChannelAndAI(t, svc)
	if _, err := svc.SetExecutorBackend(ch.ID, strptr("muse"), strptr(ai.Name)); err != nil {
		t.Fatal(err)
	}
	ai2, err := svc.RegisterAccount("cap2", "muse", "me", "worker", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetKindFields(ai2.ID, strptr("agent_service"), nil); err != nil {
		t.Fatal(err)
	}
	got, err := svc.SetExecutorBackend(ch.ID, nil, strptr("cap2"))
	if err != nil {
		t.Fatal(err)
	}
	if got.ExecutorBackend != "muse" || got.RelayAccount != "cap2" {
		t.Fatalf("got = %+v", got)
	}
}

// W22: every invalid combination refuses with the documented error.
func TestSetExecutorBackendRejects(t *testing.T) {
	svc := newTestService(t)
	ch, ai := seedChannelAndAI(t, svc)
	if _, err := svc.SetExecutorBackend(9999, strptr("muse"), strptr(ai.Name)); !errors.Is(err, ErrNoAccount) {
		t.Fatalf("missing id err = %v", err)
	}
	if _, err := svc.SetExecutorBackend(ch.ID, strptr("nope"), strptr(ai.Name)); !errors.Is(err, ErrBadBackend) {
		t.Fatalf("unknown backend err = %v", err)
	}
	if _, err := svc.SetExecutorBackend(ch.ID, strptr("muse"), nil); !errors.Is(err, ErrBadRelay) {
		t.Fatalf("missing relay err = %v", err)
	}
	if _, err := svc.SetExecutorBackend(ch.ID, strptr("muse"), strptr("ghost")); !errors.Is(err, ErrBadRelay) {
		t.Fatalf("ghost relay err = %v", err)
	}
	if _, err := svc.SetExecutorBackend(ch.ID, strptr("muse"), strptr(ch.Name)); !errors.Is(err, ErrBadRelay) {
		t.Fatalf("channel relay err = %v", err)
	}
	other, err := svc.RegisterAccount("gpt1", "chatgpt", "me", "worker", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetKindFields(other.ID, strptr("agent_service"), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetExecutorBackend(ch.ID, strptr("muse"), strptr("gpt1")); !errors.Is(err, ErrBadRelay) {
		t.Fatalf("platform mismatch err = %v", err)
	}
	if _, err := svc.SetExecutorBackend(ai.ID, strptr("muse"), strptr(ai.Name)); !errors.Is(err, ErrBadBackend) {
		t.Fatalf("AI relay backend err = %v", err)
	}
}

// W22: unready backends refuse until the probe gate flips.
func TestSetExecutorBackendNotReady(t *testing.T) {
	svc := newTestService(t)
	ch, ai := seedChannelAndAI(t, svc)
	if err := svc.store.SetPublishBackendReady("muse", false); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetExecutorBackend(ch.ID, strptr("muse"), strptr(ai.Name)); !errors.Is(err, ErrBadBackend) {
		t.Fatalf("unready err = %v", err)
	}
	if err := svc.store.SetPublishBackendReady("muse", true); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetExecutorBackend(ch.ID, strptr("muse"), strptr(ai.Name)); err != nil {
		t.Fatal(err)
	}
}

// W22: registry lists the seeded backends.
func TestListBackends(t *testing.T) {
	svc := newTestService(t)
	backs, err := svc.ListBackends()
	if err != nil || len(backs) != 2 {
		t.Fatalf("backs = %+v err = %v", backs, err)
	}
	if backs[0].Name != "direct" || backs[1].Name != "muse" || !backs[1].Ready {
		t.Fatalf("backs = %+v", backs)
	}
}
