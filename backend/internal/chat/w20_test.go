package chat

import (
	"testing"
)

// W20-1: worker turns sent with a product thread land in the store with
// the thread key, so the work's AI originals are queryable per work.
func TestW20SendStoresThread(t *testing.T) {
	st := openChatStore(t)
	svc := NewService(st)
	id := seedChatAccount(t, st, "chat-w20", "connected")
	svc.sendRunner = func(account, platform, message, thread string, files []string) (string, error) {
		if thread != "p5" {
			t.Errorf("runner thread = %q, want p5", thread)
		}
		return "echo:" + message, nil
	}
	res, err := svc.Send(id, "PROMPT-W20", "p5")
	if err != nil {
		t.Fatal(err)
	}
	if res.Pending || res.Reply != "echo:PROMPT-W20" {
		t.Fatalf("send = %+v", res)
	}
	var userThread, asstThread string
	if err := st.DB().QueryRow(`SELECT thread_key FROM chat_messages WHERE account_id=? AND role='user'`, id).Scan(&userThread); err != nil {
		t.Fatal(err)
	}
	if err := st.DB().QueryRow(`SELECT thread_key FROM chat_messages WHERE account_id=? AND role='assistant'`, id).Scan(&asstThread); err != nil {
		t.Fatal(err)
	}
	if userThread != "p5" || asstThread != "p5" {
		t.Fatalf("threads = %q/%q, want p5/p5", userThread, asstThread)
	}
	got, err := st.WorkAILog(5, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Prompt != "PROMPT-W20" || got[0].Reply != "echo:PROMPT-W20" {
		t.Fatalf("ai-log = %+v", got)
	}
}
