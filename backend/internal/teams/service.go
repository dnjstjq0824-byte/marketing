// Package teams implements marketing teams (W16): free-size groups of AI
// agents with an optional leader, dedicated businesses, and channels.
package teams

import (
	"errors"
	"strings"

	"somun/internal/store"
)

var (
	ErrNoTeam         = errors.New("team not found")
	ErrNoName         = errors.New("team name is required")
	ErrDupName        = errors.New("team name already exists")
	ErrBusinessTaken  = errors.New("business already belongs to another team")
	ErrBadRoleMode    = errors.New("role_mode must be auto or manual")
	ErrDefaultProtect = errors.New("default team cannot be deleted")
	ErrNotMember      = errors.New("account is not a team member")
	ErrBadRole        = errors.New("unknown role")
	ErrNoAccount      = errors.New("account not found")
	ErrNotAgent       = errors.New("only AI agent accounts can be team members")
	ErrNotChannel     = errors.New("only publishing accounts can be team channels")
	ErrDupMember      = errors.New("duplicate account in member list")
)

var validRoles = map[string]bool{
	"analyze": true, "strategy": true, "generate": true, "review": true,
}

// Service manages teams.
type Service struct {
	store *store.Store
}

// NewService builds the teams service.
func NewService(st *store.Store) *Service { return &Service{store: st} }

// Create validates and inserts one team.
func (s *Service) Create(name, description string, businesses []string, roleMode string) (*store.Team, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrNoName
	}
	if roleMode == "" {
		roleMode = "auto"
	}
	if roleMode != "auto" && roleMode != "manual" {
		return nil, ErrBadRoleMode
	}
	clean, err := s.checkBusinesses(businesses, 0)
	if err != nil {
		return nil, err
	}
	t, err := s.store.CreateTeam(name, strings.TrimSpace(description), strings.Join(clean, ","), 0, roleMode)
	if err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			return nil, ErrDupName
		}
		return nil, err
	}
	return t, nil
}

func (s *Service) checkBusinesses(businesses []string, excludeID int64) ([]string, error) {
	var clean []string
	seen := map[string]bool{}
	for _, b := range businesses {
		b = strings.TrimSpace(b)
		if b == "" || seen[strings.ToLower(b)] {
			continue
		}
		seen[strings.ToLower(b)] = true
		owner, err := s.store.TeamBusinessOwner(b, excludeID)
		if err != nil {
			return nil, err
		}
		if owner != nil {
			return nil, ErrBusinessTaken
		}
		clean = append(clean, b)
	}
	return clean, nil
}

// Update writes mutable fields; nil pointers leave values alone.
func (s *Service) Update(id int64, name, description *string, businesses []string, setBusinesses bool,
	roleMode *string, maxConcurrency *int, enabled *bool) (*store.Team, error) {
	t, err := s.store.GetTeam(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNoTeam
		}
		return nil, err
	}
	if name != nil {
		if strings.TrimSpace(*name) == "" {
			return nil, ErrNoName
		}
		t.Name = strings.TrimSpace(*name)
	}
	if description != nil {
		t.Description = *description
	}
	if setBusinesses {
		clean, err := s.checkBusinesses(businesses, id)
		if err != nil {
			return nil, err
		}
		t.Businesses = strings.Join(clean, ",")
	}
	if roleMode != nil {
		if *roleMode != "auto" && *roleMode != "manual" {
			return nil, ErrBadRoleMode
		}
		t.RoleMode = *roleMode
	}
	if maxConcurrency != nil && *maxConcurrency >= 0 {
		t.MaxConcurrency = *maxConcurrency
	}
	if enabled != nil {
		t.Enabled = *enabled
	}
	if err := s.store.UpdateTeam(t); err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			return nil, ErrDupName
		}
		return nil, err
	}
	return s.store.GetTeam(id)
}

// Delete removes a team after moving its members to the default team.
// Returns moved member count and running works count for the confirm text.
func (s *Service) Delete(id int64) (moved, running int, err error) {
	t, err := s.store.GetTeam(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return 0, 0, ErrNoTeam
		}
		return 0, 0, err
	}
	if t.IsDefault {
		return 0, 0, ErrDefaultProtect
	}
	def, err := s.store.GetDefaultTeam()
	if err != nil {
		return 0, 0, err
	}
	members, err := s.store.ListTeamMembers(id)
	if err != nil {
		return 0, 0, err
	}
	for _, m := range members {
		added, err := s.store.AddTeamMember(def.ID, m.AccountID)
		if err != nil {
			return 0, 0, err
		}
		if added {
			moved++
		}
	}
	running, err = s.store.CountTeamRunning(strings.Split(t.Businesses, ","))
	if err != nil {
		return 0, 0, err
	}
	if err := s.store.DeleteTeam(id); err != nil {
		return 0, 0, err
	}
	return moved, running, nil
}

// SetLeader appoints (nil clears) the team leader; must be a member.
func (s *Service) SetLeader(id int64, accountID *int64) (*store.Team, error) {
	t, err := s.store.GetTeam(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNoTeam
		}
		return nil, err
	}
	if accountID != nil {
		ok, err := s.store.IsTeamMember(id, *accountID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrNotMember
		}
		t.LeaderAccountID = accountID
	} else {
		t.LeaderAccountID = nil
	}
	if err := s.store.UpdateTeam(t); err != nil {
		return nil, err
	}
	return s.store.GetTeam(id)
}

// SetMemberRole sets fixed roles (subset of analyze/strategy/generate/review).
func (s *Service) SetMemberRole(teamID, accountID int64, roles []string) error {
	if _, err := s.store.GetTeam(teamID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return ErrNoTeam
		}
		return err
	}
	ok, err := s.store.IsTeamMember(teamID, accountID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotMember
	}
	clean, err := cleanRoles(roles)
	if err != nil {
		return err
	}
	return s.store.SetMemberRole(teamID, accountID, clean)
}

func cleanRoles(roles []string) (string, error) {
	var clean []string
	for _, r := range roles {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if !validRoles[r] {
			return "", ErrBadRole
		}
		clean = append(clean, r)
	}
	return strings.Join(clean, ","), nil
}

func (s *Service) requireTeam(id int64) (*store.Team, error) {
	t, err := s.store.GetTeam(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNoTeam
		}
		return nil, err
	}
	return t, nil
}

func (s *Service) requireAccount(id int64) (*store.ChannelAccount, error) {
	a, err := s.store.GetAccountRow(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNoAccount
		}
		return nil, err
	}
	return a, nil
}

// AddMember moves an AI agent into the team (single-team rule).
func (s *Service) AddMember(teamID, accountID int64) error {
	if _, err := s.requireTeam(teamID); err != nil {
		return err
	}
	a, err := s.requireAccount(accountID)
	if err != nil {
		return err
	}
	if a.Kind != "agent_service" {
		return ErrNotAgent
	}
	return s.store.MoveMemberToTeam(teamID, accountID, "")
}

// RemoveMember drops one membership; a departing leader is cleared.
func (s *Service) RemoveMember(teamID, accountID int64) error {
	if _, err := s.requireTeam(teamID); err != nil {
		return err
	}
	ok, err := s.store.IsTeamMember(teamID, accountID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotMember
	}
	return s.store.RemoveMemberFromTeam(teamID, accountID)
}

// MemberAssign is one batch membership row.
type MemberAssign struct {
	AccountID int64
	Roles     []string
}

// SetMembers replaces the whole membership (PUT /members): every listed
// account moves in with its roles, everyone else leaves. A leader who
// leaves is cleared; unknown or non-AI accounts abort the whole batch.
func (s *Service) SetMembers(teamID int64, list []MemberAssign) error {
	t, err := s.requireTeam(teamID)
	if err != nil {
		return err
	}
	seen := map[int64]bool{}
	cleaned := make([]MemberAssign, 0, len(list))
	for _, m := range list {
		if seen[m.AccountID] {
			return ErrDupMember
		}
		seen[m.AccountID] = true
		a, err := s.requireAccount(m.AccountID)
		if err != nil {
			return err
		}
		if a.Kind != "agent_service" {
			return ErrNotAgent
		}
		if _, err := cleanRoles(m.Roles); err != nil {
			return err
		}
		cleaned = append(cleaned, m)
	}
	current, err := s.store.ListTeamMembers(teamID)
	if err != nil {
		return err
	}
	for _, m := range current {
		if !seen[m.AccountID] {
			if err := s.store.RemoveMemberFromTeam(teamID, m.AccountID); err != nil {
				return err
			}
		}
	}
	for _, m := range cleaned {
		roles, _ := cleanRoles(m.Roles)
		if err := s.store.MoveMemberToTeam(teamID, m.AccountID, roles); err != nil {
			return err
		}
	}
	if t.LeaderAccountID != nil && !seen[*t.LeaderAccountID] {
		t.LeaderAccountID = nil
		return s.store.UpdateTeam(t)
	}
	return nil
}

// AddChannel links a publishing account; channels are shareable.
func (s *Service) AddChannel(teamID, accountID int64) error {
	if _, err := s.requireTeam(teamID); err != nil {
		return err
	}
	a, err := s.requireAccount(accountID)
	if err != nil {
		return err
	}
	if a.Kind != "channel" {
		return ErrNotChannel
	}
	_, err = s.store.AddTeamChannel(teamID, accountID)
	return err
}

// RemoveChannel unlinks one channel account.
func (s *Service) RemoveChannel(teamID, accountID int64) error {
	if _, err := s.requireTeam(teamID); err != nil {
		return err
	}
	return s.store.RemoveTeamChannel(teamID, accountID)
}

// SetChannels replaces the whole channel list (PUT /channels).
func (s *Service) SetChannels(teamID int64, accountIDs []int64) error {
	if _, err := s.requireTeam(teamID); err != nil {
		return err
	}
	seen := map[int64]bool{}
	var clean []int64
	for _, id := range accountIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		a, err := s.requireAccount(id)
		if err != nil {
			return err
		}
		if a.Kind != "channel" {
			return ErrNotChannel
		}
		clean = append(clean, id)
	}
	return s.store.SetTeamChannels(teamID, clean)
}

// UpsertBrand writes the team's brand card.
func (s *Service) UpsertBrand(teamID int64, intro, tone, banned, buyLink string) (*store.TeamBrand, error) {
	if _, err := s.requireTeam(teamID); err != nil {
		return nil, err
	}
	return s.store.UpsertTeamBrand(teamID,
		strings.TrimSpace(intro), strings.TrimSpace(tone),
		strings.TrimSpace(banned), strings.TrimSpace(buyLink))
}

func splitBusinesses(raw string) []string {
	var out []string
	for _, b := range strings.Split(raw, ",") {
		if b = strings.TrimSpace(b); b != "" {
			out = append(out, b)
		}
	}
	return out
}

// workingSplit approximates working/idle from running works until W4 ships
// real per-agent assignment.
func workingSplit(running, members int) (working, idle int) {
	working = running
	if working > members {
		working = members
	}
	if working < 0 {
		working = 0
	}
	return working, members - working
}

// TeamCard is one S7 list row.
type TeamCard struct {
	Team         *store.Team  `json:"team"`
	Businesses   []string     `json:"businesses"`
	MemberCount  int          `json:"memberCount"`
	ChannelCount int          `json:"channelCount"`
	Working      int          `json:"working"`
	Idle         int          `json:"idle"`
	Faces        []string     `json:"faces"`
	Unready      int          `json:"unready"`
	Stats        *store.TeamStats `json:"stats"`
}

// ListCards returns every team with its card numbers.
func (s *Service) ListCards() ([]*TeamCard, error) {
	teams, err := s.store.ListTeams()
	if err != nil {
		return nil, err
	}
	out := []*TeamCard{}
	for _, t := range teams {
		card, err := s.card(t)
		if err != nil {
			return nil, err
		}
		out = append(out, card)
	}
	return out, nil
}

func (s *Service) card(t *store.Team) (*TeamCard, error) {
	members, err := s.store.ListTeamMembers(t.ID)
	if err != nil {
		return nil, err
	}
	channels, err := s.store.ListTeamChannelIDs(t.ID)
	if err != nil {
		return nil, err
	}
	biz := splitBusinesses(t.Businesses)
	var owned []string
	if len(biz) == 0 {
		owned, err = s.store.OwnedBusinesses(t.ID)
		if err != nil {
			return nil, err
		}
	}
	stats, err := s.store.TeamStatsFor(biz, owned)
	if err != nil {
		return nil, err
	}
	faces, err := s.store.MemberPlatforms(t.ID)
	if err != nil {
		return nil, err
	}
	if faces == nil {
		faces = []string{}
	}
	if len(faces) > 5 {
		faces = faces[:5]
	}
	if biz == nil {
		biz = []string{}
	}
	unready, err := s.store.MemberUnready(t.ID)
	if err != nil {
		return nil, err
	}
	working, idle := workingSplit(stats.Running, len(members))
	return &TeamCard{
		Team: t, Businesses: biz, MemberCount: len(members),
		ChannelCount: len(channels), Working: working, Idle: idle,
		Faces: faces, Unready: unready, Stats: stats,
	}, nil
}

// MemberView is one S7-D agent row.
type MemberView struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Platform     string `json:"platform"`
	SessionState string `json:"sessionState"`
	LastJobAt    string `json:"lastJobAt"`
	FixedRole    string `json:"fixedRole"`
	IsLeader     bool   `json:"isLeader"`
	// TodayDone stays 0 until W4 attributes completed works to agents.
	TodayDone int `json:"todayDone"`
}

// ChannelView is one S7-D channel row.
type ChannelView struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Platform     string `json:"platform"`
	SessionState string `json:"sessionState"`
	PostingMode  string `json:"postingMode"`
}

// TeamDetail is the S7-D payload: team, brand, members, works, channels, stats.
type TeamDetail struct {
	Team     *store.Team       `json:"team"`
	Brand    *store.TeamBrand  `json:"brand"`
	Members  []*MemberView     `json:"members"`
	Works    []*store.WorkItem `json:"works"`
	Channels []*ChannelView    `json:"channels"`
	Stats    *store.TeamStats  `json:"stats"`
}

// Detail assembles the S7-D payload.
func (s *Service) Detail(id int64) (*TeamDetail, error) {
	t, err := s.requireTeam(id)
	if err != nil {
		return nil, err
	}
	brand, err := s.store.GetTeamBrand(id)
	if err != nil {
		return nil, err
	}
	raw, err := s.store.ListTeamMembers(id)
	if err != nil {
		return nil, err
	}
	members := []*MemberView{}
	for _, m := range raw {
		a, err := s.store.GetAccountRow(m.AccountID)
		if err != nil {
			continue
		}
		members = append(members, &MemberView{
			ID: a.ID, Name: a.Name, Platform: a.Platform,
			SessionState: a.SessionState, LastJobAt: a.LastJobAt,
			FixedRole: m.FixedRole,
			IsLeader: t.LeaderAccountID != nil && *t.LeaderAccountID == m.AccountID,
		})
	}
	chIDs, err := s.store.ListTeamChannelIDs(id)
	if err != nil {
		return nil, err
	}
	channels := []*ChannelView{}
	for _, cid := range chIDs {
		a, err := s.store.GetAccountRow(cid)
		if err != nil {
			continue
		}
		channels = append(channels, &ChannelView{
			ID: a.ID, Name: a.Name, Platform: a.Platform,
			SessionState: a.SessionState, PostingMode: a.PostingMode,
		})
	}
	biz := splitBusinesses(t.Businesses)
	var owned []string
	works := []*store.WorkItem{}
	if len(biz) == 0 {
		owned, err = s.store.OwnedBusinesses(id)
		if err != nil {
			return nil, err
		}
		works, err = s.store.ListWorks("running", "", "", 1, 20)
		if err != nil {
			return nil, err
		}
	} else {
		for _, b := range biz {
			rows, err := s.store.ListWorks("running", "", b, 1, 20)
			if err != nil {
				return nil, err
			}
			works = append(works, rows...)
		}
		sortWorkIDsDesc(works)
		if len(works) > 20 {
			works = works[:20]
		}
	}
	stats, err := s.store.TeamStatsFor(biz, owned)
	if err != nil {
		return nil, err
	}
	return &TeamDetail{
		Team: t, Brand: brand, Members: members,
		Works: works, Channels: channels, Stats: stats,
	}, nil
}

func sortWorkIDsDesc(works []*store.WorkItem) {
	for i := 1; i < len(works); i++ {
		for j := i; j > 0 && works[j].ID > works[j-1].ID; j-- {
			works[j], works[j-1] = works[j-1], works[j]
		}
	}
}
