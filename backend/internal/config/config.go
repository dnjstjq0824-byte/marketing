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

// Defaults for local development. Production must override secrets via env.
const (
	DefaultPort           = "8080"
	DefaultDataDir        = "data"
	DefaultDBName         = "somun.db"
	DefaultWorkerToken    = "local-worker-token"
	DefaultAdminUser      = "admin"
	DefaultAdminPass      = "changeme123"
	DefaultBackupEveryMin = 60
	DefaultBackupKeep     = 7
	DefaultWebDistDir     = "web/dist"
	MaxUploadMB           = 10
	MaxImagesPerProduct   = 10
	OperatorTokenTTLHours = 12
)

// Config is the effective server configuration.
type Config struct {
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
}

// Load reads env, applies defaults, validates, and ensures directories.
func Load() (*Config, error) {
	c := &Config{
		Port:       envOr("PORT", DefaultPort),
		DataDir:    envOr("DATA_DIR", DefaultDataDir),
		WebDistDir: envOr("WEB_DIST_DIR", DefaultWebDistDir),
		AdminUser:  envOr("ADMIN_USER", DefaultAdminUser),
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
	for _, dir := range []string{c.DataDir, c.UploadDir, c.BackupDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create dir %s: %w", dir, err)
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
	return fmt.Sprintf("port=%s data=%s db=%s backup_every_min=%d backup_keep=%d worker_token=%s admin=%s",
		c.Port, c.DataDir, c.DBPath, c.BackupEveryMin, c.BackupKeep, token, c.AdminUser)
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
