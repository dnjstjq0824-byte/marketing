package store

import (
	"path/filepath"
	"testing"
	"time"
)

func seedOverview(t *testing.T, st *Store) {
	t.Helper()
	pa, err := st.CreateProduct("https://shop.test/a", "A", 0, "LUNO")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertReading(&Reading{ProductID: pa.ID, ProductName: "A"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateContent(&Content{ProductID: pa.ID, Channel: "blog", Title: "draft-a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddImage(pa.ID, "a1.jpg", 0); err != nil {
		t.Fatal(err)
	}
	pb, err := st.CreateProduct("https://shop.test/b", "A", 0, "LUNO")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertStrategy(&Strategy{ProductID: pb.ID}); err != nil {
		t.Fatal(err)
	}
	cb, err := st.CreateContent(&Content{ProductID: pb.ID, Channel: "blog", Title: "appr-b"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateContentStatus(cb.ID, "approved"); err != nil {
		t.Fatal(err)
	}
	pc, err := st.CreateProduct("https://shop.test/c", "A", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	cc, err := st.CreateContent(&Content{ProductID: pc.ID, Channel: "blog", Title: "pub-c"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateContentStatus(cc.ID, "published"); err != nil {
		t.Fatal(err)
	}
	exp := int64(120)
	if _, err := st.CreateMetric(&Metric{ContentID: &cc.ID, Channel: "blog", Exposures: &exp}); err != nil {
		t.Fatal(err)
	}
	j, err := st.CreateJob("generate", cc.ID, "{}")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetJobFailed(j.ID, "boom"); err != nil {
		t.Fatal(err)
	}
	aiAcc, err := st.CreateAccountRow(&ChannelAccount{Name: "ai-1", Platform: "meta_ai", Role: "worker",
		SessionState: "connected", PostingMode: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	aiAcc.Kind = "agent_service"
	if err := st.UpdateAccountRow(aiAcc); err != nil {
		t.Fatal(err)
	}
	chAcc, err := st.CreateAccountRow(&ChannelAccount{Name: "ch-1", Platform: "naver_blog", Role: "worker",
		SessionState: "expired", PostingMode: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	chAcc.Kind = "channel"
	if err := st.UpdateAccountRow(chAcc); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreatePublishLog(cc.ID, "blog", "ch-1", "failed", "nope"); err != nil {
		t.Fatal(err)
	}
}

func TestOverview(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "ov.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := Migrate(st.DB()); err != nil {
		t.Fatal(err)
	}
	seedOverview(t, st)
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	fromUTC := midnight.UTC().Format(time.RFC3339)
	weekUTC := midnight.Add(-7 * 24 * time.Hour).UTC().Format(time.RFC3339)

	today, err := st.OverviewToday(fromUTC, weekUTC)
	if err != nil {
		t.Fatal(err)
	}
	if today.Running != 2 || today.PublishedToday != 1 || today.Queued != 1 || today.WeekViews != 120 || today.Failed != 1 {
		t.Fatalf("today = %+v", today)
	}

	running, err := st.OverviewRunning(5)
	if err != nil {
		t.Fatal(err)
	}
	if len(running) != 2 {
		t.Fatalf("running = %d", len(running))
	}
	stages := map[int64]string{}
	for _, w := range running {
		stages[w.ID] = w.Stage
		if w.Title == "" || w.Detail == "" || w.Ago == "" {
			t.Fatalf("work = %+v", w)
		}
	}
	// Product ids are insertion-ordered: A first, B second.
	if stages[1] != "generate" || stages[2] != "review" {
		t.Fatalf("stages = %v", stages)
	}
	if running[0].Thumb == "" && running[1].Thumb == "" {
		t.Fatal("no thumbs resolved")
	}

	posts, err := st.OverviewRecentPosts(6)
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 1 || posts[0].Title != "pub-c" || posts[0].Views != 120 {
		t.Fatalf("posts = %+v", posts)
	}

	media, err := st.OverviewRecentMedia(12)
	if err != nil {
		t.Fatal(err)
	}
	if len(media) != 1 || media[0].Kind != "upload" || media[0].URL != "/uploads/a1.jpg" {
		t.Fatalf("media = %+v", media)
	}

	failed, err := st.OverviewFailedPublishes(5)
	if err != nil {
		t.Fatal(err)
	}
	if len(failed) != 1 || failed[0].Status != "failed" {
		t.Fatalf("failed = %+v", failed)
	}

	unassigned, err := st.OverviewUnassignedQueued()
	if err != nil {
		t.Fatal(err)
	}
	if unassigned != 1 {
		t.Fatalf("unassigned = %d", unassigned)
	}

	ai, ch, work, err := st.OverviewFirstRun()
	if err != nil {
		t.Fatal(err)
	}
	if !ai || ch || !work {
		t.Fatalf("firstrun = %v %v %v", ai, ch, work)
	}
}
