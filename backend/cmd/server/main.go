// Command server boots the SOMUN marketing core engine: config, SQLite store,
// migrations, admin seed, backup scheduler, and the HTTP router.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"somun/internal/auth"
	"somun/internal/config"
	"somun/internal/content"
	"somun/internal/product"
	"somun/internal/queue"
	"somun/internal/router"
	"somun/internal/session"
	"somun/internal/store"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("db open: %w", err)
	}
	defer st.Close()
	if err := store.Migrate(st.DB()); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	authSvc := auth.NewService(st, time.Duration(config.OperatorTokenTTLHours)*time.Hour)
	if err := authSvc.EnsureAdmin(cfg.AdminUser, cfg.AdminPass); err != nil {
		return fmt.Errorf("seed admin: %w", err)
	}
	queueSvc := queue.NewService(st)
	contentSvc := content.NewService(st)
	productSvc := product.NewService(st)
	sessionSvc := session.NewService(st)

	if !cfg.WorkerTokenSet {
		log.Printf("WARN: WORKER_TOKEN not set; using default dev token (local use only)")
	}

	stopBackup := startBackupScheduler(st, cfg)
	defer stopBackup()
	stopCheck := startCheckScheduler(sessionSvc, cfg)
	defer stopCheck()

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router.New(&router.Deps{Config: cfg, Store: st, Auth: authSvc, Queue: queueSvc, Content: contentSvc, Product: productSvc, Session: sessionSvc}),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
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

// startBackupScheduler runs periodic DB+uploads backups (§7 F-10).
// A non-positive interval disables the timer; POST /api/system/backup stays.
func startBackupScheduler(st *store.Store, cfg *config.Config) (stop func()) {
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
				dest, err := store.BackupNow(st.DB(), cfg.DBPath, cfg.UploadDir, cfg.BackupDir, cfg.BackupKeep)
				if err != nil {
					log.Printf("backup failed: %v", err)
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

// startCheckScheduler runs the light periodic session inspection (§9).
// It only refreshes last-checked timestamps; waking stays per-assignment.
func startCheckScheduler(sessionSvc *session.Service, cfg *config.Config) (stop func()) {
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
				log.Printf("session check ok: %d accounts", n)
			case <-done:
				return
			}
		}
	}()
	return func() { close(done) }
}
