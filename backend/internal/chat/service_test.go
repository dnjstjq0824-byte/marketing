package chat

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"somun/internal/store"
)

func openChatStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "chat.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := store.Migrate(st.DB()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return st
}

func seedChatAccount(t *testing.T, st *store.Store, name, state string) int64 {
	t.Helper()
	row, err := st.CreateAccountRow(&store.ChannelAccount{
		Name: name, Platform: "meta_ai", Role: "worker",
		SessionState: state, PostingMode: "manual",
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	return row.ID
}

func TestSendValidation(t *testing.T) {
	st := openChatStore(t)
	svc := NewService(st)
	if _, err := svc.Send(9999, "hi"); !errors.Is(err, ErrNoAccount) {
		t.Fatalf("unknown account: %v", err)
	}
	pending := seedChatAccount(t, st, "chat-pending", "pending")
	if _, err := svc.Send(pending, "hi"); !errors.Is(err, ErrNotConnected) {
		t.Fatalf("pending account: %v", err)
	}
	id := seedChatAccount(t, st, "chat-N", "connected")
	svc.sendRunner = func(account, platform, message string) (string, error) {
		return "echo:" + message, nil
	}
	if _, err := svc.Send(id, "   "); !errors.Is(err, ErrEmptyMessage) {
		t.Fatalf("empty message: %v", err)
	}
	if _, err := svc.Send(id, strings.Repeat("x", MaxMessageRunes+1)); !errors.Is(err, ErrMessageTooBig) {
		t.Fatalf("oversize message: %v", err)
	}
}

func TestSendRoundTrip(t *testing.T) {
	st := openChatStore(t)
	svc := NewService(st)
	id := seedChatAccount(t, st, "chat-N", "connected")
	var gotName, gotPlatform, gotMsg string
	svc.sendRunner = func(account, platform, message string) (string, error) {
		gotName, gotPlatform, gotMsg = account, platform, message
		return "real reply", nil
	}
	res, err := svc.Send(id, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if res.Pending || res.Reply != "real reply" {
		t.Fatalf("result = %+v", res)
	}
	if gotName != "chat-N" || gotPlatform != "meta_ai" || gotMsg != "hello" {
		t.Fatalf("runner got (%q,%q,%q)", gotName, gotPlatform, gotMsg)
	}
	items, err := svc.History(id, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Role != "user" || items[1].Role != "assistant" ||
		items[0].Text != "hello" || items[1].Text != "real reply" {
		t.Fatalf("history = %+v", items)
	}
}

func TestSendRunnerError(t *testing.T) {
	st := openChatStore(t)
	svc := NewService(st)
	id := seedChatAccount(t, st, "chat-N", "connected")
	svc.sendRunner = func(account, platform, message string) (string, error) {
		return "", ErrChatFailed
	}
	if _, err := svc.Send(id, "hi"); !errors.Is(err, ErrChatFailed) {
		t.Fatalf("runner error: %v", err)
	}
	// The user turn stays recorded even when delivery fails.
	items, err := svc.History(id, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Role != "user" {
		t.Fatalf("history = %+v", items)
	}
}

func TestSendSlowRunnerPends(t *testing.T) {
	st := openChatStore(t)
	svc := NewService(st)
	svc.grace = 20 * time.Millisecond
	release := make(chan struct{})
	svc.sendRunner = func(account, platform, message string) (string, error) {
		<-release
		return "late reply", nil
	}
	id := seedChatAccount(t, st, "chat-slow", "connected")
	res, err := svc.Send(id, "hello?")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Pending || res.UserMessage <= 0 {
		t.Fatalf("result = %+v", res)
	}
	if _, done, _, _ := svc.Pending(res.UserMessage); done {
		t.Fatal("run done before release")
	}
	close(release)
	deadline := time.Now().Add(5 * time.Second)
	for {
		reply, done, failed, _ := svc.Pending(res.UserMessage)
		if done {
			if failed || reply != "late reply" {
				t.Fatalf("pending = %q failed=%v", reply, failed)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("run never finished")
		}
		time.Sleep(10 * time.Millisecond)
	}
	items, err := svc.History(id, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[1].Text != "late reply" {
		t.Fatalf("history = %+v", items)
	}
}

func TestPendingUnknown(t *testing.T) {
	st := openChatStore(t)
	svc := NewService(st)
	if _, done, failed, _ := svc.Pending(424242); !done || !failed {
		t.Fatal("unknown run should report failed")
	}
}

func TestHistoryValidation(t *testing.T) {
	st := openChatStore(t)
	svc := NewService(st)
	if _, err := svc.History(9999, 10); !errors.Is(err, ErrNoAccount) {
		t.Fatalf("unknown account: %v", err)
	}
	id := seedChatAccount(t, st, "chat-N", "connected")
	items, err := svc.History(id, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("empty history = %+v", items)
	}
}
