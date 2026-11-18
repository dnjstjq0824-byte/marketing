// Package router wires every HTTP route to its domain handler.
package router

import (
	"encoding/json"
	"errors"
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
	"somun/internal/member"
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
	Member   *member.Service
	// FileKey signs worker media URLs (W5-3). Empty refuses all.
	FileKey []byte
	// AdapterPython is the adapter venv python for AI media generation
	// (W14-2/3). Empty disables /api/worker/media/gen with 503.
	AdapterPython string
}

// New builds the full HTTP handler tree.
func New(d *Deps) http.Handler {
	mux := http.NewServeMux()
	authH := auth.NewHandler(d.Auth)
	queueH := queue.NewHandler(d.Queue, d.Agents, d.Config.WorkerToken, d.Config.Env != "production")
	queueH.Production = d.Config.Env == "production"
	agentH := agent.NewHandler(d.Agents)
	productH := product.NewHandler(d.Product, d.Store, d.Config.UploadDir)
	contentH := content.NewHandler(d.Content, d.Store)
	sessionH := session.NewHandler(d.Session)
	notifyH := notify.NewHandler(d.Notify)
	chatH := chat.NewHandler(d.Chat)
	mediaSvc := media.NewService(d.Store, d.Config.UploadDir)
	mediaSvc.FileKey = d.FileKey
	mediaSvc.Configure(d.Config.DataDir, d.AdapterPython)
	mediaH := media.NewHandler(mediaSvc)
	worksH := works.NewHandler(works.NewService(d.Store), d.Store, d.Config.UploadDir)
	teamsH := teams.NewHandler(teams.NewService(d.Store), d.Store)
	memberH := member.NewHandler(d.Member)

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
	mux.Handle("DELETE /api/products/{id}/images/{imageId}", op(http.HandlerFunc(productH.DeleteImage)))

	mux.Handle("GET /api/contents", op(http.HandlerFunc(contentH.List)))
	mux.Handle("GET /api/contents/{id}", op(http.HandlerFunc(contentH.Detail)))
	mux.Handle("DELETE /api/contents/{id}", op(http.HandlerFunc(contentH.Delete)))
	mux.Handle("GET /api/contents/{id}/reviews", op(http.HandlerFunc(contentH.ListReviews)))
	mux.Handle("GET /api/publish-logs", op(http.HandlerFunc(contentH.ListPublishLogs)))
	mux.Handle("GET /api/publish-trend", op(http.HandlerFunc(contentH.PublishTrend)))
	mux.Handle("GET /api/metrics", op(http.HandlerFunc(contentH.ListMetrics)))

	mux.Handle("GET /api/session/accounts", op(http.HandlerFunc(sessionH.ListAccounts)))
	mux.Handle("POST /api/session/accounts", op(http.HandlerFunc(sessionH.RegisterAccount)))
	mux.Handle("POST /api/session/accounts/bulk", op(http.HandlerFunc(sessionH.RegisterBulk)))
	mux.Handle("POST /api/session/login-queue/next", op(http.HandlerFunc(sessionH.NextLogin)))
	mux.Handle("PATCH /api/session/accounts/{id}", op(http.HandlerFunc(sessionH.UpdateAccount)))
	mux.Handle("GET /api/session/relogin-queue", op(http.HandlerFunc(sessionH.ReloginQueue)))
	mux.Handle("POST /api/session/accounts/{id}/login-start", op(http.HandlerFunc(sessionH.LoginStart)))
	mux.Handle("POST /api/session/accounts/{id}/login-finish", op(http.HandlerFunc(sessionH.LoginFinish)))
	mux.Handle("POST /api/session/accounts/{id}/login-cancel", op(http.HandlerFunc(sessionH.LoginCancel)))
	mux.Handle("POST /api/session/accounts/{id}/chat-probe", op(http.HandlerFunc(sessionH.ChatProbe)))
	mux.Handle("GET /api/jobs", op(http.HandlerFunc(queueH.ListForOperator)))
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
	mux.Handle("POST /api/system/backup", op(http.HandlerFunc(d.backup)))
	mux.Handle("GET /api/system/backups", op(http.HandlerFunc(d.backupList)))
	mux.Handle("POST /api/system/purge", op(http.HandlerFunc(d.purge)))
	mux.Handle("GET /api/system/version", op(http.HandlerFunc(d.version)))
	mux.Handle("GET /api/system/health", op(http.HandlerFunc(d.health)))
	mux.Handle("GET /api/channel-platforms", op(http.HandlerFunc(d.listPlatforms)))
	mux.Handle("PATCH /api/channel-platforms/{key}", op(http.HandlerFunc(d.patchPlatform)))
	mux.Handle("GET /api/publish-backends", op(http.HandlerFunc(sessionH.ListBackends)))

	// Public consignment surface (W23): signup/login plus the catalog.
	// No auth on reads; member middleware guards personal routes.
	mux.HandleFunc("POST /api/promo/signup", memberH.Signup)
	mux.HandleFunc("POST /api/promo/login", memberH.Login)
	mux.HandleFunc("GET /api/promo/products", memberH.ListProducts)
	mux.HandleFunc("GET /api/promo/products/{id}", memberH.ProductDetail)
	mux.HandleFunc("GET /api/promo/stats", memberH.Stats)
	mm := d.Member.Middleware
	mux.Handle("POST /api/promo/logout", mm(http.HandlerFunc(memberH.Logout)))
	mux.Handle("GET /api/promo/me", mm(http.HandlerFunc(memberH.Me)))
	mux.Handle("POST /api/promo/products/{id}/apply", mm(http.HandlerFunc(memberH.Apply)))
	mux.Handle("GET /api/promo/applications", mm(http.HandlerFunc(memberH.MyApplications)))
	mux.Handle("GET /api/promo/applications/{id}/private-link", mm(http.HandlerFunc(memberH.PrivateLink)))
	mux.Handle("GET /api/promo/work", mm(http.HandlerFunc(memberH.Work)))
	mux.Handle("POST /api/promo/applications/{id}/posts", mm(http.HandlerFunc(memberH.RegisterPost)))

	// Consignment administration (W23): operator-authenticated.
	mux.Handle("POST /api/promo/admin/products", op(http.HandlerFunc(memberH.CreateProduct)))
	mux.Handle("GET /api/promo/admin/products", op(http.HandlerFunc(memberH.ListProductsAdmin)))
	mux.Handle("PATCH /api/promo/admin/products/{id}", op(http.HandlerFunc(memberH.UpdateProduct)))
	mux.Handle("POST /api/promo/admin/products/{id}/recruit", op(http.HandlerFunc(memberH.Recruit)))
	mux.Handle("GET /api/promo/admin/applications", op(http.HandlerFunc(memberH.ListApplicationsAdmin)))
	mux.Handle("POST /api/promo/admin/applications/{id}/decision", op(http.HandlerFunc(memberH.Decide)))
	mux.Handle("GET /api/promo/admin/overview", op(http.HandlerFunc(memberH.Overview)))
	mux.Handle("POST /api/promo/admin/applications/{id}/confirm", op(http.HandlerFunc(memberH.ConfirmOrders)))
	mux.Handle("POST /api/promo/admin/invites", op(http.HandlerFunc(memberH.MintInvites)))
	mux.Handle("GET /api/promo/admin/invites", op(http.HandlerFunc(memberH.ListInvites)))
	mux.Handle("POST /api/promo/admin/invites/{id}/revoke", op(http.HandlerFunc(memberH.RevokeInvite)))
	mux.Handle("GET /api/ai-platforms", op(http.HandlerFunc(d.listAIPlatforms)))
	mux.Handle("POST /api/ai-platforms", op(http.HandlerFunc(d.createAIPlatform)))
	mux.Handle("PATCH /api/ai-platforms/{key}", op(http.HandlerFunc(d.patchAIPlatform)))
	mux.Handle("DELETE /api/ai-platforms/{key}", op(http.HandlerFunc(d.deleteAIPlatform)))
	mux.Handle("GET /api/settings/automation", op(http.HandlerFunc(d.getAutomationSettings)))
	mux.Handle("PUT /api/settings/automation", op(http.HandlerFunc(d.putAutomationSettings)))
	mux.Handle("GET /api/automation", op(http.HandlerFunc(d.getAutomation)))
	mux.Handle("PUT /api/automation", op(http.HandlerFunc(d.putAutomation)))

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
	mux.Handle("GET /api/worker/boot-check", wk(http.HandlerFunc(queueH.BootCheck)))
	// W14: workers upload local renders and request AI generations.
	mux.Handle("POST /api/worker/media/uploads", wk(http.HandlerFunc(mediaH.Init)))
	mux.Handle("PUT /api/worker/media/uploads/{id}/parts/{n}", wk(http.HandlerFunc(mediaH.PutPart)))
	mux.Handle("POST /api/worker/media/uploads/{id}/complete", wk(http.HandlerFunc(mediaH.Complete)))
	mux.Handle("POST /api/worker/media/gen", wk(http.HandlerFunc(mediaH.GenAI)))

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
	mux.Handle("GET /api/works/{id}/audit", op(http.HandlerFunc(worksH.Audit)))
	mux.Handle("POST /api/works/{id}/posts/{postId}/hold", op(http.HandlerFunc(worksH.HoldPost)))
	mux.Handle("POST /api/works/{id}/posts:generate", op(http.HandlerFunc(worksH.RegeneratePost)))
	mux.Handle("POST /api/works/{id}/posts/{postId}/confirm-published", op(http.HandlerFunc(contentH.ConfirmPublished)))

	mux.Handle("POST /api/media/uploads", op(http.HandlerFunc(mediaH.Init)))
	mux.Handle("PUT /api/media/uploads/{id}/parts/{n}", op(http.HandlerFunc(mediaH.PutPart)))
	mux.Handle("POST /api/media/uploads/{id}/complete", op(http.HandlerFunc(mediaH.Complete)))
	mux.Handle("GET /api/media", op(http.HandlerFunc(mediaH.List)))
	mux.Handle("GET /api/media/{id}", op(http.HandlerFunc(mediaH.Detail)))
	mux.Handle("GET /api/media/{id}/file", op(http.HandlerFunc(mediaH.File)))
	mux.Handle("GET /api/media/{id}/download", op(http.HandlerFunc(mediaH.File)))
	mux.Handle("GET /api/media/{id}/thumb", op(http.HandlerFunc(mediaH.Thumb)))
	mux.Handle("GET /api/media/{id}/signed", http.HandlerFunc(mediaH.SignedFile))
	mux.Handle("POST /api/media/zip", op(http.HandlerFunc(mediaH.Zip)))
	mux.Handle("DELETE /api/media/{id}", op(http.HandlerFunc(mediaH.Delete)))
	mux.Handle("GET /api/product-images/{id}", op(http.HandlerFunc(mediaH.LegacyImage)))

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

	// W14-5 + W9-2 (B17): the public /uploads/ path is retired — nothing
	// under it is served, with or without a token. Media serves only
	// through the authed /api/media/* endpoints above, and publish
	// evidence lives under data/evidence/, outside any served tree.
	mux.Handle("GET /uploads/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "retired; use /api/media")
	}))
	mux.Handle("GET /uploads", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "retired; use /api/media")
	}))

	// Web console: serve the built SPA when present, else a JSON hint.
	mux.HandleFunc("/", d.spaFallback())
	return secureHeaders(mux)
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
	dest, err := store.BackupNowWithSessions(d.Store.DB(), d.Config.DBPath, d.Config.UploadDir, d.Config.BackupDir,
		d.Config.VaultKeyPath, d.Config.BackupKeep, d.Config.MirrorDir,
		filepath.Join(d.Config.DataDir, "profiles"), d.FileKey, store.SessionsIncluded(d.Store.DB()))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "backup failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "dest": dest})
}

func (d *Deps) backupList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"backups": store.ListBackups(d.Config.BackupDir),
		"keep":    d.Config.BackupKeep,
	})
}

func (d *Deps) version(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"app": config.AppVersion, "schema": store.SchemaVersion})
}

// health serves GET /api/system/health (W20-4): DB, disk free, executor
// connectivity, per-PC load, per-team backlog, and the 10 newest errors.
// Free space under the alert line files one disk_low notification.
func (d *Deps) health(w http.ResponseWriter, r *http.Request) {
	db := d.Store.ProbeDB()
	disk := store.DiskUsage(d.Config.DataDir)
	agents, err := d.Store.AgentHealth(time.Now())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot load health")
		return
	}
	queues, err := d.Store.TeamPendingCounts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot load health")
		return
	}
	errs, err := d.Store.RecentErrors(10)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot load health")
		return
	}
	online := 0
	for _, a := range agents {
		if a.Online {
			online++
		}
	}
	if disk.TotalBytes > 0 && disk.Low && d.Notify != nil {
		_, _ = d.Notify.DiskLow(disk.FreePct)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"db": db, "disk": disk,
		"executors": map[string]any{"online": online, "total": len(agents), "agents": agents},
		"pcLoad":    store.PCLoad(agents), "teamQueue": queues, "recentErrors": errs,
		"time": time.Now().UTC().Format(time.RFC3339),
	})
}

func (d *Deps) listPlatforms(w http.ResponseWriter, r *http.Request) {
	items, err := d.Store.ListChannelPlatforms()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot list platforms")
		return
	}
	if items == nil {
		items = []*store.ChannelPlatform{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (d *Deps) patchPlatform(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := d.Store.SetChannelPlatformStatus(key, strings.TrimSpace(req.Status)); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "platform not found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	p, _ := d.Store.GetChannelPlatform(key)
	writeJSON(w, http.StatusOK, p)
}

func (d *Deps) listAIPlatforms(w http.ResponseWriter, r *http.Request) {
	items, err := d.Store.ListAIPlatforms()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot list AI platforms")
		return
	}
	if items == nil {
		items = []*store.AIPlatform{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (d *Deps) createAIPlatform(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key      string `json:"key"`
		Name     string `json:"name"`
		LoginURL string `json:"loginUrl"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	p, err := d.Store.CreateCustomAIPlatform(req.Key, req.Name, req.LoginURL)
	if err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			writeError(w, http.StatusConflict, "platform key already exists")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (d *Deps) patchAIPlatform(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	p, err := d.Store.GetAIPlatform(key)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "AI platform not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "cannot load AI platform")
		return
	}
	var req struct {
		Name              *string `json:"name"`
		LoginURL          *string `json:"loginUrl"`
		ChatURL           *string `json:"chatUrl"`
		LoggedOutMarkers  *string `json:"loggedOutMarkers"`
		ComposerSelector  *string `json:"composerSelector"`
		SendSelector      *string `json:"sendSelector"`
		ReplySelector     *string `json:"replySelector"`
		ReplyBodySelector *string `json:"replyBodySelector"`
		BusySelector      *string `json:"busySelector"`
		DoneSelector      *string `json:"doneSelector"`
		ThreadURLPattern  *string `json:"threadUrlPattern"`
		Icon              *string `json:"icon"`
		Color             *string `json:"color"`
		ChatStatus        *string `json:"chatStatus"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	apply := func(dst *string, v *string) {
		if v != nil {
			*dst = strings.TrimSpace(*v)
		}
	}
	apply(&p.Name, req.Name)
	apply(&p.LoginURL, req.LoginURL)
	apply(&p.ChatURL, req.ChatURL)
	apply(&p.LoggedOutMarkers, req.LoggedOutMarkers)
	apply(&p.ComposerSelector, req.ComposerSelector)
	apply(&p.SendSelector, req.SendSelector)
	apply(&p.ReplySelector, req.ReplySelector)
	apply(&p.ReplyBodySelector, req.ReplyBodySelector)
	apply(&p.BusySelector, req.BusySelector)
	apply(&p.DoneSelector, req.DoneSelector)
	apply(&p.ThreadURLPattern, req.ThreadURLPattern)
	apply(&p.Icon, req.Icon)
	apply(&p.Color, req.Color)
	apply(&p.ChatStatus, req.ChatStatus)
	if err := d.Store.UpdateAIPlatform(p); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated, _ := d.Store.GetAIPlatform(key)
	writeJSON(w, http.StatusOK, updated)
}

func (d *Deps) deleteAIPlatform(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if err := d.Store.DeleteCustomAIPlatform(key); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "AI platform not found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "deleted": key})
}

func (d *Deps) getAutomationSettings(w http.ResponseWriter, r *http.Request) {
	s, err := store.GetAutomationSettings(d.Store.DB())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot load settings")
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (d *Deps) putAutomationSettings(w http.ResponseWriter, r *http.Request) {
	var s store.AutomationSettings
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := store.SetAutomationSettings(d.Store.DB(), s); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_, _ = d.Store.AppendAudit(auth.OperatorName(r.Context()), store.AuditSettingsChange, "system", 0, 0, "자동화 설정 저장")
	writeJSON(w, http.StatusOK, s)
}

func (d *Deps) getAutomation(w http.ResponseWriter, r *http.Request) {
	s, err := store.GetAutomationSettings(d.Store.DB())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot load automation state")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"enabled": s.Enabled})
}

func (d *Deps) putAutomation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Enabled == nil {
		writeError(w, http.StatusBadRequest, "enabled is required")
		return
	}
	s, err := store.SetAutomationEnabled(d.Store.DB(), *req.Enabled)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot save automation state")
		return
	}
	verb := "자동화 켜기"
	if !*req.Enabled {
		verb = "자동화 끄기(비상 정지)"
	}
	_, _ = d.Store.AppendAudit(auth.OperatorName(r.Context()), store.AuditSettingsChange, "system", 0, 0, verb)
	writeJSON(w, http.StatusOK, map[string]any{"enabled": s.Enabled})
}

func (d *Deps) purge(w http.ResponseWriter, r *http.Request) {
	jobs, metrics, notifs, err := store.PurgeExpired(d.Store, config.RetentionJobsDays, config.RetentionMetricsDays, config.RetentionNotificationsDays)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "purge failed: "+err.Error())
		return
	}
	chat, err := store.PurgeChat(d.Store, config.RetentionChatDays)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "purge failed: "+err.Error())
		return
	}
	// W14-7: unused generated files age out with the rest; linked files
	// always survive.
	swept := 0
	if settings, serr := store.GetAutomationSettings(d.Store.DB()); serr == nil {
		if n, merr := media.NewService(d.Store, d.Config.UploadDir).SweepStale(settings.MediaRetentionDays, time.Now()); merr == nil {
			swept = n
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "jobs": jobs, "metrics": metrics, "notifications": notifs, "chat": chat, "media": swept})
}

// secureHeaders sets baseline response headers (beta hardening).
// No clickjacking, no MIME sniffing, no leaking full URLs to others.
func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
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
			if r.URL.Path == "/sw.js" {
				// The worker script must revalidate so a release bump retires old shells.
				w.Header().Set("Cache-Control", "no-cache")
			}
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
