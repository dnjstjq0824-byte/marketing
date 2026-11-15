package teams

import (
	"errors"
	"path/filepath"
	"testing"

	"somun/internal/store"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "teams.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := store.Migrate(st.DB()); err != nil {
		t.Fatal(err)
	}
	return st
}

func seedAccount(t *testing.T, st *store.Store, name, kind string) int64 {
	t.Helper()
	a, err := st.CreateAccountRow(&store.ChannelAccount{Name: name, Platform: "meta_ai",
		Role: "worker", SessionState: "connected", PostingMode: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	a.Kind = kind
	if err := st.UpdateAccountRow(a); err != nil {
		t.Fatal(err)
	}
	return a.ID
}

func TestCreateValidation(t *testing.T) {
	st := openTestStore(t)
	svc := NewService(st)
	if _, err := svc.Create("  ", "", nil, ""); !errors.Is(err, ErrNoName) {
		t.Fatalf("empty name err = %v", err)
	}
	if _, err := svc.Create("T", "", nil, "flat"); !errors.Is(err, ErrBadRoleMode) {
		t.Fatalf("bad roleMode err = %v", err)
	}
	a, err := svc.Create("M1", "d", []string{"LUNO"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if a.RoleMode != "auto" {
		t.Fatalf("roleMode = %s", a.RoleMode)
	}
	if _, err := svc.Create("M1", "", nil, ""); !errors.Is(err, ErrDupName) {
		t.Fatalf("dup name err = %v", err)
	}
	if _, err := svc.Create("M2", "", []string{"luno"}, ""); !errors.Is(err, ErrBusinessTaken) {
		t.Fatalf("business taken err = %v", err)
	}
	if _, err := svc.Create("M2", "", []string{"KUPAS"}, "manual"); err != nil {
		t.Fatal(err)
	}
}

func TestMemberMoveAndLeader(t *testing.T) {
	st := openTestStore(t)
	svc := NewService(st)
	ai1 := seedAccount(t, st, "ai-1", "agent_service")
	ai2 := seedAccount(t, st, "ai-2", "agent_service")
	ch := seedAccount(t, st, "ch-1", "channel")
	a, _ := svc.Create("A", "", []string{"LUNO"}, "")
	b, _ := svc.Create("B", "", []string{"KUPAS"}, "")

	if err := svc.AddMember(a.ID, ch); !errors.Is(err, ErrNotAgent) {
		t.Fatalf("channel as member err = %v", err)
	}
	if err := svc.AddMember(a.ID, 9999); !errors.Is(err, ErrNoAccount) {
		t.Fatalf("unknown account err = %v", err)
	}
	if err := svc.AddMember(a.ID, ai1); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddMember(a.ID, ai2); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetLeader(a.ID, &ai1); err != nil {
		t.Fatal(err)
	}
	// Moving the leader to B clears A's leader.
	if err := svc.AddMember(b.ID, ai1); err != nil {
		t.Fatal(err)
	}
	got, _ := st.GetTeam(a.ID)
	if got.LeaderAccountID != nil {
		t.Fatal("leader not cleared after move")
	}
	if ok, _ := st.IsTeamMember(a.ID, ai1); ok {
		t.Fatal("ai-1 still in A after move")
	}
	// Non-member cannot lead.
	if _, err := svc.SetLeader(a.ID, &ai1); !errors.Is(err, ErrNotMember) {
		t.Fatalf("non-member leader err = %v", err)
	}
	// Remove clears leader.
	if _, err := svc.SetLeader(b.ID, &ai1); err != nil {
		t.Fatal(err)
	}
	if err := svc.RemoveMember(b.ID, ai1); err != nil {
		t.Fatal(err)
	}
	got, _ = st.GetTeam(b.ID)
	if got.LeaderAccountID != nil {
		t.Fatal("leader not cleared after remove")
	}
	if err := svc.RemoveMember(b.ID, ai1); !errors.Is(err, ErrNotMember) {
		t.Fatalf("double remove err = %v", err)
	}
}

func TestSetMembersReplace(t *testing.T) {
	st := openTestStore(t)
	svc := NewService(st)
	ai1 := seedAccount(t, st, "ai-1", "agent_service")
	ai2 := seedAccount(t, st, "ai-2", "agent_service")
	ai3 := seedAccount(t, st, "ai-3", "agent_service")
	ch := seedAccount(t, st, "ch-1", "channel")
	a, _ := svc.Create("A", "", nil, "")
	if err := svc.AddMember(a.ID, ai1); err != nil {
		t.Fatal(err)
	}
	// Batch with a channel aborts: ai-1 must stay.
	err := svc.SetMembers(a.ID, []MemberAssign{{AccountID: ai2}, {AccountID: ch}})
	if !errors.Is(err, ErrNotAgent) {
		t.Fatalf("batch channel err = %v", err)
	}
	if ok, _ := st.IsTeamMember(a.ID, ai1); !ok {
		t.Fatal("aborted batch mutated membership")
	}
	// Bad role aborts too.
	if err := svc.SetMembers(a.ID, []MemberAssign{{AccountID: ai2, Roles: []string{"nope"}}}); !errors.Is(err, ErrBadRole) {
		t.Fatalf("batch role err = %v", err)
	}
	// Duplicate aborts.
	if err := svc.SetMembers(a.ID, []MemberAssign{{AccountID: ai2}, {AccountID: ai2}}); !errors.Is(err, ErrDupMember) {
		t.Fatalf("batch dup err = %v", err)
	}
	// Valid replace: ai-1 leaves, ai-2/ai-3 join with roles.
	err = svc.SetMembers(a.ID, []MemberAssign{
		{AccountID: ai2, Roles: []string{"generate"}},
		{AccountID: ai3, Roles: []string{"review", "analyze"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := st.IsTeamMember(a.ID, ai1); ok {
		t.Fatal("ai-1 should have left")
	}
	ms, _ := st.ListTeamMembers(a.ID)
	if len(ms) != 2 {
		t.Fatalf("members = %d", len(ms))
	}
	// Empty list clears all.
	if err := svc.SetMembers(a.ID, nil); err != nil {
		t.Fatal(err)
	}
	ms, _ = st.ListTeamMembers(a.ID)
	if len(ms) != 0 {
		t.Fatalf("members after clear = %d", len(ms))
	}
}

func TestChannelsAndBrand(t *testing.T) {
	st := openTestStore(t)
	svc := NewService(st)
	ai := seedAccount(t, st, "ai-1", "agent_service")
	ch1 := seedAccount(t, st, "ch-1", "channel")
	ch2 := seedAccount(t, st, "ch-2", "channel")
	a, _ := svc.Create("A", "", nil, "")
	b, _ := svc.Create("B", "", nil, "")

	if err := svc.AddChannel(a.ID, ai); !errors.Is(err, ErrNotChannel) {
		t.Fatalf("agent as channel err = %v", err)
	}
	if err := svc.AddChannel(a.ID, ch1); err != nil {
		t.Fatal(err)
	}
	// Channels are shareable across teams.
	if err := svc.AddChannel(b.ID, ch1); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetChannels(a.ID, []int64{ch2, ch2, ch1}); err != nil {
		t.Fatal(err)
	}
	ids, _ := st.ListTeamChannelIDs(a.ID)
	if len(ids) != 2 {
		t.Fatalf("channels = %v", ids)
	}
	ids, _ = st.ListTeamChannelIDs(b.ID)
	if len(ids) != 1 {
		t.Fatalf("B channels disturbed: %v", ids)
	}

	brand, err := svc.UpsertBrand(a.ID, "LUNO는…", "친근", "최저가", "https://x.test")
	if err != nil {
		t.Fatal(err)
	}
	if !brand.HasContent || brand.Tone != "친근" {
		t.Fatalf("brand = %+v", brand)
	}
	empty, err := st.GetTeamBrand(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if empty.HasContent {
		t.Fatalf(" fresh brand should be empty: %+v", empty)
	}
}

func TestDeleteMovesToDefault(t *testing.T) {
	st := openTestStore(t)
	svc := NewService(st)
	ai := seedAccount(t, st, "ai-1", "agent_service")
	a, _ := svc.Create("A", "", nil, "")
	if err := svc.AddMember(a.ID, ai); err != nil {
		t.Fatal(err)
	}
	def, err := st.GetDefaultTeam()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.Delete(def.ID); !errors.Is(err, ErrDefaultProtect) {
		t.Fatalf("default delete err = %v", err)
	}
	p, err := st.CreateProduct("https://example.com/del", "A", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetProductTeam(p.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	j, err := st.CreateJob("generate", p.ID, "{}")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetJobTeam(j.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	moved, _, err := svc.Delete(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if moved != 1 {
		t.Fatalf("moved = %d", moved)
	}
	if ok, _ := st.IsTeamMember(def.ID, ai); !ok {
		t.Fatal("member not moved to default team")
	}
	got, _ := st.GetProduct(p.ID)
	if got.TeamID == nil || *got.TeamID != def.ID {
		t.Fatalf("work team = %+v, want default", got.TeamID)
	}
	gotJob, _ := st.GetJob(j.ID)
	if gotJob.TeamID == nil || *gotJob.TeamID != def.ID {
		t.Fatalf("job team = %+v, want default", gotJob.TeamID)
	}
}

// W21-4: distribution mode/cap round-trip with validation.
func TestUpdateDistribution(t *testing.T) {
	st := openTestStore(t)
	svc := NewService(st)
	a, _ := svc.Create("D", "", nil, "")
	if a.DistributionMode != "all" || a.DistributionCap != 5 {
		t.Fatalf("defaults = %s/%d", a.DistributionMode, a.DistributionCap)
	}
	mode, cap := "one", 2
	got, err := svc.Update(a.ID, nil, nil, nil, false, nil, nil, nil, &mode, &cap)
	if err != nil {
		t.Fatal(err)
	}
	if got.DistributionMode != "one" || got.DistributionCap != 2 {
		t.Fatalf("updated = %+v", got)
	}
	bad := "many"
	if _, err := svc.Update(a.ID, nil, nil, nil, false, nil, nil, nil, &bad, nil); !errors.Is(err, ErrBadDistMode) {
		t.Fatalf("mode err = %v", err)
	}
	z := 0
	if _, err := svc.Update(a.ID, nil, nil, nil, false, nil, nil, nil, nil, &z); !errors.Is(err, ErrBadDistCap) {
		t.Fatalf("cap err = %v", err)
	}
}

func TestDetailAndCards(t *testing.T) {
	st := openTestStore(t)
	svc := NewService(st)
	ai := seedAccount(t, st, "ai-1", "agent_service")
	ch := seedAccount(t, st, "ch-1", "channel")
	a, err := svc.Create("A", "", []string{"LUNO"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.AddMember(a.ID, ai); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetLeader(a.ID, &ai); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddChannel(a.ID, ch); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpsertBrand(a.ID, "intro", "", "", ""); err != nil {
		t.Fatal(err)
	}
	// One running work + one published log + views for LUNO.
	p, err := st.CreateProduct("https://x.test/1", "A", 0, "LUNO")
	if err != nil {
		t.Fatal(err)
	}
	c, err := st.CreateContent(&store.Content{ProductID: p.ID, Channel: "blog", Title: "d"})
	if err != nil {
		t.Fatal(err)
	}
	log, err := st.CreatePublishLog(c.ID, "blog", "ch-1", "done", "https://post.test/1")
	if err != nil {
		t.Fatal(err)
	}
	_ = log
	exp := int64(40)
	if _, err := st.CreateMetric(&store.Metric{ContentID: &c.ID, Channel: "blog", Exposures: &exp}); err != nil {
		t.Fatal(err)
	}

	d, err := svc.Detail(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Members) != 1 || !d.Members[0].IsLeader {
		t.Fatalf("members = %+v", d.Members)
	}
	if len(d.Channels) != 1 || d.Channels[0].PostingMode == "" {
		t.Fatalf("channels = %+v", d.Channels)
	}
	if !d.Brand.HasContent {
		t.Fatal("brand missing")
	}
	if d.Stats.Running != 1 {
		t.Fatalf("stats = %+v", d.Stats)
	}
	if d.Stats.TodayPublished != 1 || d.Stats.TodayDone != 1 {
		t.Fatalf("today stats = %+v", d.Stats)
	}
	if d.Stats.WeekViews != 40 {
		t.Fatalf("week views = %+v", d.Stats)
	}
	if len(d.Works) != 1 || d.Works[0].ID != p.ID {
		t.Fatalf("works = %+v", d.Works)
	}
	cards, err := svc.ListCards()
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 2 {
		t.Fatalf("cards = %d", len(cards))
	}
	for _, card := range cards {
		if card.Team.ID == a.ID && (card.MemberCount != 1 || card.ChannelCount != 1 || card.Working != 1 || card.Idle != 0) {
			t.Fatalf("card = %+v", card)
		}
		if card.Team.ID == a.ID && (len(card.MemberIDs) != 1 || card.MemberIDs[0] != ai || len(card.ChannelIDs) != 1 || card.ChannelIDs[0] != ch) {
			t.Fatalf("card ids = %+v", card)
		}
	}
	if _, err := svc.Detail(9999); !errors.Is(err, ErrNoTeam) {
		t.Fatalf("missing detail err = %v", err)
	}
}
