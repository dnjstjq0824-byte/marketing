package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"somun/internal/agent"
	"somun/internal/auth"
	"somun/internal/chat"
	"somun/internal/config"
	"somun/internal/content"
	"somun/internal/executor"
	"somun/internal/notify"
	"somun/internal/product"
	"somun/internal/queue"
	"somun/internal/session"
	"somun/internal/store"
)

// W9-2 (B17): /uploads/ serves nothing — retired by W14-5, locked here with
// and without an operator token. Media goes through the authed
// /api/media/* endpoints; publish evidence lives under data/evidence/.
func TestUploadsRetired(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "uploads"), 0o755); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(dir, "r.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := store.Migrate(st.DB()); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Env: "dev", DataDir: dir, UploadDir: filepath.Join(dir, "uploads"), WorkerToken: "test-token"}
	authSvc := auth.NewService(st, time.Hour)
	if err := authSvc.EnsureAdmin("admin", "changeme123"); err != nil {
		t.Fatal(err)
	}
	token, _, err := authSvc.Login("admin", "changeme123")
	if err != nil {
		t.Fatal(err)
	}
	sessionSvc, err := session.NewService(st, filepath.Join(dir, ".vault.key"))
	if err != nil {
		t.Fatal(err)
	}
	h := New(&Deps{
		Config: cfg, Store: st, Auth: authSvc,
		Queue: queue.NewService(st), Content: content.NewService(st),
		Product: product.NewService(st), Session: sessionSvc,
		Notify:   notify.NewService(st),
		Agents:   agent.NewService(st, agent.NewHub(), 600, 25),
		Executor: executor.NewService(st, sessionSvc, nil, store.ExecutorConfig{}),
		Chat:     chat.NewService(st), FileKey: sessionSvc.FileKey(),
	})

	for _, tc := range []struct {
		name  string
		token string
	}{
		{"anonymous", ""},
		{"operator", token},
	} {
		req := httptest.NewRequest(http.MethodGet, "/uploads/evidence/x.png", nil)
		if tc.token != "" {
			req.Header.Set("Authorization", "Bearer "+tc.token)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s /uploads/ = %d, want 404", tc.name, rec.Code)
		}
	}
}
