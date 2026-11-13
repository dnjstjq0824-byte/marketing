// Command server boots the SOMUN marketing core engine: config, SQLite store,
// migrations, admin seed, backup scheduler, and the HTTP router.
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"somun/internal/agent"
	"somun/internal/auth"
	"somun/internal/chat"
	"somun/internal/config"
	"somun/internal/content"
	"somun/internal/executor"
	"somun/internal/notify"
	"somun/internal/product"
	"somun/internal/publisher"
	"somun/internal/queue"
	"somun/internal/router"
	"somun/internal/session"
	"somun/internal/store"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "restore" {
		if err := runRestore(os.Args[2:]); err != nil {
			log.Fatalf("restore: %v", err)
		}
		return
	}
	if err := run(); err != nil {
		log.Fatalf("server: %v", err)
	}
}

// adapterPython locates the adapter venv python relative to the server
// binary (backend/somun.exe -> <root>/adapter/.venv/...), so console-driven
// login works regardless of the launch working directory.
func adapterPython() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	root := filepath.Dir(filepath.Dir(exe))
	py := filepath.Join(root, "adapter", ".venv", "Scripts", "python.exe")
	if _, err := os.Stat(py); err != nil {
		return ""
	}
	return py
}

// runRestore rolls the data dir back to a backup generation (D32).
// The server must be stopped; refusal is by port probe.
func runRestore(args []string) error {
	if len(args) != 1 || args[0] == "" {
		return fmt.Errorf("usage: somun.exe restore <generation>")
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	addr := cfg.BindAddr + ":" + cfg.Port
	if cfg.BindAddr == "" {
		addr = "127.0.0.1:" + cfg.Port
	}
	refuse := func() error {
		conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
		if err != nil {
			return nil
		}
		conn.Close()
		return fmt.Errorf("server appears to be running on %s; stop it first", addr)
	}
	if err := store.RestoreGeneration(cfg.DBPath, cfg.DataDir, cfg.BackupDir, args[0], refuse); err != nil {
		return err
	}
	log.Printf("restore ok: generation %s is live; previous data parked beside %s", args[0], cfg.DataDir)
	return nil
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	st, err := store.OpenPool(cfg.DBPath, cfg.DBReadConns)
	if err != nil {
		return fmt.Errorf("db open: %w", err)
	}
	defer st.Close()
	if cur, err := store.CurrentSchemaVersion(st.DB()); err != nil {
		return fmt.Errorf("schema version: %w", err)
	} else if cur != "" && cur != store.SchemaVersion {
		dest, err := store.BackupNow(st.DB(), cfg.DBPath, cfg.UploadDir, cfg.BackupDir, cfg.VaultKeyPath, cfg.BackupKeep, cfg.MirrorDir)
		if err != nil {
			return fmt.Errorf("pre-migration backup: %w", err)
		}
		log.Printf("pre-migration backup ok: %s (v%s -> v%s)", dest, cur, store.SchemaVersion)
	}
	if err := store.Migrate(st.DB()); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	authSvc := auth.NewService(st, time.Duration(config.OperatorTokenTTLHours)*time.Hour)
	if err := authSvc.EnsureAdmin(cfg.AdminUser, cfg.AdminPass); err != nil {
		return fmt.Errorf("seed admin: %w", err)
	}
	if err := productionGuard(cfg, authSvc); err != nil {
		return err
	}
	queueSvc := queue.NewService(st)
	contentSvc := content.NewService(st)
	productSvc := product.NewService(st)
	sessionSvc, err := session.NewService(st, cfg.VaultKeyPath)
	if err != nil {
		return fmt.Errorf("session vault: %w", err)
	}
	sessionSvc.ConfigureLogin(cfg.DataDir, adapterPython())
	chatSvc := chat.NewService(st)
	chatSvc.Configure(cfg.DataDir, adapterPython())
	chatSvc.AttachSession(sessionSvc)
	notifySvc := notify.NewService(st)
	claimHub := agent.NewHub()
	store.OnJobCreated = claimHub.Notify
	agentSvc := agent.NewService(st, claimHub, cfg.LeaseSec, cfg.LongpollMaxSec)
	agentSvc.Session = sessionSvc
	agentSvc.OnLeaseExpired = func(action store.ReapAction) {
		msg := fmt.Sprintf("작업 #%d(%s): 리스 만료 — 무응답 회수", action.JobID, action.Type)
		if action.Action == "failed_held" {
			msg = fmt.Sprintf("작업 #%d(%s): 리스 만료 — 실행 중단, 게시 여부 사람 확인", action.JobID, action.Type)
		}
		if _, err := notifySvc.Notify(notify.KindLeaseExpired, "job", action.JobID, msg); err != nil {
			log.Printf("notify lease_expired #%d: %v", action.JobID, err)
		}
	}
	queueSvc.OnFail = func(job *store.Job) {
		if _, err := notifySvc.JobFailed(job.ID, job.Type, job.Error); err != nil {
			log.Printf("notify job_failed #%d: %v", job.ID, err)
		}
	}
	queueSvc.OnReviewHuman = func(contentID int64, message string) {
		if _, err := notifySvc.ReviewHuman(contentID, message); err != nil {
			log.Printf("notify review_human #%d: %v", contentID, err)
		}
	}
	sessionSvc.OnSessionState = func(id int64, name, prev, next string) {
		if _, err := notifySvc.SessionState(id, name, prev, next); err != nil {
			log.Printf("notify session %s: %v", name, err)
		}
	}

	if !cfg.WorkerTokenSet {
		log.Printf("WARN: WORKER_TOKEN not set; using default dev token (local use only)")
	}

	stopBackup := startBackupScheduler(st, cfg, notifySvc)
	defer stopBackup()
	stopCheck := startCheckScheduler(sessionSvc, cfg, notifySvc)
	defer stopCheck()
	stopRetention := startRetentionScheduler(st, cfg)
	defer stopRetention()
	stopReaper := startLeaseReaper(agentSvc)
	defer stopReaper()
	execSvc := executor.NewService(st, sessionSvc, publisher.Registry(), store.ExecutorConfig{
		HeartbeatMin: cfg.ExecHeartbeat, MinIntervalMin: cfg.ExecMinGap, MaxActive: cfg.ExecMaxActive,
	})
	execSvc.OnPublishFailed = func(contentID int64, message string) {
		if _, err := notifySvc.Notify(notify.KindPublishFail, "content", contentID, message); err != nil {
			log.Printf("notify publish_failed #%d: %v", contentID, err)
		}
	}
	stopExec := startExecutorTicker(execSvc)
	defer stopExec()

	// Long-polls must finish before the write deadline (D26).
	writeTimeout := 60 * time.Second
	if d := time.Duration(cfg.LongpollMaxSec+10) * time.Second; d > writeTimeout {
		writeTimeout = d
	}
	srv := &http.Server{
		Addr:         cfg.BindAddr + ":" + cfg.Port,
		Handler:      router.New(&router.Deps{Config: cfg, Store: st, Auth: authSvc, Queue: queueSvc, Content: contentSvc, Product: productSvc, Session: sessionSvc, Notify: notifySvc, Agents: agentSvc, Executor: execSvc, Chat: chatSvc}),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: writeTimeout,
	}
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
		<-quit
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	log.Printf("SOMUN core listening: %s", cfg.Summary())
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("listen: %w", err)
	}
	return nil
}

// productionGuard (D31) refuses production boot on default secrets.
// In dev it only warns.
func productionGuard(cfg *config.Config, authSvc *auth.Service) error {
	users, err := authSvc.ListUsers()
	if err != nil {
		return fmt.Errorf("list users for boot guard: %w", err)
	}
	hashes := make([]string, 0, len(users))
	for _, u := range users {
		hashes = append(hashes, u.PasswordHash)
	}
	err = auth.CheckProductionBoot(cfg.WorkerToken, cfg.WorkerTokenSet, config.DefaultWorkerToken, hashes, config.DefaultAdminPass)
	if err == nil {
		return nil
	}
	if cfg.Env != "production" {
		log.Printf("WARN: %v (dev mode continues)", err)
		return nil
	}
	return err
}

// startLeaseReaper reclaims unresponsive claims every minute (D25).
func startLeaseReaper(agentSvc *agent.Service) (stop func()) {
	ticker := time.NewTicker(time.Minute)
	done := make(chan struct{})
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				actions, err := agentSvc.ReapTick(time.Now())
				if err != nil {
					log.Printf("lease reaper failed: %v", err)
					continue
				}
				if len(actions) > 0 {
					log.Printf("lease reaper ok: %d claims", len(actions))
				}
			case <-done:
				return
			}
		}
	}()
	return func() { close(done) }
}

// startExecutorTicker ticks every minute; the executor reads its own
// heartbeat config per tick, so the console toggle needs no restart.
func startExecutorTicker(execSvc *executor.Service) (stop func()) {
	ticker := time.NewTicker(time.Minute)
	done := make(chan struct{})
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				n, err := execSvc.Tick(time.Now())
				if err != nil {
					log.Printf("executor tick failed: %v", err)
					continue
				}
				if n > 0 {
					log.Printf("executor tick ok: %d published", n)
				}
			case <-done:
				return
			}
		}
	}()
	return func() { close(done) }
}

// startBackupScheduler runs periodic consistent backups (D32).
// A non-positive interval disables the timer; POST /api/system/backup stays.
func startBackupScheduler(st *store.Store, cfg *config.Config, notifySvc *notify.Service) (stop func()) {
	if cfg.BackupEveryMin <= 0 {
		return func() {}
	}
	ticker := time.NewTicker(time.Duration(cfg.BackupEveryMin) * time.Minute)
	done := make(chan struct{})
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				dest, err := store.BackupNow(st.DB(), cfg.DBPath, cfg.UploadDir, cfg.BackupDir, cfg.VaultKeyPath, cfg.BackupKeep, cfg.MirrorDir)
				if err != nil {
					log.Printf("backup failed: %v", err)
					_, _ = notifySvc.BackupFailed(err)
					continue
				}
				log.Printf("backup ok: %s", dest)
			case <-done:
				return
			}
		}
	}()
	return func() { close(done) }
}

// startCheckScheduler runs the light periodic session inspection (§9)
// plus the notification sweep (stuck jobs, overdue holds).
// It only refreshes last-checked timestamps; waking stays per-assignment.
func startCheckScheduler(sessionSvc *session.Service, cfg *config.Config, notifySvc *notify.Service) (stop func()) {
	if cfg.CheckEveryMin <= 0 {
		return func() {}
	}
	ticker := time.NewTicker(time.Duration(cfg.CheckEveryMin) * time.Minute)
	done := make(chan struct{})
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				n := sessionSvc.CheckNow()
				stuck, overdue, err := notifySvc.Sweep()
				if err != nil {
					log.Printf("notify sweep failed: %v", err)
				}
				log.Printf("session check ok: %d accounts (notify: stuck=%d overdue=%d)", n, stuck, overdue)
			case <-done:
				return
			}
		}
	}()
	return func() { close(done) }
}

// startRetentionScheduler sweeps finished jobs and old metrics (D20).
// A non-positive interval disables the timer; POST /api/system/purge stays.
func startRetentionScheduler(st *store.Store, cfg *config.Config) (stop func()) {
	if cfg.RetentionEvery <= 0 {
		return func() {}
	}
	ticker := time.NewTicker(time.Duration(cfg.RetentionEvery) * time.Hour)
	done := make(chan struct{})
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				jobs, metrics, notifs, err := store.PurgeExpired(st, config.RetentionJobsDays, config.RetentionMetricsDays, config.RetentionNotificationsDays)
				if err != nil {
					log.Printf("retention sweep failed: %v", err)
					continue
				}
				log.Printf("retention sweep ok: jobs=%d metrics=%d notifications=%d", jobs, metrics, notifs)
			case <-done:
				return
			}
		}
	}()
	return func() { close(done) }
}
