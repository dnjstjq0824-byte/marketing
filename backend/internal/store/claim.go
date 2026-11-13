package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ErrLeaseDenied marks a lease heartbeat for a job this agent does not hold.
var ErrLeaseDenied = fmt.Errorf("job is not claimed by this agent")

func splitBusinesses(csv string) []string {
	var out []string
	for _, b := range strings.Split(csv, ",") {
		if b = strings.TrimSpace(b); b != "" {
			out = append(out, b)
		}
	}
	return out
}

// HasPendingCandidate reports (read-only, no write lock) whether a matching
// job waits. Long-poll uses it before sleeping on the claim hub.
func (s *Store) HasPendingCandidate(agent *Agent, types []string) (bool, error) {
	if len(types) == 0 {
		return false, nil
	}
	biz := splitBusinesses(agent.Businesses)
	q := `SELECT 1 FROM jobs WHERE status='pending' AND type IN (` + placeholders(len(types)) + `)`
	args := make([]any, 0, len(types)+len(biz)+1)
	for _, t := range types {
		args = append(args, t)
	}
	if len(biz) > 0 {
		q += ` AND (business='' OR business IN (` + placeholders(len(biz)) + `) COLLATE NOCASE)`
		for _, b := range biz {
			args = append(args, b)
		}
	}
	q += ` LIMIT 1`
	var one int
	err := s.ro.QueryRow(q, args...).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func placeholders(n int) string {
	if n <= 0 {
		return "NULL"
	}
	out := "?"
	for i := 1; i < n; i++ {
		out += ",?"
	}
	return out
}

// ClaimRoutedJob assigns the best pending job for an agent: lane types,
// business scope, capacity gate, priority DESC then id ASC. Publish jobs
// additionally require the agent's linked account. Sets worker label,
// agent id and a fresh lease. Returns (nil, nil) when nothing fits.
func (s *Store) ClaimRoutedJob(agent *Agent, types []string, leaseSec int) (*Job, error) {
	if len(types) == 0 {
		return nil, nil
	}
	live, err := s.ClaimedCountByAgent(agent.ID)
	if err != nil {
		return nil, err
	}
	if live >= agent.Capacity {
		return nil, nil
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
	biz := splitBusinesses(agent.Businesses)
	q := `SELECT ` + jobCols + ` FROM jobs WHERE status='pending' AND type IN (` + placeholders(len(types)) + `)`
	args := make([]any, 0, len(types)+len(biz))
	for _, t := range types {
		args = append(args, t)
	}
	if len(biz) > 0 {
		q += ` AND (business='' OR business IN (` + placeholders(len(biz)) + `) COLLATE NOCASE)`
		for _, b := range biz {
			args = append(args, b)
		}
	}
	q += ` ORDER BY priority DESC, id ASC LIMIT 25`
	rows, err := conn.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	var cands []*Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		cands = append(cands, j)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	now := nowUTC()
	lease := time.Now().UTC().Add(time.Duration(leaseSec) * time.Second).Format(time.RFC3339)
	for _, j := range cands {
		if j.Type == "publish" && !s.publishFitsAgent(conn, j, agent) {
			continue
		}
		res, err := conn.ExecContext(ctx, `UPDATE jobs SET status='claimed',worker_name=?,agent_id=?,claimed_at=?,lease_until=?
			WHERE id=? AND status='pending'`, agent.Label, agent.ID, now, lease, j.ID)
		if err != nil {
			return nil, err
		}
		n, _ := res.RowsAffected()
		if n != 1 {
			continue
		}
		if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
			return nil, err
		}
		committed = true
		_ = conn.Close()
		return s.GetJob(j.ID)
	}
	if _, err := conn.ExecContext(ctx, `ROLLBACK`); err != nil {
		return nil, err
	}
	committed = true
	return nil, nil
}

// publishFitsAgent checks a publish candidate against the agent's linked
// account id via the payload's account label.
func (s *Store) publishFitsAgent(conn interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}, j *Job, agent *Agent) bool {
	if agent.AccountID == nil {
		return false
	}
	var payload struct {
		Account string `json:"account"`
	}
	if err := json.Unmarshal([]byte(j.Payload), &payload); err != nil || payload.Account == "" {
		return false
	}
	var id int64
	err := conn.QueryRowContext(context.Background(),
		`SELECT id FROM channel_accounts WHERE name=?`, payload.Account).Scan(&id)
	return err == nil && id == *agent.AccountID
}

// ExtendJobLease pushes a claim's lease out. Foreign claims get ErrLeaseDenied.
func (s *Store) ExtendJobLease(id, agentID int64, leaseSec int) (string, error) {
	lease := time.Now().UTC().Add(time.Duration(leaseSec) * time.Second).Format(time.RFC3339)
	res, err := s.db.Exec(`UPDATE jobs SET lease_until=? WHERE id=? AND status='claimed' AND agent_id=?`,
		lease, id, agentID)
	if err != nil {
		return "", err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return "", ErrLeaseDenied
	}
	return lease, nil
}

// ReapAction describes one expired lease outcome.
type ReapAction struct {
	JobID  int64
	Type   string
	Action string // requeued | failed | failed_held
}

// ReapExpiredLeases applies D25 to claims past nowRFC3339: analyze/strategy/
// generate/review with attempts==0 return to pending once (attempts=1);
// attempts>=1 close as failed; publish never requeues and holds its content.
func (s *Store) ReapExpiredLeases(nowRFC3339 string) ([]ReapAction, error) {
	rows, err := s.ro.Query(`SELECT `+jobCols+` FROM jobs
		WHERE status='claimed' AND lease_until != '' AND lease_until < ?`, nowRFC3339)
	if err != nil {
		return nil, err
	}
	var expired []*Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		expired = append(expired, j)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var out []ReapAction
	for _, j := range expired {
		switch {
		case j.Type == "publish":
			if _, err := s.db.Exec(`UPDATE jobs SET status='failed',error='lease expired',finished_at=? WHERE id=? AND status='claimed' AND lease_until != '' AND lease_until < ?`,
				nowUTC(), j.ID, nowRFC3339); err != nil {
				return out, err
			}
			var payload struct {
				ContentID int64 `json:"content_id"`
			}
			if err := json.Unmarshal([]byte(j.Payload), &payload); err == nil && payload.ContentID > 0 {
				_ = s.HoldContent(payload.ContentID, "실행 중단 — 게시 여부 사람 확인")
			}
			out = append(out, ReapAction{JobID: j.ID, Type: j.Type, Action: "failed_held"})
		case j.Attempts == 0:
			if _, err := s.db.Exec(`UPDATE jobs SET status='pending',worker_name='',agent_id=0,claimed_at='',lease_until='',attempts=1
				WHERE id=? AND status='claimed'`, j.ID); err != nil {
				return out, err
			}
			out = append(out, ReapAction{JobID: j.ID, Type: j.Type, Action: "requeued"})
		default:
			if _, err := s.db.Exec(`UPDATE jobs SET status='failed',error='lease expired',finished_at=? WHERE id=? AND status='claimed'`,
				nowUTC(), j.ID); err != nil {
				return out, err
			}
			out = append(out, ReapAction{JobID: j.ID, Type: j.Type, Action: "failed"})
		}
	}
	return out, nil
}

// HoldContent flags one content row held with a reason.
func (s *Store) HoldContent(id int64, reason string) error {
	_, err := s.db.Exec(`UPDATE contents SET held=1,hold_reason=?,updated_at=? WHERE id=?`, reason, nowUTC(), id)
	return err
}

// HasActivePublishJob reports whether a pending/claimed publish job exists
// for one content+account pair (schedule dedup).
func (s *Store) HasActivePublishJob(contentID int64, account string) (bool, error) {
	rows, err := s.ro.Query(`SELECT payload FROM jobs WHERE type='publish' AND ref_id=? AND status IN ('pending','claimed')`, contentID)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return false, err
		}
		var decoded struct {
			Account string `json:"account"`
		}
		if json.Unmarshal([]byte(payload), &decoded) == nil && decoded.Account == account {
			return true, nil
		}
	}
	return false, rows.Err()
}

// RequeueJob returns a failed job to pending with attempts reset (the error
// text stays as the record). Other states get ErrNotFailed.
func (s *Store) RequeueJob(id int64) (*Job, error) {
	res, err := s.db.Exec(`UPDATE jobs SET status='pending',worker_name='',agent_id=0,claimed_at='',finished_at='',lease_until='',attempts=0
		WHERE id=? AND status='failed'`, id)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return nil, ErrNotFailed
	}
	return s.GetJob(id)
}
