package store

import (
	"database/sql"
	"strings"
	"time"
)

// Team is one marketing team (W16).
type Team struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	IsDefault       bool   `json:"isDefault"`
	Businesses      string `json:"businesses"`
	MaxConcurrency  int    `json:"maxConcurrency"`
	RoleMode        string `json:"roleMode"`
	Enabled         bool   `json:"enabled"`
	LeaderAccountID *int64 `json:"leaderAccountId"`
	// DistributionMode all/one + DistributionCap bound multi-account
	// publishing (W21-4).
	DistributionMode string `json:"distributionMode"`
	DistributionCap  int    `json:"distributionCap"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

// TeamMember is one team membership with optional fixed roles (CSV).
type TeamMember struct {
	TeamID    int64  `json:"-"`
	AccountID int64  `json:"accountId"`
	FixedRole string `json:"fixedRole"`
}

const teamCols = `id,name,description,is_default,businesses,max_concurrency,role_mode,enabled,leader_account_id,distribution_mode,distribution_cap,created_at,updated_at`

func scanTeam(row interface{ Scan(...any) error }) (*Team, error) {
	t := &Team{}
	var isDefault, enabled int
	var leader sql.NullInt64
	err := row.Scan(&t.ID, &t.Name, &t.Description, &isDefault, &t.Businesses,
		&t.MaxConcurrency, &t.RoleMode, &enabled, &leader,
		&t.DistributionMode, &t.DistributionCap, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	t.IsDefault = isDefault != 0
	t.Enabled = enabled != 0
	if leader.Valid {
		v := leader.Int64
		t.LeaderAccountID = &v
	}
	return t, nil
}

// CreateTeam inserts one team.
func (s *Store) CreateTeam(name, description, businesses string, maxConcurrency int, roleMode string) (*Team, error) {
	now := nowUTC()
	res, err := s.db.Exec(`INSERT INTO teams(name,description,is_default,businesses,max_concurrency,role_mode,enabled,created_at,updated_at)
		VALUES(?,?,0,?,?,?,1,?,?)`, name, description, businesses, maxConcurrency, roleMode, now, now)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDuplicate
		}
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetTeam(id)
}

// GetTeam loads one team by id.
func (s *Store) GetTeam(id int64) (*Team, error) {
	t, err := scanTeam(s.ro.QueryRow(`SELECT `+teamCols+` FROM teams WHERE id=?`, id))
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return t, err
}

// GetDefaultTeam loads the seeded default team.
func (s *Store) GetDefaultTeam() (*Team, error) {
	t, err := scanTeam(s.ro.QueryRow(`SELECT ` + teamCols + ` FROM teams WHERE is_default=1 LIMIT 1`))
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return t, err
}

// ListTeams returns all teams, default first.
func (s *Store) ListTeams() ([]*Team, error) {
	rows, err := s.ro.Query(`SELECT ` + teamCols + ` FROM teams ORDER BY is_default DESC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Team{}
	for rows.Next() {
		t, err := scanTeam(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// UpdateTeam writes mutable team fields.
func (s *Store) UpdateTeam(t *Team) error {
	var leader any
	if t.LeaderAccountID != nil {
		leader = *t.LeaderAccountID
	}
	_, err := s.db.Exec(`UPDATE teams SET name=?,description=?,businesses=?,max_concurrency=?,role_mode=?,enabled=?,leader_account_id=?,distribution_mode=?,distribution_cap=?,updated_at=?
		WHERE id=?`, t.Name, t.Description, t.Businesses, t.MaxConcurrency, t.RoleMode,
		boolInt(t.Enabled), leader, t.DistributionMode, t.DistributionCap, nowUTC(), t.ID)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrDuplicate
		}
		return err
	}
	return nil
}

// DeleteTeam removes a team; members/channels cascade. Callers move members first.
func (s *Store) DeleteTeam(id int64) error {
	_, err := s.db.Exec(`DELETE FROM teams WHERE id=?`, id)
	return err
}

// AddTeamMember adds an account; duplicates are a no-op false.
func (s *Store) AddTeamMember(teamID, accountID int64) (bool, error) {
	res, err := s.db.Exec(`INSERT OR IGNORE INTO team_members(team_id,account_id,fixed_role) VALUES(?,?,'')`, teamID, accountID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// RemoveTeamMember drops one membership.
func (s *Store) RemoveTeamMember(teamID, accountID int64) error {
	_, err := s.db.Exec(`DELETE FROM team_members WHERE team_id=? AND account_id=?`, teamID, accountID)
	return err
}

// SetMemberRole sets fixed roles (CSV) on one membership.
func (s *Store) SetMemberRole(teamID, accountID int64, roles string) error {
	_, err := s.db.Exec(`UPDATE team_members SET fixed_role=? WHERE team_id=? AND account_id=?`, roles, teamID, accountID)
	return err
}

// ListTeamMembers returns one team's memberships by account id.
func (s *Store) ListTeamMembers(teamID int64) ([]*TeamMember, error) {
	rows, err := s.ro.Query(`SELECT team_id,account_id,fixed_role FROM team_members WHERE team_id=? ORDER BY account_id ASC`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*TeamMember{}
	for rows.Next() {
		var m TeamMember
		if err := rows.Scan(&m.TeamID, &m.AccountID, &m.FixedRole); err != nil {
			return nil, err
		}
		out = append(out, &m)
	}
	return out, rows.Err()
}

// MemberPlatforms returns member platforms in account order.
func (s *Store) MemberPlatforms(teamID int64) ([]string, error) {
	rows, err := s.ro.Query(`SELECT a.platform FROM team_members m
		JOIN channel_accounts a ON a.id=m.account_id WHERE m.team_id=? ORDER BY m.account_id ASC`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// MemberUnready counts members whose session is not connected.
func (s *Store) MemberUnready(teamID int64) (int, error) {
	var n int
	err := s.ro.QueryRow(`SELECT COUNT(*) FROM team_members m
		JOIN channel_accounts a ON a.id=m.account_id
		WHERE m.team_id=? AND a.session_state<> 'connected'`, teamID).Scan(&n)
	return n, err
}

// IsTeamMember reports membership.
func (s *Store) IsTeamMember(teamID, accountID int64) (bool, error) {
	var n int
	if err := s.ro.QueryRow(`SELECT COUNT(*) FROM team_members WHERE team_id=? AND account_id=?`, teamID, accountID).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

// AddTeamChannel links a channel account; duplicates are a no-op false.
func (s *Store) AddTeamChannel(teamID, accountID int64) (bool, error) {
	res, err := s.db.Exec(`INSERT OR IGNORE INTO team_channels(team_id,account_id) VALUES(?,?)`, teamID, accountID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// RemoveTeamChannel unlinks one channel account.
func (s *Store) RemoveTeamChannel(teamID, accountID int64) error {
	_, err := s.db.Exec(`DELETE FROM team_channels WHERE team_id=? AND account_id=?`, teamID, accountID)
	return err
}

// ListTeamChannelIDs returns one team's channel account ids.
func (s *Store) ListTeamChannelIDs(teamID int64) ([]int64, error) {
	rows, err := s.ro.Query(`SELECT account_id FROM team_channels WHERE team_id=? ORDER BY account_id ASC`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// CountTeamRunning counts running works across businesses.
func (s *Store) CountTeamRunning(businesses []string) (int, error) {
	var clean []string
	for _, b := range businesses {
		if b = strings.TrimSpace(b); b != "" {
			clean = append(clean, b)
		}
	}
	if len(clean) == 0 {
		return 0, nil
	}
	place := strings.Repeat("?,", len(clean))
	place = place[:len(place)-1]
	args := make([]any, 0, len(clean))
	for _, b := range clean {
		args = append(args, b)
	}
	var n int
	q := `SELECT COUNT(*) FROM products p WHERE ` + runningProductCond + ` AND p.business COLLATE NOCASE IN (` + place + `)`
	if err := s.ro.QueryRow(q, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// TeamBusinessOwner finds the team (if any) already owning a business.
func (s *Store) TeamBusinessOwner(business string, excludeID int64) (*Team, error) {
	teams, err := s.ListTeams()
	if err != nil {
		return nil, err
	}
	for _, t := range teams {
		if t.ID == excludeID {
			continue
		}
		for _, b := range strings.Split(t.Businesses, ",") {
			if strings.EqualFold(strings.TrimSpace(b), business) && business != "" {
				return t, nil
			}
		}
	}
	return nil, nil
}

// OwnedBusinesses returns every business claimed by any team but excludeID.
func (s *Store) OwnedBusinesses(excludeID int64) ([]string, error) {
	teams, err := s.ListTeams()
	if err != nil {
		return nil, err
	}
	var out []string
	seen := map[string]bool{}
	for _, t := range teams {
		if t.ID == excludeID {
			continue
		}
		for _, b := range strings.Split(t.Businesses, ",") {
			b = strings.TrimSpace(b)
			if b == "" || seen[strings.ToLower(b)] {
				continue
			}
			seen[strings.ToLower(b)] = true
			out = append(out, b)
		}
	}
	return out, nil
}

// MemberTeams lists team ids containing an account.
func (s *Store) MemberTeams(accountID int64) ([]int64, error) {
	rows, err := s.ro.Query(`SELECT team_id FROM team_members WHERE account_id=?`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// clearLeaderIf removes the leader pointer when it names accountID.
func (s *Store) clearLeaderIf(teamID, accountID int64) error {
	_, err := s.db.Exec(`UPDATE teams SET leader_account_id=NULL, updated_at=?
		WHERE id=? AND leader_account_id=?`, nowUTC(), teamID, accountID)
	return err
}

// MoveMemberToTeam moves an account into teamID, leaving every other team.
// A team whose leader leaves loses its leader.
func (s *Store) MoveMemberToTeam(teamID, accountID int64, roles string) error {
	old, err := s.MemberTeams(accountID)
	if err != nil {
		return err
	}
	for _, tid := range old {
		if tid == teamID {
			continue
		}
		if _, err := s.db.Exec(`DELETE FROM team_members WHERE team_id=? AND account_id=?`, tid, accountID); err != nil {
			return err
		}
		if err := s.clearLeaderIf(tid, accountID); err != nil {
			return err
		}
	}
	if _, err := s.db.Exec(`INSERT INTO team_members(team_id,account_id,fixed_role) VALUES(?,?,?)
		ON CONFLICT(team_id,account_id) DO UPDATE SET fixed_role=excluded.fixed_role`,
		teamID, accountID, roles); err != nil {
		return err
	}
	return nil
}

// RemoveMemberFromTeam drops one membership, clearing a matching leader.
func (s *Store) RemoveMemberFromTeam(teamID, accountID int64) error {
	if err := s.RemoveTeamMember(teamID, accountID); err != nil {
		return err
	}
	return s.clearLeaderIf(teamID, accountID)
}

// SetTeamChannels replaces one team's channel list (channels are shareable:
// other teams keep their own links).
func (s *Store) SetTeamChannels(teamID int64, accountIDs []int64) error {
	if _, err := s.db.Exec(`DELETE FROM team_channels WHERE team_id=?`, teamID); err != nil {
		return err
	}
	for _, id := range accountIDs {
		if _, err := s.db.Exec(`INSERT OR IGNORE INTO team_channels(team_id,account_id) VALUES(?,?)`, teamID, id); err != nil {
			return err
		}
	}
	return nil
}

// TeamBrand is one team's brand card (S7-D).
type TeamBrand struct {
	TeamID     int64  `json:"-"`
	Intro      string `json:"intro"`
	Tone       string `json:"tone"`
	Banned     string `json:"bannedWords"`
	BuyLink    string `json:"buyLink"`
	UpdatedAt  string `json:"updatedAt"`
	HasContent bool   `json:"hasContent"`
}

// GetTeamBrand loads one team's brand card; missing rows read as empty.
func (s *Store) GetTeamBrand(teamID int64) (*TeamBrand, error) {
	b := &TeamBrand{TeamID: teamID}
	err := s.ro.QueryRow(`SELECT intro,tone,banned_words,buy_link,updated_at FROM team_brands WHERE team_id=?`,
		teamID).Scan(&b.Intro, &b.Tone, &b.Banned, &b.BuyLink, &b.UpdatedAt)
	if err == sql.ErrNoRows {
		return b, nil
	}
	if err != nil {
		return nil, err
	}
	b.HasContent = b.Intro != "" || b.Tone != "" || b.Banned != "" || b.BuyLink != ""
	return b, nil
}

// UpsertTeamBrand writes one team's brand card.
func (s *Store) UpsertTeamBrand(teamID int64, intro, tone, banned, buyLink string) (*TeamBrand, error) {
	now := nowUTC()
	if _, err := s.db.Exec(`INSERT INTO team_brands(team_id,intro,tone,banned_words,buy_link,updated_at)
		VALUES(?,?,?,?,?,?) ON CONFLICT(team_id) DO UPDATE SET
		intro=excluded.intro,tone=excluded.tone,banned_words=excluded.banned_words,
		buy_link=excluded.buy_link,updated_at=excluded.updated_at`,
		teamID, intro, tone, banned, buyLink, now); err != nil {
		return nil, err
	}
	return s.GetTeamBrand(teamID)
}

// TeamStats is the S7/S7-D number row for one business scope.
type TeamStats struct {
	Running        int `json:"runningWorks"`
	TodayDone      int `json:"todayDone"`
	TodayPublished int `json:"todayPublished"`
	TodayFailed    int `json:"todayFailed"`
	WeekViews      int `json:"weekViews"`
}

// kstDayBounds returns UTC RFC3339 bounds for the current Korea calendar day.
func kstDayBounds(now time.Time) (string, string) {
	kst := time.FixedZone("KST", 9*3600)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, kst)
	return start.UTC().Format(time.RFC3339), start.Add(24 * time.Hour).UTC().Format(time.RFC3339)
}

func bizPlaceholders(n int) string {
	place := strings.Repeat("?,", n)
	return place[:len(place)-1]
}

// TeamStatsFor returns stats for works in scope. Empty scope with owned set
// counts unassigned works (business not owned by another team); empty scope
// with no owned set counts everything (fresh install, default team only).
func (s *Store) TeamStatsFor(businesses, owned []string) (*TeamStats, error) {
	st := &TeamStats{}
	var cond string
	var args []any
	push := func(list []string) {
		for _, b := range list {
			args = append(args, b)
		}
	}
	switch {
	case len(businesses) > 0:
		cond = `p.business COLLATE NOCASE IN (` + bizPlaceholders(len(businesses)) + `)`
		push(businesses)
	case len(owned) > 0:
		cond = `(p.business IS NULL OR p.business='' OR p.business COLLATE NOCASE NOT IN (` + bizPlaceholders(len(owned)) + `))`
		push(owned)
	default:
		cond = `1=1`
	}
	dayStart, dayEnd := kstDayBounds(time.Now())
	weekAgo := time.Now().Add(-7 * 24 * time.Hour).UTC().Format(time.RFC3339)

	q := `SELECT COUNT(*) FROM products p WHERE ` + runningProductCond + ` AND ` + cond
	if err := s.ro.QueryRow(q, args...).Scan(&st.Running); err != nil {
		return nil, err
	}
	logQ := func(status string) (string, []any) {
		la := append([]any{dayStart, dayEnd}, args...)
		return `SELECT COUNT(*) FROM publish_logs l JOIN contents c ON c.id=l.content_id
			JOIN products p ON p.id=c.product_id WHERE l.status=? AND COALESCE(NULLIF(l.executed_at,''),l.created_at)>=?
			AND COALESCE(NULLIF(l.executed_at,''),l.created_at)<? AND ` + cond, append([]any{status}, la...)
	}
	qd, lad := logQ("done")
	if err := s.ro.QueryRow(qd, lad...).Scan(&st.TodayPublished); err != nil {
		return nil, err
	}
	qf, laf := logQ("failed")
	if err := s.ro.QueryRow(qf, laf...).Scan(&st.TodayFailed); err != nil {
		return nil, err
	}
	st.TodayDone = st.TodayPublished + st.TodayFailed
	vq := `SELECT COALESCE(SUM(m.exposures),0) FROM metrics m JOIN contents c ON c.id=m.content_id
		JOIN products p ON p.id=c.product_id WHERE m.updated_at>=? AND ` + cond
	if err := s.ro.QueryRow(vq, append([]any{weekAgo}, args...)...).Scan(&st.WeekViews); err != nil {
		return nil, err
	}
	return st, nil
}
