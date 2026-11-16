package store

import (
	"strconv"
	"strings"
	"time"
)

// W7 performance collection: after a post goes live, the sweep enqueues one
// P-lane collect job per window (24h, 72h, 7d) per published URL. The worker
// opens the URL, reads platform public figures (views/likes/comments) plus
// operator-screen funnel values, and submits them; SubmitCollect stores one
// metrics row per window. Missing values stay NULL with cause '원인 미분류'.

// CollectWindow is one post-publish collection point.
type CollectWindow struct {
	// Name tags the metrics origin ('collect:<name>') and the ledger key.
	Name string
	// After is the delay from the publish execution timestamp.
	After time.Duration
}

// CollectWindows are the W7 collection points, oldest first.
var CollectWindows = []CollectWindow{
	{Name: "24h", After: 24 * time.Hour},
	{Name: "72h", After: 72 * time.Hour},
	{Name: "7d", After: 7 * 24 * time.Hour},
}

// CollectOrigin tags auto-collected metric rows.
func CollectOrigin(window string) string { return "collect:" + window }

// CollectURL extracts the published URL from a publish-log note. Worker
// results append " | evidence: ..."; the executor stores the bare URL.
func CollectURL(note string) string {
	url := note
	if i := strings.Index(note, " | "); i >= 0 {
		url = note[:i]
	}
	url = strings.TrimSpace(url)
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return url
	}
	return ""
}

// DueCollect is one collect job the sweep must enqueue.
type DueCollect struct {
	ContentID int64
	ProductID int64
	Channel   string
	Platform  string
	Account   string
	URL       string
	Window    string
	Business  string
	Priority  int
}

// DueCollects lists every (published URL, window) whose time has come and
// which the ledger does not cover yet. Unparseable execution timestamps
// and URL-less logs are skipped: without a publish moment or a URL there
// is nothing defensible to collect.
func (s *Store) DueCollects(now time.Time) ([]*DueCollect, error) {
	rows, err := s.ro.Query(`SELECT c.id, c.product_id, c.channel, c.target_platform,
		l.account, l.channel, l.note, l.executed_at, p.business, p.priority
		FROM contents c
		JOIN publish_logs l ON l.content_id=c.id AND l.status='done'
		JOIN products p ON p.id=c.product_id
		WHERE c.status='published' ORDER BY c.id ASC, l.id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type cand struct {
		content, product                                                 int64
		channel, platform, account, logChannel, note, executed, business string
		priority                                                         int
	}
	var cands []cand
	for rows.Next() {
		var c cand
		if err := rows.Scan(&c.content, &c.product, &c.channel, &c.platform,
			&c.account, &c.logChannel, &c.note, &c.executed, &c.business, &c.priority); err != nil {
			return nil, err
		}
		if strings.TrimSpace(c.platform) == "" {
			c.platform = c.logChannel
		}
		cands = append(cands, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	ledger := map[string]bool{}
	lrows, err := s.ro.Query(`SELECT content_id, window, account FROM collect_windows`)
	if err != nil {
		return nil, err
	}
	defer lrows.Close()
	for lrows.Next() {
		var cid int64
		var window, account string
		if err := lrows.Scan(&cid, &window, &account); err != nil {
			return nil, err
		}
		ledger[collectKey(cid, window, account)] = true
	}
	if err := lrows.Err(); err != nil {
		return nil, err
	}
	var out []*DueCollect
	for _, c := range cands {
		url := CollectURL(c.note)
		if url == "" {
			continue
		}
		doneAt, err := time.Parse(time.RFC3339, strings.TrimSpace(c.executed))
		if err != nil {
			continue
		}
		for _, w := range CollectWindows {
			if now.UTC().Before(doneAt.Add(w.After)) {
				continue
			}
			if ledger[collectKey(c.content, w.Name, c.account)] {
				continue
			}
			out = append(out, &DueCollect{
				ContentID: c.content, ProductID: c.product, Channel: c.channel,
				Platform: c.platform, Account: c.account, URL: url,
				Window: w.Name, Business: c.business, Priority: c.priority,
			})
		}
	}
	return out, nil
}

func collectKey(contentID int64, window, account string) string {
	return strings.Join([]string{strconv.FormatInt(contentID, 10), window, account}, "\x00")
}

// EnsureCollectWindow records one enqueued window. Replays return
// (false, nil): the UNIQUE key makes the sweep idempotent.
func (s *Store) EnsureCollectWindow(contentID int64, window, account string, jobID int64) (bool, error) {
	res, err := s.db.Exec(`INSERT OR IGNORE INTO collect_windows(content_id,window,account,job_id,created_at)
		VALUES(?,?,?,?,?)`, contentID, window, account, jobID, nowUTC())
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// HasCollectWindow reports whether a window is already enqueued.
func (s *Store) HasCollectWindow(contentID int64, window, account string) (bool, error) {
	var n int
	err := s.ro.QueryRow(`SELECT COUNT(*) FROM collect_windows
		WHERE content_id=? AND window=? AND account=?`, contentID, window, account).Scan(&n)
	return n > 0, err
}

// ReleaseCollectWindow drops one ledger row so the sweep re-enqueues the
// window (after a failed collect round). Missing rows are a silent no-op.
func (s *Store) ReleaseCollectWindow(contentID int64, window, account string) error {
	_, err := s.db.Exec(`DELETE FROM collect_windows
		WHERE content_id=? AND window=? AND account=?`, contentID, window, account)
	return err
}

// TopMetric is one past-performance summary line for strategy prompts.
type TopMetric struct {
	ContentID int64  `json:"contentId"`
	Channel   string `json:"channel"`
	Title     string `json:"title"`
	Views     int64  `json:"views"`
	Likes     int64  `json:"likes"`
	Comments  int64  `json:"comments"`
	Collected string `json:"collectedAt"`
}

// TopMetricsByBusiness returns the latest metric per post for one business,
// ranked by engagement (views+likes+comments, then newest). Only posts with
// at least one collected public figure qualify; empty collection rounds
// never steer strategy.
func (s *Store) TopMetricsByBusiness(business string, limit int) ([]*TopMetric, error) {
	if strings.TrimSpace(business) == "" || limit <= 0 {
		return []*TopMetric{}, nil
	}
	rows, err := s.ro.Query(`SELECT m.content_id, m.channel,
		COALESCE(m.views,0), COALESCE(m.likes,0), COALESCE(m.comments,0), m.created_at,
		COALESCE(c.title,'')
		FROM metrics m JOIN contents c ON c.id=m.content_id
		JOIN products p ON p.id=c.product_id
		WHERE p.business=? COLLATE NOCASE AND m.content_id IS NOT NULL
		AND (m.views IS NOT NULL OR m.likes IS NOT NULL OR m.comments IS NOT NULL)
		ORDER BY m.content_id ASC, m.id DESC`, business)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := map[int64]bool{}
	out := []*TopMetric{}
	for rows.Next() {
		var t TopMetric
		var cid int64
		if err := rows.Scan(&cid, &t.Channel, &t.Views, &t.Likes, &t.Comments, &t.Collected, &t.Title); err != nil {
			return nil, err
		}
		if seen[cid] {
			continue // older round of the same post
		}
		seen[cid] = true
		t.ContentID = cid
		out = append(out, &t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Insertion rank: engagement desc, collected desc. Few rows by design.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && topLess(out[j-1], out[j]); j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func topEngagement(t *TopMetric) int64 { return t.Views + t.Likes + t.Comments }

// topLess reports whether b outranks a.
func topLess(a, b *TopMetric) bool {
	if ea, eb := topEngagement(a), topEngagement(b); ea != eb {
		return eb > ea
	}
	return b.Collected > a.Collected
}
