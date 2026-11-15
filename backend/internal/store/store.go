// Package store is the only layer that touches SQL (짠3).
// Every other package reads and writes data through this package so a future
// PostgreSQL migration only rewrites this directory.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

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
	ErrNotFailed     = errors.New("job is not failed")
)

// Store wraps the SQLite database. ClaimMu serializes job claims: every claim
// flows through this single server process, so the mutex plus one transaction
// makes assignment atomic with no double delivery.
//
// D26: one write connection plus a read pool. WAL snapshot reads never block
// on the writer, so the read pool needs no pragmas of its own.
type Store struct {
	db      *sql.DB
	ro      *sql.DB
	claimMu sync.Mutex
}

// DefaultReadConns sizes the read pool when callers do not say otherwise.
const DefaultReadConns = 4

// Open connects with the default read pool.
func Open(path string) (*Store, error) {
	return OpenPool(path, DefaultReadConns)
}

// OpenPool connects to the SQLite file at path: a single write connection
// with pragmatic PRAGMAs plus a read pool of readConns.
func OpenPool(path string, readConns int) (*Store, error) {
	if readConns < 1 {
		readConns = 1
	}
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
	ro, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("open read pool: %w", err)
	}
	ro.SetMaxOpenConns(readConns)
	return &Store{db: db, ro: ro}, nil
}

// DB exposes the write handle for migrations and backups only.
func (s *Store) DB() *sql.DB { return s.db }

// Close closes both pools.
func (s *Store) Close() error {
	rerr := s.ro.Close()
	if err := s.db.Close(); err != nil {
		return err
	}
	return rerr
}

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
	if err := s.ro.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
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

// ListUsers returns all operators oldest-first.
func (s *Store) ListUsers() ([]*User, error) {
	rows, err := s.ro.Query(`SELECT id,username,password_hash,role,created_at,updated_at
		FROM users ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*User{}
	for rows.Next() {
		u := &User{}
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// UpdateUserPassword replaces one user's password hash.
func (s *Store) UpdateUserPassword(id int64, hash string) error {
	res, err := s.db.Exec(`UPDATE users SET password_hash=?, updated_at=? WHERE id=?`, hash, nowUTC(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteUser removes one operator.
func (s *Store) DeleteUser(id int64) error {
	res, err := s.db.Exec(`DELETE FROM users WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// CountUsers returns the operator count.
func (s *Store) CountUsers() (int, error) {
	var n int
	err := s.ro.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

// GetUserByUsername finds a user by login name.
func (s *Store) GetUserByUsername(username string) (*User, error) {
	u := &User{}
	err := s.ro.QueryRow(`SELECT id,username,password_hash,role,created_at,updated_at
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
// Business groups products so one engine serves every business line.
type Product struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	SourceURL string `json:"sourceUrl"`
	ExitType  string `json:"exitType"`
	Priority  int    `json:"priority"`
	Business  string `json:"business"`
	BriefJSON string `json:"-"`
	// TeamID owns the work (W16). NULL resolves to the default team.
	TeamID *int64 `json:"teamId"`
	// InputKind url|image|topic + Topic + TargetPlatforms (W1).
	InputKind       string `json:"inputKind"`
	Topic           string `json:"topic"`
	TargetPlatforms string `json:"targetPlatforms"`
	// Industry drives the W18-2 wordlists (cosmetics/health/food/"").
	Industry  string `json:"industry"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

const productCols = `id,name,source_url,exit_type,priority,business,brief_json,team_id,input_kind,topic,target_platforms,industry,created_at,updated_at`

func scanProduct(row interface{ Scan(...any) error }) (*Product, error) {
	p := &Product{}
	var team sql.NullInt64
	err := row.Scan(&p.ID, &p.Name, &p.SourceURL, &p.ExitType, &p.Priority, &p.Business, &p.BriefJSON, &team,
		&p.InputKind, &p.Topic, &p.TargetPlatforms, &p.Industry, &p.CreatedAt, &p.UpdatedAt)
	if team.Valid {
		v := team.Int64
		p.TeamID = &v
	}
	return p, err
}

// CreateProduct inserts a product row.
func (s *Store) CreateProduct(sourceURL, exitType string, priority int, business string) (*Product, error) {
	now := nowUTC()
	res, err := s.db.Exec(`INSERT INTO products(name,source_url,exit_type,priority,business,created_at,updated_at)
		VALUES('',?,?,?,?,?,?)`, sourceURL, exitType, priority, business, now, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetProduct(id)
}

// GetProduct loads one product.
func (s *Store) GetProduct(id int64) (*Product, error) {
	p, err := scanProduct(s.ro.QueryRow(`SELECT `+productCols+` FROM products WHERE id=?`, id))
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return p, err
}

// ListProducts returns newest-first products, optionally filtered by business.
// Business matching is case-insensitive so spelling variants stay together.
func (s *Store) ListProducts(limit int, business string) ([]*Product, error) {
	q := `SELECT ` + productCols + ` FROM products`
	var args []any
	if business != "" {
		q += ` WHERE business=? COLLATE NOCASE`
		args = append(args, business)
	}
	q += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.ro.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Product
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
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

// SetProductBrief stores the S2 work brief JSON on the product.
func (s *Store) SetProductBrief(id int64, briefJSON string) error {
	_, err := s.db.Exec(`UPDATE products SET brief_json=?, updated_at=? WHERE id=?`, briefJSON, nowUTC(), id)
	return err
}

// SetProductIndustry records the W18-2 industry tag.
func (s *Store) SetProductIndustry(id int64, industry string) error {
	_, err := s.db.Exec(`UPDATE products SET industry=?, updated_at=? WHERE id=?`, industry, nowUTC(), id)
	return err
}

// SetProductInput records the W1 input typing: kind url|image|topic, the
// topic text, and the canonical target platform list (JSON array).
func (s *Store) SetProductInput(id int64, kind, topic, targets string) error {
	_, err := s.db.Exec(`UPDATE products SET input_kind=?, topic=?, target_platforms=?, updated_at=? WHERE id=?`,
		kind, topic, targets, nowUTC(), id)
	return err
}

// UpdateProduct rewrites the editable product fields.
func (s *Store) UpdateProduct(id int64, name, sourceURL, exitType string, priority int, business string) error {
	return s.UpdateProductFull(id, name, sourceURL, exitType, priority, business, "")
}

// UpdateProductFull rewrites a product including the industry tag. Empty
// industry keeps the stored value.
func (s *Store) UpdateProductFull(id int64, name, sourceURL, exitType string, priority int, business, industry string) error {
	if industry != "" {
		if _, err := s.db.Exec(`UPDATE products SET industry=? WHERE id=?`, industry, id); err != nil {
			return err
		}
	}
	res, err := s.db.Exec(`UPDATE products SET name=?, source_url=?, exit_type=?, priority=?, business=?, updated_at=?
		WHERE id=?`, name, sourceURL, exitType, priority, business, nowUTC(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteProduct hard-deletes a product and everything under it: jobs (which
// carry no FK), images, reading, contents, and affiliate links. Publish logs
// and metrics fall via the contents cascade. Upload files on disk are removed
// by the caller. Missing ids return ErrNotFound.
func (s *Store) DeleteProduct(id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM jobs WHERE ref_id=?`, id); err != nil {
		return err
	}
	res, err := tx.Exec(`DELETE FROM products WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
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
	rows, err := s.ro.Query(`SELECT id,product_id,path,sort,created_at FROM product_images
		WHERE product_id=? ORDER BY sort ASC, id ASC`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*ProductImage{}
	for rows.Next() {
		im := &ProductImage{}
		if err := rows.Scan(&im.ID, &im.ProductID, &im.Path, &im.Sort, &im.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, im)
	}
	return out, rows.Err()
}

// GetProductImage loads one image by id.
func (s *Store) GetProductImage(id int64) (*ProductImage, error) {
	im := &ProductImage{}
	err := s.ro.QueryRow(`SELECT id,product_id,path,sort,created_at FROM product_images
		WHERE id=?`, id).Scan(&im.ID, &im.ProductID, &im.Path, &im.Sort, &im.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return im, err
}

// DeleteProductImage removes one image row. The file is unlinked by the caller.
func (s *Store) DeleteProductImage(id int64) error {
	res, err := s.db.Exec(`DELETE FROM product_images WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// Readings
// ---------------------------------------------------------------------------

// Reading is the stored analyze result: 8 string fields (짠5).
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
	err := s.ro.QueryRow(`SELECT id FROM readings WHERE product_id=?`, r.ProductID).Scan(&id)
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

// UpdateReadingByProduct rewrites a product's reading fields (operator fix).
func (s *Store) UpdateReadingByProduct(r *Reading) error {
	res, err := s.db.Exec(`UPDATE readings SET product_name=?,price_summary=?,options_text=?,
		shipping_text=?,detail_core=?,review_summary=?,image_desc=?,unknowns=?,updated_at=?
		WHERE product_id=?`, r.ProductName, r.PriceSummary, r.OptionsText, r.ShippingText,
		r.DetailCore, r.ReviewSummary, r.ImageDesc, r.Unknowns, nowUTC(), r.ProductID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Strategy is the stored strategy result: 4 strings (agent A's output).
type Strategy struct {
	ID           int64  `json:"id"`
	ProductID    int64  `json:"productId"`
	Angle        string `json:"angle"`
	Channels     string `json:"channels"`
	KeyMessages  string `json:"keyMessages"`
	ScheduleNote string `json:"scheduleNote"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

// UpsertStrategy stores the strategy result for a product (one per product).
func (s *Store) UpsertStrategy(st *Strategy) (*Strategy, error) {
	now := nowUTC()
	var id int64
	err := s.ro.QueryRow(`SELECT id FROM strategies WHERE product_id=?`, st.ProductID).Scan(&id)
	switch {
	case err == sql.ErrNoRows:
		res, err := s.db.Exec(`INSERT INTO strategies(product_id,angle,channels,key_messages,
			schedule_note,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`,
			st.ProductID, st.Angle, st.Channels, st.KeyMessages, st.ScheduleNote, now, now)
		if err != nil {
			return nil, err
		}
		id, _ = res.LastInsertId()
	case err != nil:
		return nil, err
	default:
		if _, err := s.db.Exec(`UPDATE strategies SET angle=?,channels=?,key_messages=?,
			schedule_note=?,updated_at=? WHERE id=?`, st.Angle, st.Channels,
			st.KeyMessages, st.ScheduleNote, now, id); err != nil {
			return nil, err
		}
	}
	return s.GetStrategyByProduct(st.ProductID)
}

// GetStrategyByProduct loads a product's strategy.
func (s *Store) GetStrategyByProduct(productID int64) (*Strategy, error) {
	st := &Strategy{}
	err := s.ro.QueryRow(`SELECT id,product_id,angle,channels,key_messages,schedule_note,
		created_at,updated_at FROM strategies WHERE product_id=?`, productID).
		Scan(&st.ID, &st.ProductID, &st.Angle, &st.Channels, &st.KeyMessages,
			&st.ScheduleNote, &st.CreatedAt, &st.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return st, err
}

// UpdateStrategyByProduct rewrites a product's strategy (operator fix).
func (s *Store) UpdateStrategyByProduct(st *Strategy) error {
	res, err := s.db.Exec(`UPDATE strategies SET angle=?,channels=?,key_messages=?,
		schedule_note=?,updated_at=? WHERE product_id=?`, st.Angle, st.Channels,
		st.KeyMessages, st.ScheduleNote, nowUTC(), st.ProductID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetReadingByProduct loads a product's reading.
func (s *Store) GetReadingByProduct(productID int64) (*Reading, error) {
	r := &Reading{}
	err := s.ro.QueryRow(`SELECT id,product_id,product_name,price_summary,options_text,shipping_text,
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
	AgentID    int64  `json:"agentId"`
	Business   string `json:"business"`
	Priority   int    `json:"priority"`
	Attempts   int    `json:"attempts"`
	LeaseUntil string `json:"leaseUntil"`
	// TeamID owns the job (W16). NULL resolves to the default team.
	TeamID *int64 `json:"teamId"`
}

func scanJob(rows interface{ Scan(...any) error }) (*Job, error) {
	j := &Job{}
	var team sql.NullInt64
	err := rows.Scan(&j.ID, &j.Type, &j.RefID, &j.Status, &j.WorkerName, &j.Payload,
		&j.Result, &j.Error, &j.CreatedAt, &j.ClaimedAt, &j.FinishedAt,
		&j.AgentID, &j.Business, &j.Priority, &j.Attempts, &j.LeaseUntil, &team)
	if team.Valid {
		v := team.Int64
		j.TeamID = &v
	}
	return j, err
}

const jobCols = `id,type,ref_id,status,worker_name,payload,result,error,created_at,claimed_at,finished_at,agent_id,business,priority,attempts,lease_until,team_id`

// CreateJob enqueues a pending job. Routing columns inherit from the
// referenced product when it exists (all current flows reference products).
func (s *Store) CreateJob(jobType string, refID int64, payload string) (*Job, error) {
	return s.CreateJobFull(jobType, refID, payload, "", 0, true)
}

// CreateJobFull enqueues with explicit routing columns. When inheritProduct
// is set, empty business / zero priority fall back to the product row.
func (s *Store) CreateJobFull(jobType string, refID int64, payload, business string, priority int, inheritProduct bool) (*Job, error) {
	now := nowUTC()
	if inheritProduct && (business == "" || priority == 0) {
		if p, err := s.GetProduct(refID); err == nil {
			if business == "" {
				business = p.Business
			}
			if priority == 0 {
				priority = p.Priority
			}
		}
	}
	res, err := s.db.Exec(`INSERT INTO jobs(type,ref_id,status,worker_name,payload,created_at,business,priority)
		VALUES(?,?,'pending','',?,?,?,?)`, jobType, refID, payload, now, business, priority)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	job, err := s.GetJob(id)
	if err != nil {
		return nil, err
	}
	if OnJobCreated != nil {
		OnJobCreated()
	}
	return job, nil
}

// OnJobCreated fires (best effort, must not block) after any job insert.
// The server sets it to wake long-polling claim waiters (D26).
var OnJobCreated func()

// GetJob loads one job.
func (s *Store) GetJob(id int64) (*Job, error) {
	j, err := scanJob(s.ro.QueryRow(`SELECT `+jobCols+` FROM jobs WHERE id=?`, id))
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return j, err
}

// CountOpenJobs counts pending/claimed/done jobs of a type+ref (guard against
// double auto-creation when a worker retries after a mid-flight DB error).
func (s *Store) CountOpenJobs(jobType string, refID int64) (int, error) {
	var n int
	err := s.ro.QueryRow(`SELECT COUNT(*) FROM jobs WHERE type=? AND ref_id=? AND status IN ('pending','claimed','done')`,
		jobType, refID).Scan(&n)
	return n, err
}

// ClaimNextJob atomically assigns the oldest pending job of a type.
// Returns (nil, nil) when no job waits.
func (s *Store) ClaimNextJob(jobType, worker string) (*Job, error) {
	if !s.AutomationEnabled() {
		return nil, nil // W0: stopped automation claims nothing.
	}
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
	res, err := s.db.Exec(`UPDATE jobs SET status='done',result=?,error='',finished_at=? WHERE id=? AND status='claimed'`,
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
	rows, err := s.ro.Query(`SELECT ` + jobCols + ` FROM jobs ORDER BY id DESC LIMIT 100`)
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

// DeleteJob hard-deletes one job row.
func (s *Store) DeleteJob(id int64) error {
	res, err := s.db.Exec(`DELETE FROM jobs WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListJobsByRef returns a product's jobs (analyze + generate) oldest-first.
func (s *Store) ListJobsByRef(refID int64) ([]*Job, error) {
	rows, err := s.ro.Query(`SELECT `+jobCols+` FROM jobs WHERE ref_id=? ORDER BY id ASC`, refID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Job{}
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
	RevisionOf       int64  `json:"revisionOf"`
	RevisionRootID   int64  `json:"revisionRootId"`
	// GeneratedByAccountID is the linked AI account that wrote the draft
	// (W6 writer≠reviewer). NULL for legacy and unlinked-agent drafts.
	GeneratedByAccountID *int64 `json:"generatedByAccountId,omitempty"`
	// MediaSpec is the worker's media shopping list (W3-4, JSON).
	MediaSpec string `json:"mediaSpec,omitempty"`
	// WantPlatform pins the brief entry (W3-2); empty = first match.
	WantPlatform string `json:"wantPlatform,omitempty"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
	// Revisions is transport-only: chain siblings, set by the detail handler.
	Revisions []int64 `json:"revisions,omitempty"`
}

func scanContent(row interface{ Scan(...any) error }) (*Content, error) {
	c := &Content{}
	var sponsored, persona, held int
	var genBy sql.NullInt64
	err := row.Scan(&c.ID, &c.ProductID, &c.Channel, &c.TargetPlatform, &c.ChannelGroup,
		&c.Title, &c.Body, &c.Disclosure, &sponsored, &persona, &c.AILabel, &c.Status,
		&c.TargetAccount, &c.ScheduledAt, &c.ReserveSeq, &held, &c.HoldReason,
		&c.RevisionOf, &c.RevisionRootID, &genBy, &c.MediaSpec, &c.WantPlatform, &c.CreatedAt, &c.UpdatedAt)
	c.IsSponsored, c.IsVirtualPersona, c.Held = sponsored == 1, persona == 1, held == 1
	if genBy.Valid {
		v := genBy.Int64
		c.GeneratedByAccountID = &v
	}
	return c, err
}

const contentCols = `id,product_id,channel,target_platform,channel_group,title,body,disclosure,is_sponsored,is_virtual_persona,ai_label,status,target_account,scheduled_at,reserve_seq,held,hold_reason,revision_of,revision_root_id,generated_by_account_id,media_spec,want_platform,created_at,updated_at`

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// CreateContent inserts a draft content item. A zero RevisionRootID makes
// the new row its own chain root; otherwise the caller-supplied chain
// (RevisionOf/RevisionRootID) is kept.
func (s *Store) CreateContent(c *Content) (*Content, error) {
	now := nowUTC()
	var genBy any
	if c.GeneratedByAccountID != nil {
		genBy = *c.GeneratedByAccountID
	}
	res, err := s.db.Exec(`INSERT INTO contents(product_id,channel,target_platform,channel_group,title,body,
		disclosure,is_sponsored,is_virtual_persona,ai_label,status,target_account,scheduled_at,held,hold_reason,revision_of,revision_root_id,generated_by_account_id,media_spec,want_platform,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, c.ProductID, c.Channel, "", "", c.Title, c.Body,
		c.Disclosure, boolInt(c.IsSponsored), boolInt(c.IsVirtualPersona), c.AILabel, "draft", "", "", 0, "",
		c.RevisionOf, c.RevisionRootID, genBy, c.MediaSpec, c.WantPlatform, now, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	if c.RevisionRootID == 0 {
		if _, err := s.db.Exec(`UPDATE contents SET revision_root_id=? WHERE id=?`, id, id); err != nil {
			return nil, err
		}
	}
	return s.GetContent(id)
}

// GetContent loads one content item.
func (s *Store) GetContent(id int64) (*Content, error) {
	c, err := scanContent(s.ro.QueryRow(`SELECT `+contentCols+` FROM contents WHERE id=?`, id))
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return c, err
}

// ListContents filters by status and business ("" = all), newest-first.
func (s *Store) ListContents(status, business string, limit int) ([]*Content, error) {
	q := `SELECT c.` + strings.ReplaceAll(contentCols, ",", ",c.") + ` FROM contents c JOIN products p ON p.id=c.product_id`
	var conds []string
	var args []any
	if status != "" {
		conds = append(conds, `c.status=?`)
		args = append(args, status)
	}
	if business != "" {
		conds = append(conds, `p.business=? COLLATE NOCASE`)
		args = append(args, business)
	}
	if len(conds) > 0 {
		q += ` WHERE ` + strings.Join(conds, " AND ")
	}
	q += ` ORDER BY c.id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.ro.Query(q, args...)
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
	rows, err := s.ro.Query(`SELECT `+contentCols+` FROM contents
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

// RecentPublished lists same-platform published bodies since a cutoff
// (RFC3339) for the W18-1 similarity corpus, newest first.
func (s *Store) RecentPublished(platform, since string) ([]*Content, error) {
	rows, err := s.ro.Query(`SELECT `+contentCols+` FROM contents
		WHERE status='published' AND target_platform=? AND updated_at>=?
		ORDER BY updated_at DESC, id DESC`, platform, since)
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

// SetContentHeld holds or releases one content item.
func (s *Store) SetContentHeld(id int64, held bool) error {
	flag := 0
	if held {
		flag = 1
	}
	_, err := s.db.Exec(`UPDATE contents SET held=?, updated_at=? WHERE id=?`, flag, nowUTC(), id)
	return err
}

// ScheduleContent sets the publish reservation of a queued item and stamps it
// with a fresh reservation sequence so same-account/same-time ties execute in
// the order reservations were loaded onto the queue (짠9).
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
	err := s.ro.QueryRow(`SELECT COUNT(*) FROM contents WHERE target_account=? AND scheduled_at=?
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

// CountQueuedByAccount counts an account's queued items, held or not.
func (s *Store) CountQueuedByAccount(account string) (int, error) {
	var n int
	err := s.ro.QueryRow(`SELECT COUNT(*) FROM contents
		WHERE target_account=? AND status='queued'`, account).Scan(&n)
	return n, err
}

// ListHeldByAccount returns held queued items of one account.
func (s *Store) ListHeldByAccount(account string) ([]*Content, error) {
	rows, err := s.ro.Query(`SELECT `+contentCols+` FROM contents
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

// ReviewDecision is one recorded review judgment (human or chief agent).
type ReviewDecision struct {
	ID        int64  `json:"id"`
	ContentID int64  `json:"contentId"`
	Decider   string `json:"decider"`
	Decision  string `json:"decision"`
	Reason    string `json:"reason"`
	CreatedAt string `json:"createdAt"`
}

// CreateReviewDecision appends one judgment. Decision must be approved/rejected.
func (s *Store) CreateReviewDecision(contentID int64, decider, decision, reason string) (*ReviewDecision, error) {
	now := nowUTC()
	res, err := s.db.Exec(`INSERT INTO review_decisions(content_id,decider,decision,reason,created_at)
		VALUES(?,?,?,?,?)`, contentID, decider, decision, reason, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &ReviewDecision{ID: id, ContentID: contentID, Decider: decider,
		Decision: decision, Reason: reason, CreatedAt: now}, nil
}

// CountRejected returns how many times a content was rejected.
func (s *Store) CountRejected(contentID int64) (int, error) {
	var n int
	err := s.ro.QueryRow(`SELECT COUNT(*) FROM review_decisions
		WHERE content_id=? AND decision='rejected'`, contentID).Scan(&n)
	return n, err
}

// chainRoot resolves a content's revision root (self when unlinked).
func (s *Store) chainRoot(contentID int64) (int64, error) {
	c, err := s.GetContent(contentID)
	if err != nil {
		return 0, err
	}
	if c.RevisionRootID == 0 {
		return c.ID, nil
	}
	return c.RevisionRootID, nil
}

// CountRejectedChain counts rejected judgments across the whole revision
// chain (D21): every content sharing the chain root.
func (s *Store) CountRejectedChain(contentID int64) (int, error) {
	root, err := s.chainRoot(contentID)
	if err != nil {
		return 0, err
	}
	var n int
	err = s.ro.QueryRow(`SELECT COUNT(*) FROM review_decisions
		WHERE decision='rejected' AND content_id IN
		(SELECT id FROM contents WHERE revision_root_id=?)`, root).Scan(&n)
	return n, err
}

// ListRevisionIDs returns the chain's content IDs oldest-first, excluding
// the chain head itself (for the "revised #N" links).
func (s *Store) ListRevisionIDs(contentID int64) ([]int64, error) {
	root, err := s.chainRoot(contentID)
	if err != nil {
		return nil, err
	}
	rows, err := s.ro.Query(`SELECT id FROM contents WHERE revision_root_id=? AND id != ? ORDER BY id`, root, root)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// ListReviewDecisions returns one content's judgments newest-first.
func (s *Store) ListReviewDecisions(contentID int64) ([]*ReviewDecision, error) {
	rows, err := s.ro.Query(`SELECT id,content_id,decider,decision,reason,created_at
		FROM review_decisions WHERE content_id=? ORDER BY id DESC`, contentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*ReviewDecision{}
	for rows.Next() {
		d := &ReviewDecision{}
		if err := rows.Scan(&d.ID, &d.ContentID, &d.Decider, &d.Decision, &d.Reason, &d.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// DeleteContent hard-deletes one content item. Its publish logs and metrics
// fall via FK cascade. Missing ids return ErrNotFound.
func (s *Store) DeleteContent(id int64) error {
	res, err := s.db.Exec(`DELETE FROM contents WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// Publish logs (duplicate-publish guard structure)
// ---------------------------------------------------------------------------

// PublishLog records a publish plan/completion per content+channel+account.
// The UNIQUE(content_id, channel, account) constraint is the duplicate guard.
type PublishLog struct {
	ID             int64  `json:"id"`
	ContentID      int64  `json:"contentId"`
	Channel        string `json:"channel"`
	Account        string `json:"account"`
	Status         string `json:"status"`
	ExecutedAt     string `json:"executedAt"`
	Note           string `json:"note"`
	CreatedAt      string `json:"createdAt"`
	Attempts       int    `json:"attempts"`
	LastError      string `json:"lastError"`
	NextEligibleAt string `json:"nextEligibleAt"`
}

const publishLogCols = `id,content_id,channel,account,status,executed_at,note,created_at,attempts,last_error,next_eligible_at`

func scanPublishLog(row interface{ Scan(...any) error }) (*PublishLog, error) {
	l := &PublishLog{}
	err := row.Scan(&l.ID, &l.ContentID, &l.Channel, &l.Account, &l.Status,
		&l.ExecutedAt, &l.Note, &l.CreatedAt, &l.Attempts, &l.LastError, &l.NextEligibleAt)
	return l, err
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
	l, err := scanPublishLog(s.ro.QueryRow(`SELECT `+publishLogCols+`
		FROM publish_logs WHERE content_id=? AND channel=? AND account=?`, contentID, channel, account))
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return l, err
}

// CreatePublishDone inserts a completion record with execution time.
// Duplicate triplets return ErrDuplicate.
func (s *Store) CreatePublishDone(contentID int64, channel, account, executedAt, note string) (*PublishLog, error) {
	now := nowUTC()
	_, err := s.db.Exec(`INSERT INTO publish_logs(content_id,channel,account,status,executed_at,note,created_at)
		VALUES(?,?,?,'done',?,?,?)`, contentID, channel, account, executedAt, note, now)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDuplicate
		}
		return nil, err
	}
	return s.GetPublishLog(contentID, channel, account)
}

// CompletePublishLog flips one triplet record to done with execution evidence.
func (s *Store) CompletePublishLog(contentID int64, channel, account, executedAt, note string) (*PublishLog, error) {
	res, err := s.db.Exec(`UPDATE publish_logs SET status='done', executed_at=?, note=?
		WHERE content_id=? AND channel=? AND account=?`,
		executedAt, note, contentID, channel, account)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, ErrNotFound
	}
	return s.GetPublishLog(contentID, channel, account)
}

// GetPublishLogByID loads one record by id.
func (s *Store) GetPublishLogByID(id int64) (*PublishLog, error) {
	l := &PublishLog{}
	err := s.ro.QueryRow(`SELECT id,content_id,channel,account,status,executed_at,note,created_at
		FROM publish_logs WHERE id=?`, id).
		Scan(&l.ID, &l.ContentID, &l.Channel, &l.Account, &l.Status, &l.ExecutedAt, &l.Note, &l.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return l, err
}

// UpdatePublishLog rewrites a plan record's status and note.
func (s *Store) UpdatePublishLog(id int64, status, note string) error {
	res, err := s.db.Exec(`UPDATE publish_logs SET status=?, note=? WHERE id=?`, status, note, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeletePublishLog removes one plan record.
func (s *Store) DeletePublishLog(id int64) error {
	res, err := s.db.Exec(`DELETE FROM publish_logs WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListPublishLogs returns newest-first records.
func (s *Store) ListPublishLogs(limit int) ([]*PublishLog, error) {
	rows, err := s.ro.Query(`SELECT `+publishLogCols+` FROM publish_logs ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*PublishLog{}
	for rows.Next() {
		l, err := scanPublishLog(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// MarkLogAttempt records one failed executor attempt.
func (s *Store) MarkLogAttempt(id int64, attempts int, lastError, nextEligibleAt string) error {
	_, err := s.db.Exec(`UPDATE publish_logs SET attempts=?,last_error=?,next_eligible_at=? WHERE id=?`,
		attempts, lastError, nextEligibleAt, id)
	return err
}

// MarkLogDone closes a record as done with its execution time and URL note.
func (s *Store) MarkLogDone(id int64, executedAt, note string) error {
	_, err := s.db.Exec(`UPDATE publish_logs SET status='done',executed_at=?,note=? WHERE id=?`,
		executedAt, note, id)
	return err
}

// MarkLogFailed closes a record as failed, keeping the final attempt count.
func (s *Store) MarkLogFailed(id int64, attempts int, lastError string) error {
	_, err := s.db.Exec(`UPDATE publish_logs SET status='failed',attempts=?,last_error=? WHERE id=?`,
		attempts, lastError, id)
	return err
}

// LastDoneAt returns the newest done execution time for an account ("" if none).
func (s *Store) LastDoneAt(account string) (string, error) {
	var at sql.NullString
	err := s.ro.QueryRow(`SELECT MAX(executed_at) FROM publish_logs WHERE account=? AND status='done'`, account).Scan(&at)
	if err != nil {
		return "", err
	}
	if !at.Valid {
		return "", nil
	}
	return at.String, nil
}

// CountAccountDoneSince counts done executions for an account at or after
// sinceUTC (RFC3339). The W4 scheduler uses it for daily-cap headroom.
func (s *Store) CountAccountDoneSince(account, sinceUTC string) (int, error) {
	var n int
	err := s.ro.QueryRow(`SELECT COUNT(*) FROM publish_logs
		WHERE account=? AND status='done' AND executed_at>=?`, account, sinceUTC).Scan(&n)
	return n, err
}

// CountAccountQueued counts an account's waiting (queued, unheld) items.
func (s *Store) CountAccountQueued(account string) (int, error) {
	var n int
	err := s.ro.QueryRow(`SELECT COUNT(*) FROM contents
		WHERE target_account=? AND status='queued' AND held=0`, account).Scan(&n)
	return n, err
}

// CountProductPlatformQueued counts one work+platform's waiting items for
// the W21-4 distribution cap.
func (s *Store) CountProductPlatformQueued(productID int64, platform string) (int, error) {
	var n int
	err := s.ro.QueryRow(`SELECT COUNT(*) FROM contents
		WHERE product_id=? AND target_platform=? AND status='queued' AND held=0`,
		productID, platform).Scan(&n)
	return n, err
}

// MarkContentPublished flips one queued item to published (executor path).
func (s *Store) MarkContentPublished(id int64) error {
	res, err := s.db.Exec(`UPDATE contents SET status='published',updated_at=? WHERE id=? AND status='queued'`, nowUTC(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return ErrNotFound
	}
	return nil
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
	m, err := scanMetric(s.ro.QueryRow(`SELECT `+metricCols+` FROM metrics WHERE id=?`, id))
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return m, err
}

// DeleteMetric removes one metric record.
func (s *Store) DeleteMetric(id int64) error {
	res, err := s.db.Exec(`DELETE FROM metrics WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListMetrics returns newest-first records.
func (s *Store) ListMetrics(limit int) ([]*Metric, error) {
	rows, err := s.ro.Query(`SELECT `+metricCols+` FROM metrics ORDER BY id DESC LIMIT ?`, limit)
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
// Affiliate links (exit B). No commission-rate column by design (짠6: unconfirmed).
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
	rows, err := s.ro.Query(`SELECT id,product_id,partner_name,url,created_at FROM affiliate_links
		WHERE product_id=? ORDER BY id ASC`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*AffiliateLink{}
	for rows.Next() {
		l := &AffiliateLink{}
		if err := rows.Scan(&l.ID, &l.ProductID, &l.PartnerName, &l.URL, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// GetAffiliateLink loads one link by id.
func (s *Store) GetAffiliateLink(id int64) (*AffiliateLink, error) {
	l := &AffiliateLink{}
	err := s.ro.QueryRow(`SELECT id,product_id,partner_name,url,created_at FROM affiliate_links
		WHERE id=?`, id).Scan(&l.ID, &l.ProductID, &l.PartnerName, &l.URL, &l.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return l, err
}

// UpdateAffiliateLink rewrites one link's partner name and URL.
func (s *Store) UpdateAffiliateLink(id int64, partner, url string) error {
	res, err := s.db.Exec(`UPDATE affiliate_links SET partner_name=?, url=? WHERE id=?`, partner, url, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteAffiliateLink hard-deletes one link. Missing ids return ErrNotFound.
func (s *Store) DeleteAffiliateLink(id int64) error {
	res, err := s.db.Exec(`DELETE FROM affiliate_links WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// Dashboard counters
// ---------------------------------------------------------------------------

// ChannelAccount is the persisted channel-agent account row. EncID/EncPW hold
// ciphertext; plaintext never reaches this layer.
type ChannelAccount struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Platform      string `json:"platform"`
	Owner         string `json:"owner"`
	Role          string `json:"role"`
	Business      string `json:"business"`
	SessionState  string `json:"sessionState"`
	PostingMode   string `json:"postingMode"`
	EncID         string `json:"-"`
	EncPW         string `json:"-"`
	LastCheckedAt string `json:"lastCheckedAt"`
	LastWakeAt    string `json:"lastWakeAt"`
	Usage         string `json:"usage"`
	LastJobAt     string `json:"lastJobAt"`
	DailyCap      int    `json:"dailyCap"`
	Kind         string `json:"kind"`
	PlatformUser string `json:"platformUserId"`
	// EncAPIToken/ApiModel are v11 residue. The API-key chat path was
	// retired 2026-11-14 (paid billing); columns stay for existing DBs
	// but nothing reads them for sending.
	EncAPIToken string `json:"-"`
	ApiModel    string `json:"-"`
	// PlanTier is the user-picked plan label (W15-9); Capabilities is a JSON
	// array of proven features (text/image/video/file). ProfileMode tracks
	// the login browser flavor (incognito, or real for the single account
	// allowed on the operator's own Chrome profile).
	PlanTier     string `json:"planTier"`
	Capabilities string `json:"capabilities"`
	ProfileMode  string `json:"profileMode"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

const accountCols = `id,name,platform,owner,role,business,session_state,posting_mode,enc_id,enc_pw,last_checked_at,last_wake_at,usage,last_job_at,daily_cap,kind,platform_user_id,enc_api_token,api_model,plan_tier,capabilities,profile_mode,created_at,updated_at`

func scanAccount(row interface{ Scan(...any) error }) (*ChannelAccount, error) {
	a := &ChannelAccount{}
	err := row.Scan(&a.ID, &a.Name, &a.Platform, &a.Owner, &a.Role, &a.Business,
		&a.SessionState, &a.PostingMode, &a.EncID, &a.EncPW, &a.LastCheckedAt,
		&a.LastWakeAt, &a.Usage, &a.LastJobAt, &a.DailyCap, &a.Kind, &a.PlatformUser,
		&a.EncAPIToken, &a.ApiModel, &a.PlanTier, &a.Capabilities, &a.ProfileMode,
		&a.CreatedAt, &a.UpdatedAt)
	return a, err
}

// CreateAccountRow inserts one account. Duplicate names return ErrDuplicate.
func (s *Store) CreateAccountRow(a *ChannelAccount) (*ChannelAccount, error) {
	now := nowUTC()
	res, err := s.db.Exec(`INSERT INTO channel_accounts(name,platform,owner,role,business,
		session_state,posting_mode,enc_id,enc_pw,last_checked_at,last_wake_at,usage,last_job_at,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, a.Name, a.Platform, a.Owner, a.Role, a.Business,
		a.SessionState, a.PostingMode, a.EncID, a.EncPW, a.LastCheckedAt, a.LastWakeAt,
		a.Usage, a.LastJobAt, now, now)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDuplicate
		}
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetAccountRow(id)
}

// GetAccountRow loads one account by id.
func (s *Store) GetAccountRow(id int64) (*ChannelAccount, error) {
	a, err := scanAccount(s.ro.QueryRow(`SELECT `+accountCols+` FROM channel_accounts WHERE id=?`, id))
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return a, err
}

// GetAccountRowByName loads one account by its unique name.
func (s *Store) GetAccountRowByName(name string) (*ChannelAccount, error) {
	a, err := scanAccount(s.ro.QueryRow(`SELECT `+accountCols+` FROM channel_accounts WHERE name=?`, name))
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return a, err
}

// ListAccountRows returns all accounts by id.
func (s *Store) ListAccountRows() ([]*ChannelAccount, error) {
	rows, err := s.ro.Query(`SELECT ` + accountCols + ` FROM channel_accounts ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*ChannelAccount{}
	for rows.Next() {
		a, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// UpdateAccountRow rewrites one account row by id.
func (s *Store) UpdateAccountRow(a *ChannelAccount) error {
	res, err := s.db.Exec(`UPDATE channel_accounts SET platform=?,owner=?,role=?,business=?,
		session_state=?,posting_mode=?,enc_id=?,enc_pw=?,last_checked_at=?,last_wake_at=?,usage=?,last_job_at=?,daily_cap=?,kind=?,platform_user_id=?,enc_api_token=?,api_model=?,plan_tier=?,capabilities=?,profile_mode=?,updated_at=?
		WHERE id=?`, a.Platform, a.Owner, a.Role, a.Business, a.SessionState, a.PostingMode,
		a.EncID, a.EncPW, a.LastCheckedAt, a.LastWakeAt, a.Usage, a.LastJobAt, a.DailyCap, a.Kind, a.PlatformUser,
		a.EncAPIToken, a.ApiModel, a.PlanTier, a.Capabilities, a.ProfileMode, nowUTC(), a.ID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteAccountRow removes one account row.
func (s *Store) DeleteAccountRow(id int64) error {
	res, err := s.db.Exec(`DELETE FROM channel_accounts WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListScheduledRange returns queued items with scheduled_at in [fromUTC, toUTC),
// in execution order. Bounds are RFC3339 UTC; callers convert the local day.
func (s *Store) ListScheduledRange(fromUTC, toUTC string) ([]*Content, error) {
	rows, err := s.ro.Query(`SELECT `+contentCols+` FROM contents
		WHERE status='queued' AND scheduled_at>=? AND scheduled_at<?
		ORDER BY scheduled_at ASC, reserve_seq ASC, id ASC`, fromUTC, toUTC)
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

// BusinessCounts is one business line's dashboard summary.
type BusinessCounts struct {
	Business string `json:"business"`
	Products int    `json:"products"`
	Queued   int    `json:"queued"`
	Review   int    `json:"review"`
}

// DashboardByBusiness aggregates counts per business line, ??last.
// Grouping is case-insensitive; the displayed spelling is the group minimum.
func (s *Store) DashboardByBusiness() ([]*BusinessCounts, error) {
	rows, err := s.ro.Query(`SELECT MIN(p.business), COUNT(DISTINCT p.id),
		COALESCE(SUM(CASE WHEN c.status='queued' THEN 1 ELSE 0 END),0),
		COALESCE(SUM(CASE WHEN c.status='review' THEN 1 ELSE 0 END),0)
		FROM products p LEFT JOIN contents c ON c.product_id=p.id
		GROUP BY p.business COLLATE NOCASE ORDER BY (p.business=''), p.business ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*BusinessCounts{}
	for rows.Next() {
		b := &BusinessCounts{}
		if err := rows.Scan(&b.Business, &b.Products, &b.Queued, &b.Review); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// Counts is the dashboard summary.
type Counts struct {
	ReviewWaiting  int `json:"reviewWaiting"`
	JobsPending    int `json:"jobsPending"`
	JobsClaimed    int `json:"jobsClaimed"`
	JobsFailed     int `json:"jobsFailed"`
	JobsStuck      int `json:"jobsStuck"`
	QueuedTotal    int `json:"queuedTotal"`
	HeldTotal      int `json:"heldTotal"`
	ScheduledToday int `json:"scheduledToday"`
	ExecutorDue    int `json:"executorDue"`
}

// StuckClaimedMin is how long a claim may run before it counts as stuck.
const StuckClaimedMin = 30

// DashboardCounts aggregates the home-view summary in fixed queries.
// The day window is [fromUTC, toUTC) in RFC3339 UTC, converted by the caller.
func (s *Store) DashboardCounts(fromUTC, toUTC string) (*Counts, error) {
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
		{&c.ScheduledToday, `SELECT COUNT(*) FROM contents WHERE status='queued' AND scheduled_at>=? AND scheduled_at<?`, []any{fromUTC, toUTC}},
		{&c.ExecutorDue, `SELECT COUNT(*) FROM contents c JOIN channel_accounts a ON a.name=c.target_account
			WHERE c.status='queued' AND c.channel_group='official_api' AND c.held=0
			AND a.session_state='connected' AND a.posting_mode='auto'
			AND (c.scheduled_at='' OR c.scheduled_at<?)`, []any{toUTC}},
	}
	for _, item := range queries {
		if err := s.ro.QueryRow(item.q, item.args...).Scan(item.dest); err != nil {
			return nil, err
		}
	}
	stuck, err := s.ListStuckClaimed(StuckClaimedMin)
	if err != nil {
		return nil, err
	}
	c.JobsStuck = len(stuck)
	return c, nil
}

// ListStuckClaimed returns claimed jobs older than the given minutes.
// Timestamps parse in Go so no SQLite date quirks leak into the rule.
func (s *Store) ListStuckClaimed(olderThanMin int) ([]*Job, error) {
	rows, err := s.ro.Query(`SELECT ` + jobCols + ` FROM jobs WHERE status='claimed'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cutoff := time.Now().Add(-time.Duration(olderThanMin) * time.Minute)
	var out []*Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		t, err := time.Parse(time.RFC3339, j.ClaimedAt)
		if err != nil || !t.Before(cutoff) {
			continue
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// DeleteJobsBefore purges finished jobs older than the RFC3339 cutoff.
// Pending and claimed jobs are never touched. Returns rows removed.
func (s *Store) DeleteJobsBefore(cutoffRFC3339 string) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM jobs WHERE status IN ('done','failed')
		AND finished_at != '' AND finished_at < ?`, cutoffRFC3339)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DeleteMetricsBefore purges metric records older than the RFC3339 cutoff.
func (s *Store) DeleteMetricsBefore(cutoffRFC3339 string) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM metrics WHERE created_at < ?`, cutoffRFC3339)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// PurgeExpired removes finished jobs older than jobsDays, metrics older
// than metricsDays, and read notifications older than notifDays.
// Windows are whole UTC days back from now.
func PurgeExpired(s *Store, jobsDays, metricsDays, notifDays int) (jobs, metrics, notifs int64, err error) {
	now := time.Now().UTC()
	jobs, err = s.DeleteJobsBefore(now.AddDate(0, 0, -jobsDays).Format(time.RFC3339))
	if err != nil {
		return 0, 0, 0, err
	}
	metrics, err = s.DeleteMetricsBefore(now.AddDate(0, 0, -metricsDays).Format(time.RFC3339))
	if err != nil {
		return jobs, 0, 0, err
	}
	notifs, err = s.DeleteReadNotificationsBefore(now.AddDate(0, 0, -notifDays).Format(time.RFC3339))
	if err != nil {
		return jobs, metrics, 0, err
	}
	return jobs, metrics, notifs, nil
}

// Notification is one local-outbox row (D30).
type Notification struct {
	ID        int64  `json:"id"`
	Kind      string `json:"kind"`
	Severity  string `json:"severity"`
	RefType   string `json:"refType"`
	RefID     int64  `json:"refId"`
	DedupKey  string `json:"dedupKey"`
	Message   string `json:"message"`
	CreatedAt string `json:"createdAt"`
	ReadAt    string `json:"readAt"`
}

// CreateNotificationDedup inserts unless the same unread dedup key exists.
// It reports whether a row was created. Callers serialize on one mutex
// (single process), so check-then-insert cannot race itself.
func (s *Store) CreateNotificationDedup(kind, severity, refType string, refID int64, dedupKey, message string) (bool, error) {
	var n int
	if err := s.ro.QueryRow(`SELECT COUNT(*) FROM notifications WHERE dedup_key=? AND read_at=''`, dedupKey).Scan(&n); err != nil {
		return false, err
	}
	if n > 0 {
		return false, nil
	}
	_, err := s.db.Exec(`INSERT INTO notifications(kind,severity,ref_type,ref_id,dedup_key,message,created_at,read_at)
		VALUES(?,?,?,?,?,?,?,?)`, kind, severity, refType, refID, dedupKey, message, nowUTC(), "")
	if err != nil {
		return false, err
	}
	return true, nil
}

// ListNotifications returns newest-first rows with paging. limit<=0 means 50.
func (s *Store) ListNotifications(unreadOnly bool, limit, offset int) ([]*Notification, error) {
	if limit <= 0 {
		limit = 50
	}
	q := `SELECT id,kind,severity,ref_type,ref_id,dedup_key,message,created_at,read_at FROM notifications`
	if unreadOnly {
		q += ` WHERE read_at=''`
	}
	q += ` ORDER BY id DESC LIMIT ? OFFSET ?`
	rows, err := s.ro.Query(q, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Notification{}
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.Kind, &n.Severity, &n.RefType, &n.RefID, &n.DedupKey, &n.Message, &n.CreatedAt, &n.ReadAt); err != nil {
			return nil, err
		}
		out = append(out, &n)
	}
	return out, nil
}

// CountUnreadNotifications counts unread outbox rows.
func (s *Store) CountUnreadNotifications() (int, error) {
	var n int
	if err := s.ro.QueryRow(`SELECT COUNT(*) FROM notifications WHERE read_at=''`).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// MarkNotificationRead marks one row read, reporting whether it existed.
func (s *Store) MarkNotificationRead(id int64) (bool, error) {
	var n int
	if err := s.ro.QueryRow(`SELECT COUNT(*) FROM notifications WHERE id=?`, id).Scan(&n); err != nil {
		return false, err
	}
	if n == 0 {
		return false, nil
	}
	_, err := s.db.Exec(`UPDATE notifications SET read_at=? WHERE id=? AND read_at=''`, nowUTC(), id)
	return true, err
}

// MarkAllNotificationsRead marks every unread row read and reports how many.
func (s *Store) MarkAllNotificationsRead() (int64, error) {
	res, err := s.db.Exec(`UPDATE notifications SET read_at=? WHERE read_at=''`, nowUTC())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DeleteReadNotificationsBefore purges read rows older than the cutoff.
func (s *Store) DeleteReadNotificationsBefore(cutoffRFC3339 string) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM notifications WHERE read_at != '' AND read_at < ?`, cutoffRFC3339)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// OverdueHold is a held queue item older than the cutoff.
type OverdueHold struct {
	ID      int64
	Account string
}

// ListOverdueHolds finds held queued items held longer than the cutoff.
func (s *Store) ListOverdueHolds(cutoffRFC3339 string) ([]*OverdueHold, error) {
	rows, err := s.ro.Query(`SELECT id,target_account FROM contents WHERE held=1 AND status='queued' AND updated_at != '' AND updated_at < ? ORDER BY id`, cutoffRFC3339)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*OverdueHold
	for rows.Next() {
		var h OverdueHold
		if err := rows.Scan(&h.ID, &h.Account); err != nil {
			return nil, err
		}
		out = append(out, &h)
	}
	return out, nil
}
