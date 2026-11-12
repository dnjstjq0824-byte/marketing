// Package store is the only layer that touches SQL (§3).
// Every other package reads and writes data through this package so a future
// PostgreSQL migration only rewrites this directory.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	_ "modernc.org/sqlite"
)

// Domain errors returned by this package.
var (
	ErrNotFound      = errors.New("not found")
	ErrDuplicate     = errors.New("duplicate")
	ErrAlreadyDone   = errors.New("job already finished")
	ErrNotClaimed    = errors.New("job is not claimed")
	ErrTypeMismatch  = errors.New("job type mismatch")
	ErrReadingExists = errors.New("reading already exists")
)

// Store wraps the SQLite database. ClaimMu serializes job claims: every claim
// flows through this single server process, so the mutex plus one transaction
// makes assignment atomic with no double delivery.
type Store struct {
	db      *sql.DB
	claimMu sync.Mutex
}

// Open connects to the SQLite file at path and applies pragmatic PRAGMAs.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	for _, pragma := range []string{
		`PRAGMA busy_timeout = 5000`,
		`PRAGMA journal_mode = WAL`,
		`PRAGMA foreign_keys = ON`,
		`PRAGMA synchronous = NORMAL`,
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("pragma %q: %w", pragma, err)
		}
	}
	db.SetMaxOpenConns(1)
	return &Store{db: db}, nil
}

// DB exposes the handle for migrations and backups only.
func (s *Store) DB() *sql.DB { return s.db }

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// Ping checks connectivity.
func (s *Store) Ping() error { return s.db.Ping() }

// ---------------------------------------------------------------------------
// Users
// ---------------------------------------------------------------------------

// User is an operator account.
type User struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

// EnsureAdmin creates the admin user when the users table is empty.
func (s *Store) EnsureAdmin(username, passwordHash string) (*User, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return nil, err
	}
	if n > 0 {
		return s.GetUserByUsername(username)
	}
	return s.CreateUser(username, passwordHash, "admin")
}

// CreateUser inserts an operator account.
func (s *Store) CreateUser(username, passwordHash, role string) (*User, error) {
	now := nowUTC()
	res, err := s.db.Exec(`INSERT INTO users(username,password_hash,role,created_at,updated_at)
		VALUES(?,?,?,?,?)`, username, passwordHash, role, now, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &User{ID: id, Username: username, PasswordHash: passwordHash, Role: role, CreatedAt: now, UpdatedAt: now}, nil
}

// GetUserByUsername finds a user by login name.
func (s *Store) GetUserByUsername(username string) (*User, error) {
	u := &User{}
	err := s.db.QueryRow(`SELECT id,username,password_hash,role,created_at,updated_at
		FROM users WHERE username=?`, username).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return u, err
}

// ---------------------------------------------------------------------------
// Products
// ---------------------------------------------------------------------------

// Product is an input product. ExitType A = partner mall link, B = affiliate link.
type Product struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	SourceURL string `json:"sourceUrl"`
	ExitType  string `json:"exitType"`
	Priority  int    `json:"priority"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// CreateProduct inserts a product row.
func (s *Store) CreateProduct(sourceURL, exitType string, priority int) (*Product, error) {
	now := nowUTC()
	res, err := s.db.Exec(`INSERT INTO products(name,source_url,exit_type,priority,created_at,updated_at)
		VALUES('',?,?,?,?,?)`, sourceURL, exitType, priority, now, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetProduct(id)
}

// GetProduct loads one product.
func (s *Store) GetProduct(id int64) (*Product, error) {
	p := &Product{}
	err := s.db.QueryRow(`SELECT id,name,source_url,exit_type,priority,created_at,updated_at
		FROM products WHERE id=?`, id).
		Scan(&p.ID, &p.Name, &p.SourceURL, &p.ExitType, &p.Priority, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return p, err
}

// ListProducts returns newest-first products.
func (s *Store) ListProducts(limit int) ([]*Product, error) {
	rows, err := s.db.Query(`SELECT id,name,source_url,exit_type,priority,created_at,updated_at
		FROM products ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Product
	for rows.Next() {
		p := &Product{}
		if err := rows.Scan(&p.ID, &p.Name, &p.SourceURL, &p.ExitType, &p.Priority, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SetProductName fills the product name from a reading result.
func (s *Store) SetProductName(id int64, name string) error {
	_, err := s.db.Exec(`UPDATE products SET name=?, updated_at=? WHERE id=?`, name, nowUTC(), id)
	return err
}

// ---------------------------------------------------------------------------
// Product images
// ---------------------------------------------------------------------------

// ProductImage is one uploaded product image.
type ProductImage struct {
	ID        int64  `json:"id"`
	ProductID int64  `json:"productId"`
	Path      string `json:"path"`
	Sort      int    `json:"sort"`
	CreatedAt string `json:"createdAt"`
}

// AddImage records an uploaded image file.
func (s *Store) AddImage(productID int64, path string, sort int) (*ProductImage, error) {
	now := nowUTC()
	res, err := s.db.Exec(`INSERT INTO product_images(product_id,path,sort,created_at) VALUES(?,?,?,?)`,
		productID, path, sort, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &ProductImage{ID: id, ProductID: productID, Path: path, Sort: sort, CreatedAt: now}, nil
}

// ListImages returns a product's images in sort order.
func (s *Store) ListImages(productID int64) ([]*ProductImage, error) {
	rows, err := s.db.Query(`SELECT id,product_id,path,sort,created_at FROM product_images
		WHERE product_id=? ORDER BY sort ASC, id ASC`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*ProductImage
	for rows.Next() {
		im := &ProductImage{}
		if err := rows.Scan(&im.ID, &im.ProductID, &im.Path, &im.Sort, &im.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, im)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Readings
// ---------------------------------------------------------------------------

// Reading is the stored analyze result: 8 string fields (§5).
type Reading struct {
	ID            int64  `json:"id"`
	ProductID     int64  `json:"productId"`
	ProductName   string `json:"productName"`
	PriceSummary  string `json:"priceSummary"`
	OptionsText   string `json:"optionsText"`
	ShippingText  string `json:"shippingText"`
	DetailCore    string `json:"detailCore"`
	ReviewSummary string `json:"reviewSummary"`
	ImageDesc     string `json:"imageDesc"`
	Unknowns      string `json:"unknowns"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

// UpsertReading stores the analyze result for a product (one reading per product).
func (s *Store) UpsertReading(r *Reading) (*Reading, error) {
	now := nowUTC()
	var id int64
	err := s.db.QueryRow(`SELECT id FROM readings WHERE product_id=?`, r.ProductID).Scan(&id)
	switch {
	case err == sql.ErrNoRows:
		res, err := s.db.Exec(`INSERT INTO readings(product_id,product_name,price_summary,options_text,
			shipping_text,detail_core,review_summary,image_desc,unknowns,created_at,updated_at)
			VALUES(?,?,?,?,?,?,?,?,?,?,?)`, r.ProductID, r.ProductName, r.PriceSummary,
			r.OptionsText, r.ShippingText, r.DetailCore, r.ReviewSummary, r.ImageDesc, r.Unknowns, now, now)
		if err != nil {
			return nil, err
		}
		id, _ = res.LastInsertId()
	case err != nil:
		return nil, err
	default:
		if _, err := s.db.Exec(`UPDATE readings SET product_name=?,price_summary=?,options_text=?,
			shipping_text=?,detail_core=?,review_summary=?,image_desc=?,unknowns=?,updated_at=?
			WHERE id=?`, r.ProductName, r.PriceSummary, r.OptionsText, r.ShippingText,
			r.DetailCore, r.ReviewSummary, r.ImageDesc, r.Unknowns, now, id); err != nil {
			return nil, err
		}
	}
	return s.GetReadingByProduct(r.ProductID)
}

// GetReadingByProduct loads a product's reading.
func (s *Store) GetReadingByProduct(productID int64) (*Reading, error) {
	r := &Reading{}
	err := s.db.QueryRow(`SELECT id,product_id,product_name,price_summary,options_text,shipping_text,
		detail_core,review_summary,image_desc,unknowns,created_at,updated_at FROM readings WHERE product_id=?`, productID).
		Scan(&r.ID, &r.ProductID, &r.ProductName, &r.PriceSummary, &r.OptionsText, &r.ShippingText,
			&r.DetailCore, &r.ReviewSummary, &r.ImageDesc, &r.Unknowns, &r.CreatedAt, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return r, err
}

// ---------------------------------------------------------------------------
// Jobs (work queue)
// ---------------------------------------------------------------------------

// Job is one worker queue entry.
type Job struct {
	ID         int64  `json:"id"`
	Type       string `json:"type"`
	RefID      int64  `json:"refId"`
	Status     string `json:"status"`
	WorkerName string `json:"workerName"`
	Payload    string `json:"payload"`
	Result     string `json:"result"`
	Error      string `json:"error"`
	CreatedAt  string `json:"createdAt"`
	ClaimedAt  string `json:"claimedAt"`
	FinishedAt string `json:"finishedAt"`
}

func scanJob(rows interface{ Scan(...any) error }) (*Job, error) {
	j := &Job{}
	err := rows.Scan(&j.ID, &j.Type, &j.RefID, &j.Status, &j.WorkerName, &j.Payload,
		&j.Result, &j.Error, &j.CreatedAt, &j.ClaimedAt, &j.FinishedAt)
	return j, err
}

// CreateJob enqueues a pending job.
func (s *Store) CreateJob(jobType string, refID int64, payload string) (*Job, error) {
	now := nowUTC()
	res, err := s.db.Exec(`INSERT INTO jobs(type,ref_id,status,worker_name,payload,created_at)
		VALUES(?,?,'pending','',?,?)`, jobType, refID, payload, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetJob(id)
}

// GetJob loads one job.
func (s *Store) GetJob(id int64) (*Job, error) {
	j, err := scanJob(s.db.QueryRow(`SELECT id,type,ref_id,status,worker_name,payload,result,
		error,created_at,claimed_at,finished_at FROM jobs WHERE id=?`, id))
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return j, err
}

// CountOpenJobs counts pending/claimed/done jobs of a type+ref (guard against
// double auto-creation when a worker retries after a mid-flight DB error).
func (s *Store) CountOpenJobs(jobType string, refID int64) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM jobs WHERE type=? AND ref_id=? AND status IN ('pending','claimed','done')`,
		jobType, refID).Scan(&n)
	return n, err
}

// ClaimNextJob atomically assigns the oldest pending job of a type.
// Returns (nil, nil) when no job waits.
func (s *Store) ClaimNextJob(jobType, worker string) (*Job, error) {
	s.claimMu.Lock()
	defer s.claimMu.Unlock()
	ctx := context.Background()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(ctx, `ROLLBACK`)
		}
	}()
	var id int64
	err = conn.QueryRowContext(ctx, `SELECT id FROM jobs WHERE status='pending' AND type=?
		ORDER BY id ASC LIMIT 1`, jobType).Scan(&id)
	if err == sql.ErrNoRows {
		_, _ = conn.ExecContext(ctx, `ROLLBACK`)
		committed = true // rolled back intentionally; suppress deferred rollback
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	now := nowUTC()
	res, err := conn.ExecContext(ctx, `UPDATE jobs SET status='claimed',worker_name=?,claimed_at=?
		WHERE id=? AND status='pending'`, worker, now, id)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return nil, fmt.Errorf("claim lost race for job %d", id)
	}
	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		return nil, err
	}
	committed = true
	// Release the pooled connection before any follow-up query: with a
	// single pooled connection, querying while `conn` is checked out
	// deadlocks. The deferred Close becomes a harmless no-op.
	_ = conn.Close()
	return s.GetJob(id)
}

// SetJobResult marks a claimed job done. Not idempotent: re-registering on a
// finished job returns ErrAlreadyDone; an unclaimed job returns ErrNotClaimed.
func (s *Store) SetJobResult(id int64, result string) (*Job, error) {
	res, err := s.db.Exec(`UPDATE jobs SET status='done',result=?,finished_at=? WHERE id=? AND status='claimed'`,
		result, nowUTC(), id)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 1 {
		return s.GetJob(id)
	}
	cur, err := s.GetJob(id)
	if err != nil {
		return nil, err
	}
	switch cur.Status {
	case "done", "failed":
		return nil, ErrAlreadyDone
	default:
		return nil, ErrNotClaimed
	}
}

// SetJobFailed marks a pending/claimed job failed. Failed jobs never requeue.
func (s *Store) SetJobFailed(id int64, reason string) (*Job, error) {
	res, err := s.db.Exec(`UPDATE jobs SET status='failed',error=?,finished_at=?
		WHERE id=? AND status IN ('pending','claimed')`, reason, nowUTC(), id)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 1 {
		return s.GetJob(id)
	}
	cur, err := s.GetJob(id)
	if err != nil {
		return nil, err
	}
	if cur.Status == "done" || cur.Status == "failed" {
		return nil, ErrAlreadyDone
	}
	return nil, ErrNotClaimed
}

// ListRecentJobs returns the newest 100 jobs.
func (s *Store) ListRecentJobs() ([]*Job, error) {
	rows, err := s.db.Query(`SELECT id,type,ref_id,status,worker_name,payload,result,error,
		created_at,claimed_at,finished_at FROM jobs ORDER BY id DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// ListJobsByRef returns a product's jobs (analyze + generate) oldest-first.
func (s *Store) ListJobsByRef(refID int64) ([]*Job, error) {
	rows, err := s.db.Query(`SELECT id,type,ref_id,status,worker_name,payload,result,error,
		created_at,claimed_at,finished_at FROM jobs WHERE ref_id=? ORDER BY id ASC`, refID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Contents
// ---------------------------------------------------------------------------

// Content is one generated content item.
type Content struct {
	ID               int64  `json:"id"`
	ProductID        int64  `json:"productId"`
	Channel          string `json:"channel"`
	TargetPlatform   string `json:"targetPlatform"`
	ChannelGroup     string `json:"channelGroup"`
	Title            string `json:"title"`
	Body             string `json:"body"`
	Disclosure       string `json:"disclosure"`
	IsSponsored      bool   `json:"isSponsored"`
	IsVirtualPersona bool   `json:"isVirtualPersona"`
	AILabel          string `json:"aiLabel"`
	Status           string `json:"status"`
	TargetAccount    string `json:"targetAccount"`
	ScheduledAt      string `json:"scheduledAt"`
	ReserveSeq       int64  `json:"reserveSeq"`
	Held             bool   `json:"held"`
	HoldReason       string `json:"holdReason"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

func scanContent(row interface{ Scan(...any) error }) (*Content, error) {
	c := &Content{}
	var sponsored, persona, held int
	err := row.Scan(&c.ID, &c.ProductID, &c.Channel, &c.TargetPlatform, &c.ChannelGroup,
		&c.Title, &c.Body, &c.Disclosure, &sponsored, &persona, &c.AILabel, &c.Status,
		&c.TargetAccount, &c.ScheduledAt, &c.ReserveSeq, &held, &c.HoldReason, &c.CreatedAt, &c.UpdatedAt)
	c.IsSponsored, c.IsVirtualPersona, c.Held = sponsored == 1, persona == 1, held == 1
	return c, err
}

const contentCols = `id,product_id,channel,target_platform,channel_group,title,body,disclosure,
	is_sponsored,is_virtual_persona,ai_label,status,target_account,scheduled_at,reserve_seq,held,hold_reason,created_at,updated_at`

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// CreateContent inserts a draft content item.
func (s *Store) CreateContent(c *Content) (*Content, error) {
	now := nowUTC()
	res, err := s.db.Exec(`INSERT INTO contents(product_id,channel,target_platform,channel_group,title,body,
		disclosure,is_sponsored,is_virtual_persona,ai_label,status,target_account,scheduled_at,held,hold_reason,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, c.ProductID, c.Channel, "", "", c.Title, c.Body,
		c.Disclosure, boolInt(c.IsSponsored), boolInt(c.IsVirtualPersona), c.AILabel, "draft", "", "", 0, "", now, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetContent(id)
}

// GetContent loads one content item.
func (s *Store) GetContent(id int64) (*Content, error) {
	c, err := scanContent(s.db.QueryRow(`SELECT `+contentCols+` FROM contents WHERE id=?`, id))
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return c, err
}

// ListContents filters by status ("" = all), newest-first.
func (s *Store) ListContents(status string, limit int) ([]*Content, error) {
	q := `SELECT ` + contentCols + ` FROM contents`
	var args []any
	if status != "" {
		q += ` WHERE status=?`
		args = append(args, status)
	}
	q += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Content
	for rows.Next() {
		c, err := scanContent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListQueuedByGroup lists publish-queue items of one channel group in execution
// order: scheduled time first (empty last), then queue insertion order.
// The two groups are never mixed in one call.
func (s *Store) ListQueuedByGroup(group string) ([]*Content, error) {
	rows, err := s.db.Query(`SELECT `+contentCols+` FROM contents
		WHERE status='queued' AND channel_group=?
		ORDER BY (scheduled_at=''), scheduled_at ASC, reserve_seq ASC, id ASC`, group)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Content
	for rows.Next() {
		c, err := scanContent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// UpdateContentStatus moves a content item to a new status.
func (s *Store) UpdateContentStatus(id int64, status string) error {
	_, err := s.db.Exec(`UPDATE contents SET status=?, updated_at=? WHERE id=?`, status, nowUTC(), id)
	return err
}

// UpdateContentBody edits title/body only; disclosure is never touched here,
// so review-screen edits cannot delete the compensation notice.
func (s *Store) UpdateContentBody(id int64, title, body string) error {
	_, err := s.db.Exec(`UPDATE contents SET title=?, body=?, updated_at=? WHERE id=?`, title, body, nowUTC(), id)
	return err
}

// UpdateContentFlags sets virtual-persona flag and AI label together.
func (s *Store) UpdateContentFlags(id int64, persona bool, aiLabel string) error {
	_, err := s.db.Exec(`UPDATE contents SET is_virtual_persona=?, ai_label=?, updated_at=?
		WHERE id=?`, boolInt(persona), aiLabel, nowUTC(), id)
	return err
}

// ScheduleContent sets the publish reservation of a queued item and stamps it
// with a fresh reservation sequence so same-account/same-time ties execute in
// the order reservations were loaded onto the queue (§9).
func (s *Store) ScheduleContent(id int64, account, platform, group, scheduledAt string) error {
	_, err := s.db.Exec(`UPDATE contents SET target_account=?, target_platform=?, channel_group=?,
		scheduled_at=?, reserve_seq=(SELECT COALESCE(MAX(reserve_seq),0)+1 FROM contents),
		updated_at=? WHERE id=?`, account, platform, group, scheduledAt, nowUTC(), id)
	return err
}

// QueuePosition returns the 1-based serial position among reservations of the
// same account and same scheduled time, ordered by reservation sequence.
func (s *Store) QueuePosition(account, scheduledAt string, id int64) (int, error) {
	var pos int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM contents WHERE target_account=? AND scheduled_at=?
		AND status='queued' AND reserve_seq<=(SELECT reserve_seq FROM contents WHERE id=?)`,
		account, scheduledAt, id).Scan(&pos)
	return pos, err
}

// SetHeldByAccount holds every queued item of one account. Other accounts are
// untouched. Returns the number of items held.
func (s *Store) SetHeldByAccount(account, reason string) (int, error) {
	res, err := s.db.Exec(`UPDATE contents SET held=1, hold_reason=?, updated_at=?
		WHERE target_account=? AND status='queued' AND held=0`, reason, nowUTC(), account)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// ListHeldByAccount returns held queued items of one account.
func (s *Store) ListHeldByAccount(account string) ([]*Content, error) {
	rows, err := s.db.Query(`SELECT `+contentCols+` FROM contents
		WHERE target_account=? AND status='queued' AND held=1 ORDER BY id ASC`, account)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Content
	for rows.Next() {
		c, err := scanContent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ClearHold releases one held item.
func (s *Store) ClearHold(id int64) error {
	_, err := s.db.Exec(`UPDATE contents SET held=0, hold_reason='', updated_at=? WHERE id=?`, nowUTC(), id)
	return err
}

// SetHoldReason rewrites the hold reason of one item (stays held).
func (s *Store) SetHoldReason(id int64, reason string) error {
	_, err := s.db.Exec(`UPDATE contents SET hold_reason=?, updated_at=? WHERE id=?`, reason, nowUTC(), id)
	return err
}

// ---------------------------------------------------------------------------
// Publish logs (duplicate-publish guard structure)
// ---------------------------------------------------------------------------

// PublishLog records a publish plan/completion per content+channel+account.
// The UNIQUE(content_id, channel, account) constraint is the duplicate guard.
type PublishLog struct {
	ID         int64  `json:"id"`
	ContentID  int64  `json:"contentId"`
	Channel    string `json:"channel"`
	Account    string `json:"account"`
	Status     string `json:"status"`
	ExecutedAt string `json:"executedAt"`
	Note       string `json:"note"`
	CreatedAt  string `json:"createdAt"`
}

// CreatePublishLog inserts a plan record. Duplicate triplets return ErrDuplicate.
func (s *Store) CreatePublishLog(contentID int64, channel, account, status, note string) (*PublishLog, error) {
	now := nowUTC()
	_, err := s.db.Exec(`INSERT INTO publish_logs(content_id,channel,account,status,executed_at,note,created_at)
		VALUES(?,?,?,?,?,?,?)`, contentID, channel, account, status, "", note, now)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDuplicate
		}
		return nil, err
	}
	return s.GetPublishLog(contentID, channel, account)
}

// GetPublishLog loads one record by its unique triplet.
func (s *Store) GetPublishLog(contentID int64, channel, account string) (*PublishLog, error) {
	l := &PublishLog{}
	err := s.db.QueryRow(`SELECT id,content_id,channel,account,status,executed_at,note,created_at
		FROM publish_logs WHERE content_id=? AND channel=? AND account=?`, contentID, channel, account).
		Scan(&l.ID, &l.ContentID, &l.Channel, &l.Account, &l.Status, &l.ExecutedAt, &l.Note, &l.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return l, err
}

// ListPublishLogs returns newest-first records.
func (s *Store) ListPublishLogs(limit int) ([]*PublishLog, error) {
	rows, err := s.db.Query(`SELECT id,content_id,channel,account,status,executed_at,note,created_at
		FROM publish_logs ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*PublishLog
	for rows.Next() {
		l := &PublishLog{}
		if err := rows.Scan(&l.ID, &l.ContentID, &l.Channel, &l.Account, &l.Status, &l.ExecutedAt, &l.Note, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return containsFold(msg, "UNIQUE constraint failed") || containsFold(msg, "unique_violation") ||
		containsFold(msg, "duplicate key")
}

func containsFold(haystack, needle string) bool {
	if len(needle) > len(haystack) {
		return false
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			a, b := haystack[i+j], needle[j]
			if 'A' <= a && a <= 'Z' {
				a += 'a' - 'A'
			}
			if 'A' <= b && b <= 'Z' {
				b += 'a' - 'A'
			}
			if a != b {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Metrics
// ---------------------------------------------------------------------------

// Metric is one collected performance record. Nil values mean "not collected".
type Metric struct {
	ID           int64  `json:"id"`
	ContentID    *int64 `json:"contentId"`
	Channel      string `json:"channel"`
	Exposures    *int64 `json:"exposures"`
	Inflows      *int64 `json:"inflows"`
	Orders       *int64 `json:"orders"`
	Inquiries    *int64 `json:"inquiries"`
	CauseClass   string `json:"causeClass"`
	CheckURL     bool   `json:"checkUrl"`
	CheckSession bool   `json:"checkSession"`
	CheckDisplay bool   `json:"checkDisplay"`
	Note         string `json:"note"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

func scanMetric(row interface{ Scan(...any) error }) (*Metric, error) {
	m := &Metric{}
	var contentID, exp, inf, ord, inq sql.NullInt64
	var url, session, display int
	err := row.Scan(&m.ID, &contentID, &m.Channel, &exp, &inf, &ord, &inq,
		&m.CauseClass, &url, &session, &display, &m.Note, &m.CreatedAt, &m.UpdatedAt)
	if contentID.Valid {
		v := contentID.Int64
		m.ContentID = &v
	}
	if exp.Valid {
		v := exp.Int64
		m.Exposures = &v
	}
	if inf.Valid {
		v := inf.Int64
		m.Inflows = &v
	}
	if ord.Valid {
		v := ord.Int64
		m.Orders = &v
	}
	if inq.Valid {
		v := inq.Int64
		m.Inquiries = &v
	}
	m.CheckURL, m.CheckSession, m.CheckDisplay = url == 1, session == 1, display == 1
	return m, err
}

const metricCols = `id,content_id,channel,exposures,inflows,orders,inquiries,cause_class,
	check_url,check_session,check_display,note,created_at,updated_at`

func nullInt64(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
}

// CreateMetric inserts one record with its precomputed cause class.
func (s *Store) CreateMetric(m *Metric) (*Metric, error) {
	now := nowUTC()
	res, err := s.db.Exec(`INSERT INTO metrics(content_id,channel,exposures,inflows,orders,inquiries,
		cause_class,check_url,check_session,check_display,note,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, nullInt64(m.ContentID), m.Channel, nullInt64(m.Exposures),
		nullInt64(m.Inflows), nullInt64(m.Orders), nullInt64(m.Inquiries), m.CauseClass,
		boolInt(m.CheckURL), boolInt(m.CheckSession), boolInt(m.CheckDisplay), m.Note, now, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetMetric(id)
}

// GetMetric loads one record.
func (s *Store) GetMetric(id int64) (*Metric, error) {
	m, err := scanMetric(s.db.QueryRow(`SELECT `+metricCols+` FROM metrics WHERE id=?`, id))
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return m, err
}

// ListMetrics returns newest-first records.
func (s *Store) ListMetrics(limit int) ([]*Metric, error) {
	rows, err := s.db.Query(`SELECT `+metricCols+` FROM metrics ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Metric
	for rows.Next() {
		m, err := scanMetric(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// UpdateMetricChecks records the 3-step judgment flow progress on a record.
func (s *Store) UpdateMetricChecks(id int64, url, session, display bool, note string) error {
	_, err := s.db.Exec(`UPDATE metrics SET check_url=?, check_session=?, check_display=?, note=?, updated_at=?
		WHERE id=?`, boolInt(url), boolInt(session), boolInt(display), note, nowUTC(), id)
	return err
}

// ---------------------------------------------------------------------------
// Affiliate links (exit B). No commission-rate column by design (§6: unconfirmed).
// ---------------------------------------------------------------------------

// AffiliateLink is one exit-B destination.
type AffiliateLink struct {
	ID          int64  `json:"id"`
	ProductID   int64  `json:"productId"`
	PartnerName string `json:"partnerName"`
	URL         string `json:"url"`
	CreatedAt   string `json:"createdAt"`
}

// CreateAffiliateLink inserts one link.
func (s *Store) CreateAffiliateLink(productID int64, partner, url string) (*AffiliateLink, error) {
	now := nowUTC()
	res, err := s.db.Exec(`INSERT INTO affiliate_links(product_id,partner_name,url,created_at)
		VALUES(?,?,?,?)`, productID, partner, url, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &AffiliateLink{ID: id, ProductID: productID, PartnerName: partner, URL: url, CreatedAt: now}, nil
}

// ListAffiliateLinks returns a product's links oldest-first.
func (s *Store) ListAffiliateLinks(productID int64) ([]*AffiliateLink, error) {
	rows, err := s.db.Query(`SELECT id,product_id,partner_name,url,created_at FROM affiliate_links
		WHERE product_id=? ORDER BY id ASC`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*AffiliateLink
	for rows.Next() {
		l := &AffiliateLink{}
		if err := rows.Scan(&l.ID, &l.ProductID, &l.PartnerName, &l.URL, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Dashboard counters
// ---------------------------------------------------------------------------

// Counts is the dashboard summary.
type Counts struct {
	ReviewWaiting  int `json:"reviewWaiting"`
	JobsPending    int `json:"jobsPending"`
	JobsClaimed    int `json:"jobsClaimed"`
	JobsFailed     int `json:"jobsFailed"`
	QueuedTotal    int `json:"queuedTotal"`
	HeldTotal      int `json:"heldTotal"`
	ScheduledToday int `json:"scheduledToday"`
}

// DashboardCounts aggregates the home-view summary in fixed queries.
func (s *Store) DashboardCounts(todayPrefix string) (*Counts, error) {
	c := &Counts{}
	queries := []struct {
		dest *int
		q    string
		args []any
	}{
		{&c.ReviewWaiting, `SELECT COUNT(*) FROM contents WHERE status='review'`, nil},
		{&c.JobsPending, `SELECT COUNT(*) FROM jobs WHERE status='pending'`, nil},
		{&c.JobsClaimed, `SELECT COUNT(*) FROM jobs WHERE status='claimed'`, nil},
		{&c.JobsFailed, `SELECT COUNT(*) FROM jobs WHERE status='failed'`, nil},
		{&c.QueuedTotal, `SELECT COUNT(*) FROM contents WHERE status='queued'`, nil},
		{&c.HeldTotal, `SELECT COUNT(*) FROM contents WHERE status='queued' AND held=1`, nil},
		{&c.ScheduledToday, `SELECT COUNT(*) FROM contents WHERE status='queued' AND scheduled_at LIKE ?`, []any{todayPrefix + "%"}},
	}
	for _, item := range queries {
		if err := s.db.QueryRow(item.q, item.args...).Scan(item.dest); err != nil {
			return nil, err
		}
	}
	return c, nil
}
