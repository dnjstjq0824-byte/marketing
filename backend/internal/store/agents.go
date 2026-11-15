package store

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Agent is one registered resident worker (D22). TokenHash/TokenPrefix are
// the only token traces: plaintext appears once at issue/rotate time.
type Agent struct {
	ID           int64  `json:"id"`
	Label        string `json:"label"`
	Lane         string `json:"lane"`
	Host         string `json:"host"`
	Businesses   string `json:"businesses"`
	Platforms    string `json:"platforms"`
	Capacity     int    `json:"capacity"`
	Enabled      bool   `json:"enabled"`
	TokenHash    string `json:"-"`
	TokenPrefix  string `json:"tokenPrefix"`
	AccountID    *int64 `json:"accountId"`
	LastSeenAt   string `json:"lastSeenAt"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
	CurrentJobID *int64 `json:"currentJobId,omitempty"`
	Done24h      int    `json:"done24h,omitempty"`
	Failed24h    int    `json:"failed24h,omitempty"`
}

// LaneTypes maps lanes to claimable job types (D24). T is the W16 team
// executor: one PC process claiming the whole AI chain for its team.
var LaneTypes = map[string][]string{
	"A": {"analyze", "strategy", "research"},
	"B": {"generate"},
	"C": {"review"},
	"P": {"publish"},
	"T": {"analyze", "strategy", "generate", "review", "research"},
}

// ValidLane reports whether lane is A/B/C/P/T.
func ValidLane(lane string) bool {
	_, ok := LaneTypes[lane]
	return ok
}

const agentCols = `id,label,lane,host,businesses,platforms,capacity,enabled,token_hash,token_prefix,account_id,last_seen_at,created_at,updated_at`

func scanAgent(row interface{ Scan(...any) error }) (*Agent, error) {
	a := &Agent{}
	var enabled int
	var accountID sql.NullInt64
	err := row.Scan(&a.ID, &a.Label, &a.Lane, &a.Host, &a.Businesses, &a.Platforms,
		&a.Capacity, &enabled, &a.TokenHash, &a.TokenPrefix, &accountID,
		&a.LastSeenAt, &a.CreatedAt, &a.UpdatedAt)
	a.Enabled = enabled == 1
	if accountID.Valid {
		v := accountID.Int64
		a.AccountID = &v
	}
	return a, err
}

// GenerateAgentToken makes a 32-byte plaintext token with its SHA-256 hex
// hash and 8-char display prefix.
func GenerateAgentToken() (plaintext, hashHex, prefix string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", "", err
	}
	plaintext = hex.EncodeToString(buf)
	sum := sha256.Sum256([]byte(plaintext))
	hashHex = hex.EncodeToString(sum[:])
	return plaintext, hashHex, plaintext[:8], nil
}

// HashAgentToken hashes a presented token for lookup.
func HashAgentToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// CreateAgent registers an agent and returns it with a one-time plaintext token.
func (s *Store) CreateAgent(label, lane, host, businesses, platforms string, capacity int, accountID *int64) (*Agent, string, error) {
	if !ValidLane(lane) {
		return nil, "", fmt.Errorf("lane must be A, B, C or P")
	}
	if capacity < 1 {
		return nil, "", fmt.Errorf("capacity must be >= 1")
	}
	plaintext, hash, prefix, err := GenerateAgentToken()
	if err != nil {
		return nil, "", err
	}
	now := nowUTC()
	res, err := s.db.Exec(`INSERT INTO agents(label,lane,host,businesses,platforms,capacity,enabled,token_hash,token_prefix,account_id,last_seen_at,created_at,updated_at)
		VALUES(?,?,?,?,?,?,1,?,?,?,'' ,?,?)`, label, lane, host, businesses, platforms, capacity, hash, prefix, nullInt(accountID), now, now)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, "", ErrDuplicate
		}
		return nil, "", err
	}
	id, _ := res.LastInsertId()
	a, err := s.GetAgent(id)
	if err != nil {
		return nil, "", err
	}
	return a, plaintext, nil
}

func nullInt(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}

// GetAgent loads one agent by id.
func (s *Store) GetAgent(id int64) (*Agent, error) {
	a, err := scanAgent(s.ro.QueryRow(`SELECT `+agentCols+` FROM agents WHERE id=?`, id))
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return a, err
}

// GetAgentByTokenHash resolves an agent from a presented token hash.
// Disabled agents resolve fine; callers enforce enabled.
func (s *Store) GetAgentByTokenHash(hash string) (*Agent, error) {
	a, err := scanAgent(s.ro.QueryRow(`SELECT `+agentCols+` FROM agents WHERE token_hash=?`, hash))
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return a, err
}

// AgentFilter narrows agent listings. Empty fields match everything.
type AgentFilter struct {
	Lane     string
	Enabled  *bool
	Business string
	Host     string
	Query    string // label substring
}

// ListAgents returns newest-last agents with paging.
func (s *Store) ListAgents(f AgentFilter, limit, offset int) ([]*Agent, error) {
	if limit <= 0 {
		limit = 50
	}
	q := `SELECT ` + agentCols + ` FROM agents`
	var conds []string
	var args []any
	if f.Lane != "" {
		conds = append(conds, `lane=?`)
		args = append(args, f.Lane)
	}
	if f.Enabled != nil {
		conds = append(conds, `enabled=?`)
		args = append(args, boolInt(*f.Enabled))
	}
	if f.Host != "" {
		conds = append(conds, `host=?`)
		args = append(args, f.Host)
	}
	if f.Query != "" {
		conds = append(conds, `label LIKE ?`)
		args = append(args, "%"+f.Query+"%")
	}
	if f.Business != "" {
		conds = append(conds, `(businesses='' OR ','||businesses||',' LIKE ? ESCAPE '\')`)
		args = append(args, "%,"+escapeLike(f.Business)+",%")
	}
	if len(conds) > 0 {
		q += ` WHERE ` + strings.Join(conds, " AND ")
	}
	q += ` ORDER BY id ASC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := s.ro.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Agent{}
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

// CountAgents counts agents under the same filter.
func (s *Store) CountAgents(f AgentFilter) (int, error) {
	q := `SELECT COUNT(*) FROM agents`
	var conds []string
	var args []any
	if f.Lane != "" {
		conds = append(conds, `lane=?`)
		args = append(args, f.Lane)
	}
	if f.Enabled != nil {
		conds = append(conds, `enabled=?`)
		args = append(args, boolInt(*f.Enabled))
	}
	if f.Host != "" {
		conds = append(conds, `host=?`)
		args = append(args, f.Host)
	}
	if f.Query != "" {
		conds = append(conds, `label LIKE ?`)
		args = append(args, "%"+f.Query+"%")
	}
	if f.Business != "" {
		conds = append(conds, `(businesses='' OR ','||businesses||',' LIKE ? ESCAPE '\')`)
		args = append(args, "%,"+escapeLike(f.Business)+",%")
	}
	if len(conds) > 0 {
		q += ` WHERE ` + strings.Join(conds, " AND ")
	}
	var n int
	if err := s.ro.QueryRow(q, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// UpdateAgent writes mutable agent fields. Label is immutable.
func (s *Store) UpdateAgent(a *Agent) error {
	if !ValidLane(a.Lane) {
		return fmt.Errorf("lane must be A, B, C or P")
	}
	if a.Capacity < 1 {
		return fmt.Errorf("capacity must be >= 1")
	}
	res, err := s.db.Exec(`UPDATE agents SET lane=?,host=?,businesses=?,platforms=?,capacity=?,enabled=?,account_id=?,updated_at=?
		WHERE id=?`, a.Lane, a.Host, a.Businesses, a.Platforms, a.Capacity, boolInt(a.Enabled),
		nullInt(a.AccountID), nowUTC(), a.ID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteAgent removes an agent row. Claimed-job guard lives in the service.
func (s *Store) DeleteAgent(id int64) error {
	res, err := s.db.Exec(`DELETE FROM agents WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// RotateAgentToken replaces the token, returning the one-time plaintext.
func (s *Store) RotateAgentToken(id int64) (string, error) {
	plaintext, hash, prefix, err := GenerateAgentToken()
	if err != nil {
		return "", err
	}
	res, err := s.db.Exec(`UPDATE agents SET token_hash=?,token_prefix=?,updated_at=? WHERE id=?`,
		hash, prefix, nowUTC(), id)
	if err != nil {
		return "", err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return "", ErrNotFound
	}
	return plaintext, nil
}

// TouchAgent records a heartbeat.
func (s *Store) TouchAgent(id int64) error {
	_, err := s.db.Exec(`UPDATE agents SET last_seen_at=?,updated_at=? WHERE id=?`, nowUTC(), nowUTC(), id)
	return err
}

// ClaimedCountByAgent counts an agent's live claims (capacity gate).
func (s *Store) ClaimedCountByAgent(agentID int64) (int, error) {
	var n int
	err := s.ro.QueryRow(`SELECT COUNT(*) FROM jobs WHERE agent_id=? AND status='claimed'`, agentID).Scan(&n)
	return n, err
}

// AgentStats is the dashboard panel rollup.
type AgentStats struct {
	Online        int            `json:"online"`
	Offline       int            `json:"offline"`
	Paused        int            `json:"paused"`
	BacklogByType map[string]int `json:"backlogByType"`
	Done1h        int            `json:"done1h"`
	Failed1h      int            `json:"failed1h"`
	TopFailing    []AgentFailure `json:"topFailing"`
}

// AgentFailure ranks an agent's 24h failures.
type AgentFailure struct {
	AgentID int64  `json:"agentId"`
	Label   string `json:"label"`
	Failed  int    `json:"failed"`
}

// OnlineCutoffMin bounds last_seen_at for the online verdict.
const OnlineCutoffMin = 3

// StatsForAgents computes the dashboard agent panel.
func (s *Store) StatsForAgents(nowUTC string) (*AgentStats, error) {
	st := &AgentStats{BacklogByType: map[string]int{}, TopFailing: []AgentFailure{}}
	cutoff, err := time.Parse(time.RFC3339, nowUTC)
	if err != nil {
		return nil, err
	}
	seenCutoff := cutoff.Add(-OnlineCutoffMin * time.Minute).Format(time.RFC3339)
	row := s.ro.QueryRow(`SELECT
		SUM(CASE WHEN enabled=1 AND last_seen_at != '' AND last_seen_at >= ? THEN 1 ELSE 0 END),
		SUM(CASE WHEN enabled=1 AND (last_seen_at = '' OR last_seen_at < ?) THEN 1 ELSE 0 END),
		SUM(CASE WHEN enabled=0 THEN 1 ELSE 0 END) FROM agents`, seenCutoff, seenCutoff)
	var online, offline, paused sql.NullInt64
	if err := row.Scan(&online, &offline, &paused); err != nil {
		return nil, err
	}
	st.Online, st.Offline, st.Paused = int(nullToZero(online)), int(nullToZero(offline)), int(nullToZero(paused))
	rows, err := s.ro.Query(`SELECT type,COUNT(*) FROM jobs WHERE status='pending' GROUP BY type`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var typ string
		var n int
		if err := rows.Scan(&typ, &n); err != nil {
			rows.Close()
			return nil, err
		}
		st.BacklogByType[typ] = n
	}
	rows.Close()
	hourAgo := cutoff.Add(-time.Hour).Format(time.RFC3339)
	if err := s.ro.QueryRow(`SELECT COUNT(*) FROM jobs WHERE status='done' AND finished_at >= ?`, hourAgo).Scan(&st.Done1h); err != nil {
		return nil, err
	}
	if err := s.ro.QueryRow(`SELECT COUNT(*) FROM jobs WHERE status='failed' AND finished_at >= ?`, hourAgo).Scan(&st.Failed1h); err != nil {
		return nil, err
	}
	dayAgo := cutoff.Add(-24 * time.Hour).Format(time.RFC3339)
	frows, err := s.ro.Query(`SELECT j.agent_id,a.label,COUNT(*) FROM jobs j JOIN agents a ON a.id=j.agent_id
		WHERE j.status='failed' AND j.finished_at >= ? AND j.agent_id != 0
		GROUP BY j.agent_id,a.label ORDER BY COUNT(*) DESC LIMIT 5`, dayAgo)
	if err != nil {
		return nil, err
	}
	defer frows.Close()
	for frows.Next() {
		var f AgentFailure
		if err := frows.Scan(&f.AgentID, &f.Label, &f.Failed); err != nil {
			return nil, err
		}
		st.TopFailing = append(st.TopFailing, f)
	}
	return st, frows.Err()
}

func nullToZero(v sql.NullInt64) int64 {
	if !v.Valid {
		return 0
	}
	return v.Int64
}

// ListJobsByAgent returns an agent's newest jobs.
func (s *Store) ListJobsByAgent(agentID int64, limit int) ([]*Job, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.ro.Query(`SELECT `+jobCols+` FROM jobs WHERE agent_id=? ORDER BY id DESC LIMIT ?`, agentID, limit)
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

// AgentActivityMap carries per-agent live and 24h figures in three queries.
type AgentActivityMap struct {
	Claimed map[int64]int64
	Done    map[int64]int
	Failed  map[int64]int
}

// AgentActivity loads live claims plus 24h done/failed counts per agent.
func (s *Store) AgentActivity() (*AgentActivityMap, error) {
	m := &AgentActivityMap{Claimed: map[int64]int64{}, Done: map[int64]int{}, Failed: map[int64]int{}}
	rows, err := s.ro.Query(`SELECT agent_id,MIN(id) FROM jobs WHERE status='claimed' AND agent_id != 0 GROUP BY agent_id`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var aid, id int64
		if err := rows.Scan(&aid, &id); err != nil {
			rows.Close()
			return nil, err
		}
		m.Claimed[aid] = id
	}
	rows.Close()
	dayAgo := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	for status, dest := range map[string]map[int64]int{"done": m.Done, "failed": m.Failed} {
		r2, err := s.ro.Query(`SELECT agent_id,COUNT(*) FROM jobs WHERE status=? AND finished_at >= ? AND agent_id != 0 GROUP BY agent_id`, status, dayAgo)
		if err != nil {
			return nil, err
		}
		for r2.Next() {
			var aid int64
			var n int
			if err := r2.Scan(&aid, &n); err != nil {
				r2.Close()
				return nil, err
			}
			dest[aid] = n
		}
		r2.Close()
	}
	return m, nil
}
