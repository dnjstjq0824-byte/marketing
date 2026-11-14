package store

// Overview queries for the S1 home screen (UI_DESIGN §7-2).
// Works map onto products, posts onto contents, media onto product images
// until W14/W16/W19 add their own tables.

import (
	"net/url"
	"strings"
)

// OverviewToday is the S1 stat row.
type OverviewToday struct {
	Running        int `json:"running"`
	PublishedToday int `json:"publishedToday"`
	Queued         int `json:"queued"`
	WeekViews      int `json:"weekViews"`
	Failed         int `json:"failed"`
}

// OverviewWork is one running work card.
type OverviewWork struct {
	ID     int64  `json:"id"`
	Title  string `json:"title"`
	Kind   string `json:"kind"`
	Thumb  string `json:"thumb"`
	Stage  string `json:"stage"`
	Detail string `json:"detail"`
	Ago    string `json:"ago"`
}

// OverviewPost is one recent published post.
type OverviewPost struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Channel     string `json:"channel"`
	Thumb       string `json:"thumb"`
	PublishedAt string `json:"publishedAt"`
	Views       int    `json:"views"`
	URL         string `json:"url"`
}

// OverviewMedia is one recent media tile (uploads until W14).
type OverviewMedia struct {
	ID    int64  `json:"id"`
	Kind  string `json:"kind"`
	Thumb string `json:"thumb"`
	URL   string `json:"url"`
}

// activeContentStatuses are contents still in the pipeline.
const activeContentStatuses = "'draft','review','approved','queued'"

// runningProductCond matches products with pipeline activity: active
// contents or an open job touching the product or its contents. A fresh
// work (analyze pending, no contents yet) counts as running.
const runningProductCond = `(EXISTS (SELECT 1 FROM contents c WHERE c.product_id=p.id AND c.status IN (` + activeContentStatuses + `))
		OR EXISTS (SELECT 1 FROM jobs j WHERE j.status IN ('pending','claimed')
			AND (j.ref_id=p.id OR j.ref_id IN (SELECT id FROM contents WHERE product_id=p.id))))`

// OverviewToday aggregates the S1 stat row. dayStartUTC is today's local
// midnight in UTC (D20); weekAgoUTC is 7 days back.
func (s *Store) OverviewToday(dayStartUTC, weekAgoUTC string) (*OverviewToday, error) {
	t := &OverviewToday{}
	queries := []struct {
		dest *int
		q    string
		args []any
	}{
		{&t.Running, `SELECT COUNT(*) FROM products p WHERE ` + runningProductCond, nil},
		{&t.PublishedToday, `SELECT COUNT(*) FROM contents WHERE status='published' AND updated_at>=?`, []any{dayStartUTC}},
		{&t.Queued, `SELECT COUNT(*) FROM contents WHERE status IN ('queued','approved')`, nil},
		{&t.WeekViews, `SELECT COALESCE(SUM(exposures),0) FROM metrics WHERE created_at>=?`, []any{weekAgoUTC}},
		{&t.Failed, `SELECT COUNT(*) FROM jobs WHERE status='failed' AND finished_at>=?`, []any{weekAgoUTC}},
	}
	for _, c := range queries {
		if err := s.ro.QueryRow(c.q, c.args...).Scan(c.dest); err != nil {
			return nil, err
		}
	}
	return t, nil
}

var activeDetailText = map[string]string{
	"draft":    "제작 진행 중",
	"review":   "검수 진행 중",
	"approved": "승인됨 · 배정 대기",
	"queued":   "발행 대기",
}

// OverviewRunning returns up to limit products with pipeline activity,
// newest activity first, with inferred stage (UI_DESIGN S3-D order).
func (s *Store) OverviewRunning(limit int) ([]*OverviewWork, error) {
	rows, err := s.ro.Query(`SELECT p.id, COALESCE((SELECT MAX(updated_at) FROM contents WHERE product_id=p.id), p.updated_at) FROM products p
		WHERE `+runningProductCond+`
		ORDER BY 2 DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		var maxUpdated string
		if err := rows.Scan(&id, &maxUpdated); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := []*OverviewWork{}
	for _, id := range ids {
		w, err := s.OverviewWorkSummary(id)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, nil
}

// OverviewWorkSummary projects one product as a work card (shared by S1/S3).
func (s *Store) OverviewWorkSummary(productID int64) (*OverviewWork, error) {
	p, err := s.GetProduct(productID)
	if err != nil {
		return nil, err
	}
	w := &OverviewWork{ID: p.ID, Title: p.Name, Kind: "url"}
	if w.Title == "" {
		w.Title = hostOrURL(p.SourceURL)
	}
	var thumb string
	_ = s.ro.QueryRow(`SELECT path FROM product_images WHERE product_id=? ORDER BY sort ASC, id ASC LIMIT 1`, productID).Scan(&thumb)
	if thumb != "" {
		w.Thumb = "/uploads/" + strings.TrimPrefix(thumb, "/")
	}
	counts := map[string]int{}
	crows, err := s.ro.Query(`SELECT status, COUNT(*) FROM contents WHERE product_id=? GROUP BY status`, productID)
	if err != nil {
		return nil, err
	}
	for crows.Next() {
		var status string
		var n int
		if err := crows.Scan(&status, &n); err != nil {
			crows.Close()
			return nil, err
		}
		counts[status] = n
	}
	crows.Close()
	var hasMetrics, hasStrategy, hasReading int
	_ = s.ro.QueryRow(`SELECT COUNT(*) FROM metrics m JOIN contents c ON c.id=m.content_id WHERE c.product_id=?`, productID).Scan(&hasMetrics)
	_ = s.ro.QueryRow(`SELECT COUNT(*) FROM strategies WHERE product_id=?`, productID).Scan(&hasStrategy)
	_ = s.ro.QueryRow(`SELECT COUNT(*) FROM readings WHERE product_id=?`, productID).Scan(&hasReading)
	switch {
	case hasMetrics > 0 && counts["published"] > 0:
		w.Stage = "collect"
	case counts["published"] > 0:
		w.Stage = "publish"
	case counts["approved"] > 0 || counts["review"] > 0:
		w.Stage = "review"
	case counts["draft"] > 0 || counts["queued"] > 0:
		w.Stage = "generate"
	case hasStrategy > 0:
		w.Stage = "strategy"
	case hasReading > 0:
		w.Stage = "analyze"
	default:
		w.Stage = "intake"
	}
	var status, updated string
	_ = s.ro.QueryRow(`SELECT status, updated_at FROM contents WHERE product_id=? AND status IN (`+activeContentStatuses+`)
		ORDER BY updated_at DESC LIMIT 1`, productID).Scan(&status, &updated)
	if detail, ok := activeDetailText[status]; ok {
		w.Detail = detail
		w.Ago = updated
		return w, nil
	}
	var jobType, jobAt string
	_ = s.ro.QueryRow(`SELECT type, created_at FROM jobs WHERE status IN ('pending','claimed')
		AND (ref_id=? OR ref_id IN (SELECT id FROM contents WHERE product_id=?))
		ORDER BY id DESC LIMIT 1`, productID, productID).Scan(&jobType, &jobAt)
	preStageDetail := map[string]string{
		"analyze": "분석 대기 중", "strategy": "전략 대기 중", "generate": "제작 대기 중",
		"review": "검수 대기 중", "publish": "발행 대기 중", "collect": "성과 수집 중",
	}
	if detail, ok := preStageDetail[jobType]; ok {
		w.Detail = detail
		w.Ago = jobAt
	}
	return w, nil
}

func hostOrURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		if raw == "" {
			return "새 작업"
		}
		return raw
	}
	return u.Host
}

// OverviewRecentPosts returns published contents newest first.
func (s *Store) OverviewRecentPosts(limit int) ([]*OverviewPost, error) {
	rows, err := s.ro.Query(`SELECT id,product_id,channel,title,updated_at FROM contents
		WHERE status='published' ORDER BY updated_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*OverviewPost{}
	for rows.Next() {
		var p OverviewPost
		var productID int64
		if err := rows.Scan(&p.ID, &productID, &p.Channel, &p.Title, &p.PublishedAt); err != nil {
			return nil, err
		}
		_ = s.ro.QueryRow(`SELECT path FROM product_images WHERE product_id=? ORDER BY sort ASC, id ASC LIMIT 1`, productID).Scan(&p.Thumb)
		if p.Thumb != "" && !strings.HasPrefix(p.Thumb, "/") {
			p.Thumb = "/uploads/" + p.Thumb
		}
		_ = s.ro.QueryRow(`SELECT COALESCE(SUM(exposures),0) FROM metrics WHERE content_id=?`, p.ID).Scan(&p.Views)
		var note string
		_ = s.ro.QueryRow(`SELECT note FROM publish_logs WHERE content_id=? ORDER BY id DESC LIMIT 1`, p.ID).Scan(&note)
		if strings.HasPrefix(note, "http") {
			p.URL = note
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}

// OverviewRecentMedia returns newest uploads (W14 adds generated media).
func (s *Store) OverviewRecentMedia(limit int) ([]*OverviewMedia, error) {
	rows, err := s.ro.Query(`SELECT id,path FROM product_images ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*OverviewMedia{}
	for rows.Next() {
		var m OverviewMedia
		var path string
		if err := rows.Scan(&m.ID, &path); err != nil {
			return nil, err
		}
		m.Kind = "upload"
		m.Thumb = "/uploads/" + strings.TrimPrefix(path, "/")
		m.URL = m.Thumb
		out = append(out, &m)
	}
	return out, rows.Err()
}

// OverviewFailedPublishes returns recent failed publish logs for actions.
func (s *Store) OverviewFailedPublishes(limit int) ([]*PublishLog, error) {
	rows, err := s.ro.Query(`SELECT `+publishLogCols+` FROM publish_logs
		WHERE status='failed' ORDER BY id DESC LIMIT ?`, limit)
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

// OverviewUnassignedQueued counts queued/approved contents with no account.
func (s *Store) OverviewUnassignedQueued() (int, error) {
	var n int
	err := s.ro.QueryRow(`SELECT COUNT(*) FROM contents
		WHERE status IN ('queued','approved') AND (target_account IS NULL OR target_account='')`).Scan(&n)
	return n, err
}

// OverviewFirstRun reports the S1 checklist inputs.
func (s *Store) OverviewFirstRun() (aiConnected, channelConnected, hasWork bool, err error) {
	var n int
	if err = s.ro.QueryRow(`SELECT COUNT(*) FROM channel_accounts
		WHERE kind='agent_service' AND session_state='connected'`).Scan(&n); err != nil {
		return false, false, false, err
	}
	aiConnected = n > 0
	if err = s.ro.QueryRow(`SELECT COUNT(*) FROM channel_accounts
		WHERE (kind IS NULL OR kind='' OR kind='channel') AND session_state='connected'`).Scan(&n); err != nil {
		return false, false, false, err
	}
	channelConnected = n > 0
	if err = s.ro.QueryRow(`SELECT COUNT(*) FROM products`).Scan(&n); err != nil {
		return false, false, false, err
	}
	hasWork = n > 0
	return aiConnected, channelConnected, hasWork, nil
}

