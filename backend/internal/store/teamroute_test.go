package store

import (
	"path/filepath"
	"testing"
	"time"
)

func openTeamStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "team.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := Migrate(st.DB()); err != nil {
		t.Fatal(err)
	}
	return st
}

func seedTeamAccount(t *testing.T, st *Store, name, platform string) *ChannelAccount {
	t.Helper()
	a, err := st.CreateAccountRow(&ChannelAccount{Name: name, Platform: platform,
		Role: "worker", SessionState: "connected", PostingMode: "auto"})
	if err != nil {
		t.Fatal(err)
	}
	a.Kind = "agent_service"
	if err := st.UpdateAccountRow(a); err != nil {
		t.Fatal(err)
	}
	return a
}

// W16-3: user pick wins, then business team, then default.
func TestResolveWorkTeam(t *testing.T) {
	st := openTeamStore(t)
	mkt, err := st.CreateTeam("마케팅1팀", "", "LUNO", 0, "auto")
	if err != nil {
		t.Fatal(err)
	}
	def, err := st.DefaultTeam()
	if err != nil {
		t.Fatal(err)
	}
	if got, err := st.ResolveWorkTeam("마케팅1팀", "OTHER"); err != nil || got != mkt.ID {
		t.Fatalf("pick = %d, %v", got, err)
	}
	if got, err := st.ResolveWorkTeam("", "luno"); err != nil || got != mkt.ID {
		t.Fatalf("business = %d, %v", got, err)
	}
	if got, err := st.ResolveWorkTeam("", ""); err != nil || got != def.ID {
		t.Fatalf("default = %d, %v", got, err)
	}
	if _, err := st.ResolveWorkTeam("없는팀", ""); err == nil {
		t.Fatal("unknown team accepted")
	}
}

// W16: claims stay inside the agent's team; strangers cannot steal.
func TestClaimTeamScope(t *testing.T) {
	st := openTeamStore(t)
	t1, _ := st.CreateTeam("T1", "", "A", 0, "auto")
	t2, _ := st.CreateTeam("T2", "", "B", 0, "auto")
	a1 := seedTeamAccount(t, st, "a1", "meta_ai")
	a2 := seedTeamAccount(t, st, "a2", "claude_app")
	if err := st.MoveMemberToTeam(t1.ID, a1.ID, ""); err != nil {
		t.Fatal(err)
	}
	if err := st.MoveMemberToTeam(t2.ID, a2.ID, ""); err != nil {
		t.Fatal(err)
	}
	ag1, _, _ := st.CreateAgent("g1", "B", "", "", "", 1, &a1.ID)
	ag2, _, _ := st.CreateAgent("g2", "B", "", "", "", 1, &a2.ID)
	p, _ := st.CreateProduct("https://example.com/t1", "A", 0, "A")
	if err := st.SetProductTeam(p.ID, t1.ID); err != nil {
		t.Fatal(err)
	}
	j, _ := st.CreateJob("generate", p.ID, "{}")
	if err := st.SetJobTeam(j.ID, t1.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := st.ClaimRoutedJob(ag2, []string{"generate"}, 600); err != nil || got != nil {
		t.Fatalf("cross-team claim = %+v, %v; want nil", got, err)
	}
	if got, err := st.ClaimRoutedJob(ag1, []string{"generate"}, 600); err != nil || got == nil {
		t.Fatalf("same-team claim = %+v, %v", got, err)
	}
}

// W16-4 manual roles: only holders claim; missing roles fall back to auto.
func TestClaimManualRoles(t *testing.T) {
	st := openTeamStore(t)
	tm, _ := st.CreateTeam("TM", "", "", 0, "manual")
	gen := seedTeamAccount(t, st, "gen", "meta_ai")
	rev := seedTeamAccount(t, st, "rev", "claude_app")
	if err := st.MoveMemberToTeam(tm.ID, gen.ID, "generate"); err != nil {
		t.Fatal(err)
	}
	if err := st.MoveMemberToTeam(tm.ID, rev.ID, "review"); err != nil {
		t.Fatal(err)
	}
	agG, _, _ := st.CreateAgent("gG", "B", "", "", "", 1, &gen.ID)
	agR, _, _ := st.CreateAgent("gR", "C", "", "", "", 1, &rev.ID)
	p, _ := st.CreateProduct("https://example.com/tm", "A", 0, "")
	st.SetProductTeam(p.ID, tm.ID)
	j, _ := st.CreateJob("generate", p.ID, "{}")
	st.SetJobTeam(j.ID, tm.ID)
	if got, err := st.ClaimRoutedJob(agR, []string{"generate"}, 600); err != nil || got != nil {
		t.Fatalf("wrong-role claim = %+v, %v; want nil", got, err)
	}
	if got, err := st.ClaimRoutedJob(agG, []string{"generate"}, 600); err != nil || got == nil {
		t.Fatalf("role claim = %+v, %v", got, err)
	}
	// No analyze holder on the team: auto fallback lets anyone take it.
	j2, _ := st.CreateJob("analyze", p.ID, "{}")
	st.SetJobTeam(j2.ID, tm.ID)
	if got, err := st.ClaimRoutedJob(agR, []string{"analyze"}, 600); err != nil || got == nil {
		t.Fatalf("fallback claim = %+v, %v", got, err)
	}
}

// W16-4: the designated leader reviews when able; otherwise the team may.
func TestClaimLeaderReview(t *testing.T) {
	st := openTeamStore(t)
	tm, _ := st.CreateTeam("TL", "", "", 0, "auto")
	wr := seedTeamAccount(t, st, "writer", "meta_ai")
	ld := seedTeamAccount(t, st, "leader", "claude_app")
	mb := seedTeamAccount(t, st, "member", "chatgpt")
	for _, a := range []*ChannelAccount{wr, ld, mb} {
		if err := st.MoveMemberToTeam(tm.ID, a.ID, ""); err != nil {
			t.Fatal(err)
		}
	}
	full, _ := st.GetTeam(tm.ID)
	full.LeaderAccountID = &ld.ID
	if _, err := st.DB().Exec(`UPDATE teams SET leader_account_id=? WHERE id=?`, ld.ID, tm.ID); err != nil {
		t.Fatal(err)
	}
	_ = full
	agL, _, _ := st.CreateAgent("gL", "C", "", "", "", 1, &ld.ID)
	agM, _, _ := st.CreateAgent("gM", "C", "", "", "", 1, &mb.ID)
	p, _ := st.CreateProduct("https://example.com/tl", "A", 0, "")
	st.SetProductTeam(p.ID, tm.ID)
	c, _ := st.CreateContent(&Content{ProductID: p.ID, Channel: "blog", Title: "t", Body: "b",
		GeneratedByAccountID: &wr.ID})
	j, _ := st.CreateJob("review", p.ID, `{"content_ids":[`+itoa2(c.ID)+`]}`)
	st.SetJobTeam(j.ID, tm.ID)
	// Leader able: member waits.
	if got, err := st.ClaimRoutedJob(agM, []string{"review"}, 600); err != nil || got != nil {
		t.Fatalf("member claim with able leader = %+v, %v; want nil", got, err)
	}
	if got, err := st.ClaimRoutedJob(agL, []string{"review"}, 600); err != nil || got == nil {
		t.Fatalf("leader claim = %+v, %v", got, err)
	}
}

// W16-3: stale jobs of dead teams hand off to default with one notice.
func TestTeamHandoff(t *testing.T) {
	st := openTeamStore(t)
	dead, _ := st.CreateTeam("DEAD", "", "", 0, "auto")
	def, _ := st.DefaultTeam()
	p, _ := st.CreateProduct("https://example.com/dead", "A", 0, "")
	st.SetProductTeam(p.ID, dead.ID)
	j, _ := st.CreateJob("generate", p.ID, "{}")
	st.SetJobTeam(j.ID, dead.ID)
	old := time.Now().UTC().Add(-31 * time.Minute).Format(time.RFC3339)
	if _, err := st.DB().Exec(`UPDATE jobs SET created_at=? WHERE id=?`, old, j.ID); err != nil {
		t.Fatal(err)
	}
	jj, _ := st.GetJob(j.ID)
	moved, err := st.TeamHandoff(jj, time.Now())
	if err != nil || !moved {
		t.Fatalf("handoff = %v, %v; want true", moved, err)
	}
	got, _ := st.GetJob(j.ID)
	if got.TeamID == nil || *got.TeamID != def.ID {
		t.Fatalf("team = %+v, want default", got.TeamID)
	}
	var n int
	st.DB().QueryRow(`SELECT COUNT(*) FROM notifications WHERE kind='team_handoff' AND ref_id=?`, j.ID).Scan(&n)
	if n != 1 {
		t.Fatalf("notices = %d, want 1", n)
	}
	// Fresh jobs stay put even on dead teams.
	j2, _ := st.CreateJob("generate", p.ID, "{}")
	st.SetJobTeam(j2.ID, dead.ID)
	jj2, _ := st.GetJob(j2.ID)
	if moved, _ := st.TeamHandoff(jj2, time.Now()); moved {
		t.Fatal("fresh job handed off")
	}
}

// W16-6: PC cap and team caps gate claims.
func TestConcurrencyCaps(t *testing.T) {
	st := openTeamStore(t)
	tm, _ := st.CreateTeam("TC", "", "", 1, "auto")
	a1 := seedTeamAccount(t, st, "c1", "meta_ai")
	st.MoveMemberToTeam(tm.ID, a1.ID, "")
	ag, _, _ := st.CreateAgent("gC", "B", "", "", "", 10, &a1.ID)
	p, _ := st.CreateProduct("https://example.com/tc", "A", 0, "")
	st.SetProductTeam(p.ID, tm.ID)
	j1, _ := st.CreateJob("generate", p.ID, "{}")
	st.SetJobTeam(j1.ID, tm.ID)
	j2, _ := st.CreateJob("generate", p.ID, "{}")
	st.SetJobTeam(j2.ID, tm.ID)
	if _, err := st.ClaimRoutedJob(ag, []string{"generate"}, 600); err != nil {
		t.Fatal(err)
	}
	// Team cap 1 reached: second claim waits.
	if got, err := st.ClaimRoutedJob(ag, []string{"generate"}, 600); err != nil || got != nil {
		t.Fatalf("over-cap claim = %+v, %v; want nil", got, err)
	}
	_ = j1
	_ = j2
}

// W16-8: pausing a team parks its fresh jobs; the other team runs on.
func TestPausedTeamWaits(t *testing.T) {
	st := openTeamStore(t)
	t1, _ := st.CreateTeam("P1", "", "", 0, "auto")
	t2, _ := st.CreateTeam("P2", "", "", 0, "auto")
	a1 := seedTeamAccount(t, st, "p1", "meta_ai")
	a2 := seedTeamAccount(t, st, "p2", "claude_app")
	st.MoveMemberToTeam(t1.ID, a1.ID, "")
	st.MoveMemberToTeam(t2.ID, a2.ID, "")
	ag1, _, _ := st.CreateAgent("gP1", "B", "", "", "", 1, &a1.ID)
	ag2, _, _ := st.CreateAgent("gP2", "B", "", "", "", 1, &a2.ID)
	p1, _ := st.CreateProduct("https://example.com/p1", "A", 0, "")
	p2, _ := st.CreateProduct("https://example.com/p2", "A", 0, "")
	st.SetProductTeam(p1.ID, t1.ID)
	st.SetProductTeam(p2.ID, t2.ID)
	j1, _ := st.CreateJob("generate", p1.ID, "{}")
	st.SetJobTeam(j1.ID, t1.ID)
	j2, _ := st.CreateJob("generate", p2.ID, "{}")
	st.SetJobTeam(j2.ID, t2.ID)
	full, _ := st.GetTeam(t1.ID)
	full.Enabled = false
	if _, err := st.DB().Exec(`UPDATE teams SET enabled=0 WHERE id=?`, t1.ID); err != nil {
		t.Fatal(err)
	}
	_ = full
	if got, err := st.ClaimRoutedJob(ag1, []string{"generate"}, 600); err != nil || got != nil {
		t.Fatalf("paused claim = %+v, %v; want nil", got, err)
	}
	if got, err := st.ClaimRoutedJob(ag2, []string{"generate"}, 600); err != nil || got == nil || got.ID != j2.ID {
		t.Fatalf("running claim = %+v, %v", got, err)
	}
	_ = j1
}

// W16-6: the PC-wide cap gates claims across teams.
func TestGlobalCap(t *testing.T) {
	st := openTeamStore(t)
	stt := DefaultAutomationSettings()
	stt.MaxConcurrent = 1
	if err := SetAutomationSettings(st.DB(), stt); err != nil {
		t.Fatal(err)
	}
	tm, _ := st.CreateTeam("TG", "", "", 10, "auto")
	a1 := seedTeamAccount(t, st, "g1", "meta_ai")
	st.MoveMemberToTeam(tm.ID, a1.ID, "")
	ag, _, _ := st.CreateAgent("gG", "B", "", "", "", 10, &a1.ID)
	p, _ := st.CreateProduct("https://example.com/tg", "A", 0, "")
	st.SetProductTeam(p.ID, tm.ID)
	for i := 0; i < 2; i++ {
		j, _ := st.CreateJob("generate", p.ID, "{}")
		st.SetJobTeam(j.ID, tm.ID)
	}
	if _, err := st.ClaimRoutedJob(ag, []string{"generate"}, 600); err != nil {
		t.Fatal(err)
	}
	if got, err := st.ClaimRoutedJob(ag, []string{"generate"}, 600); err != nil || got != nil {
		t.Fatalf("over-global-cap claim = %+v, %v; want nil", got, err)
	}
}

// W16-6: waiting teams take turns from the cursor.
func TestRotateTeams(t *testing.T) {
	st := openTeamStore(t)
	t1, _ := st.CreateTeam("R1", "", "", 0, "auto")
	t2, _ := st.CreateTeam("R2", "", "", 0, "auto")
	p1, _ := st.CreateProduct("https://example.com/r1", "A", 0, "")
	p2, _ := st.CreateProduct("https://example.com/r2", "A", 0, "")
	st.SetProductTeam(p1.ID, t1.ID)
	st.SetProductTeam(p2.ID, t2.ID)
	j1, _ := st.CreateJob("generate", p1.ID, "{}")
	st.SetJobTeam(j1.ID, t1.ID)
	j2, _ := st.CreateJob("generate", p2.ID, "{}")
	st.SetJobTeam(j2.ID, t2.ID)
	fresh := func() []*Job {
		a, _ := st.GetJob(j1.ID)
		b, _ := st.GetJob(j2.ID)
		return []*Job{a, b}
	}
	if got := st.rotateTeams(fresh()); got[0].ID != j1.ID {
		t.Fatalf("first rotation = %d, want %d", got[0].ID, j1.ID)
	}
	st.SetRoundRobinCursor(t1.ID)
	if got := st.rotateTeams(fresh()); got[0].ID != j2.ID {
		t.Fatalf("second rotation = %d, want %d", got[0].ID, j2.ID)
	}
	st.SetRoundRobinCursor(t2.ID)
	if got := st.rotateTeams(fresh()); got[0].ID != j1.ID {
		t.Fatalf("wrap rotation = %d, want %d", got[0].ID, j1.ID)
	}
}

func itoa2(n int64) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
