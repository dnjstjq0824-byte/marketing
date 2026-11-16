package store

import (
	"path/filepath"
	"testing"
	"time"
)

func openW14Store(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "w14.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := Migrate(st.DB()); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestMediaV26RoundTrip(t *testing.T) {
	st := openW14Store(t)
	for _, col := range []string{"content_id", "thumb_path", "width", "height", "duration_sec", "prompt", "ai_account_id", "source"} {
		var n int
		if err := st.DB().QueryRow(`SELECT COUNT(*) FROM pragma_table_info('media') WHERE name=?`, col).Scan(&n); err != nil || n != 1 {
			t.Fatalf("column %s: n=%d err=%v", col, n, err)
		}
	}
	p, err := st.CreateProduct("https://x.test/", "A", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	c, err := st.CreateContent(&Content{ProductID: p.ID, Channel: "instagram", Title: "t", Body: "b"})
	if err != nil {
		t.Fatal(err)
	}
	acc, err := st.CreateAccountRow(&ChannelAccount{Name: "ai-w14", Platform: "meta_ai", Role: "worker", SessionState: "connected"})
	if err != nil {
		t.Fatal(err)
	}
	m, err := st.CreateMediaFull(MediaAttrs{
		ProductID: &p.ID, ContentID: &c.ID, Kind: MediaKindGenImage,
		Path: "lib/g.png", ThumbPath: "thumbs/g.jpg", Width: 1080, Height: 1350,
		Bytes: 42, Mime: "image/png", SHA256: "sha-w14", Prompt: "a cat",
		AIAccountID: &acc.ID, Source: MediaSourceGenerated,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := st.GetMedia(m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != MediaKindGenImage || got.Prompt != "a cat" || got.Source != MediaSourceGenerated {
		t.Fatalf("media = %+v", got)
	}
	if got.ContentID == nil || *got.ContentID != c.ID || got.AIAccountID == nil || *got.AIAccountID != acc.ID {
		t.Fatalf("links = %+v", got)
	}
	if got.Width != 1080 || got.Height != 1350 || got.ThumbPath != "thumbs/g.jpg" {
		t.Fatalf("dims = %+v", got)
	}
	if _, err := st.CreateMediaFull(MediaAttrs{Kind: "teleport", Path: "x", SHA256: "s"}); err == nil {
		t.Fatal("unknown kind accepted")
	}
	// Legacy upload rows still read with empty tracking.
	u, err := st.CreateMedia("upload", "lib/u.png", 10, "image/png", "sha-u")
	if err != nil {
		t.Fatal(err)
	}
	if u.ContentID != nil || u.Prompt != "" || u.Source != "" {
		t.Fatalf("legacy = %+v", u)
	}
}

func TestMediaLinkAndList(t *testing.T) {
	st := openW14Store(t)
	p, _ := st.CreateProduct("https://x.test/", "A", 0, "")
	c, _ := st.CreateContent(&Content{ProductID: p.ID, Channel: "blog", Title: "t", Body: "b"})
	m, err := st.CreateMedia("upload", "lib/l.png", 10, "image/png", "sha-l")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.LinkMediaContent(m.ID, c.ID, p.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.LinkMediaContent(m.ID, c.ID, p.ID); err != ErrDuplicate {
		t.Fatalf("double link = %v", err)
	}
	rows, err := st.MediaForContent(c.ID)
	if err != nil || len(rows) != 1 || rows[0].ID != m.ID {
		t.Fatalf("for content = %+v %v", rows, err)
	}
	got, err := st.GetMedia(m.ID)
	if err != nil || got.ProductID == nil || *got.ProductID != p.ID {
		t.Fatalf("attached = %+v %v", got, err)
	}
}

func TestMediaPerInputCap(t *testing.T) {
	st := openW14Store(t)
	p, _ := st.CreateProduct("https://x.test/", "A", 0, "")
	var all []int64
	for i := 0; i < MaxMediaPerInput; i++ {
		m, err := st.CreateMedia("upload", "lib/c.png", 10, "image/png", "sha-cap")
		if err != nil {
			t.Fatal(err)
		}
		all = append(all, m.ID)
	}
	// 20 attach; the 21st is refused.
	if err := st.AttachMedia(all, p.ID); err != nil {
		t.Fatal(err)
	}
	extra, err := st.CreateMedia("upload", "lib/x.png", 10, "image/png", "sha-extra")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AttachMedia([]int64{extra.ID}, p.ID); err == nil {
		t.Fatal("21st attach accepted")
	}
}

func TestDeleteStaleMedia(t *testing.T) {
	st := openW14Store(t)
	old := time.Now().UTC().AddDate(0, 0, -40).Format(time.RFC3339)
	stale, err := st.CreateMediaFull(MediaAttrs{Kind: MediaKindGenImage, Path: "lib/old.png", Bytes: 1, Mime: "image/png", SHA256: "sha-old"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`UPDATE media SET created_at=? WHERE id=?`, old, stale.ID); err != nil {
		t.Fatal(err)
	}
	fresh, err := st.CreateMediaFull(MediaAttrs{Kind: MediaKindGenVideo, Path: "lib/new.mp4", Bytes: 1, Mime: "video/mp4", SHA256: "sha-new"})
	if err != nil {
		t.Fatal(err)
	}
	p, _ := st.CreateProduct("https://x.test/", "A", 0, "")
	linked, err := st.CreateMediaFull(MediaAttrs{Kind: MediaKindCard, Path: "lib/k.png", Bytes: 1, Mime: "image/png", SHA256: "sha-k"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`UPDATE media SET created_at=?, product_id=? WHERE id=?`, old, p.ID, linked.ID); err != nil {
		t.Fatal(err)
	}
	up, err := st.CreateMedia("upload", "lib/u.png", 1, "image/png", "sha-u")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`UPDATE media SET created_at=? WHERE id=?`, old, up.ID); err != nil {
		t.Fatal(err)
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -30).Format(time.RFC3339)
	gone, err := st.DeleteStaleMedia(cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if len(gone) != 1 || gone[0].ID != stale.ID {
		t.Fatalf("stale = %+v", gone)
	}
	for _, id := range []int64{fresh.ID, linked.ID, up.ID} {
		if _, err := st.GetMedia(id); err != nil {
			t.Fatalf("kept %d: %v", id, err)
		}
	}
}

func TestAccountCaps(t *testing.T) {
	if !AccountHasCap(`["text","image"]`, "image") || AccountHasCap(`["text"]`, "image") {
		t.Fatal("cap parse")
	}
	if AccountHasCap(`broken`, "image") || AccountHasCap(`[]`, "video") {
		t.Fatal("cap fail-open")
	}
	for tier, paid := range map[string]bool{"Pro": true, "Plus": true, "Max": true, "유료": true, "": false, "free": false, "무료": false, "Free": false} {
		if AccountPaidTier(tier) != paid {
			t.Fatalf("tier %q paid=%v", tier, !paid)
		}
	}
}

func seedW14Account(t *testing.T, st *Store, name, platform, state, tier, caps string) int64 {
	t.Helper()
	acc, err := st.CreateAccountRow(&ChannelAccount{Name: name, Platform: platform, Role: "worker", SessionState: state})
	if err != nil {
		t.Fatal(err)
	}
	acc.PlanTier, acc.Capabilities = tier, caps
	if err := st.UpdateAccountRow(acc); err != nil {
		t.Fatal(err)
	}
	return acc.ID
}

func TestTeamMediaCoverage(t *testing.T) {
	st := openW14Store(t)
	team, err := st.CreateTeam("m1", "", "", 0, "auto")
	if err != nil {
		t.Fatal(err)
	}
	free := seedW14Account(t, st, "w14-free", "meta_ai", "connected", "", `["text","image"]`)
	paid := seedW14Account(t, st, "w14-paid", "gemini", "connected", "Pro", `["text","image","video"]`)
	dead := seedW14Account(t, st, "w14-dead", "meta_ai", "expired", "Pro", `["text","image","video"]`)
	chanAcc := seedW14Account(t, st, "w14-ch", "naver_blog", "connected", "Pro", `["text","image","video"]`)
	for _, id := range []int64{free, paid, dead, chanAcc} {
		if _, err := st.AddTeamMember(team.ID, id); err != nil {
			t.Fatal(err)
		}
	}
	cov, err := st.TeamMediaCoverage(team.ID, MediaCapImage)
	if err != nil {
		t.Fatal(err)
	}
	if !cov.Any || !cov.Paid {
		t.Fatalf("image = %+v", cov)
	}
	cov, err = st.TeamMediaCoverage(team.ID, MediaCapVideo)
	if err != nil {
		t.Fatal(err)
	}
	if !cov.Any || !cov.Paid {
		t.Fatalf("video = %+v", cov)
	}
	team2, _ := st.CreateTeam("m2", "", "", 0, "auto")
	if _, err := st.AddTeamMember(team2.ID, free); err != nil {
		t.Fatal(err)
	}
	cov, _ = st.TeamMediaCoverage(team2.ID, MediaCapImage)
	if !cov.Any || cov.Paid {
		t.Fatalf("free-only = %+v", cov)
	}
	cov, _ = st.TeamMediaCoverage(team2.ID, MediaCapVideo)
	if cov.Any || cov.Paid {
		t.Fatalf("no-video = %+v", cov)
	}
}

func TestMediaClaimGating(t *testing.T) {
	st := openW14Store(t)
	team, _ := st.CreateTeam("mc", "", "", 0, "auto")
	imgAcc := seedW14Account(t, st, "mc-img", "meta_ai", "connected", "", `["text","image"]`)
	vidAcc := seedW14Account(t, st, "mc-vid", "gemini", "connected", "Pro", `["text","image","video"]`)
	txtAcc := seedW14Account(t, st, "mc-txt", "meta_ai", "connected", "", `["text"]`)
	for _, id := range []int64{imgAcc, vidAcc, txtAcc} {
		if _, err := st.AddTeamMember(team.ID, id); err != nil {
			t.Fatal(err)
		}
	}
	newAgent := func(label string, acc int64) *Agent {
		a, _, err := st.CreateAgent(label, "T", "", "", "", 1, &acc)
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	imgAg, vidAg, txtAg := newAgent("img", imgAcc), newAgent("vid", vidAcc), newAgent("txt", txtAcc)
	p, _ := st.CreateProduct("https://x.test/", "A", 0, "")
	mkJob := func(payload string) *Job {
		j, err := st.CreateJobFull("media", p.ID, payload, "", 0, true)
		if err != nil {
			t.Fatal(err)
		}
		if err := st.SetJobTeam(j.ID, team.ID); err != nil {
			t.Fatal(err)
		}
		return j
	}
	ai := `{"route":"ai","required_cap":"image","paid_first":false,"product_id":1,"content_id":1}`
	// Capable image agent claims; text-only agent skips.
	j1 := mkJob(ai)
	got, err := st.ClaimRoutedJob(txtAg, []string{"media"}, 600)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("text-only claimed %+v", got)
	}
	got, err = st.ClaimRoutedJob(imgAg, []string{"media"}, 600)
	if err != nil || got == nil || got.ID != j1.ID {
		t.Fatalf("image claim = %+v %v", got, err)
	}
	if _, err := st.SetJobResult(j1.ID, "{}"); err != nil {
		t.Fatal(err)
	}
	// Unlinked agents take local work but never AI work.
	unlinked, _, err := st.CreateAgent("free", "T", "", "", "", 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	j2 := mkJob(ai)
	if got, _ := st.ClaimRoutedJob(unlinked, []string{"media"}, 600); got != nil {
		t.Fatalf("unlinked claimed AI %+v", got)
	}
	if _, err := st.SetJobFailed(j2.ID, "cleanup"); err != nil {
		t.Fatal(err)
	}
	j3 := mkJob(`{"route":"local","required_cap":"image","product_id":1,"content_id":1}`)
	if got, _ := st.ClaimRoutedJob(unlinked, []string{"media"}, 600); got == nil || got.ID != j3.ID {
		t.Fatalf("unlinked local = %+v", got)
	}
	// Paid-first grace: the free agent waits while the job is young.
	cfg, _ := GetAutomationSettings(st.DB())
	cfg.PreferPaidMedia = true
	if err := SetAutomationSettings(st.DB(), cfg); err != nil {
		t.Fatal(err)
	}
	j4 := mkJob(`{"route":"ai","required_cap":"image","paid_first":true,"product_id":1,"content_id":1}`)
	if got, _ := st.ClaimRoutedJob(imgAg, []string{"media"}, 600); got != nil {
		t.Fatalf("free claimed in grace %+v", got)
	}
	if got, _ := st.ClaimRoutedJob(vidAg, []string{"media"}, 600); got == nil || got.ID != j4.ID {
		t.Fatalf("paid claim = %+v", got)
	}
	j5 := mkJob(`{"route":"ai","required_cap":"image","paid_first":true,"product_id":1,"content_id":1}`)
	old := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	if _, err := st.DB().Exec(`UPDATE jobs SET created_at=? WHERE id=?`, old, j5.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := st.ClaimRoutedJob(imgAg, []string{"media"}, 600); got == nil || got.ID != j5.ID {
		t.Fatalf("free after grace = %+v", got)
	}
	if _, err := st.SetJobResult(j5.ID, "{}"); err != nil {
		t.Fatal(err)
	}
	// The operator can switch paid-first off entirely.
	cfg.PreferPaidMedia = false
	if err := SetAutomationSettings(st.DB(), cfg); err != nil {
		t.Fatal(err)
	}
	j6 := mkJob(`{"route":"ai","required_cap":"image","paid_first":true,"product_id":1,"content_id":1}`)
	if got, _ := st.ClaimRoutedJob(imgAg, []string{"media"}, 600); got == nil || got.ID != j6.ID {
		t.Fatalf("free with pref off = %+v", got)
	}
}

func TestSettingsLegacyKeys(t *testing.T) {
	st := openW14Store(t)
	// Stored rows from before W14 lack the new keys: they default on.
	raw := `{"enabled":true,"publishIntervalMin":120,"dailyCap":3,"windowStart":"09:00","windowEnd":"22:00","blogMinLen":1500,"mediaRetentionDays":30,"maxConcurrent":3}`
	if _, err := st.DB().Exec(`INSERT INTO app_meta(k,v) VALUES('settings.automation',?)`, raw); err != nil {
		t.Fatal(err)
	}
	s, err := GetAutomationSettings(st.DB())
	if err != nil {
		t.Fatal(err)
	}
	if !s.PreferPaidMedia || !s.Outputs.Image || !s.Outputs.Video || !s.Outputs.Text {
		t.Fatalf("legacy defaults = %+v", s)
	}
	// An explicit operator false survives.
	s.PreferPaidMedia, s.Outputs.Video = false, false
	if err := SetAutomationSettings(st.DB(), s); err != nil {
		t.Fatal(err)
	}
	got, _ := GetAutomationSettings(st.DB())
	if got.PreferPaidMedia || got.Outputs.Video {
		t.Fatalf("explicit off = %+v", got)
	}
}
