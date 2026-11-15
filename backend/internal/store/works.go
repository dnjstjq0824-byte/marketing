package store

import (
	"encoding/json"
	"strings"
)

// Works queries: products projected as S3 works (UI_DESIGN §7-2).

// WorkItem is one S3 list row.
type WorkItem struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Kind     string `json:"kind"`
	Thumb    string `json:"thumb"`
	Stage    string `json:"stage"`
	Detail   string `json:"detail"`
	Ago      string `json:"ago"`
	State    string `json:"state"`
	Business string `json:"business"`
}

// ListWorks returns newest-first work items for one tab.
func (s *Store) ListWorks(tab, q, business string, page, size int) ([]*WorkItem, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	like := "%" + q + "%"
	var ids []int64
	var err error
	switch tab {
	case "", "running":
		ids, err = s.workIDsByState(runningProductCond, business, like, page, size)
	case "queued":
		ids, err = s.workIDsByState(`EXISTS (SELECT 1 FROM contents c WHERE c.product_id=p.id AND c.status IN ('queued','approved'))
			AND NOT EXISTS (SELECT 1 FROM contents c WHERE c.product_id=p.id AND c.status IN ('draft','review'))`, business, like, page, size)
	case "published":
		ids, err = s.workIDsByState(`EXISTS (SELECT 1 FROM contents c WHERE c.product_id=p.id AND c.status='published')
			AND NOT EXISTS (SELECT 1 FROM contents c WHERE c.product_id=p.id AND c.status IN (`+activeContentStatuses+`))`, business, like, page, size)
	case "held":
		ids, err = s.workIDsByState(`EXISTS (SELECT 1 FROM contents c WHERE c.product_id=p.id AND c.held=1)`, business, like, page, size)
	case "failed":
		ids, err = s.workIDsByState(`(EXISTS (SELECT 1 FROM publish_logs l JOIN contents c ON c.id=l.content_id WHERE c.product_id=p.id AND l.status='failed')
			OR EXISTS (SELECT 1 FROM jobs j WHERE j.status='failed' AND (j.ref_id=p.id OR j.ref_id IN (SELECT id FROM contents WHERE product_id=p.id))))`, business, like, page, size)
	default:
		return []*WorkItem{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []*WorkItem{}
	for _, id := range ids {
		w, err := s.OverviewWorkSummary(id)
		if err != nil {
			return nil, err
		}
		p, err := s.GetProduct(id)
		if err != nil {
			return nil, err
		}
		out = append(out, &WorkItem{
			ID: w.ID, Title: w.Title, Kind: w.Kind, Thumb: w.Thumb,
			Stage: w.Stage, Detail: w.Detail, Ago: w.Ago,
			State: tab, Business: p.Business,
		})
	}
	return out, nil
}

func (s *Store) workIDsByState(cond, business, like string, page, size int) ([]int64, error) {
	q := `SELECT p.id FROM products p WHERE ` + cond
	var args []any
	if business != "" {
		q += ` AND p.business=? COLLATE NOCASE`
		args = append(args, business)
	}
	if strings.Trim(like, "%") != "" {
		q += ` AND (p.name LIKE ? OR p.source_url LIKE ? OR p.business LIKE ?)`
		args = append(args, like, like, like)
	}
	q += ` ORDER BY p.id DESC LIMIT ? OFFSET ?`
	args = append(args, size, (page-1)*size)
	rows, err := s.ro.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// WorkDetail is the S3-D projection.
type WorkDetail struct {
	Product  *Product       `json:"product"`
	Brief    map[string]any `json:"brief"`
	Stage    string         `json:"stage"`
	Contents []*WorkPost    `json:"contents"`
	Images   []*WorkImage   `json:"images"`
}

// WorkPost is one post row with live stats.
type WorkPost struct {
	ID             int64  `json:"id"`
	Title          string `json:"title"`
	Channel        string `json:"channel"`
	TargetPlatform string `json:"targetPlatform"`
	TargetAccount  string `json:"targetAccount"`
	Status         string `json:"status"`
	ScheduledAt    string `json:"scheduledAt"`
	Held           bool   `json:"held"`
	UpdatedAt      string `json:"updatedAt"`
	Views          int    `json:"views"`
	URL            string `json:"url"`
	// LastNote is the latest rejection reason (chief or W18 guard), so the
	// work detail shows why a post went back for rewrite.
	LastNote string `json:"lastNote"`
}

// WorkImage is attached media (legacy uploads + library).
type WorkImage struct {
	ID   int64  `json:"id"`
	Kind string `json:"kind"`
	URL  string `json:"url"`
}

// AILogEntry is one AI turn for S3-D history.
type AILogEntry struct {
	Stage   string `json:"stage"`
	Type    string `json:"type"`
	Worker  string `json:"worker"`
	At      string `json:"at"`
	Summary string `json:"summary"`
	Error   string `json:"error"`
}

// WorkDetailByID loads the S3-D projection. Unknown brief JSON degrades to
// an empty object; the raw row is never exposed.
func (s *Store) WorkDetailByID(id int64) (*WorkDetail, error) {
	p, err := s.GetProduct(id)
	if err != nil {
		return nil, err
	}
	brief := map[string]any{}
	if strings.TrimSpace(p.BriefJSON) != "" {
		var raw map[string]any
		if err := json.Unmarshal([]byte(p.BriefJSON), &raw); err == nil && raw != nil {
			brief = raw
		}
	}
	sum, err := s.OverviewWorkSummary(id)
	if err != nil {
		return nil, err
	 }
	contents, err := s.WorkPosts(id)
	if err != nil {
		return nil, err
	}
	images, err := s.WorkImages(id)
	if err != nil {
		return nil, err
	}
	return &WorkDetail{Product: p, Brief: brief, Stage: sum.Stage, Contents: contents, Images: images}, nil
}

// WorkPosts lists one product's posts newest first with live stats.
func (s *Store) WorkPosts(productID int64) ([]*WorkPost, error) {
	rows, err := s.ro.Query(`SELECT id,title,channel,target_platform,target_account,status,
		scheduled_at,held,updated_at FROM contents WHERE product_id=? ORDER BY id DESC`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*WorkPost{}
	for rows.Next() {
		var p WorkPost
		var held int
		if err := rows.Scan(&p.ID, &p.Title, &p.Channel, &p.TargetPlatform, &p.TargetAccount,
			&p.Status, &p.ScheduledAt, &held, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.Held = held != 0
		_ = s.ro.QueryRow(`SELECT COALESCE(SUM(exposures),0) FROM metrics WHERE content_id=?`, p.ID).Scan(&p.Views)
		var note string
		_ = s.ro.QueryRow(`SELECT note FROM publish_logs WHERE content_id=? ORDER BY id DESC LIMIT 1`, p.ID).Scan(&note)
		if strings.HasPrefix(note, "http") {
			p.URL = note
		}
		_ = s.ro.QueryRow(`SELECT decider || ': ' || reason FROM review_decisions
			WHERE content_id=? AND decision='rejected' ORDER BY id DESC LIMIT 1`, p.ID).Scan(&p.LastNote)
		out = append(out, &p)
	}
	return out, rows.Err()
}

// WorkImages unions legacy uploads and library attachments.
func (s *Store) WorkImages(productID int64) ([]*WorkImage, error) {
	out := []*WorkImage{}
	legacy, err := s.ListImages(productID)
	if err != nil {
		return nil, err
	}
	for _, im := range legacy {
		out = append(out, &WorkImage{ID: -im.ID, Kind: "upload", URL: "/uploads/" + strings.TrimPrefix(im.Path, "/")})
	}
	lib, err := s.ListMedia(productID, 100)
	if err != nil {
		return nil, err
	}
	for _, m := range lib {
		out = append(out, &WorkImage{ID: m.ID, Kind: m.Kind, URL: "/uploads/" + strings.TrimPrefix(m.Path, "/")})
	}
	return out, nil
}

var jobStage = map[string]string{
	"analyze": "analyze", "strategy": "strategy", "generate": "generate",
	"review": "review", "publish": "publish", "collect": "collect",
	"research": "research",
}

// WorkAILog lists AI turns for one product, oldest first.
func (s *Store) WorkAILog(productID int64, limit int) ([]*AILogEntry, error) {
	rows, err := s.ro.Query(`SELECT type,worker_name,COALESCE(finished_at,claimed_at,created_at),result,error
		FROM jobs WHERE ref_id=? OR ref_id IN (SELECT id FROM contents WHERE product_id=?)
		ORDER BY id ASC LIMIT ?`, productID, productID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*AILogEntry{}
	for rows.Next() {
		var e AILogEntry
		var finished string
		if err := rows.Scan(&e.Type, &e.Worker, &finished, &e.Summary, &e.Error); err != nil {
			return nil, err
		}
		e.Stage = jobStage[e.Type]
		e.At = finished
		if len(e.Summary) > 500 {
			e.Summary = e.Summary[:500] + "…"
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}

// SetWorkHeld holds or releases all pipeline contents of one product.
func (s *Store) SetWorkHeld(productID int64, held bool) (int, error) {
	flag := 0
	if held {
		flag = 1
	}
	res, err := s.db.Exec(`UPDATE contents SET held=?, updated_at=? WHERE product_id=?
		AND status IN (`+activeContentStatuses+`)`, flag, nowUTC(), productID)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}
