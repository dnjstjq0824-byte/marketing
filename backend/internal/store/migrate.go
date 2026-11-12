package store

import (
	"database/sql"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// SchemaVersion is the current DDL version recorded in app_meta.
const SchemaVersion = "1"

// schema is the whole Tranche-1 data model: 9 entities (§6).
// No order/payment/delivery/personal-data tables: this system never takes orders.
const schema = `
CREATE TABLE IF NOT EXISTS app_meta (
	k TEXT PRIMARY KEY,
	v TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	role TEXT NOT NULL DEFAULT 'operator',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS products (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL DEFAULT '',
	source_url TEXT NOT NULL DEFAULT '',
	exit_type TEXT NOT NULL DEFAULT 'A' CHECK (exit_type IN ('A','B')),
	priority INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS product_images (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	product_id INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
	path TEXT NOT NULL,
	sort INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_product_images_product ON product_images(product_id);
CREATE TABLE IF NOT EXISTS readings (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	product_id INTEGER NOT NULL UNIQUE REFERENCES products(id) ON DELETE CASCADE,
	product_name TEXT NOT NULL DEFAULT '',
	price_summary TEXT NOT NULL DEFAULT '',
	options_text TEXT NOT NULL DEFAULT '',
	shipping_text TEXT NOT NULL DEFAULT '',
	detail_core TEXT NOT NULL DEFAULT '',
	review_summary TEXT NOT NULL DEFAULT '',
	image_desc TEXT NOT NULL DEFAULT '',
	unknowns TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS contents (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	product_id INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
	channel TEXT NOT NULL DEFAULT '' CHECK (channel IN ('','instagram','shortform','cardnews','blog')),
	target_platform TEXT NOT NULL DEFAULT '',
	channel_group TEXT NOT NULL DEFAULT '' CHECK (channel_group IN ('','official_api','manual')),
	title TEXT NOT NULL DEFAULT '',
	body TEXT NOT NULL DEFAULT '',
	disclosure TEXT NOT NULL DEFAULT '',
	is_sponsored INTEGER NOT NULL DEFAULT 0,
	is_virtual_persona INTEGER NOT NULL DEFAULT 0,
	ai_label TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','review','approved','queued','published')),
	target_account TEXT NOT NULL DEFAULT '',
	scheduled_at TEXT NOT NULL DEFAULT '',
	reserve_seq INTEGER NOT NULL DEFAULT 0,
	held INTEGER NOT NULL DEFAULT 0,
	hold_reason TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_contents_status ON contents(status);
CREATE INDEX IF NOT EXISTS idx_contents_product ON contents(product_id);
CREATE INDEX IF NOT EXISTS idx_contents_account ON contents(target_account);
CREATE TABLE IF NOT EXISTS publish_logs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	content_id INTEGER NOT NULL REFERENCES contents(id) ON DELETE CASCADE,
	channel TEXT NOT NULL DEFAULT '',
	account TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'scheduled' CHECK (status IN ('scheduled','held','done','failed')),
	executed_at TEXT NOT NULL DEFAULT '',
	note TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	UNIQUE (content_id, channel, account)
);
CREATE TABLE IF NOT EXISTS metrics (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	content_id INTEGER REFERENCES contents(id) ON DELETE CASCADE,
	channel TEXT NOT NULL DEFAULT '',
	exposures INTEGER,
	inflows INTEGER,
	orders INTEGER,
	inquiries INTEGER,
	cause_class TEXT NOT NULL DEFAULT '',
	check_url INTEGER NOT NULL DEFAULT 0,
	check_session INTEGER NOT NULL DEFAULT 0,
	check_display INTEGER NOT NULL DEFAULT 0,
	note TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS jobs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	type TEXT NOT NULL CHECK (type IN ('analyze','generate')),
	ref_id INTEGER NOT NULL,
	status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','claimed','done','failed')),
	worker_name TEXT NOT NULL DEFAULT '',
	payload TEXT NOT NULL DEFAULT '',
	result TEXT NOT NULL DEFAULT '',
	error TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	claimed_at TEXT NOT NULL DEFAULT '',
	finished_at TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_jobs_status_type ON jobs(status, type, id);
CREATE INDEX IF NOT EXISTS idx_jobs_ref ON jobs(type, ref_id);
CREATE TABLE IF NOT EXISTS affiliate_links (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	product_id INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
	partner_name TEXT NOT NULL DEFAULT '',
	url TEXT NOT NULL,
	created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_affiliate_product ON affiliate_links(product_id);
`

// Migrate creates the schema and records the schema version.
// It is idempotent and safe to run on every boot.
func Migrate(db *sql.DB) error {
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	if _, err := db.Exec(
		`INSERT INTO app_meta(k,v) VALUES('schema_version',?) ON CONFLICT(k) DO UPDATE SET v=excluded.v`,
		SchemaVersion); err != nil {
		return fmt.Errorf("migrate version: %w", err)
	}
	return nil
}

func nowUTC() string { return time.Now().UTC().Format(time.RFC3339) }

// BackupNow copies the DB file (+WAL sidecars) and the uploads folder into a
// timestamped directory under backupDir, then prunes to keep newest `keep`.
// Returns the created directory path.
func BackupNow(db *sql.DB, dbPath, uploadDir, backupDir string, keep int) (string, error) {
	// Best effort checkpoint so the file copy is consistent under WAL mode.
	_, _ = db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	dest := filepath.Join(backupDir, time.Now().Format("20060102-150405"))
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return "", fmt.Errorf("backup mkdir: %w", err)
	}
	for _, src := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
		if _, err := os.Stat(src); err != nil {
			continue
		}
		if err := copyFile(src, filepath.Join(dest, filepath.Base(src))); err != nil {
			return "", fmt.Errorf("backup db file: %w", err)
		}
	}
	if st, err := os.Stat(uploadDir); err == nil && st.IsDir() {
		if err := copyDir(uploadDir, filepath.Join(dest, "uploads")); err != nil {
			return "", fmt.Errorf("backup uploads: %w", err)
		}
	}
	if err := pruneBackups(backupDir, keep); err != nil {
		return "", err
	}
	return dest, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func pruneBackups(backupDir string, keep int) error {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return fmt.Errorf("backup prune: %w", err)
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	sort.Strings(dirs)
	for len(dirs) > keep {
		oldest := filepath.Join(backupDir, dirs[0])
		dirs = dirs[1:]
		if err := os.RemoveAll(oldest); err != nil {
			return fmt.Errorf("backup prune %s: %w", oldest, err)
		}
	}
	return nil
}
