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

	"somun/internal/auth"
	"somun/internal/config"
	"somun/internal/content"
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
	Config  *config.Config
	Store   *store.Store
	Auth    *auth.Service
	Queue   *queue.Service
	Content *content.Service
	Product *product.Service
	Session *session.Service
}

// New builds the full HTTP handler tree.
func New(d *Deps) http.Handler {
	mux := http.NewServeMux()
	authH := auth.NewHandler(d.Auth)
	queueH := queue.NewHandler(d.Queue, d.Config.WorkerToken)
	productH := product.NewHandler(d.Product, d.Store, d.Config.UploadDir)
	contentH := content.NewHandler(d.Content, d.Store)
	sessionH := session.NewHandler(d.Session)

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
	mux.Handle("DELETE /api/products/{id}/images/{imageId}", op(http.HandlerFunc(productH.DeleteImage)))
	mux.Handle("POST /api/products/{id}/affiliate-links", op(http.HandlerFunc(productH.CreateAffiliate)))
	mux.Handle("PATCH /api/products/{id}/affiliate-links/{linkId}", op(http.HandlerFunc(productH.UpdateAffiliate)))
	mux.Handle("DELETE /api/products/{id}/affiliate-links/{linkId}", op(http.HandlerFunc(productH.DeleteAffiliate)))

	mux.Handle("GET /api/contents", op(http.HandlerFunc(contentH.List)))
	mux.Handle("GET /api/contents/{id}", op(http.HandlerFunc(contentH.Detail)))
	mux.Handle("PATCH /api/contents/{id}", op(http.HandlerFunc(contentH.Update)))
	mux.Handle("DELETE /api/contents/{id}", op(http.HandlerFunc(contentH.Delete)))
	mux.Handle("POST /api/contents/{id}/schedule", op(http.HandlerFunc(contentH.Schedule)))
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
	mux.Handle("GET /api/jobs", op(http.HandlerFunc(queueH.ListForOperator)))
	mux.Handle("POST /api/jobs/{id}/cancel", op(http.HandlerFunc(queueH.CancelForOperator)))
	mux.Handle("DELETE /api/jobs/{id}", op(http.HandlerFunc(queueH.DeleteForOperator)))
	mux.Handle("DELETE /api/session/accounts/{id}", op(http.HandlerFunc(sessionH.Unregister)))
	mux.Handle("GET /api/session/slots", op(http.HandlerFunc(sessionH.ListSlots)))
	mux.Handle("POST /api/session/slots", op(http.HandlerFunc(sessionH.AddSlot)))
	mux.Handle("PATCH /api/session/slots/{id}", op(http.HandlerFunc(sessionH.UpdateSlot)))
	mux.Handle("DELETE /api/session/slots/{id}", op(http.HandlerFunc(sessionH.DeleteSlot)))

	mux.Handle("GET /api/dashboard", op(http.HandlerFunc(d.dashboard)))
	mux.Handle("POST /api/system/backup", op(http.HandlerFunc(d.backup)))
	mux.Handle("GET /api/system/backups", op(http.HandlerFunc(d.backupList)))

	// Worker-authenticated (§5).
	wk := queueH.Auth
	mux.Handle("GET /api/worker/jobs/next", wk(http.HandlerFunc(queueH.Next)))
	mux.Handle("GET /api/worker/jobs", wk(http.HandlerFunc(queueH.List)))
	mux.Handle("POST /api/worker/jobs/{id}/result", wk(http.HandlerFunc(queueH.Result)))
	mux.Handle("POST /api/worker/jobs/{id}/fail", wk(http.HandlerFunc(queueH.Fail)))

	// Uploaded images (read-only).
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(d.Config.UploadDir))))

	// Web console: serve the built SPA when present, else a JSON hint.
	mux.HandleFunc("/", d.spaFallback())
	return mux
}

func (d *Deps) dashboard(w http.ResponseWriter, r *http.Request) {
	today := time.Now().Format("2006-01-02")
	counts, err := d.Store.DashboardCounts(today)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot load dashboard")
		return
	}
	items, err := d.Store.ListScheduledOn(today)
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
	dest, err := store.BackupNow(d.Store.DB(), d.Config.DBPath, d.Config.UploadDir, d.Config.BackupDir, d.Config.BackupKeep)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "backup failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "dest": dest})
}

func (d *Deps) backupList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"backups": store.ListBackups(d.Config.BackupDir)})
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
