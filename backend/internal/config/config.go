// Package config loads and validates server configuration from environment
// variables. All tunables live here so other layers never read os.Getenv.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// AppVersion is the console "버전" row (S6). Bump on user-visible releases.
const AppVersion = "2026.11.15"

// Defaults for local development. Production must override secrets via env.
const (
	DefaultPort           = "8080"
	DefaultBindDev        = "" // all interfaces (dev only)
	DefaultBindProduction = "127.0.0.1"
	DefaultEnv            = "dev"
	DefaultDataDir        = "data"
	DefaultDBName         = "somun.db"
	DefaultWorkerToken    = "local-worker-token"
	DefaultAdminUser      = "admin"
	DefaultAdminPass      = "changeme123"
	DefaultBackupEveryMin = 60
	DefaultBackupKeep     = 7
	DefaultWebDistDir     = "web/dist"
	DefaultVaultKeyName   = ".vault.key"
	// Retention windows (D20). Finished jobs 90 days, metrics 180 days.
	RetentionJobsDays    = 90
	RetentionMetricsDays = 180
	// Read notifications 90 days (P0-3).
	RetentionNotificationsDays = 90
	// Scale base (P1-0).
	DefaultLeaseSec    = 600
	DefaultLongpollMax = 25
	DefaultDBReadConns = 4
	// Executor (P1-1). Heartbeat 0 stops execution. W0: fresh installs run.
	DefaultExecutorHeartbeatMin = 15
	DefaultExecutorMinInterval  = 30
	DefaultExecutorMaxActive    = 2
	DefaultRetentionEvery       = 24
	MaxUploadMB                 = 10
	MaxImagesPerProduct         = 10
	OperatorTokenTTLHours       = 12
)

// Config is the effective server configuration.
type Config struct {
	Env            string // dev (default) or production
	BindAddr       string // "" = all interfaces (dev); production defaults to loopback
	Port           string
	DataDir        string
	DBPath         string
	UploadDir      string
	BackupDir      string
	WebDistDir     string
	WorkerToken    string
	WorkerTokenSet bool // true only when provided via env
	AdminUser      string
	AdminPass      string
	BackupEveryMin int // 0 disables the scheduler; manual backup stays available
	BackupKeep     int
	CheckEveryMin  int // light session inspection interval; 0 disables the timer
	VaultKeyPath   string
	RetentionEvery int // retention sweep interval in hours; 0 disables the timer
	MirrorDir      string
	LeaseSec       int
	LongpollMaxSec int
	DBReadConns    int
	ExecHeartbeat  int
	ExecMinGap     int
	ExecMaxActive  int
}

// Load reads env, applies defaults, validates, and ensures directories.
func Load() (*Config, error) {
	c := &Config{
		Env:        envOr("SOMUN_ENV", DefaultEnv),
		Port:       envOr("PORT", DefaultPort),
		DataDir:    envOr("DATA_DIR", DefaultDataDir),
		WebDistDir: envOr("WEB_DIST_DIR", DefaultWebDistDir),
		AdminUser:  envOr("ADMIN_USER", DefaultAdminUser),
	}
	if c.Env != "dev" && c.Env != "production" {
		return nil, fmt.Errorf("SOMUN_ENV must be dev or production")
	}
	if c.Env == "production" {
		c.BindAddr = envOr("BIND_ADDR", DefaultBindProduction)
	} else {
		c.BindAddr = strings.TrimSpace(os.Getenv("BIND_ADDR"))
	}
	c.WorkerToken = os.Getenv("WORKER_TOKEN")
	if c.WorkerToken == "" {
		c.WorkerToken = DefaultWorkerToken
	} else {
		c.WorkerTokenSet = true
	}
	c.AdminPass = os.Getenv("ADMIN_PASS")
	if c.AdminPass == "" {
		c.AdminPass = DefaultAdminPass
	}
	every, err := strconv.Atoi(envOr("BACKUP_INTERVAL_MIN", strconv.Itoa(DefaultBackupEveryMin)))
	if err != nil || every < 0 {
		return nil, fmt.Errorf("BACKUP_INTERVAL_MIN must be a non-negative integer")
	}
	c.BackupEveryMin = every
	keep, err := strconv.Atoi(envOr("BACKUP_KEEP", strconv.Itoa(DefaultBackupKeep)))
	if err != nil || keep < 1 {
		return nil, fmt.Errorf("BACKUP_KEEP must be a positive integer")
	}
	c.BackupKeep = keep
	checkEvery, err := strconv.Atoi(envOr("CHECK_INTERVAL_MIN", "15"))
	if err != nil || checkEvery < 0 {
		return nil, fmt.Errorf("CHECK_INTERVAL_MIN must be a non-negative integer")
	}
	c.CheckEveryMin = checkEvery
	retentionEvery, err := strconv.Atoi(envOr("RETENTION_CHECK_HOURS", strconv.Itoa(DefaultRetentionEvery)))
	if err != nil || retentionEvery < 0 {
		return nil, fmt.Errorf("RETENTION_CHECK_HOURS must be a non-negative integer")
	}
	c.RetentionEvery = retentionEvery
	leaseSec, err := strconv.Atoi(envOr("LEASE_SEC", strconv.Itoa(DefaultLeaseSec)))
	if err != nil || leaseSec < 1 {
		return nil, fmt.Errorf("LEASE_SEC must be a positive integer")
	}
	c.LeaseSec = leaseSec
	longpollMax, err := strconv.Atoi(envOr("LONGPOLL_MAX_SEC", strconv.Itoa(DefaultLongpollMax)))
	if err != nil || longpollMax < 1 {
		return nil, fmt.Errorf("LONGPOLL_MAX_SEC must be a positive integer")
	}
	c.LongpollMaxSec = longpollMax
	readConns, err := strconv.Atoi(envOr("DB_READ_CONNS", strconv.Itoa(DefaultDBReadConns)))
	if err != nil || readConns < 1 {
		return nil, fmt.Errorf("DB_READ_CONNS must be a positive integer")
	}
	c.DBReadConns = readConns
	heartbeat, err := strconv.Atoi(envOr("EXECUTOR_HEARTBEAT_MIN", strconv.Itoa(DefaultExecutorHeartbeatMin)))
	if err != nil || heartbeat < 0 {
		return nil, fmt.Errorf("EXECUTOR_HEARTBEAT_MIN must be a non-negative integer")
	}
	c.ExecHeartbeat = heartbeat
	minGap, err := strconv.Atoi(envOr("EXECUTOR_MIN_INTERVAL_MIN", strconv.Itoa(DefaultExecutorMinInterval)))
	if err != nil || minGap < 0 || minGap > 1440 {
		return nil, fmt.Errorf("EXECUTOR_MIN_INTERVAL_MIN must be 0..1440")
	}
	c.ExecMinGap = minGap
	maxActive, err := strconv.Atoi(envOr("EXECUTOR_MAX_ACTIVE", strconv.Itoa(DefaultExecutorMaxActive)))
	if err != nil || maxActive < 1 {
		return nil, fmt.Errorf("EXECUTOR_MAX_ACTIVE must be a positive integer")
	}
	c.ExecMaxActive = maxActive

	if strings.TrimSpace(c.Port) == "" {
		return nil, fmt.Errorf("PORT must not be empty")
	}
	if _, err := strconv.Atoi(c.Port); err != nil {
		return nil, fmt.Errorf("PORT must be numeric")
	}
	if strings.TrimSpace(c.AdminUser) == "" {
		return nil, fmt.Errorf("ADMIN_USER must not be empty")
	}

	c.DBPath = filepath.Join(c.DataDir, DefaultDBName)
	c.UploadDir = filepath.Join(c.DataDir, "uploads")
	c.BackupDir = filepath.Join(c.DataDir, "backups")
	c.VaultKeyPath = filepath.Join(c.DataDir, DefaultVaultKeyName)
	c.MirrorDir = strings.TrimSpace(os.Getenv("BACKUP_MIRROR_DIR"))
	for _, dir := range []string{c.DataDir, c.UploadDir, c.BackupDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create dir %s: %w", dir, err)
		}
	}
	if c.MirrorDir != "" {
		if err := os.MkdirAll(c.MirrorDir, 0o755); err != nil {
			return nil, fmt.Errorf("create mirror dir %s: %w", c.MirrorDir, err)
		}
	}
	return c, nil
}

// Summary returns a startup log line with secrets redacted.
func (c *Config) Summary() string {
	token := "(default dev token)"
	if c.WorkerTokenSet {
		token = "(from env)"
	}
	bind := c.BindAddr
	if bind == "" {
		bind = "(all)"
	}
	return fmt.Sprintf("env=%s bind=%s port=%s data=%s db=%s backup_every_min=%d backup_keep=%d check_every_min=%d retention_every_h=%d worker_token=%s admin=%s",
		c.Env, bind, c.Port, c.DataDir, c.DBPath, c.BackupEveryMin, c.BackupKeep, c.CheckEveryMin, c.RetentionEvery, token, c.AdminUser)
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
