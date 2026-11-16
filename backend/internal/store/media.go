package store

import (
	"database/sql"
	"errors"
	"strings"
)

// ErrAttached refuses media deletion while linked to a work.
var ErrAttached = errors.New("media is attached to a work")

// Media kinds: operator uploads plus W14 generated files.
const (
	MediaKindUpload   = "upload"
	MediaKindGenImage = "gen_image"
	MediaKindGenVideo = "gen_video"
	MediaKindCard     = "card"
)

// MediaSource values record file rights (W18): where the bytes came from.
// Empty on legacy rows means untracked (treated as an original upload,
// never excluded).
const (
	MediaSourceOriginal  = "original"
	MediaSourceGenerated = "generated"
)

// MaxMediaPerInput bounds attachments per work (W14-1).
const MaxMediaPerInput = 20

// Media library rows (S2/S4 chunked uploads; W14 grows kinds).

// Media is one stored file. ProductID is nil until attached to a work;
// ContentID is nil until linked to a post. Prompt/AIAccountID record the
// generation behind gen_image/gen_video/card rows.
type Media struct {
	ID          int64   `json:"id"`
	ProductID   *int64  `json:"productId"`
	ContentID   *int64  `json:"contentId,omitempty"`
	Kind        string  `json:"kind"`
	Path        string  `json:"path"`
	ThumbPath   string  `json:"thumbPath,omitempty"`
	Width       int     `json:"width,omitempty"`
	Height      int     `json:"height,omitempty"`
	DurationSec float64 `json:"durationSec,omitempty"`
	Bytes       int64   `json:"bytes"`
	Mime        string  `json:"mime"`
	SHA256      string  `json:"sha256"`
	Prompt      string  `json:"prompt,omitempty"`
	AIAccountID *int64  `json:"aiAccountId,omitempty"`
	Source      string  `json:"source,omitempty"`
	CreatedAt   string  `json:"createdAt"`
}

const mediaCols = `id,product_id,content_id,kind,path,thumb_path,width,height,duration_sec,bytes,mime,sha256,prompt,ai_account_id,source,created_at`

func scanMedia(row interface{ Scan(...any) error }) (*Media, error) {
	m := &Media{}
	var productID, contentID, aiAccountID sql.NullInt64
	err := row.Scan(&m.ID, &productID, &contentID, &m.Kind, &m.Path, &m.ThumbPath,
		&m.Width, &m.Height, &m.DurationSec, &m.Bytes, &m.Mime, &m.SHA256,
		&m.Prompt, &aiAccountID, &m.Source, &m.CreatedAt)
	if productID.Valid {
		v := productID.Int64
		m.ProductID = &v
	}
	if contentID.Valid {
		v := contentID.Int64
		m.ContentID = &v
	}
	if aiAccountID.Valid {
		v := aiAccountID.Int64
		m.AIAccountID = &v
	}
	return m, err
}

// CreateMedia inserts one completed upload, unattached.
func (s *Store) CreateMedia(kind, path string, bytes int64, mime, sha string) (*Media, error) {
	return s.CreateMediaFull(MediaAttrs{Kind: kind, Path: path, Bytes: bytes, Mime: mime, SHA256: sha})
}

// MediaAttrs carries the full W14 media row for generated files.
type MediaAttrs struct {
	ProductID   *int64
	ContentID   *int64
	Kind        string
	Path        string
	ThumbPath   string
	Width       int
	Height      int
	DurationSec float64
	Bytes       int64
	Mime        string
	SHA256      string
	Prompt      string
	AIAccountID *int64
	Source      string
}

// CreateMediaFull inserts one media row with generation tracking. Unknown
// kinds are refused; textures of the row stay the caller's duty.
func (s *Store) CreateMediaFull(a MediaAttrs) (*Media, error) {
	switch a.Kind {
	case MediaKindUpload, MediaKindGenImage, MediaKindGenVideo, MediaKindCard:
	default:
		return nil, errors.New("unknown media kind")
	}
	now := nowUTC()
	res, err := s.db.Exec(`INSERT INTO media(product_id,content_id,kind,path,thumb_path,width,height,duration_sec,bytes,mime,sha256,prompt,ai_account_id,source,created_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		nullInt64(a.ProductID), nullInt64(a.ContentID), a.Kind, a.Path, a.ThumbPath,
		a.Width, a.Height, a.DurationSec, a.Bytes, a.Mime, a.SHA256, a.Prompt,
		nullInt64(a.AIAccountID), a.Source, now)
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

// MediaForContent lists one post's linked files, oldest first.
func (s *Store) MediaForContent(contentID int64) ([]*Media, error) {
	rows, err := s.ro.Query(`SELECT `+mediaCols+` FROM media WHERE content_id=? ORDER BY id ASC`, contentID)
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

// LinkMediaContent links one media row to a post (and its work when the
// row is still unattached). Already-linked rows are rejected so one file
// never double-counts.
func (s *Store) LinkMediaContent(id, contentID, productID int64) error {
	m, err := s.GetMedia(id)
	if err != nil {
		return err
	}
	if m.ContentID != nil {
		return ErrDuplicate
	}
	if m.ProductID != nil && *m.ProductID != productID {
		return ErrDuplicate
	}
	if _, err := s.db.Exec(`UPDATE media SET content_id=?, product_id=? WHERE id=?`, contentID, productID, id); err != nil {
		return err
	}
	return nil
}

// AttachMedia links unattached uploads to a product. Already-attached rows
// are rejected so one file never double-counts. The per-work cap (W14-1)
// counts existing attachments plus this batch.
func (s *Store) AttachMedia(ids []int64, productID int64) error {
	var have int
	if err := s.ro.QueryRow(`SELECT COUNT(*) FROM media WHERE product_id=?`, productID).Scan(&have); err != nil {
		return err
	}
	if have+len(ids) > MaxMediaPerInput {
		return errors.New("too many files for one work (max 20)")
	}
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
	return s.ListMediaQuery("", productID, "", "", "", limit, 0)
}

// ListMediaQuery filters by kind, product, business (via products join),
// and created_at date range (from/to are YYYY-MM-DD prefixes, "" open).
func (s *Store) ListMediaQuery(kind string, productID int64, business, from, to string, limit, offset int) ([]*Media, error) {
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
	if from != "" {
		conds = append(conds, `m.created_at>=?`)
		args = append(args, from)
	}
	if to != "" {
		conds = append(conds, `m.created_at<?`)
		args = append(args, to)
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

// DeleteStaleMedia removes generated-but-unused rows older than cutoff
// (RFC3339): files linked to a work or a post are always preserved.
// It returns the removed rows so the caller can unlink the bytes.
func (s *Store) DeleteStaleMedia(cutoff string) ([]*Media, error) {
	rows, err := s.ro.Query(`SELECT `+mediaCols+` FROM media
		WHERE product_id IS NULL AND content_id IS NULL AND created_at<?
		AND kind IN ('gen_image','gen_video','card')`, cutoff)
	if err != nil {
		return nil, err
	}
	var stale []*Media
	for rows.Next() {
		m, err := scanMedia(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		stale = append(stale, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, m := range stale {
		if _, err := s.db.Exec(`DELETE FROM media WHERE id=? AND product_id IS NULL AND content_id IS NULL`, m.ID); err != nil {
			return nil, err
		}
	}
	return stale, nil
}
