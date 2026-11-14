// Package router wires every HTTP route to its domain handler.
package router

import (
	"encoding/json"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"somun/internal/agent"
	"somun/internal/auth"
	"somun/internal/chat"
	"somun/internal/config"
	"somun/internal/content"
	"somun/internal/executor"
	"somun/internal/media"
	"somun/internal/notify"
	"somun/internal/product"
	"somun/internal/queue"
	"somun/internal/session"
	"somun/internal/store"
	"somun/internal/teams"
	"somun/internal/works"
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
	mediaH := media.NewHandler(media.NewService(d.Store, d.Config.UploadDir))
	worksH := works.NewHandler(works.NewService(d.Store), d.Store, d.Config.UploadDir)
	teamsH := teams.NewHandler(teams.NewService(d.Store), d.Store)

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
	mux.Handle("POST /api/session/accounts/bulk", op(http.HandlerFunc(sessionH.RegisterBulk)))
	mux.Handle("POST /api/session/login-queue/next", op(http.HandlerFunc(sessionH.NextLogin)))
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
	mux.Handle("GET /api/overview", op(http.HandlerFunc(d.overview)))
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
	mux.Handle("POST /api/chat/threads/reset", op(http.HandlerFunc(chatH.ResetThread)))

	mux.Handle("POST /api/works", op(http.HandlerFunc(worksH.Create)))
	mux.Handle("GET /api/works", op(http.HandlerFunc(worksH.List)))
	mux.Handle("GET /api/works/{id}", op(http.HandlerFunc(worksH.Detail)))
	mux.Handle("POST /api/works/{id}/pause", op(http.HandlerFunc(worksH.Pause)))
	mux.Handle("POST /api/works/{id}/resume", op(http.HandlerFunc(worksH.Resume)))
	mux.Handle("DELETE /api/works/{id}", op(http.HandlerFunc(worksH.DeleteWork)))
	mux.Handle("GET /api/works/{id}/ai-log", op(http.HandlerFunc(worksH.AILog)))
	mux.Handle("POST /api/works/{id}/posts/{postId}/hold", op(http.HandlerFunc(worksH.HoldPost)))
	mux.Handle("POST /api/works/{id}/posts:generate", op(http.HandlerFunc(worksH.RegeneratePost)))

	mux.Handle("POST /api/media/uploads", op(http.HandlerFunc(mediaH.Init)))
	mux.Handle("PUT /api/media/uploads/{id}/parts/{n}", op(http.HandlerFunc(mediaH.PutPart)))
	mux.Handle("POST /api/media/uploads/{id}/complete", op(http.HandlerFunc(mediaH.Complete)))
	mux.Handle("GET /api/media", op(http.HandlerFunc(mediaH.List)))
	mux.Handle("GET /api/media/{id}", op(http.HandlerFunc(mediaH.Detail)))
	mux.Handle("GET /api/media/{id}/file", op(http.HandlerFunc(mediaH.File)))
	mux.Handle("POST /api/media/zip", op(http.HandlerFunc(mediaH.Zip)))
	mux.Handle("DELETE /api/media/{id}", op(http.HandlerFunc(mediaH.Delete)))

	mux.Handle("GET /api/teams", op(http.HandlerFunc(teamsH.List)))
	mux.Handle("POST /api/teams", op(http.HandlerFunc(teamsH.Create)))
	mux.Handle("GET /api/teams/{id}", op(http.HandlerFunc(teamsH.Detail)))
	mux.Handle("PATCH /api/teams/{id}", op(http.HandlerFunc(teamsH.Update)))
	mux.Handle("DELETE /api/teams/{id}", op(http.HandlerFunc(teamsH.Delete)))
	mux.Handle("PUT /api/teams/{id}/members", op(http.HandlerFunc(teamsH.PutMembers)))
	mux.Handle("POST /api/teams/{id}/members", op(http.HandlerFunc(teamsH.AddMember)))
	mux.Handle("DELETE /api/teams/{id}/members/{accountId}", op(http.HandlerFunc(teamsH.RemoveMember)))
	mux.Handle("POST /api/teams/{id}/members/{accountId}/roles", op(http.HandlerFunc(teamsH.SetMemberRole)))
	mux.Handle("POST /api/teams/{id}/leader", op(http.HandlerFunc(teamsH.SetLeader)))
	mux.Handle("PUT /api/teams/{id}/channels", op(http.HandlerFunc(teamsH.PutChannels)))
	mux.Handle("POST /api/teams/{id}/channels", op(http.HandlerFunc(teamsH.AddChannel)))
	mux.Handle("DELETE /api/teams/{id}/channels/{accountId}", op(http.HandlerFunc(teamsH.RemoveChannel)))
	mux.Handle("PUT /api/teams/{id}/brand", op(http.HandlerFunc(teamsH.PutBrand)))

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

func (d *Deps) overview(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	fromUTC := midnight.UTC().Format(time.RFC3339)
	weekUTC := midnight.Add(-7 * 24 * time.Hour).UTC().Format(time.RFC3339)
	today, err := d.Store.OverviewToday(fromUTC, weekUTC)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot load overview")
		return
	}
	running, err := d.Store.OverviewRunning(5)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot load overview")
		return
	}
	posts, err := d.Store.OverviewRecentPosts(6)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot load overview")
		return
	}
	media, err := d.Store.OverviewRecentMedia(12)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot load overview")
		return
	}
	aiConnected, channelConnected, hasWork, err := d.Store.OverviewFirstRun()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot load overview")
		return
	}
	actions := d.overviewActions()
	writeJSON(w, http.StatusOK, map[string]any{
		"today": today, "running": running, "recentPosts": posts, "recentMedia": media,
		"actions": actions, "teams": []any{},
		"firstRun": map[string]bool{"aiConnected": aiConnected, "channelConnected": channelConnected, "hasWork": hasWork},
	})
}

// overviewActions builds the S1 action banner (UI_DESIGN S1). Automation-off
// and per-platform gaps arrive with W0/W21; teams with W16.
func (d *Deps) overviewActions() []any {
	actions := []any{}
	for _, item := range d.Session.ReloginQueue() {
		actions = append(actions, map[string]string{
			"kind": "login", "account": item.AccountName,
			"message": item.AccountName + " 로그인이 풀렸습니다",
			"href":    "/connect",
		})
	}
	if n, err := d.Store.OverviewUnassignedQueued(); err == nil && n > 0 {
		actions = append(actions, map[string]string{
			"kind": "account",
			"message": "연결 계정 없는 발행 " + strconv.Itoa(n) + "건",
			"href":    "/connect",
		})
	}
	if logs, err := d.Store.OverviewFailedPublishes(5); err == nil {
		for range logs {
			actions = append(actions, map[string]string{
				"kind": "failed",
				"message": "발행 실패: 확인이 필요합니다",
				"href":    "/works",
			})
		}
	}
	if same, err := d.sameReviewerAccount(); err == nil && same {
		actions = append(actions, map[string]string{
			"kind": "reviewer",
			"message": "만든 AI와 검수 AI가 같습니다. AI를 추가하세요",
			"href":    "/connect",
		})
	}
	return actions
}

func (d *Deps) sameReviewerAccount() (bool, error) {
	gen, err := d.Store.ListAgents(store.AgentFilter{Lane: "B"}, 10, 0)
	if err != nil {
		return false, err
	}
	rev, err := d.Store.ListAgents(store.AgentFilter{Lane: "C"}, 10, 0)
	if err != nil {
		return false, err
	}
	for _, g := range gen {
		if !g.Enabled || g.AccountID == nil {
			continue
		}
		for _, r := range rev {
			if r.Enabled && r.AccountID != nil && *r.AccountID == *g.AccountID {
				return true, nil
			}
		}
	}
	return false, nil
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
