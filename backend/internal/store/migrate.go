package store

import (
	"database/sql"
	"encoding/json"
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
const SchemaVersion = "30"

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
	status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','review','approved','queued','published','archived')),
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
	daily_cap INTEGER NOT NULL DEFAULT 0,
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
	if err := migrateToV15(db); err != nil {
		return err
	}
	if err := migrateToV16(db); err != nil {
		return err
	}
	if err := migrateToV17(db); err != nil {
		return err
	}
	if err := migrateToV18(db); err != nil {
		return err
	}
	if err := migrateToV19(db); err != nil {
		return err
	}
	if err := migrateToV20(db); err != nil {
		return err
	}
	if err := migrateToV21(db); err != nil {
		return err
	}
	if err := migrateToV22(db); err != nil {
		return err
	}
	if err := migrateToV23(db); err != nil {
		return err
	}
	if err := migrateToV24(db); err != nil {
		return err
	}
	if err := migrateToV25(db); err != nil {
		return err
	}
	if err := migrateToV26(db); err != nil {
		return err
	}
	if err := migrateToV27(db); err != nil {
		return err
	}
	if err := migrateToV28(db); err != nil {
		return err
	}
	if err := migrateToV29(db); err != nil {
		return err
	}
	if err := migrateToV30(db); err != nil {
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
			kind TEXT NOT NULL CHECK (kind IN ('job_failed','job_stuck','lease_expired','session_expired','session_limited','publish_failed','backup_failed','overdue_hold','review_human','team_handoff')),
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
				lane TEXT NOT NULL CHECK (lane IN ('A','B','C','P','T')),
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

// migrateToV15 adds channel_accounts.daily_cap (W0). 0 means "follow the
// global settings cap"; existing rows keep behavior via the default.
func migrateToV15(db *sql.DB) error {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('channel_accounts') WHERE name='daily_cap'`).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		if _, err := db.Exec(`ALTER TABLE channel_accounts ADD COLUMN daily_cap INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("migrate v15 daily_cap: %w", err)
		}
	}
	return nil
}

// migrateToV16 adds the chat_threads mirror (W13). The adapter thread files
// stay the source of truth; this table is server-side observability only.
func migrateToV16(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS chat_threads (
		account_id INTEGER NOT NULL,
		thread_key TEXT NOT NULL DEFAULT '',
		message_count INTEGER NOT NULL DEFAULT 0,
		updated_at TEXT NOT NULL,
		PRIMARY KEY (account_id, thread_key)
	)`); err != nil {
		return fmt.Errorf("migrate v16 chat_threads: %w", err)
	}
	return nil
}

// channelPlatformSeed is the W21-1 registry. All rows start preparing:
// publishing flips to ready only after selectors/flows are proven on real
// sessions (W5-1). Compose selectors live in adapter/platforms.py (single
// source); the registry tracks readiness, not selector text.
var channelPlatformSeed = []string{
	"('naver_blog','네이버 블로그','manual','https://nid.naver.com/nidlogin.login','{\"minLen\":1500,\"subheads\":true}','{\"imagesMax\":10}','preparing')",
	"('tistory','티스토리','manual','https://www.tistory.com/auth/login','{\"minLen\":1500,\"subheads\":true}','{\"imagesMax\":10}','preparing')",
	"('instagram','인스타그램','manual','https://www.instagram.com/accounts/login/','{\"captionMax\":2200,\"hashtagsMax\":30,\"links\":false}','{\"imagesMin\":1,\"imagesMax\":10}','preparing')",
	"('threads','스레드','manual','https://www.threads.com/login','{\"lenMax\":500}','{\"imagesMax\":4}','preparing')",
	"('facebook','페이스북 페이지','manual','https://www.facebook.com/','{\"lenMax\":2000}','{\"imagesMax\":10}','preparing')",
	"('youtube','유튜브 쇼츠','manual','https://www.youtube.com/','{\"lenMax\":150}','{\"video\":\"vertical\"}','preparing')",
	"('tiktok','틱톡','manual','https://www.tiktok.com/','{\"lenMax\":150}','{\"video\":\"vertical\"}','preparing')",
	"('x','X(트위터)','manual','https://x.com/i/flow/login','{\"lenMax\":280}','{\"imagesMax\":4}','preparing')",
	"('blogger','블로거','manual','https://www.blogger.com/','{\"minLen\":800}','{\"imagesMax\":10}','preparing')",
	"('wordpress','워드프레스','manual','https://wordpress.com/log-in','{\"minLen\":800}','{\"imagesMax\":10}','preparing')",
}

// aiSeedRow is one builtin AI registry row (W15-1). Values mirror
// adapter/platforms.py exactly; the adapter keeps its constants for the
// builtin five, and the server hands JSON only to custom (builtin=0) rows.
type aiSeedRow struct {
	key, name, login, chat, markers              string
	composer, send, reply, replyBody, busy, done string
}

var aiPlatformSeed = []aiSeedRow{
	{"meta_ai", "Meta AI", "https://www.meta.ai/", "https://www.meta.ai/",
		`["Log in","로그인"]`,
		"[data-testid='composer-input']:visible", "button[data-testid='composer-send-button']",
		"[data-testid='assistant-message']", "", "", "[class*='assistant-message-actions']"},
	{"chatgpt", "ChatGPT", "https://chatgpt.com/", "https://chatgpt.com/",
		`["Log in","Sign up","로그인"]`,
		"[contenteditable='true'][role='textbox']:visible",
		"button[aria-label='보내기'], button[aria-label='Send message'], button[aria-label='Send']",
		"[data-conversation-role='assistant']", "", "[data-talvt-turn-state='in_progress']", ""},
	{"gemini", "Gemini", "https://gemini.google.com/", "https://gemini.google.com/app",
		`["Sign in","로그인"]`,
		"[role='textbox'][contenteditable='true']:visible",
		".input-area button[aria-label='메시지 보내기'], .input-area button[aria-label='Send message']",
		"message-content", "",
		".input-area button[aria-label='대답 생성 중지'], .input-area button[aria-label='Stop generating']", ""},
	{"claude_app", "Claude", "https://claude.ai/login", "https://claude.ai/new",
		`["Log in","Continue with Google","로그인"]`,
		"[data-testid='chat-input']:visible", "[data-testid='chat-input-send']",
		"[data-testid='assistant-message']", "[data-cds='Prose']",
		"[data-testid='assistant-message'][data-is-streaming='true']", "[data-testid='message-actions']"},
	{"muse", "Muse", "https://muse.ai/", "https://muse.ai/",
		`["Log in","로그인"]`,
		"textarea[aria-label='메시지 보내기']:visible", "button[aria-label='보내기']",
		"[data-message-role='assistant']", "[data-hatch-assistant-message-body]", "", ""},
}

// migrateToV24 tags works with an industry for the W18-2 wordlists.
// Empty means "common list only"; works.Create auto-detects from the
// business/brief text and the API accepts an explicit override.
func migrateToV24(db *sql.DB) error {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('products') WHERE name='industry'`).Scan(&n); err != nil {
		return fmt.Errorf("migrate v24 check: %w", err)
	}
	if n == 0 {
		if _, err := db.Exec(`ALTER TABLE products ADD COLUMN industry TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("migrate v24: %w", err)
		}
	}
	return nil
}

// migrateToV25 backfills legacy rows for the W19 cutover. Every statement
// is conditional, so re-runs are no-ops and fresh DBs pass through
// untouched:
//   - products with a source URL but no input_kind become url works;
//   - superseded rejected review originals (rejected decision + a newer
//     live revision, the W6 disposal rule) move to archived;
//   - AI agent accounts without any team join the default team;
//   - stale internal-ops notices (lease/job machinery, session states the
//     account already recovered from) are marked read;
//   - retired A/B/C lane agents are disabled; the operator registers the
//     lane-T team executor via scripts/install-workers.ps1, which also
//     removes the retired registrations, runners, and startup links;
//   - brief-less works with approved/queued posts gain the minimal brief
//     the W4 assigner needs, but only for channels that name a registry
//     platform (today: instagram); anything unmappable keeps waiting
//     visibly instead of guessing a platform.
func migrateToV25(db *sql.DB) error {
	now := nowUTC()
	if _, err := db.Exec(`UPDATE products SET input_kind='url', updated_at=?
		WHERE input_kind='' AND source_url<>''`, now); err != nil {
		return fmt.Errorf("migrate v25 input_kind: %w", err)
	}
	if _, err := db.Exec(`UPDATE contents SET status='archived', updated_at=?
		WHERE status='review'
		AND EXISTS (SELECT 1 FROM review_decisions d
			WHERE d.content_id=contents.id AND d.decision='rejected')
		AND EXISTS (SELECT 1 FROM contents k
			WHERE k.id<>contents.id
			AND (k.revision_of=contents.id
				OR (contents.revision_root_id<>0 AND k.revision_root_id=contents.revision_root_id))
			AND k.status IN ('approved','queued','published'))`, now); err != nil {
		return fmt.Errorf("migrate v25 archive: %w", err)
	}
	if _, err := db.Exec(`INSERT OR IGNORE INTO team_members(team_id,account_id,fixed_role)
		SELECT (SELECT id FROM teams WHERE is_default=1), a.id, ''
		FROM channel_accounts a
		WHERE a.kind='agent_service'
		AND NOT EXISTS (SELECT 1 FROM team_members m WHERE m.account_id=a.id)`); err != nil {
		return fmt.Errorf("migrate v25 team: %w", err)
	}
	if _, err := db.Exec(`UPDATE notifications SET read_at=?
		WHERE read_at='' AND kind IN ('lease_expired','job_stuck')`, now); err != nil {
		return fmt.Errorf("migrate v25 notices: %w", err)
	}
	if _, err := db.Exec(`UPDATE notifications SET read_at=? WHERE read_at=''
		AND kind IN ('session_expired','session_limited') AND ref_type='account'
		AND EXISTS (SELECT 1 FROM channel_accounts a
			WHERE a.id=notifications.ref_id AND a.session_state='connected')`, now); err != nil {
		return fmt.Errorf("migrate v25 sessions: %w", err)
	}
	if _, err := db.Exec(`UPDATE agents SET enabled=0, updated_at=?
		WHERE lane IN ('A','B','C') AND enabled<>0`, now); err != nil {
		return fmt.Errorf("migrate v25 agents: %w", err)
	}
	return migrateV25Briefs(db)
}

// migrateV25Briefs gives brief-less legacy works the minimal brief the W4
// assigner resolves: one channel entry per distinct approved/queued post
// channel, but only when the channel names a channel_platforms registry
// key. Posts on other channels stay visibly unassigned ("platform undecided")
// instead of inheriting a guessed platform. Idempotent: works that already
// carry a brief are never touched.
func migrateV25Briefs(db *sql.DB) error {
	keys := map[string]bool{}
	rows, err := db.Query(`SELECT key FROM channel_platforms`)
	if err != nil {
		return fmt.Errorf("migrate v25 registry: %w", err)
	}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			rows.Close()
			return fmt.Errorf("migrate v25 registry: %w", err)
		}
		keys[k] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("migrate v25 registry: %w", err)
	}
	works, err := db.Query(`SELECT id FROM products WHERE TRIM(COALESCE(brief_json,''))=''`)
	if err != nil {
		return fmt.Errorf("migrate v25 works: %w", err)
	}
	var ids []int64
	for works.Next() {
		var id int64
		if err := works.Scan(&id); err != nil {
			works.Close()
			return fmt.Errorf("migrate v25 works: %w", err)
		}
		ids = append(ids, id)
	}
	works.Close()
	if err := works.Err(); err != nil {
		return fmt.Errorf("migrate v25 works: %w", err)
	}
	for _, id := range ids {
		channels, err := db.Query(`SELECT DISTINCT channel FROM contents
			WHERE product_id=? AND status IN ('approved','queued') AND channel<>''`, id)
		if err != nil {
			return fmt.Errorf("migrate v25 channels: %w", err)
		}
		var entries []map[string]any
		for channels.Next() {
			var ch string
			if err := channels.Scan(&ch); err != nil {
				channels.Close()
				return fmt.Errorf("migrate v25 channels: %w", err)
			}
			if keys[ch] {
				entries = append(entries, map[string]any{"channel": ch, "platform": ch, "versions": 1})
			}
		}
		channels.Close()
		if err := channels.Err(); err != nil {
			return fmt.Errorf("migrate v25 channels: %w", err)
		}
		if len(entries) == 0 {
			continue
		}
		brief, err := json.Marshal(map[string]any{"channels": entries})
		if err != nil {
			return fmt.Errorf("migrate v25 brief: %w", err)
		}
		if _, err := db.Exec(`UPDATE products SET brief_json=?, updated_at=? WHERE id=?`,
			string(brief), nowUTC(), id); err != nil {
			return fmt.Errorf("migrate v25 brief: %w", err)
		}
	}
	return nil
}

// migrateToV26 grows the media library for W14 generation tracking: the
// post link, thumbnail + dimensions, the prompt and AI account behind a
// generated file, and the rights source (W18). Legacy rows keep NULL/empty
// values (untracked upload, never excluded). Idempotent.
func migrateToV26(db *sql.DB) error {
	for _, stmt := range []struct{ table, col, ddl string }{
		{"media", "content_id", `ALTER TABLE media ADD COLUMN content_id INTEGER NULL REFERENCES contents(id) ON DELETE SET NULL`},
		{"media", "thumb_path", `ALTER TABLE media ADD COLUMN thumb_path TEXT NOT NULL DEFAULT ''`},
		{"media", "width", `ALTER TABLE media ADD COLUMN width INTEGER NOT NULL DEFAULT 0`},
		{"media", "height", `ALTER TABLE media ADD COLUMN height INTEGER NOT NULL DEFAULT 0`},
		{"media", "duration_sec", `ALTER TABLE media ADD COLUMN duration_sec REAL NOT NULL DEFAULT 0`},
		{"media", "prompt", `ALTER TABLE media ADD COLUMN prompt TEXT NOT NULL DEFAULT ''`},
		{"media", "ai_account_id", `ALTER TABLE media ADD COLUMN ai_account_id INTEGER NULL REFERENCES channel_accounts(id) ON DELETE SET NULL`},
		{"media", "source", `ALTER TABLE media ADD COLUMN source TEXT NOT NULL DEFAULT ''`},
	} {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name=?`, stmt.table, stmt.col).Scan(&n); err != nil {
			return fmt.Errorf("migrate v26 check: %w", err)
		}
		if n == 0 {
			if _, err := db.Exec(stmt.ddl); err != nil {
				return fmt.Errorf("migrate v26: %w", err)
			}
		}
	}
	for _, ddl := range []string{
		`CREATE INDEX IF NOT EXISTS idx_media_content ON media(content_id)`,
		`CREATE INDEX IF NOT EXISTS idx_media_sha ON media(sha256)`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			return fmt.Errorf("migrate v26 idx: %w", err)
		}
	}
	return nil
}

// migrateToV27 adds W7 performance collection: public metric columns on
// metrics (views/likes/comments, NULL = not collected) plus an origin tag
// (blank for legacy rows, collect:<window> for W7 rows), and the
// collect_windows ledger that keeps the 24h/72h/7d sweep idempotent.
func migrateToV27(db *sql.DB) error {
	for _, stmt := range []struct{ table, col, ddl string }{
		{"metrics", "views", `ALTER TABLE metrics ADD COLUMN views INTEGER NULL`},
		{"metrics", "likes", `ALTER TABLE metrics ADD COLUMN likes INTEGER NULL`},
		{"metrics", "comments", `ALTER TABLE metrics ADD COLUMN comments INTEGER NULL`},
		{"metrics", "origin", `ALTER TABLE metrics ADD COLUMN origin TEXT NOT NULL DEFAULT ''`},
	} {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name=?`, stmt.table, stmt.col).Scan(&n); err != nil {
			return fmt.Errorf("migrate v27 check: %w", err)
		}
		if n == 0 {
			if _, err := db.Exec(stmt.ddl); err != nil {
				return fmt.Errorf("migrate v27: %w", err)
			}
		}
	}
	for _, ddl := range []string{
		`CREATE TABLE IF NOT EXISTS collect_windows (
			content_id INTEGER NOT NULL REFERENCES contents(id) ON DELETE CASCADE,
			window TEXT NOT NULL,
			account TEXT NOT NULL DEFAULT '',
			job_id INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			UNIQUE (content_id, window, account)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_collect_windows_content ON collect_windows(content_id)`,
		`CREATE INDEX IF NOT EXISTS idx_metrics_content ON metrics(content_id)`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			return fmt.Errorf("migrate v27 ddl: %w", err)
		}
	}
	return nil
}

// migrateToV28 adds the W20 operations layer: the chat_messages thread
// key (per-work AI transcripts), the operator audit log, and the
// disk_low notification kind (same rebuild pattern as v21).
func migrateToV28(db *sql.DB) error {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('chat_messages') WHERE name='thread_key'`).Scan(&n); err != nil {
		return fmt.Errorf("migrate v28 check: %w", err)
	}
	if n == 0 {
		if _, err := db.Exec(`ALTER TABLE chat_messages ADD COLUMN thread_key TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("migrate v28 thread_key: %w", err)
		}
	}
	for _, ddl := range []string{
		`CREATE INDEX IF NOT EXISTS idx_chat_thread ON chat_messages(thread_key, id)`,
		`CREATE TABLE IF NOT EXISTS audit_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			actor TEXT NOT NULL DEFAULT '',
			action TEXT NOT NULL DEFAULT '',
			ref_type TEXT NOT NULL DEFAULT '',
			ref_id INTEGER NOT NULL DEFAULT 0,
			work_id INTEGER NOT NULL DEFAULT 0,
			detail TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_work ON audit_log(work_id, id)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_ref ON audit_log(ref_type, ref_id)`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			return fmt.Errorf("migrate v28 ddl: %w", err)
		}
	}
	return migrateV28DiskLow(db)
}

// migrateV28DiskLow widens the notifications kind CHECK with 'disk_low'.
// No FK points at notifications: plain rebuild inside one transaction.
func migrateV28DiskLow(db *sql.DB) error {
	var ddl string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='notifications'`).Scan(&ddl); err != nil {
		return fmt.Errorf("migrate v28 inspect: %w", err)
	}
	if strings.Contains(ddl, "'disk_low'") {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("migrate v28 begin: %w", err)
	}
	defer tx.Rollback()
	cols := `id,kind,severity,ref_type,ref_id,dedup_key,message,created_at,read_at`
	stmts := []string{
		`CREATE TABLE notifications_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			kind TEXT NOT NULL CHECK (kind IN ('job_failed','job_stuck','lease_expired','session_expired','session_limited','publish_failed','backup_failed','overdue_hold','review_human','team_handoff','disk_low')),
			severity TEXT NOT NULL DEFAULT 'warn' CHECK (severity IN ('info','warn','error')),
			ref_type TEXT NOT NULL DEFAULT '',
			ref_id INTEGER NOT NULL DEFAULT 0,
			dedup_key TEXT NOT NULL,
			message TEXT NOT NULL,
			created_at TEXT NOT NULL,
			read_at TEXT NOT NULL DEFAULT ''
		)`,
		`INSERT INTO notifications_new (` + cols + `) SELECT ` + cols + ` FROM notifications`,
		`DROP TABLE notifications`,
		`ALTER TABLE notifications_new RENAME TO notifications`,
		`CREATE UNIQUE INDEX idx_notifications_dedup_unread ON notifications(dedup_key) WHERE read_at=''`,
		`CREATE INDEX idx_notifications_read ON notifications(read_at, id)`,
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return fmt.Errorf("migrate v28 rebuild: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migrate v28 commit: %w", err)
	}
	return nil
}

// migrateToV29 adds W22 agent-relay publishing: per-account executor
// selection (direct stays the default) plus the probe-gated backend
// registry. direct and muse seed ready; later agents flip ready only
// after a supervised live probe pins their approval path.
func migrateToV29(db *sql.DB) error {
	for _, stmt := range []struct{ table, col, ddl string }{
		{"channel_accounts", "executor_backend", `ALTER TABLE channel_accounts ADD COLUMN executor_backend TEXT NOT NULL DEFAULT 'direct'`},
		{"channel_accounts", "relay_account", `ALTER TABLE channel_accounts ADD COLUMN relay_account TEXT NOT NULL DEFAULT ''`},
	} {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name=?`, stmt.table, stmt.col).Scan(&n); err != nil {
			return fmt.Errorf("migrate v29 check: %w", err)
		}
		if n == 0 {
			if _, err := db.Exec(stmt.ddl); err != nil {
				return fmt.Errorf("migrate v29: %w", err)
			}
		}
	}
	for _, ddl := range []string{
		`CREATE TABLE IF NOT EXISTS publish_backends (
			name TEXT PRIMARY KEY,
			ai_platform TEXT NOT NULL DEFAULT '',
			ready INTEGER NOT NULL DEFAULT 0
		)`,
		`INSERT OR IGNORE INTO publish_backends(name, ai_platform, ready) VALUES('direct', '', 1)`,
		`INSERT OR IGNORE INTO publish_backends(name, ai_platform, ready) VALUES('muse', 'muse', 1)`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			return fmt.Errorf("migrate v29 ddl: %w", err)
		}
	}
	return nil
}

// migrateToV30 adds W23 consignment tables: public members with their own
// sessions (operator auth untouched), consignment products carrying the
// sealed private URL plus the fixed per-order fee, and member applications
// with a pending/approved/rejected state machine.
func migrateToV30(db *sql.DB) error {
	for _, ddl := range []string{
		`CREATE TABLE IF NOT EXISTS members (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			nickname TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS member_sessions (
			token_hash TEXT PRIMARY KEY,
			member_id INTEGER NOT NULL REFERENCES members(id) ON DELETE CASCADE,
			expires_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_member_sessions_member ON member_sessions(member_id)`,
		`CREATE TABLE IF NOT EXISTS consignment_products (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL DEFAULT '',
			store TEXT NOT NULL DEFAULT '',
			price INTEGER NOT NULL DEFAULT 0,
			photo TEXT NOT NULL DEFAULT '',
			facts TEXT NOT NULL DEFAULT '',
			points TEXT NOT NULL DEFAULT '',
			banned_words TEXT NOT NULL DEFAULT '',
			materials TEXT NOT NULL DEFAULT '',
			fee_per_order INTEGER NOT NULL DEFAULT 0,
			settle_terms TEXT NOT NULL DEFAULT '',
			private_url_enc TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'recruiting' CHECK (status IN ('recruiting','closed')),
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS consignment_applications (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			product_id INTEGER NOT NULL REFERENCES consignment_products(id) ON DELETE CASCADE,
			member_id INTEGER NOT NULL REFERENCES members(id) ON DELETE CASCADE,
			channels TEXT NOT NULL DEFAULT '',
			plan TEXT NOT NULL DEFAULT '',
			fee_agreed INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected')),
			reason TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			UNIQUE (product_id, member_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_consign_app_product ON consignment_applications(product_id)`,
		`CREATE INDEX IF NOT EXISTS idx_consign_app_member ON consignment_applications(member_id)`,
	} {
		if _, err := db.Exec(ddl); err != nil {
			return fmt.Errorf("migrate v30 ddl: %w", err)
		}
	}
	return nil
}

// migrateToV23 adds work input typing (W1), the research job type (W1),
// and the media-spec passthrough (W3-4).
func migrateToV23(db *sql.DB) error {
	for _, stmt := range []struct{ table, col, ddl string }{
		{"products", "input_kind", `ALTER TABLE products ADD COLUMN input_kind TEXT NOT NULL DEFAULT ''`},
		{"products", "topic", `ALTER TABLE products ADD COLUMN topic TEXT NOT NULL DEFAULT ''`},
		{"products", "target_platforms", `ALTER TABLE products ADD COLUMN target_platforms TEXT NOT NULL DEFAULT '[]'`},
		{"contents", "media_spec", `ALTER TABLE contents ADD COLUMN media_spec TEXT NOT NULL DEFAULT ''`},
		{"contents", "want_platform", `ALTER TABLE contents ADD COLUMN want_platform TEXT NOT NULL DEFAULT ''`},
	} {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name=?`, stmt.table, stmt.col).Scan(&n); err != nil {
			return fmt.Errorf("migrate v23 check: %w", err)
		}
		if n == 0 {
			if _, err := db.Exec(stmt.ddl); err != nil {
				return fmt.Errorf("migrate v23: %w", err)
			}
		}
	}
	var ddl string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='jobs'`).Scan(&ddl); err != nil {
		return fmt.Errorf("migrate v23 inspect: %w", err)
	}
	if strings.Contains(ddl, "'research'") {
		return nil
	}
	cols := `id,type,ref_id,status,worker_name,payload,result,error,created_at,claimed_at,finished_at,agent_id,business,priority,attempts,lease_until,team_id`
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("migrate v23 begin: %w", err)
	}
	defer tx.Rollback()
	for _, s := range []string{
		`CREATE TABLE jobs_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			type TEXT NOT NULL CHECK (type IN ('analyze','strategy','generate','review','publish','research','collect','media')),
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
			lease_until TEXT NOT NULL DEFAULT '',
			team_id INTEGER NULL REFERENCES teams(id)
		)`,
		`INSERT INTO jobs_new (` + cols + `) SELECT ` + cols + ` FROM jobs`,
		`DROP TABLE jobs`,
		`ALTER TABLE jobs_new RENAME TO jobs`,
		`CREATE INDEX idx_jobs_routing ON jobs(status, type, priority DESC, id)`,
		`CREATE INDEX idx_jobs_ref ON jobs(type, ref_id)`,
		`CREATE INDEX idx_jobs_agent ON jobs(agent_id, status)`,
		`CREATE INDEX idx_jobs_lease ON jobs(status, lease_until)`,
	} {
		if _, err := tx.Exec(s); err != nil {
			return fmt.Errorf("migrate v23 rebuild: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migrate v23 commit: %w", err)
	}
	return nil
}

// migrateToV22 adds per-team distribution controls (W21-4): all accounts
// vs one account, plus a version cap.
func migrateToV22(db *sql.DB) error {
	for _, stmt := range []struct{ table, col, ddl string }{
		{"teams", "distribution_mode", `ALTER TABLE teams ADD COLUMN distribution_mode TEXT NOT NULL DEFAULT 'all'`},
		{"teams", "distribution_cap", `ALTER TABLE teams ADD COLUMN distribution_cap INTEGER NOT NULL DEFAULT 5`},
	} {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name=?`, stmt.table, stmt.col).Scan(&n); err != nil {
			return fmt.Errorf("migrate v22 check: %w", err)
		}
		if n == 0 {
			if _, err := db.Exec(stmt.ddl); err != nil {
				return fmt.Errorf("migrate v22: %w", err)
			}
		}
	}
	return nil
}

// migrateToV21 stamps team ownership on works and jobs (W16) and adds the
// team_handoff notice kind. Legacy NULL team ids resolve to the default
// team at read time, so old rows keep flowing without backfill.
func migrateToV21(db *sql.DB) error {
	for _, stmt := range []struct{ table, col, ddl string }{
		{"products", "team_id", `ALTER TABLE products ADD COLUMN team_id INTEGER NULL REFERENCES teams(id)`},
		{"jobs", "team_id", `ALTER TABLE jobs ADD COLUMN team_id INTEGER NULL REFERENCES teams(id)`},
	} {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name=?`, stmt.table, stmt.col).Scan(&n); err != nil {
			return fmt.Errorf("migrate v21 check: %w", err)
		}
		if n == 0 {
			if _, err := db.Exec(stmt.ddl); err != nil {
				return fmt.Errorf("migrate v21: %w", err)
			}
		}
	}
	if err := migrateV21AgentsLane(db); err != nil {
		return err
	}
	var ddl string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='notifications'`).Scan(&ddl); err != nil {
		return fmt.Errorf("migrate v21 inspect: %w", err)
	}
	if strings.Contains(ddl, "'team_handoff'") {
		return nil
	}
	// No FK points at notifications: plain rebuild inside one transaction.
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("migrate v21 begin: %w", err)
	}
	defer tx.Rollback()
	cols := `id,kind,severity,ref_type,ref_id,dedup_key,message,created_at,read_at`
	stmts := []string{
		`CREATE TABLE notifications_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			kind TEXT NOT NULL CHECK (kind IN ('job_failed','job_stuck','lease_expired','session_expired','session_limited','publish_failed','backup_failed','overdue_hold','review_human','team_handoff')),
			severity TEXT NOT NULL DEFAULT 'warn' CHECK (severity IN ('info','warn','error')),
			ref_type TEXT NOT NULL DEFAULT '',
			ref_id INTEGER NOT NULL DEFAULT 0,
			dedup_key TEXT NOT NULL,
			message TEXT NOT NULL,
			created_at TEXT NOT NULL,
			read_at TEXT NOT NULL DEFAULT ''
		)`,
		`INSERT INTO notifications_new (` + cols + `) SELECT ` + cols + ` FROM notifications`,
		`DROP TABLE notifications`,
		`ALTER TABLE notifications_new RENAME TO notifications`,
		`CREATE UNIQUE INDEX idx_notifications_dedup_unread ON notifications(dedup_key) WHERE read_at=''`,
		`CREATE INDEX idx_notifications_read ON notifications(read_at, id)`,
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return fmt.Errorf("migrate v21 rebuild: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migrate v21 commit: %w", err)
	}
	return nil
}

// migrateV21AgentsLane widens the agents lane CHECK with 'T' (W16 team
// executor). No FK points at agents: plain rebuild, idempotent.
func migrateV21AgentsLane(db *sql.DB) error {
	var ddl string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='agents'`).Scan(&ddl); err != nil {
		return fmt.Errorf("migrate v21 agents inspect: %w", err)
	}
	if strings.Contains(ddl, "'T'") {
		return nil
	}
	cols := `id,label,lane,host,businesses,platforms,capacity,enabled,token_hash,token_prefix,account_id,last_seen_at,created_at,updated_at`
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("migrate v21 agents begin: %w", err)
	}
	defer tx.Rollback()
	for _, s := range []string{
		`CREATE TABLE agents_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			label TEXT NOT NULL UNIQUE,
			lane TEXT NOT NULL CHECK (lane IN ('A','B','C','P','T')),
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
		`INSERT INTO agents_new (` + cols + `) SELECT ` + cols + ` FROM agents`,
		`DROP TABLE agents`,
		`ALTER TABLE agents_new RENAME TO agents`,
		`CREATE INDEX idx_agents_lane ON agents(lane)`,
	} {
		if _, err := tx.Exec(s); err != nil {
			return fmt.Errorf("migrate v21 agents rebuild: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migrate v21 agents commit: %w", err)
	}
	return nil
}

// migrateToV20 adds the ai_platforms registry plus per-account plan,
// capabilities, and profile mode (W15).
func migrateToV20(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS ai_platforms (
		key TEXT PRIMARY KEY,
		name TEXT NOT NULL DEFAULT '',
		login_url TEXT NOT NULL DEFAULT '',
		chat_url TEXT NOT NULL DEFAULT '',
		logged_out_markers TEXT NOT NULL DEFAULT '[]',
		composer_selector TEXT NOT NULL DEFAULT '',
		send_selector TEXT NOT NULL DEFAULT '',
		reply_selector TEXT NOT NULL DEFAULT '',
		reply_body_selector TEXT NOT NULL DEFAULT '',
		busy_selector TEXT NOT NULL DEFAULT '',
		done_selector TEXT NOT NULL DEFAULT '',
		thread_url_pattern TEXT NOT NULL DEFAULT '',
		icon TEXT NOT NULL DEFAULT '',
		color TEXT NOT NULL DEFAULT '',
		builtin INTEGER NOT NULL DEFAULT 0,
		chat_status TEXT NOT NULL DEFAULT 'unlinked',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`); err != nil {
		return fmt.Errorf("migrate v20 platforms: %w", err)
	}
	for _, row := range aiPlatformSeed {
		if _, err := db.Exec(`INSERT INTO ai_platforms
			(key,name,login_url,chat_url,logged_out_markers,composer_selector,send_selector,
			reply_selector,reply_body_selector,busy_selector,done_selector,thread_url_pattern,
			icon,color,builtin,chat_status,created_at,updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,1,'ok',datetime('now'),datetime('now')) ON CONFLICT(key) DO NOTHING`,
			row.key, row.name, row.login, row.chat, row.markers, row.composer, row.send,
			row.reply, row.replyBody, row.busy, row.done, "", "", ""); err != nil {
			return fmt.Errorf("migrate v20 seed: %w", err)
		}
	}
	for _, col := range []string{
		`plan_tier TEXT NOT NULL DEFAULT ''`,
		`capabilities TEXT NOT NULL DEFAULT '[]'`,
		`profile_mode TEXT NOT NULL DEFAULT 'incognito'`,
	} {
		name := strings.Split(col, " ")[0]
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('channel_accounts') WHERE name=?`, name).Scan(&n); err != nil {
			return fmt.Errorf("migrate v20 check: %w", err)
		}
		if n == 0 {
			if _, err := db.Exec(`ALTER TABLE channel_accounts ADD COLUMN ` + col); err != nil {
				return fmt.Errorf("migrate v20: %w", err)
			}
		}
	}
	return nil
}

// migrateToV19 widens the contents status CHECK with 'archived' (W6
// disposal). SQLite cannot alter a CHECK in place, so pre-v19 tables are
// rebuilt; FKs pointing at contents force the toggle-FKs procedure.
// Idempotent: fresh v19 tables (whose DDL already lists archived) skip.
func migrateToV19(db *sql.DB) error {
	var ddl string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='contents'`).Scan(&ddl); err != nil {
		return fmt.Errorf("migrate v19 inspect: %w", err)
	}
	if strings.Contains(ddl, "'archived'") {
		return nil
	}
	cols := `id,product_id,channel,target_platform,channel_group,title,body,disclosure,` +
		`is_sponsored,is_virtual_persona,ai_label,status,target_account,scheduled_at,` +
		`reserve_seq,held,hold_reason,created_at,updated_at,` +
		`revision_of,revision_root_id,generated_by_account_id`
	if _, err := db.Exec(`PRAGMA foreign_keys=OFF`); err != nil {
		return fmt.Errorf("migrate v19 fk off: %w", err)
	}
	defer db.Exec(`PRAGMA foreign_keys=ON`)
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("migrate v19 begin: %w", err)
	}
	defer tx.Rollback()
	stmts := []string{
		`CREATE TABLE contents_new (
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
			status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','review','approved','queued','published','archived')),
			target_account TEXT NOT NULL DEFAULT '',
			scheduled_at TEXT NOT NULL DEFAULT '',
			reserve_seq INTEGER NOT NULL DEFAULT 0,
			held INTEGER NOT NULL DEFAULT 0,
			hold_reason TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			revision_of INTEGER NOT NULL DEFAULT 0,
			revision_root_id INTEGER NOT NULL DEFAULT 0,
			generated_by_account_id INTEGER NULL REFERENCES channel_accounts(id)
		)`,
		`INSERT INTO contents_new (` + cols + `) SELECT ` + cols + ` FROM contents`,
		`DROP TABLE contents`,
		`ALTER TABLE contents_new RENAME TO contents`,
		`CREATE INDEX idx_contents_status ON contents(status)`,
		`CREATE INDEX idx_contents_product ON contents(product_id)`,
		`CREATE INDEX idx_contents_account ON contents(target_account)`,
		`CREATE INDEX idx_contents_revision_root ON contents(revision_root_id)`,
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return fmt.Errorf("migrate v19 rebuild: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migrate v19 commit: %w", err)
	}
	return nil
}

// migrateToV18 stamps draft authorship for W6 writer≠reviewer routing.
// Legacy rows keep NULL (unknown author, never excluded).
func migrateToV18(db *sql.DB) error {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('contents') WHERE name='generated_by_account_id'`).Scan(&n); err != nil {
		return fmt.Errorf("migrate v18 check: %w", err)
	}
	if n == 0 {
		if _, err := db.Exec(`ALTER TABLE contents ADD COLUMN generated_by_account_id INTEGER NULL REFERENCES channel_accounts(id)`); err != nil {
			return fmt.Errorf("migrate v18: %w", err)
		}
	}
	return nil
}

// migrateToV17 adds the channel_platforms registry (W21-1).
func migrateToV17(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS channel_platforms (
		key TEXT PRIMARY KEY,
		name TEXT NOT NULL DEFAULT '',
		grp TEXT NOT NULL DEFAULT 'manual',
		login_url TEXT NOT NULL DEFAULT '',
		text_rules TEXT NOT NULL DEFAULT '{}',
		media_rules TEXT NOT NULL DEFAULT '{}',
		status TEXT NOT NULL DEFAULT 'preparing'
	)`); err != nil {
		return fmt.Errorf("migrate v17 platforms: %w", err)
	}
	for _, row := range channelPlatformSeed {
		if _, err := db.Exec(`INSERT INTO channel_platforms
			(key,name,grp,login_url,text_rules,media_rules,status)
			VALUES ` + row + ` ON CONFLICT(key) DO NOTHING`); err != nil {
			return fmt.Errorf("migrate v17 seed: %w", err)
		}
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
