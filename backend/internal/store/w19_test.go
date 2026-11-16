package store

import (
	"path/filepath"
	"testing"
)

func openW19Store(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "w19.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := Migrate(st.DB()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return st
}

// seedW19Legacy builds one legacy-shaped work: a URL product with no brief
// and no input kind, a rejected review original with an approved instagram
// revision, a lone rejected review awaiting regen, and a live review with
// no decision yet.
func seedW19Legacy(t *testing.T, st *Store) (prodID, origID, childID, loneID, liveID int64) {
	t.Helper()
	p, err := st.CreateProduct("https://example.com/g", "A", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	mk := func(channel string) *Content {
		c, err := st.CreateContent(&Content{ProductID: p.ID, Channel: channel, Title: "t", Body: "b"})
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	orig := mk("instagram")
	if err := st.UpdateContentStatus(orig.ID, "review"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateReviewDecision(orig.ID, "chief", "rejected", "off-brand"); err != nil {
		t.Fatal(err)
	}
	child, err := st.CreateContent(&Content{ProductID: p.ID, Channel: "instagram", Title: "t2", Body: "b2",
		RevisionOf: orig.ID, RevisionRootID: orig.RevisionRootID})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateContentStatus(child.ID, "review"); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateContentStatus(child.ID, "approved"); err != nil {
		t.Fatal(err)
	}
	lone := mk("blog")
	if err := st.UpdateContentStatus(lone.ID, "review"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateReviewDecision(lone.ID, "chief", "rejected", "retry"); err != nil {
		t.Fatal(err)
	}
	live := mk("blog")
	if err := st.UpdateContentStatus(live.ID, "review"); err != nil {
		t.Fatal(err)
	}
	return p.ID, orig.ID, child.ID, lone.ID, live.ID
}

func contentStatus(t *testing.T, st *Store, id int64) string {
	t.Helper()
	c, err := st.GetContent(id)
	if err != nil {
		t.Fatal(err)
	}
	return c.Status
}

func TestMigrateV25WorkBackfill(t *testing.T) {
	st := openW19Store(t)
	prodID, origID, childID, loneID, liveID := seedW19Legacy(t, st)

	if err := migrateToV25(st.DB()); err != nil {
		t.Fatalf("migrateToV25: %v", err)
	}
	p, err := st.GetProduct(prodID)
	if err != nil {
		t.Fatal(err)
	}
	if p.InputKind != "url" {
		t.Fatalf("input_kind = %q, want url", p.InputKind)
	}
	if p.BriefJSON != `{"channels":[{"channel":"instagram","platform":"instagram","versions":1}]}` {
		t.Fatalf("brief = %s", p.BriefJSON)
	}
	if got := contentStatus(t, st, origID); got != "archived" {
		t.Fatalf("superseded original = %s, want archived", got)
	}
	if got := contentStatus(t, st, childID); got != "approved" {
		t.Fatalf("child = %s, want approved", got)
	}
	if got := contentStatus(t, st, loneID); got != "review" {
		t.Fatalf("lone rejected = %s, want review (regen pending)", got)
	}
	if got := contentStatus(t, st, liveID); got != "review" {
		t.Fatalf("live review = %s, want review", got)
	}
	// Idempotent: a second pass changes nothing.
	before, _ := st.GetProduct(prodID)
	if err := migrateToV25(st.DB()); err != nil {
		t.Fatalf("second migrateToV25: %v", err)
	}
	after, _ := st.GetProduct(prodID)
	if after.BriefJSON != before.BriefJSON || after.InputKind != "url" {
		t.Fatalf("second pass mutated work: %+v", after)
	}
	if got := contentStatus(t, st, loneID); got != "review" {
		t.Fatalf("second pass archived lone review: %s", got)
	}
}

func TestMigrateV25BriefSkipsUnmappable(t *testing.T) {
	st := openW19Store(t)
	p, err := st.CreateProduct("https://example.com/h", "A", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	c, err := st.CreateContent(&Content{ProductID: p.ID, Channel: "blog", Title: "t", Body: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateContentStatus(c.ID, "review"); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateContentStatus(c.ID, "approved"); err != nil {
		t.Fatal(err)
	}
	if err := migrateToV25(st.DB()); err != nil {
		t.Fatalf("migrateToV25: %v", err)
	}
	got, err := st.GetProduct(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.InputKind != "url" {
		t.Fatalf("input_kind = %q, want url", got.InputKind)
	}
	if got.BriefJSON != "" {
		t.Fatalf("brief = %s, want empty (blog names no registry platform)", got.BriefJSON)
	}
}

func seedW19Account(t *testing.T, st *Store, name, platform, kind, state string) *ChannelAccount {
	t.Helper()
	a, err := st.CreateAccountRow(&ChannelAccount{Name: name, Platform: platform,
		Role: "worker", SessionState: state, PostingMode: "auto"})
	if err != nil {
		t.Fatal(err)
	}
	a.Kind = kind
	if err := st.UpdateAccountRow(a); err != nil {
		t.Fatal(err)
	}
	return a
}

func TestMigrateV25TeamAndAgents(t *testing.T) {
	st := openW19Store(t)
	def, err := st.GetDefaultTeam()
	if err != nil {
		t.Fatal(err)
	}
	ai := seedW19Account(t, st, "w19ai", "meta_ai", "agent_service", "connected")
	other, err := st.CreateTeam("other", "", "", 0, "auto")
	if err != nil {
		t.Fatal(err)
	}
	placed := seedW19Account(t, st, "w19placed", "gemini", "agent_service", "connected")
	if err := st.MoveMemberToTeam(other.ID, placed.ID, ""); err != nil {
		t.Fatal(err)
	}
	ch := seedW19Account(t, st, "w19ch", "naver_blog", "channel", "connected")
	var oldIDs []int64
	for _, lane := range []string{"A", "B", "C"} {
		a, _, err := st.CreateAgent("w19old-"+lane, lane, "h", "", "", 1, nil)
		if err != nil {
			t.Fatal(err)
		}
		oldIDs = append(oldIDs, a.ID)
	}
	exec, _, err := st.CreateAgent("w19exec", "T", "h", "", "", 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := migrateToV25(st.DB()); err != nil {
		t.Fatalf("migrateToV25: %v", err)
	}
	if ok, _ := st.IsTeamMember(def.ID, ai.ID); !ok {
		t.Fatal("unplaced AI account did not join the default team")
	}
	if ok, _ := st.IsTeamMember(other.ID, placed.ID); !ok {
		t.Fatal("placed AI account moved teams")
	}
	if ok, _ := st.IsTeamMember(def.ID, ch.ID); ok {
		t.Fatal("channel account joined a team")
	}
	for _, id := range oldIDs {
		a, err := st.GetAgent(id)
		if err != nil {
			t.Fatal(err)
		}
		if a.Enabled {
			t.Fatalf("lane agent %d (%s) still enabled", a.ID, a.Label)
		}
	}
	if got, err := st.GetAgent(exec.ID); err != nil || !got.Enabled {
		t.Fatalf("lane-T executor enabled = %v, %v", got.Enabled, err)
	}
}

func TestMigrateV25Notices(t *testing.T) {
	st := openW19Store(t)
	conn := seedW19Account(t, st, "w19conn", "meta_ai", "agent_service", "connected")
	pend := seedW19Account(t, st, "w19pend", "gemini", "agent_service", "pending")
	mk := func(kind, refType string, refID int64, key string) {
		if _, err := st.CreateNotificationDedup(kind, "warn", refType, refID, key, "m"); err != nil {
			t.Fatal(err)
		}
	}
	mk("lease_expired", "job", 1, "w19:lease:1")
	mk("job_stuck", "job", 2, "w19:stuck:2")
	mk("session_expired", "account", conn.ID, "w19:se:conn")
	mk("session_limited", "account", pend.ID, "w19:sl:pend")
	mk("review_human", "content", 3, "w19:rh:3")
	mk("job_failed", "job", 4, "w19:jf:4")
	if err := migrateToV25(st.DB()); err != nil {
		t.Fatalf("migrateToV25: %v", err)
	}
	read := map[string]bool{}
	items, err := st.ListNotifications(false, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range items {
		read[n.DedupKey] = n.ReadAt != ""
	}
	for key, want := range map[string]bool{
		"w19:lease:1": true, "w19:stuck:2": true, "w19:se:conn": true,
		"w19:sl:pend": false, "w19:rh:3": false, "w19:jf:4": false,
	} {
		if read[key] != want {
			t.Fatalf("notice %s read = %v, want %v", key, read[key], want)
		}
	}
}
