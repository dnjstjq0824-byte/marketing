package session

import (
	"testing"
)

// P0-3: session moves fire the notify hook with the transition.
func TestSetSessionStateFiresNotifyHook(t *testing.T) {
	svc := newTestService(t)
	a, err := svc.RegisterAccount("acc", "instagram", "me", "worker", "biz")
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		id         int64
		name, prev string
		next       string
	}
	svc.OnSessionState = func(id int64, name, prev, next string) {
		got.id, got.name, got.prev, got.next = id, name, prev, next
	}
	if _, err := svc.SetSessionState(a.ID, StateExpired, ""); err != nil {
		t.Fatal(err)
	}
	if got.id != a.ID || got.name != "acc" || got.prev != StatePending || got.next != StateExpired {
		t.Fatalf("hook got %+v", got)
	}
}
