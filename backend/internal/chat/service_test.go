package chat

import (
	"errors"
	"os"
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

// Same-account sends run one at a time; other accounts are unaffected.
func TestAccountSerialization(t *testing.T) {
	st := openChatStore(t)
	svc := NewService(st)
	svc.grace = 20 * time.Millisecond
	id := seedChatAccount(t, st, "chat-ser", "connected")
	entered := make(chan string, 4)
	release := make(chan struct{})
	svc.sendRunner = func(account, platform, message string) (string, error) {
		entered <- message
		<-release
		entered <- "done:" + message
		return "ok", nil
	}
	res1, err := svc.Send(id, "first")
	if err != nil {
		t.Fatal(err)
	}
	res2, err := svc.Send(id, "second")
	if err != nil {
		t.Fatal(err)
	}
	if !res1.Pending || !res2.Pending {
		t.Fatalf("results = %+v %+v", res1, res2)
	}
	// Only the first send may enter; the second waits for the lock.
	select {
	case got := <-entered:
		if got != "first" {
			t.Fatalf("first enter = %q", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("first send never entered")
	}
	select {
	case got := <-entered:
		t.Fatalf("second entered early: %q", got)
	case <-time.After(200 * time.Millisecond):
	}
	close(release)
	deadline := time.Now().Add(5 * time.Second)
	for {
		r1, d1, f1, _ := svc.Pending(res1.UserMessage)
		r2, d2, f2, _ := svc.Pending(res2.UserMessage)
		if d1 && d2 {
			if f1 || f2 || r1 != "ok" || r2 != "ok" {
				t.Fatalf("replies = %q,%q failed=%v,%v", r1, r2, f1, f2)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("runs never finished")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestResetThread(t *testing.T) {
	st := openChatStore(t)
	svc := NewService(st)
	svc.dataDir = t.TempDir()
	id := seedChatAccount(t, st, "chat-reset", "connected")
	dir := filepath.Join(svc.dataDir, "profiles", "chat-reset")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"thread.json", "thread_chain9.json", "keep.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.ResetThread(id, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "thread.json")); !os.IsNotExist(err) {
		t.Fatal("default thread kept")
	}
	if _, err := os.Stat(filepath.Join(dir, "thread_chain9.json")); err != nil {
		t.Fatal("named thread wrongly dropped")
	}
	if err := svc.ResetThread(id, "*"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "thread_chain9.json")); !os.IsNotExist(err) {
		t.Fatal("named thread kept")
	}
	if _, err := os.Stat(filepath.Join(dir, "keep.txt")); err != nil {
		t.Fatal("non-thread file touched")
	}
	if err := svc.ResetThread(id, "../x"); !errors.Is(err, ErrBadThread) {
		t.Fatalf("traversal: %v", err)
	}
	if err := svc.ResetThread(9999, ""); !errors.Is(err, ErrNoAccount) {
		t.Fatalf("unknown: %v", err)
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
