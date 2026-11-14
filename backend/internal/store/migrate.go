package store

import (
	"database/sql"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SchemaVersion is the current DDL version recorded in app_meta.
const SchemaVersion = "14"

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
	business TEXT NOT NULL DEFAULT '',
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
	type TEXT NOT NULL CHECK (type IN ('analyze','strategy','generate','review')),
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
CREATE TABLE IF NOT EXISTS channel_accounts (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE,
	platform TEXT NOT NULL DEFAULT '',
	owner TEXT NOT NULL DEFAULT '',
	role TEXT NOT NULL DEFAULT 'worker',
	business TEXT NOT NULL DEFAULT '',
	session_state TEXT NOT NULL DEFAULT 'pending',
	posting_mode TEXT NOT NULL DEFAULT 'manual',
	enc_id TEXT NOT NULL DEFAULT '',
	enc_pw TEXT NOT NULL DEFAULT '',
	last_checked_at TEXT NOT NULL DEFAULT '',
	last_wake_at TEXT NOT NULL DEFAULT '',
	usage TEXT NOT NULL DEFAULT '',
	last_job_at TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS strategies (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	product_id INTEGER NOT NULL UNIQUE REFERENCES products(id) ON DELETE CASCADE,
	angle TEXT NOT NULL DEFAULT '',
	channels TEXT NOT NULL DEFAULT '',
	key_messages TEXT NOT NULL DEFAULT '',
	schedule_note TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS review_decisions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	content_id INTEGER NOT NULL REFERENCES contents(id) ON DELETE CASCADE,
	decider TEXT NOT NULL DEFAULT '',
	decision TEXT NOT NULL DEFAULT '' CHECK (decision IN ('','approved','rejected')),
	reason TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_reviews_content ON review_decisions(content_id);
`

// Migrate creates the schema and records the schema version.
// It is idempotent and safe to run on every boot.
func Migrate(db *sql.DB) error {
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	if err := migrateToV2(db); err != nil {
		return err
	}
	if err := migrateToV4(db); err != nil {
		return err
	}
	if err := migrateToV5(db); err != nil {
		return err
	}
	if err := migrateToV6Notifications(db); err != nil {
		return err
	}
	if err := migrateToV6Revisions(db); err != nil {
		return err
	}
	if err := migrateToV7(db); err != nil {
		return err
	}
	if err := migrateToV8(db); err != nil {
		return err
	}
	if err := migrateToV9(db); err != nil {
		return err
	}
	if err := migrateToV10(db); err != nil {
		return err
	}
	if err := migrateToV11(db); err != nil {
		return err
	}
	if err := migrateToV12(db); err != nil {
		return err
	}
	if err := migrateToV13(db); err != nil {
		return err
	}
	if err := migrateToV14(db); err != nil {
		return err
	}
	if _, err := db.Exec(
		`INSERT INTO app_meta(k,v) VALUES('schema_version',?) ON CONFLICT(k) DO UPDATE SET v=excluded.v`,
		SchemaVersion); err != nil {
		return fmt.Errorf("migrate version: %w", err)
	}
	return nil
}

// migrateToV2 adds products.business to pre-v2 databases.
// Existing rows keep their data with business defaulting to ”.
func migrateToV2(db *sql.DB) error {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('products') WHERE name='business'`).Scan(&n); err != nil {
		return fmt.Errorf("migrate v2 check: %w", err)
	}
	if n == 0 {
		if _, err := db.Exec(`ALTER TABLE products ADD COLUMN business TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("migrate v2: %w", err)
		}
	}
	return nil
}

// migrateToV4 widens the jobs type check to include strategy. SQLite cannot
// alter a CHECK in place, so pre-v4 tables are rebuilt. No FK points at jobs.
func migrateToV4(db *sql.DB) error {
	var ddl string
	err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='jobs'`).Scan(&ddl)
	if err != nil {
		return fmt.Errorf("migrate v4 inspect: %w", err)
	}
	if strings.Contains(ddl, "'strategy'") {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmts := []string{
		`CREATE TABLE jobs_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			type TEXT NOT NULL CHECK (type IN ('analyze','strategy','generate')),
			ref_id INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','claimed','done','failed')),
			worker_name TEXT NOT NULL DEFAULT '',
			payload TEXT NOT NULL DEFAULT '',
			result TEXT NOT NULL DEFAULT '',
			error TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			claimed_at TEXT NOT NULL DEFAULT '',
			finished_at TEXT NOT NULL DEFAULT ''
		)`,
		`INSERT INTO jobs_new SELECT * FROM jobs`,
		`DROP TABLE jobs`,
		`ALTER TABLE jobs_new RENAME TO jobs`,
		`CREATE INDEX idx_jobs_status_type ON jobs(status, type, id)`,
		`CREATE INDEX idx_jobs_ref ON jobs(type, ref_id)`,
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return fmt.Errorf("migrate v4: %w", err)
		}
	}
	return tx.Commit()
}

// migrateToV5 widens the jobs type check to include review (same rebuild
// pattern as v4; no FK points at jobs).
func migrateToV5(db *sql.DB) error {
	var ddl string
	err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='jobs'`).Scan(&ddl)
	if err != nil {
		return fmt.Errorf("migrate v5 inspect: %w", err)
	}
	if strings.Contains(ddl, "'review'") {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmts := []string{
		`CREATE TABLE jobs_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			type TEXT NOT NULL CHECK (type IN ('analyze','strategy','generate','review')),
			ref_id INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','claimed','done','failed')),
			worker_name TEXT NOT NULL DEFAULT '',
			payload TEXT NOT NULL DEFAULT '',
			result TEXT NOT NULL DEFAULT '',
			error TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			claimed_at TEXT NOT NULL DEFAULT '',
			finished_at TEXT NOT NULL DEFAULT ''
		)`,
		`INSERT INTO jobs_new SELECT * FROM jobs`,
		`DROP TABLE jobs`,
		`ALTER TABLE jobs_new RENAME TO jobs`,
		`CREATE INDEX idx_jobs_status_type ON jobs(status, type, id)`,
		`CREATE INDEX idx_jobs_ref ON jobs(type, ref_id)`,
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return fmt.Errorf("migrate v5: %w", err)
		}
	}
	return tx.Commit()
}

// migrateToV6Notifications adds the local outbox (D30). P0-4 adds the
// contents revision columns under the same version.
func migrateToV6Notifications(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS notifications (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			kind TEXT NOT NULL CHECK (kind IN ('job_failed','job_stuck','lease_expired','session_expired','session_limited','publish_failed','backup_failed','overdue_hold','review_human')),
			severity TEXT NOT NULL DEFAULT 'warn' CHECK (severity IN ('info','warn','error')),
			ref_type TEXT NOT NULL DEFAULT '',
			ref_id INTEGER NOT NULL DEFAULT 0,
			dedup_key TEXT NOT NULL,
			message TEXT NOT NULL,
			created_at TEXT NOT NULL,
			read_at TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_notifications_dedup_unread ON notifications(dedup_key) WHERE read_at=''`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_read ON notifications(read_at, id)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("migrate v6 notifications: %w", err)
		}
	}
	return nil
}

// migrateToV6Revisions adds the revision chain columns (D21): revision_of
// points at the previous draft (0 = first), revision_root_id at the chain
// head. Existing rows become their own roots. Idempotent.
func migrateToV6Revisions(db *sql.DB) error {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('contents') WHERE name='revision_of'`).Scan(&n); err != nil {
		return fmt.Errorf("migrate v6 revisions check: %w", err)
	}
	if n == 0 {
		for _, col := range []string{"revision_of", "revision_root_id"} {
			if _, err := db.Exec(`ALTER TABLE contents ADD COLUMN ` + col + ` INTEGER NOT NULL DEFAULT 0`); err != nil {
				return fmt.Errorf("migrate v6 revisions: %w", err)
			}
		}
		if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_contents_revision_root ON contents(revision_root_id)`); err != nil {
			return fmt.Errorf("migrate v6 revisions index: %w", err)
		}
	}
	if _, err := db.Exec(`UPDATE contents SET revision_root_id=id WHERE revision_root_id=0`); err != nil {
		return fmt.Errorf("migrate v6 revisions backfill: %w", err)
	}
	return nil
}

// migrateToV7 adds the agent registry (D22), widens jobs for routing and
// leases (D24/D25), and tags channel accounts (D22). Existing jobs inherit
// business/priority from their products. Idempotent.
func migrateToV7(db *sql.DB) error {
	var hasAgents int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='agents'`).Scan(&hasAgents); err != nil {
		return fmt.Errorf("migrate v7 check: %w", err)
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	execAll := func(stmts []string) error {
		for _, s := range stmts {
			if _, err := tx.Exec(s); err != nil {
				return err
			}
		}
		return nil
	}
	if hasAgents == 0 {
		if err := execAll([]string{
			`CREATE TABLE agents (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				label TEXT NOT NULL UNIQUE,
				lane TEXT NOT NULL CHECK (lane IN ('A','B','C','P')),
				host TEXT NOT NULL DEFAULT '',
				businesses TEXT NOT NULL DEFAULT '',
				platforms TEXT NOT NULL DEFAULT '',
				capacity INTEGER NOT NULL DEFAULT 1,
				enabled INTEGER NOT NULL DEFAULT 1,
				token_hash TEXT NOT NULL UNIQUE,
				token_prefix TEXT NOT NULL,
				account_id INTEGER REFERENCES channel_accounts(id) ON DELETE SET NULL,
				last_seen_at TEXT NOT NULL DEFAULT '',
				created_at TEXT NOT NULL,
				updated_at TEXT NOT NULL
			)`,
			`CREATE INDEX idx_agents_lane ON agents(lane)`,
			`ALTER TABLE channel_accounts ADD COLUMN kind TEXT NOT NULL DEFAULT 'channel' CHECK (kind IN ('channel','agent_service'))`,
			`ALTER TABLE channel_accounts ADD COLUMN platform_user_id TEXT NOT NULL DEFAULT ''`,
			`CREATE TABLE jobs_new (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				type TEXT NOT NULL CHECK (type IN ('analyze','strategy','generate','review','publish')),
				ref_id INTEGER NOT NULL,
				status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','claimed','done','failed')),
				worker_name TEXT NOT NULL DEFAULT '',
				payload TEXT NOT NULL DEFAULT '',
				result TEXT NOT NULL DEFAULT '',
				error TEXT NOT NULL DEFAULT '',
				created_at TEXT NOT NULL,
				claimed_at TEXT NOT NULL DEFAULT '',
				finished_at TEXT NOT NULL DEFAULT '',
				agent_id INTEGER NOT NULL DEFAULT 0,
				business TEXT NOT NULL DEFAULT '',
				priority INTEGER NOT NULL DEFAULT 0,
				attempts INTEGER NOT NULL DEFAULT 0,
				lease_until TEXT NOT NULL DEFAULT ''
			)`,
			`INSERT INTO jobs_new(id,type,ref_id,status,worker_name,payload,result,error,created_at,claimed_at,finished_at,agent_id,business,priority,attempts,lease_until)
				SELECT j.id,j.type,j.ref_id,j.status,j.worker_name,j.payload,j.result,j.error,j.created_at,j.claimed_at,j.finished_at,0,
				COALESCE(p.business,''),COALESCE(p.priority,0),0,''
				FROM jobs j LEFT JOIN products p ON p.id=j.ref_id`,
			`DROP TABLE jobs`,
			`ALTER TABLE jobs_new RENAME TO jobs`,
			`CREATE INDEX idx_jobs_routing ON jobs(status, type, priority DESC, id)`,
			`CREATE INDEX idx_jobs_ref ON jobs(type, ref_id)`,
			`CREATE INDEX idx_jobs_agent ON jobs(agent_id, status)`,
			`CREATE INDEX idx_jobs_lease ON jobs(status, lease_until)`,
		}); err != nil {
			return fmt.Errorf("migrate v7: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migrate v7: %w", err)
	}
	return nil
}

// migrateToV8 adds executor attempt tracking to publish logs (D28).
// Idempotent.
func migrateToV8(db *sql.DB) error {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('publish_logs') WHERE name='attempts'`).Scan(&n); err != nil {
		return fmt.Errorf("migrate v8 check: %w", err)
	}
	if n == 0 {
		for _, stmt := range []string{
			`ALTER TABLE publish_logs ADD COLUMN attempts INTEGER NOT NULL DEFAULT 0`,
			`ALTER TABLE publish_logs ADD COLUMN last_error TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE publish_logs ADD COLUMN next_eligible_at TEXT NOT NULL DEFAULT ''`,
		} {
			if _, err := db.Exec(stmt); err != nil {
				return fmt.Errorf("migrate v8: %w", err)
			}
		}
	}
	return nil
}

// migrateToV9 adds operator_sessions so operator logins survive restarts.
// Tokens persist as SHA-256 hashes; the plaintext lives only in the
// operator's browser.
func migrateToV9(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS operator_sessions (
	token_hash TEXT PRIMARY KEY,
	user_id INTEGER NOT NULL,
	username TEXT NOT NULL DEFAULT '',
	expires_at TEXT NOT NULL,
	created_at TEXT NOT NULL
)`)
	if err != nil {
		return fmt.Errorf("migrate v9: %w", err)
	}
	return nil
}

// migrateToV10 adds chat_messages for console/worker AI conversations.
func migrateToV10(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS chat_messages (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	account_id INTEGER NOT NULL,
	role TEXT NOT NULL CHECK (role IN ('user','assistant')),
	text TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
)`); err != nil {
		return fmt.Errorf("migrate v10: %w", err)
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_chat_account ON chat_messages(account_id,id)`); err != nil {
		return fmt.Errorf("migrate v10 index: %w", err)
	}
	return nil
}

// migrateToV11 adds API-key columns to channel accounts. The key itself is
// stored sealed (session vault); the model name stays plaintext. Idempotent.
func migrateToV11(db *sql.DB) error {
	for _, col := range []string{"enc_api_token", "api_model"} {
		var n int
		q := `SELECT COUNT(*) FROM pragma_table_info('channel_accounts') WHERE name='` + col + `'`
		if err := db.QueryRow(q).Scan(&n); err != nil {
			return fmt.Errorf("migrate v11 check: %w", err)
		}
		if n == 0 {
			stmt := `ALTER TABLE channel_accounts ADD COLUMN ` + col + ` TEXT NOT NULL DEFAULT ''`
			if _, err := db.Exec(stmt); err != nil {
				return fmt.Errorf("migrate v11: %w", err)
			}
		}
	}
	return nil
}

// migrateToV12 adds the S2 work brief to products (the JSON the new-work
// screen collects: channels, versions, affiliate disclosure, start time,
// team; workers and the W4 scheduler read it) and the media library table
// for chunked uploads (S2/S4). Idempotent.
func migrateToV12(db *sql.DB) error {
	var n int
	q := `SELECT COUNT(*) FROM pragma_table_info('products') WHERE name='brief_json'`
	if err := db.QueryRow(q).Scan(&n); err != nil {
		return fmt.Errorf("migrate v12 check: %w", err)
	}
	if n == 0 {
		if _, err := db.Exec(`ALTER TABLE products ADD COLUMN brief_json TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("migrate v12: %w", err)
		}
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS media (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		product_id INTEGER REFERENCES products(id) ON DELETE CASCADE,
		kind TEXT NOT NULL DEFAULT 'upload',
		path TEXT NOT NULL DEFAULT '',
		bytes INTEGER NOT NULL DEFAULT 0,
		mime TEXT NOT NULL DEFAULT '',
		sha256 TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL
	)`); err != nil {
		return fmt.Errorf("migrate v12 media: %w", err)
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_media_product ON media(product_id)`); err != nil {
		return fmt.Errorf("migrate v12 media idx: %w", err)
	}
	return nil
}

// migrateToV13 adds marketing teams (W16): teams, team_members,
// team_channels, plus the seeded default team. Idempotent.
func migrateToV13(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS teams (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		description TEXT NOT NULL DEFAULT '',
		is_default INTEGER NOT NULL DEFAULT 0,
		businesses TEXT NOT NULL DEFAULT '',
		max_concurrency INTEGER NOT NULL DEFAULT 0,
		role_mode TEXT NOT NULL DEFAULT 'auto',
		enabled INTEGER NOT NULL DEFAULT 1,
		leader_account_id INTEGER REFERENCES channel_accounts(id) ON DELETE SET NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`); err != nil {
		return fmt.Errorf("migrate v13 teams: %w", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS team_members (
		team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
		account_id INTEGER NOT NULL REFERENCES channel_accounts(id) ON DELETE CASCADE,
		fixed_role TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (team_id, account_id)
	)`); err != nil {
		return fmt.Errorf("migrate v13 members: %w", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS team_channels (
		team_id INTEGER NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
		account_id INTEGER NOT NULL REFERENCES channel_accounts(id) ON DELETE CASCADE,
		PRIMARY KEY (team_id, account_id)
	)`); err != nil {
		return fmt.Errorf("migrate v13 channels: %w", err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM teams WHERE is_default=1`).Scan(&n); err != nil {
		return fmt.Errorf("migrate v13 check: %w", err)
	}
	if n == 0 {
		if _, err := db.Exec(`INSERT INTO teams(name,description,is_default,businesses,max_concurrency,role_mode,enabled,created_at,updated_at)
			VALUES('기본 팀','',1,'',0,'auto',1,?,?)`, nowUTC(), nowUTC()); err != nil {
			return fmt.Errorf("migrate v13 seed: %w", err)
		}
	}
	return nil
}

// migrateToV14 adds the per-team brand card (S7-D): intro, tone,
// banned words, default buy link. One row per team. Idempotent.
func migrateToV14(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS team_brands (
		team_id INTEGER PRIMARY KEY REFERENCES teams(id) ON DELETE CASCADE,
		intro TEXT NOT NULL DEFAULT '',
		tone TEXT NOT NULL DEFAULT '',
		banned_words TEXT NOT NULL DEFAULT '',
		buy_link TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL
	)`); err != nil {
		return fmt.Errorf("migrate v14 brands: %w", err)
	}
	return nil
}

// ExecutorConfig is the runtime-tunable executor triplet (D28).
type ExecutorConfig struct {
	HeartbeatMin   int `json:"heartbeatMin"`
	MinIntervalMin int `json:"minIntervalMin"`
	MaxActive      int `json:"maxActive"`
}

// GetExecutorConfig reads the triplet from app_meta, seeding ENV defaults.
func GetExecutorConfig(db *sql.DB, defHeartbeat, defInterval, defMax int) (ExecutorConfig, error) {
	cfg := ExecutorConfig{HeartbeatMin: defHeartbeat, MinIntervalMin: defInterval, MaxActive: defMax}
	rows, err := db.Query(`SELECT k,v FROM app_meta WHERE k IN ('executor.heartbeat_min','executor.min_interval_min','executor.max_active')`)
	if err != nil {
		return cfg, err
	}
	defer rows.Close()
	vals := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return cfg, err
		}
		vals[k] = v
	}
	if err := rows.Err(); err != nil {
		return cfg, err
	}
	parse := func(key string, dst *int) {
		if raw, ok := vals[key]; ok {
			var n int
			if _, err := fmt.Sscanf(raw, "%d", &n); err == nil {
				*dst = n
			}
		}
	}
	parse("executor.heartbeat_min", &cfg.HeartbeatMin)
	parse("executor.min_interval_min", &cfg.MinIntervalMin)
	parse("executor.max_active", &cfg.MaxActive)
	return cfg, nil
}

// SetExecutorConfig persists the triplet to app_meta.
func SetExecutorConfig(db *sql.DB, cfg ExecutorConfig) error {
	vals := map[string]int{
		"executor.heartbeat_min":    cfg.HeartbeatMin,
		"executor.min_interval_min": cfg.MinIntervalMin,
		"executor.max_active":       cfg.MaxActive,
	}
	for k, v := range vals {
		if _, err := db.Exec(`INSERT INTO app_meta(k,v) VALUES(?,?)
			ON CONFLICT(k) DO UPDATE SET v=excluded.v`, k, fmt.Sprintf("%d", v)); err != nil {
			return err
		}
	}
	return nil
}

// CurrentSchemaVersion reads app_meta.schema_version ("" on a fresh DB).
func CurrentSchemaVersion(db *sql.DB) (string, error) {
	var v string
	err := db.QueryRow(`SELECT v FROM app_meta WHERE k='schema_version'`).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		// Fresh DBs have no app_meta table yet.
		if strings.Contains(err.Error(), "no such table") {
			return "", nil
		}
		return "", err
	}
	return v, nil
}

func nowUTC() string { return time.Now().UTC().Format(time.RFC3339) }

// BackupNow snapshots a consistent DB with VACUUM INTO (D32), then bundles
// the vault key and uploads into a timestamped directory under backupDir and
// prunes to newest `keep`. A same-second collision gets a -2/-3 suffix.
// When mirrorDir is set, the finished generation is also copied there.
// Returns the created directory path.
func BackupNow(db *sql.DB, dbPath, uploadDir, backupDir, keyPath string, keep int, mirrorDir string) (string, error) {
	base := filepath.Join(backupDir, time.Now().Format("20060102-150405"))
	dest := base
	for i := 2; ; i++ {
		if _, err := os.Stat(dest); os.IsNotExist(err) {
			break
		}
		dest = fmt.Sprintf("%s-%d", base, i)
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return "", fmt.Errorf("backup mkdir: %w", err)
	}
	snapPath := filepath.Join(dest, filepath.Base(dbPath))
	quoted := "'" + strings.ReplaceAll(snapPath, "'", "''") + "'"
	if _, err := db.Exec(`VACUUM INTO ` + quoted); err != nil {
		return "", fmt.Errorf("backup vacuum into: %w", err)
	}
	if keyPath != "" {
		if _, err := os.Stat(keyPath); err == nil {
			if err := copyFile(keyPath, filepath.Join(dest, filepath.Base(keyPath))); err != nil {
				return "", fmt.Errorf("backup vault key: %w", err)
			}
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
	if mirrorDir != "" {
		if err := os.MkdirAll(mirrorDir, 0o755); err != nil {
			return "", fmt.Errorf("backup mirror mkdir: %w", err)
		}
		mirrorDest := filepath.Join(mirrorDir, filepath.Base(dest))
		if err := os.RemoveAll(mirrorDest); err != nil {
			return "", fmt.Errorf("backup mirror clear: %w", err)
		}
		if err := copyDir(dest, mirrorDest); err != nil {
			return "", fmt.Errorf("backup mirror copy: %w", err)
		}
	}
	return dest, nil
}

// RestoreGeneration replaces the live data dir with a backup generation.
// refuseRunning must report whether the server is up (restore refuses then).
// Live data moves first to a sibling pre-restore-<ts> dir, then the
// generation (now under the sibling) is copied into a fresh data dir.
// Any failure rolls the sibling back. The copy must pass
// PRAGMA integrity_check.
func RestoreGeneration(dbPath, dataDir, backupDir, gen string, refuseRunning func() error) error {
	if err := refuseRunning(); err != nil {
		return err
	}
	genDir := filepath.Join(backupDir, gen)
	snapSrc := filepath.Join(genDir, filepath.Base(dbPath))
	if st, err := os.Stat(snapSrc); err != nil || st.IsDir() {
		return fmt.Errorf("restore: generation %q holds no database", gen)
	}
	sibling := fmt.Sprintf("%s-pre-restore-%s", strings.TrimSuffix(dataDir, string(filepath.Separator)), time.Now().Format("20060102-150405"))
	// The OS can hold a just-stopped server's handles briefly; retry the park.
	var rerr error
	for i := 0; i < 8; i++ {
		if rerr = os.Rename(dataDir, sibling); rerr == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if rerr != nil {
		return fmt.Errorf("restore: park live data (is another process using %s?): %w", dataDir, rerr)
	}
	rollback := func() {
		os.RemoveAll(dataDir)
		_ = os.Rename(sibling, dataDir)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		rollback()
		return fmt.Errorf("restore: recreate data dir: %w", err)
	}
	rel, err := filepath.Rel(dataDir, backupDir)
	if err != nil {
		rollback()
		return fmt.Errorf("restore: locate parked generation: %w", err)
	}
	parkedGen := filepath.Join(sibling, rel, gen)
	entries, err := os.ReadDir(parkedGen)
	if err != nil {
		rollback()
		return fmt.Errorf("restore: read generation: %w", err)
	}
	for _, e := range entries {
		src := filepath.Join(parkedGen, e.Name())
		dst := filepath.Join(dataDir, e.Name())
		if e.IsDir() {
			err = copyDir(src, dst)
		} else {
			err = copyFile(src, dst)
		}
		if err != nil {
			rollback()
			return fmt.Errorf("restore: copy %s: %w", e.Name(), err)
		}
	}
	if err := checkIntegrity(dbPath); err != nil {
		rollback()
		return err
	}
	return nil
}

func checkIntegrity(dbPath string) error {
	db, err := sql.Open("sqlite", "file:"+dbPath)
	if err != nil {
		return fmt.Errorf("restore: reopen: %w", err)
	}
	defer db.Close()
	var verdict string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&verdict); err != nil {
		return fmt.Errorf("restore: integrity_check: %w", err)
	}
	if verdict != "ok" {
		return fmt.Errorf("restore: integrity_check says %q", verdict)
	}
	return nil
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

// ListBackups returns backup directory names newest-first. A missing
// directory yields an empty list, not an error.
func ListBackups(backupDir string) []string {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return []string{}
	}
	dirs := []string{}
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dirs)))
	return dirs
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
