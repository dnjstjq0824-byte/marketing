package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// W16 team routing: works and jobs belong to one team, and claims stay
// inside the team. NULL team ids resolve to the default team so legacy
// rows keep flowing.

// DefaultTeam returns the is_default team, creating it when missing.
func (s *Store) DefaultTeam() (*Team, error) {
	if t, err := s.GetDefaultTeam(); err == nil {
		return t, nil
	} else if err != ErrNotFound {
		return nil, err
	}
	now := nowUTC()
	res, err := s.db.Exec(`INSERT INTO teams(name,description,is_default,businesses,max_concurrency,role_mode,enabled,created_at,updated_at)
		VALUES('湲곕낯 ?','',1,'',0,'auto',1,?,?)`, now, now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetTeam(id)
}

// TeamByRef resolves a UI team reference: numeric id first, then exact
// name match. Empty returns (nil, nil): caller falls through to business.
func (s *Store) TeamByRef(ref string) (*Team, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, nil
	}
	if id, err := strconv.ParseInt(ref, 10, 64); err == nil && id > 0 {
		t, err := s.GetTeam(id)
		if err == nil {
			return t, nil
		}
	}
	teams, err := s.ListTeams()
	if err != nil {
		return nil, err
	}
	for _, t := range teams {
		if t.Name == ref {
			return t, nil
		}
	}
	return nil, ErrNotFound
}

// TeamForBusiness returns the team whose businesses CSV covers the name
// (case-insensitive). Empty business never matches.
func (s *Store) TeamForBusiness(business string) (*Team, error) {
	business = strings.TrimSpace(business)
	if business == "" {
		return nil, ErrNotFound
	}
	teams, err := s.ListTeams()
	if err != nil {
		return nil, err
	}
	for _, t := range teams {
		for _, b := range strings.Split(t.Businesses, ",") {
			if strings.EqualFold(strings.TrimSpace(b), business) {
				return t, nil
			}
		}
	}
	return nil, ErrNotFound
}

// ResolveWorkTeam applies W16-3 priority: user pick, business team, default.
func (s *Store) ResolveWorkTeam(pick, business string) (int64, error) {
	if t, err := s.TeamByRef(pick); err == nil && t != nil {
		return t.ID, nil
	} else if err != nil && err != ErrNotFound {
		return 0, err
	} else if t == nil && strings.TrimSpace(pick) != "" {
		return 0, fmt.Errorf("unknown team %q", pick)
	}
	if t, err := s.TeamForBusiness(business); err == nil {
		return t.ID, nil
	}
	def, err := s.DefaultTeam()
	if err != nil {
		return 0, err
	}
	return def.ID, nil
}

// SetProductTeam stamps work ownership.
func (s *Store) SetProductTeam(productID, teamID int64) error {
	_, err := s.db.Exec(`UPDATE products SET team_id=?, updated_at=? WHERE id=?`, teamID, nowUTC(), productID)
	return err
}

// SetJobTeam stamps job ownership.
func (s *Store) SetJobTeam(jobID, teamID int64) error {
	_, err := s.db.Exec(`UPDATE jobs SET team_id=? WHERE id=?`, teamID, jobID)
	return err
}

// EffectiveProductTeam resolves a work's team (NULL = default).
func (s *Store) EffectiveProductTeam(productID int64) (int64, error) {
	p, err := s.GetProduct(productID)
	if err != nil {
		return 0, err
	}
	if p.TeamID != nil && *p.TeamID > 0 {
		return *p.TeamID, nil
	}
	def, err := s.DefaultTeam()
	if err != nil {
		return 0, err
	}
	return def.ID, nil
}

// EffectiveJobTeam resolves a job's team: stamped id, else the owning
// work's team via ref/payload, else default.
func (s *Store) EffectiveJobTeam(j *Job) (int64, error) {
	if j.TeamID != nil && *j.TeamID > 0 {
		return *j.TeamID, nil
	}
	if pid := s.JobProductID(j); pid > 0 {
		return s.EffectiveProductTeam(pid)
	}
	def, err := s.DefaultTeam()
	if err != nil {
		return 0, err
	}
	return def.ID, nil
}

// JobProductID resolves the owning work id through payloads and refs.
func (s *Store) JobProductID(j *Job) int64 {
	var payload struct {
		ProductID  int64   `json:"product_id"`
		ContentID  int64   `json:"content_id"`
		ContentIDs []int64 `json:"content_ids"`
	}
	if err := json.Unmarshal([]byte(j.Payload), &payload); err == nil {
		if payload.ProductID > 0 {
			return payload.ProductID
		}
		ids := payload.ContentIDs
		if payload.ContentID > 0 {
			ids = append(ids, payload.ContentID)
		}
		for _, cid := range ids {
			if c, err := s.GetContent(cid); err == nil && c.ProductID > 0 {
				return c.ProductID
			}
		}
	}
	if p, err := s.GetProduct(j.RefID); err == nil && p != nil {
		return p.ID
	}
	return 0
}

// AgentTeam resolves an agent's team via its linked account. Unlinked
// agents return (0, nil): no team restriction (legacy/shared flows).
func (s *Store) AgentTeam(agent *Agent) (int64, error) {
	if agent.AccountID == nil {
		return 0, nil
	}
	teams, err := s.MemberTeams(*agent.AccountID)
	if err != nil {
		return 0, err
	}
	if len(teams) > 0 {
		return teams[0], nil
	}
	def, err := s.DefaultTeam()
	if err != nil {
		return 0, err
	}
	return def.ID, nil
}

// TeamWorkable reports whether a team can take work right now: enabled
// with at least one connected member. Single-member teams run A/B work;
// review self-exclusion (W6) holds their reviews until a second account
// joins, and team health flags the state.
func (s *Store) TeamWorkable(teamID int64) (bool, error) {
	t, err := s.GetTeam(teamID)
	if err != nil {
		return false, err
	}
	if !t.Enabled {
		return false, nil
	}
	var n int
	err = s.ro.QueryRow(`SELECT COUNT(*) FROM team_members m
		JOIN channel_accounts a ON a.id=m.account_id
		WHERE m.team_id=? AND a.session_state='connected'`, teamID).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// TeamRoleAgents lists member account ids holding a job-type role under
// role_mode=manual. Empty fixed_role means "any role". Nil return means
// no member holds the role: the caller falls back to auto rules.
func (s *Store) TeamRoleAgents(teamID int64, jobType string) ([]int64, error) {
	members, err := s.TeamMembers(teamID)
	if err != nil {
		return nil, err
	}
	var out []int64
	for _, m := range members {
		roles := strings.Split(m.FixedRole, ",")
		if strings.TrimSpace(m.FixedRole) == "" {
			out = append(out, m.AccountID)
			continue
		}
		for _, r := range roles {
			if strings.TrimSpace(r) == jobType {
				out = append(out, m.AccountID)
				break
			}
		}
	}
	return out, nil
}

// TeamMembers lists one team's memberships.
func (s *Store) TeamMembers(teamID int64) ([]*TeamMember, error) {
	rows, err := s.ro.Query(`SELECT team_id,account_id,fixed_role FROM team_members WHERE team_id=? ORDER BY account_id ASC`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*TeamMember
	for rows.Next() {
		var m TeamMember
		if err := rows.Scan(&m.TeamID, &m.AccountID, &m.FixedRole); err != nil {
			return nil, err
		}
		out = append(out, &m)
	}
	return out, rows.Err()
}

// HandoffAgeMin is the W16-3 wait before an unworkable team's job moves.
const HandoffAgeMin = 30

// TeamHandoff restamps one stale job of an unworkable/disabled team onto
// the default team and files one deduped notice. It returns false when
// the job is not handoff-eligible.
func (s *Store) TeamHandoff(j *Job, now time.Time) (bool, error) {
	teamID, err := s.EffectiveJobTeam(j)
	if err != nil {
		return false, err
	}
	def, err := s.DefaultTeam()
	if err != nil {
		return false, err
	}
	if teamID == def.ID {
		return false, nil
	}
	created, err := time.Parse(time.RFC3339, j.CreatedAt)
	if err != nil || now.Sub(created) < HandoffAgeMin*time.Minute {
		return false, nil
	}
	if ok, err := s.TeamWorkable(teamID); err != nil || ok {
		return false, err
	}
	if err := s.SetJobTeam(j.ID, def.ID); err != nil {
		return false, err
	}
	t, _ := s.GetTeam(teamID)
	name := ""
	if t != nil {
		name = t.Name
	}
	_, _ = s.CreateNotificationDedup("team_handoff", "warn", "job", j.ID,
		fmt.Sprintf("team_handoff:job:%d", j.ID),
		fmt.Sprintf("? %s ?묒뾽 遺덇? ??#%d 湲곕낯 ??쇰줈 ?닿?", name, j.ID))
	return true, nil
}

// ClaimedAILoad counts live analyze/strategy/generate/review claims for
// the PC-wide concurrency cap (W16-6).
func (s *Store) ClaimedAILoad() (int, error) {
	var n int
	err := s.ro.QueryRow(`SELECT COUNT(*) FROM jobs WHERE status='claimed'
		AND type IN ('analyze','strategy','generate','review','research')`).Scan(&n)
	return n, err
}

// TeamLoad counts one team's live AI claims for max_concurrency.
func (s *Store) TeamLoad(teamID int64) (int, error) {
	var n int
	err := s.ro.QueryRow(`SELECT COUNT(*) FROM jobs WHERE status='claimed' AND team_id=?
		AND type IN ('analyze','strategy','generate','review','research')`, teamID).Scan(&n)
	return n, err
}

// teamScopeFits keeps AI jobs inside the agent's team. Unlinked agents
// (agentTeam 0) keep legacy access to every team.
func (s *Store) teamScopeFits(j *Job, agent *Agent, agentTeam int64) bool {
	if agentTeam == 0 {
		return true
	}
	jobTeam, err := s.EffectiveJobTeam(j)
	if err != nil {
		return true
	}
	if jobTeam != agentTeam {
		return false
	}
	// Paused teams wait; the 30-minute handoff moves stale jobs out.
	if t, err := s.GetTeam(jobTeam); err == nil && !t.Enabled {
		return false
	}
	return true
}

// teamRoleFits enforces role_mode=manual fixed roles. Teams without a
// holder for the job type fall back to auto rules (any member).
func (s *Store) teamRoleFits(j *Job, agent *Agent) bool {
	teamID, err := s.EffectiveJobTeam(j)
	if err != nil {
		return true
	}
	t, err := s.GetTeam(teamID)
	if err != nil || t.RoleMode != "manual" {
		return true
	}
	role := j.Type
	if role == "research" {
		role = "analyze" // topic reading rides the analyze role
	}
	holders, err := s.TeamRoleAgents(teamID, role)
	if err != nil || len(holders) == 0 {
		return true
	}
	if agent.AccountID == nil {
		return true
	}
	for _, id := range holders {
		if id == *agent.AccountID {
			return true
		}
	}
	return false
}

// reviewLeaderGate reserves review claims for the team leader when one is
// designated and able. Anything that disqualifies the leader (offline,
// writer, no review worker, at capacity) opens the claim to the team.
func (s *Store) reviewLeaderGate(conn interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}, j *Job, agent *Agent) bool {
	if agent.AccountID == nil {
		return true // unlinked legacy flows bypass reservation
	}
	teamID, err := s.EffectiveJobTeam(j)
	if err != nil {
		return true
	}
	t, err := s.GetTeam(teamID)
	if err != nil || t.LeaderAccountID == nil {
		return true
	}
	leader := *t.LeaderAccountID
	if agent.AccountID != nil && *agent.AccountID == leader {
		return true
	}
	acc, err := s.GetAccountRow(leader)
	if err != nil || acc.SessionState != "connected" {
		return true
	}
	if !s.reviewFitsAgent(conn, j, &Agent{AccountID: &leader}) {
		return true // leader wrote it: anyone else may review
	}
	workers, err := s.AgentsForAccount(leader, "C")
	if err != nil || len(workers) == 0 {
		return true
	}
	for _, w := range workers {
		live, err := s.ClaimedCountByAgent(w.ID)
		if err == nil && live < w.Capacity {
			return false // an able leader worker exists: wait for it
		}
	}
	return true
}

// AgentsForAccount lists agents bound to one account, optionally one lane.
func (s *Store) AgentsForAccount(accountID int64, lane string) ([]*Agent, error) {
	q := `SELECT ` + agentCols + ` FROM agents WHERE account_id=?`
	args := []any{accountID}
	if lane != "" {
		q += ` AND lane=?`
		args = append(args, lane)
	}
	rows, err := s.ro.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Agent
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// teamCapFits enforces max_concurrency (0 = member count).
func (s *Store) teamCapFits(j *Job) bool {
	teamID, err := s.EffectiveJobTeam(j)
	if err != nil {
		return true
	}
	t, err := s.GetTeam(teamID)
	if err != nil {
		return true
	}
	max := t.MaxConcurrency
	if max <= 0 {
		members, err := s.TeamMembers(teamID)
		if err != nil {
			return true
		}
		max = len(members)
	}
	if max <= 0 {
		// Unmanaged team (no members yet): legacy linked agents keep
		// flowing instead of stalling on a zero cap.
		return true
	}
	load, err := s.TeamLoad(teamID)
	if err != nil {
		return true
	}
	return load < max
}

// rotateTeams orders candidates team-major from the round-robin cursor so
// waiting teams take turns. Single-team sets keep priority/id order.
func (s *Store) rotateTeams(cands []*Job) []*Job {
	if len(cands) < 2 {
		return cands
	}
	teams := map[int64][]*Job{}
	var order []int64
	for _, j := range cands {
		tid, err := s.EffectiveJobTeam(j)
		if err != nil {
			tid = 0
		}
		if _, ok := teams[tid]; !ok {
			order = append(order, tid)
		}
		teams[tid] = append(teams[tid], j)
	}
	if len(order) < 2 {
		return cands
	}
	cursor := s.RoundRobinCursor()
	start := 0
	for i, tid := range order {
		if tid == cursor {
			start = (i + 1) % len(order)
		}
	}
	var out []*Job
	for k := 0; k < len(order); k++ {
		out = append(out, teams[order[(start+k)%len(order)]]...)
	}
	return out
}

// ReassignTeamWorks moves one team's works and jobs to another team
// (team delete migration, W16-8).
func (s *Store) ReassignTeamWorks(fromID, toID int64) error {
	if _, err := s.db.Exec(`UPDATE products SET team_id=? WHERE team_id=?`, toID, fromID); err != nil {
		return err
	}
	_, err := s.db.Exec(`UPDATE jobs SET team_id=? WHERE team_id=?`, toID, fromID)
	return err
}

// sweepHandoffs moves stale jobs of unworkable teams to the default team
// (W16-3). Idempotent and bounded; claim calls it before routing.
func (s *Store) sweepHandoffs(now time.Time) {
	rows, err := s.ro.Query(`SELECT `+jobCols+` FROM jobs WHERE status='pending'
		AND type IN ('analyze','strategy','generate','review','research')
		ORDER BY id ASC LIMIT 200`)
	if err != nil {
		return
	}
	var jobs []*Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			rows.Close()
			return
		}
		jobs = append(jobs, j)
	}
	rows.Close()
	for _, j := range jobs {
		s.TeamHandoff(j, now)
	}
}

// maxConcurrent reads the PC-wide AI cap from automation settings.
func (s *Store) maxConcurrent() (int, error) {
	st, err := GetAutomationSettings(s.db)
	if err != nil {
		return 0, err
	}
	return st.MaxConcurrent, nil
}

// RoundRobinCursor reads the last served team for fair rotation.
func (s *Store) RoundRobinCursor() int64 {
	var raw string
	if err := s.ro.QueryRow(`SELECT v FROM app_meta WHERE k='rr_team_cursor'`).Scan(&raw); err != nil {
		return 0
	}
	id, _ := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	return id
}

// SetRoundRobinCursor records the last served team.
func (s *Store) SetRoundRobinCursor(teamID int64) {
	_, _ = s.db.Exec(`INSERT INTO app_meta(k,v) VALUES('rr_team_cursor',?)
		ON CONFLICT(k) DO UPDATE SET v=excluded.v`, strconv.FormatInt(teamID, 10))
}
