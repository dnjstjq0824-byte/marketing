package session

import (
	"os"
	"path/filepath"
	"testing"
)

// W20-3: connected accounts without session files move to expired (login
// required + relogin queue); accounts with files stay connected.
func TestW20ReconcileProfiles(t *testing.T) {
	svc := newTestService(t)
	dataDir := t.TempDir()
	svc.ConfigureLogin(dataDir, "")
	keep, err := svc.RegisterAccount("keep-1", "meta_ai", "me", "worker", "")
	if err != nil {
		t.Fatal(err)
	}
	gone, err := svc.RegisterAccount("gone-1", "meta_ai", "me", "worker", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetSessionState(keep.ID, StateConnected, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetSessionState(gone.ID, StateConnected, ""); err != nil {
		t.Fatal(err)
	}
	profile := SessionStatePath(dataDir, "keep-1")
	if err := os.MkdirAll(filepath.Dir(profile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profile, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if n := svc.ReconcileProfiles(); n != 1 {
		t.Fatalf("moved = %d, want 1", n)
	}
	if acc, _ := svc.GetAccount(keep.ID); acc.SessionState != StateConnected {
		t.Fatalf("keep state = %s", acc.SessionState)
	}
	if acc, _ := svc.GetAccount(gone.ID); acc.SessionState != StateExpired {
		t.Fatalf("gone state = %s, want expired", acc.SessionState)
	}
	queue := svc.ReloginQueue()
	if len(queue) != 1 || queue[0].AccountName != "gone-1" {
		t.Fatalf("relogin queue = %+v", queue)
	}
}
