// Package router wires every HTTP route to its domain handler.
package router

import (
	"encoding/json"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

func init() {
	// PWA manifest MIME; OS registries often lack it.
	_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")
}

// Deps bundles the services every route needs.
type Deps struct {
	Config   *config.Config
	Store    *store.Store
	Auth     *auth.Service
	Queue    *queue.Service
	Content  *content.Service
	Product  *product.Service
	Session  *session.Service
	Notify   *notify.Service
	Agents   *agent.Service
	Executor *executor.Service
	Chat     *chat.Service
}

// New builds the full HTTP handler tree.
func New(d *Deps) http.Handler {
	mux := http.NewServeMux()
	authH := auth.NewHandler(d.Auth)
	queueH := queue.NewHandler(d.Queue, d.Agents, d.Config.WorkerToken, d.Config.Env != "production")
	agentH := agent.NewHandler(d.Agents)
	productH := product.NewHandler(d.Product, d.Store, d.Config.UploadDir)
	contentH := content.NewHandler(d.Content, d.Store)
	sessionH := session.NewHandler(d.Session)
	notifyH := notify.NewHandler(d.Notify)
	execH := executor.NewHandler(d.Executor)
	chatH := chat.NewHandler(d.Chat)

	// Public.
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "time": time.Now().UTC().Format(time.RFC3339)})
	})
	mux.HandleFunc("POST /api/auth/login", authH.Login)

	// Operator-authenticated.
	op := d.Auth.Middleware
	mux.Handle("GET /api/auth/me", op(http.HandlerFunc(authH.Me)))
	mux.Handle("POST /api/auth/logout", op(http.HandlerFunc(authH.Logout)))
	mux.Handle("PATCH /api/auth/password", op(http.HandlerFunc(authH.ChangePassword)))
	mux.Handle("GET /api/users", op(http.HandlerFunc(authH.ListUsers)))
	mux.Handle("POST /api/users", op(http.HandlerFunc(authH.CreateUser)))
	mux.Handle("DELETE /api/users/{id}", op(http.HandlerFunc(authH.DeleteUser)))

	mux.Handle("POST /api/products", op(http.HandlerFunc(productH.Create)))
	mux.Handle("GET /api/products", op(http.HandlerFunc(productH.List)))
	mux.Handle("GET /api/products/{id}", op(http.HandlerFunc(productH.Detail)))
	mux.Handle("DELETE /api/products/{id}", op(http.HandlerFunc(productH.Delete)))
	mux.Handle("PATCH /api/products/{id}", op(http.HandlerFunc(productH.Update)))
	mux.Handle("PATCH /api/products/{id}/reading", op(http.HandlerFunc(productH.UpdateReading)))
	mux.Handle("PATCH /api/products/{id}/strategy", op(http.HandlerFunc(productH.UpdateStrategy)))
	mux.Handle("DELETE /api/products/{id}/images/{imageId}", op(http.HandlerFunc(productH.DeleteImage)))
	mux.Handle("POST /api/products/{id}/affiliate-links", op(http.HandlerFunc(productH.CreateAffiliate)))
	mux.Handle("PATCH /api/products/{id}/affiliate-links/{linkId}", op(http.HandlerFunc(productH.UpdateAffiliate)))
	mux.Handle("DELETE /api/products/{id}/affiliate-links/{linkId}", op(http.HandlerFunc(productH.DeleteAffiliate)))

	mux.Handle("GET /api/contents", op(http.HandlerFunc(contentH.List)))
	mux.Handle("GET /api/contents/{id}", op(http.HandlerFunc(contentH.Detail)))
	mux.Handle("PATCH /api/contents/{id}", op(http.HandlerFunc(contentH.Update)))
	mux.Handle("DELETE /api/contents/{id}", op(http.HandlerFunc(contentH.Delete)))
	mux.Handle("POST /api/contents/{id}/schedule", op(http.HandlerFunc(contentH.Schedule)))
	mux.Handle("POST /api/contents/{id}/publish-record", op(http.HandlerFunc(contentH.RecordPublish)))
	mux.Handle("GET /api/contents/{id}/reviews", op(http.HandlerFunc(contentH.ListReviews)))
	mux.Handle("GET /api/publish/plan", op(http.HandlerFunc(sessionH.PublishPlan)))
	mux.Handle("GET /api/publish-queue", op(http.HandlerFunc(contentH.PublishQueue)))
	mux.Handle("GET /api/publish-logs", op(http.HandlerFunc(contentH.ListPublishLogs)))
	mux.Handle("POST /api/publish-logs", op(http.HandlerFunc(contentH.CreatePublishLog)))
	mux.Handle("PATCH /api/publish-logs/{id}", op(http.HandlerFunc(contentH.UpdatePublishLog)))
	mux.Handle("DELETE /api/publish-logs/{id}", op(http.HandlerFunc(contentH.DeletePublishLog)))
	mux.Handle("GET /api/metrics", op(http.HandlerFunc(contentH.ListMetrics)))
	mux.Handle("POST /api/metrics", op(http.HandlerFunc(contentH.CreateMetric)))
	mux.Handle("PATCH /api/metrics/{id}", op(http.HandlerFunc(contentH.UpdateMetricChecks)))
	mux.Handle("DELETE /api/metrics/{id}", op(http.HandlerFunc(contentH.DeleteMetric)))

	mux.Handle("GET /api/session/accounts", op(http.HandlerFunc(sessionH.ListAccounts)))
	mux.Handle("POST /api/session/accounts", op(http.HandlerFunc(sessionH.RegisterAccount)))
	mux.Handle("PATCH /api/session/accounts/{id}", op(http.HandlerFunc(sessionH.UpdateAccount)))
	mux.Handle("POST /api/session/check", op(http.HandlerFunc(sessionH.CheckNow)))
	mux.Handle("GET /api/session/relogin-queue", op(http.HandlerFunc(sessionH.ReloginQueue)))
	mux.Handle("POST /api/session/accounts/{id}/wake", op(http.HandlerFunc(sessionH.Wake)))
	mux.Handle("POST /api/session/accounts/{id}/login-start", op(http.HandlerFunc(sessionH.LoginStart)))
	mux.Handle("POST /api/session/accounts/{id}/login-finish", op(http.HandlerFunc(sessionH.LoginFinish)))
	mux.Handle("POST /api/session/accounts/{id}/login-cancel", op(http.HandlerFunc(sessionH.LoginCancel)))
	mux.Handle("GET /api/jobs", op(http.HandlerFunc(queueH.ListForOperator)))
	mux.Handle("POST /api/jobs/{id}/cancel", op(http.HandlerFunc(queueH.CancelForOperator)))
	mux.Handle("DELETE /api/jobs/{id}", op(http.HandlerFunc(queueH.DeleteForOperator)))
	mux.Handle("POST /api/jobs/{id}/requeue", op(http.HandlerFunc(agentH.Requeue)))
	mux.Handle("DELETE /api/session/accounts/{id}", op(http.HandlerFunc(sessionH.Unregister)))

	mux.Handle("GET /api/agents", op(http.HandlerFunc(agentH.List)))
	mux.Handle("POST /api/agents", op(http.HandlerFunc(agentH.Register)))
	mux.Handle("POST /api/agents/bulk", op(http.HandlerFunc(agentH.Bulk)))
	mux.Handle("GET /api/agents/stats", op(http.HandlerFunc(agentH.Stats)))
	mux.Handle("GET /api/agents/{id}", op(http.HandlerFunc(agentH.Detail)))
	mux.Handle("PATCH /api/agents/{id}", op(http.HandlerFunc(agentH.Update)))
	mux.Handle("DELETE /api/agents/{id}", op(http.HandlerFunc(agentH.Delete)))
	mux.Handle("POST /api/agents/{id}/rotate-token", op(http.HandlerFunc(agentH.Rotate)))

	mux.Handle("GET /api/dashboard", op(http.HandlerFunc(d.dashboard)))
	mux.Handle("GET /api/notifications", op(http.HandlerFunc(notifyH.List)))
	mux.Handle("POST /api/notifications/{id}/read", op(http.HandlerFunc(notifyH.Read)))
	mux.Handle("POST /api/notifications/read-all", op(http.HandlerFunc(notifyH.ReadAll)))
	mux.Handle("GET /api/executor/config", op(http.HandlerFunc(execH.GetConfig)))
	mux.Handle("POST /api/executor/config", op(http.HandlerFunc(execH.SetConfig)))
	mux.Handle("POST /api/system/backup", op(http.HandlerFunc(d.backup)))
	mux.Handle("GET /api/system/backups", op(http.HandlerFunc(d.backupList)))
	mux.Handle("POST /api/system/purge", op(http.HandlerFunc(d.purge)))

	// Worker-authenticated (§5).
	wk := queueH.Auth
	mux.Handle("GET /api/worker/jobs/next", wk(http.HandlerFunc(queueH.Next)))
	mux.Handle("GET /api/worker/jobs", wk(http.HandlerFunc(queueH.List)))
	mux.Handle("POST /api/worker/jobs/{id}/result", wk(http.HandlerFunc(queueH.Result)))
	mux.Handle("POST /api/worker/jobs/{id}/fail", wk(http.HandlerFunc(queueH.Fail)))
	mux.Handle("POST /api/worker/heartbeat", wk(http.HandlerFunc(agentH.Heartbeat)))
	mux.Handle("POST /api/worker/jobs/{id}/heartbeat", wk(http.HandlerFunc(agentH.JobHeartbeat)))
	mux.Handle("POST /api/worker/chat/send", wk(http.HandlerFunc(chatH.Send)))
	mux.Handle("GET /api/worker/chat/pending", wk(http.HandlerFunc(chatH.Pending)))

	mux.Handle("POST /api/chat/send", op(http.HandlerFunc(chatH.Send)))
	mux.Handle("GET /api/chat/pending", op(http.HandlerFunc(chatH.Pending)))
	mux.Handle("GET /api/chat/history", op(http.HandlerFunc(chatH.History)))

	// Uploaded images (read-only).
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(d.Config.UploadDir))))

	// Web console: serve the built SPA when present, else a JSON hint.
	mux.HandleFunc("/", d.spaFallback())
	return mux
}

func (d *Deps) dashboard(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	fromUTC := midnight.UTC().Format(time.RFC3339)
	toUTC := midnight.Add(24 * time.Hour).UTC().Format(time.RFC3339)
	counts, err := d.Store.DashboardCounts(fromUTC, toUTC)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot load dashboard")
		return
	}
	items, err := d.Store.ListScheduledRange(fromUTC, toUTC)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot load dashboard")
		return
	}
	if items == nil {
		items = []*store.Content{}
	}
	relogin := d.Session.ReloginQueue()
	byBiz, err := d.Store.DashboardByBusiness()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot load dashboard")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"counts": counts, "today": items, "byBusiness": byBiz, "reloginNeeded": relogin})
}

func (d *Deps) backup(w http.ResponseWriter, r *http.Request) {
	dest, err := store.BackupNow(d.Store.DB(), d.Config.DBPath, d.Config.UploadDir, d.Config.BackupDir, d.Config.VaultKeyPath, d.Config.BackupKeep, d.Config.MirrorDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "backup failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "dest": dest})
}

func (d *Deps) backupList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"backups": store.ListBackups(d.Config.BackupDir)})
}

func (d *Deps) purge(w http.ResponseWriter, r *http.Request) {
	jobs, metrics, notifs, err := store.PurgeExpired(d.Store, config.RetentionJobsDays, config.RetentionMetricsDays, config.RetentionNotificationsDays)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "purge failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "jobs": jobs, "metrics": metrics, "notifications": notifs})
}

func (d *Deps) spaFallback() http.HandlerFunc {
	index := filepath.Join(d.Config.WebDistDir, "index.html")
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && strings.HasPrefix(r.URL.Path, "/api/") {
			writeError(w, http.StatusNotFound, "unknown API route")
			return
		}
		if _, err := os.Stat(index); err != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":      true,
				"message": "SOMUN marketing core API is running; build the web console (npm run build in web/) to serve it here.",
				"time":    time.Now().UTC().Format(time.RFC3339),
			})
			return
		}
		candidate := filepath.Join(d.Config.WebDistDir, filepath.Clean("/"+r.URL.Path))
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			http.ServeFile(w, r, candidate)
			return
		}
		// The shell must never stick: hashed assets cache, index.html revalidates.
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, index)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}
