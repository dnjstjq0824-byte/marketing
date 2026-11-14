package store

import (
	"database/sql"
	"errors"
	"strings"
)

// ErrAttached refuses media deletion while linked to a work.
var ErrAttached = errors.New("media is attached to a work")

// Media library rows (S2/S4 chunked uploads; W14 grows kinds).

// Media is one stored file. ProductID is nil until attached to a work.
type Media struct {
	ID        int64  `json:"id"`
	ProductID *int64 `json:"productId"`
	Kind      string `json:"kind"`
	Path      string `json:"path"`
	Bytes     int64  `json:"bytes"`
	Mime      string `json:"mime"`
	SHA256    string `json:"sha256"`
	CreatedAt string `json:"createdAt"`
}

const mediaCols = `id,product_id,kind,path,bytes,mime,sha256,created_at`

func scanMedia(row interface{ Scan(...any) error }) (*Media, error) {
	m := &Media{}
	var productID sql.NullInt64
	err := row.Scan(&m.ID, &productID, &m.Kind, &m.Path, &m.Bytes, &m.Mime, &m.SHA256, &m.CreatedAt)
	if productID.Valid {
		v := productID.Int64
		m.ProductID = &v
	}
	return m, err
}

// CreateMedia inserts one completed upload, unattached.
func (s *Store) CreateMedia(kind, path string, bytes int64, mime, sha string) (*Media, error) {
	now := nowUTC()
	res, err := s.db.Exec(`INSERT INTO media(product_id,kind,path,bytes,mime,sha256,created_at)
		VALUES(NULL,?,?,?,?,?,?)`, kind, path, bytes, mime, sha, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetMedia(id)
}

// GetMedia loads one row by id.
func (s *Store) GetMedia(id int64) (*Media, error) {
	m, err := scanMedia(s.ro.QueryRow(`SELECT `+mediaCols+` FROM media WHERE id=?`, id))
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return m, err
}

// MediaByHash finds a stored file by content hash (upload dedupe).
func (s *Store) MediaByHash(sha string) (*Media, error) {
	m, err := scanMedia(s.ro.QueryRow(`SELECT `+mediaCols+` FROM media WHERE sha256=? LIMIT 1`, sha))
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return m, err
}

// AttachMedia links unattached uploads to a product. Already-attached rows
// are rejected so one file never double-counts.
func (s *Store) AttachMedia(ids []int64, productID int64) error {
	for _, id := range ids {
		m, err := s.GetMedia(id)
		if err != nil {
			return err
		}
		if m.ProductID != nil {
			return ErrDuplicate
		}
		if _, err := s.db.Exec(`UPDATE media SET product_id=? WHERE id=?`, productID, id); err != nil {
			return err
		}
	}
	return nil
}

// ListMedia returns newest-first rows, optionally filtered by product.
func (s *Store) ListMedia(productID int64, limit int) ([]*Media, error) {
	return s.ListMediaQuery("", productID, "", limit, 0)
}

// ListMediaQuery filters by kind, product, and business (via products join).
func (s *Store) ListMediaQuery(kind string, productID int64, business string, limit, offset int) ([]*Media, error) {
	q := `SELECT m.` + strings.Join(strings.Split(mediaCols, ","), ",m.") + ` FROM media m LEFT JOIN products p ON p.id=m.product_id`
	var conds []string
	var args []any
	if kind != "" {
		conds = append(conds, `m.kind=?`)
		args = append(args, kind)
	}
	if productID > 0 {
		conds = append(conds, `m.product_id=?`)
		args = append(args, productID)
	}
	if business != "" {
		conds = append(conds, `p.business=? COLLATE NOCASE`)
		args = append(args, business)
	}
	if len(conds) > 0 {
		q += ` WHERE ` + strings.Join(conds, " AND ")
	}
	q += ` ORDER BY m.id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := s.ro.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Media{}
	for rows.Next() {
		m, err := scanMedia(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// DeleteMedia removes one unattached row. Attached files are refused.
func (s *Store) DeleteMedia(id int64) (*Media, error) {
	m, err := s.GetMedia(id)
	if err != nil {
		return nil, err
	}
	if m.ProductID != nil {
		return nil, ErrAttached
	}
	if _, err := s.db.Exec(`DELETE FROM media WHERE id=?`, id); err != nil {
		return nil, err
	}
	return m, nil
}
