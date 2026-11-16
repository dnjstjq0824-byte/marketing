package queue

import (
	"encoding/json"
	"errors"
	"testing"

	"somun/internal/store"
)

func submitW14Generate(t *testing.T, svc *Service, st *store.Store, pid int64, items []GenerateItem) {
	t.Helper()
	j, err := st.CreateJob(JobGenerate, pid, "{}")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimNextJob(JobGenerate, "w"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.SubmitGenerate(j.ID, GenerateResult{Contents: items}); err != nil {
		t.Fatal(err)
	}
}

func TestMediaEnqueue(t *testing.T) {
	st := openTestStore(t)
	pid := seedProduct(t, st, "A")
	svc := NewService(st)
	submitW14Generate(t, svc, st, pid, []GenerateItem{
		{Channel: "instagram", Title: "t", Body: "b", Media: "상품 전경 1장, 4:5"},
		{Channel: "blog", Title: "t2", Body: "b2"},
	})
	mj, err := st.ClaimNextJob(JobMedia, "w")
	if err != nil || mj == nil {
		t.Fatalf("media job = %+v, %v", mj, err)
	}
	var payload struct {
		Route    string `json:"route"`
		Required string `json:"required_cap"`
		Channel  string `json:"channel"`
		Spec     string `json:"media_spec"`
	}
	if err := json.Unmarshal([]byte(mj.Payload), &payload); err != nil {
		t.Fatal(err)
	}
	// No capable team account: the local cost-0 route.
	if payload.Route != "local" || payload.Required != "image" || payload.Channel != "instagram" {
		t.Fatalf("payload = %+v", payload)
	}
	if payload.Spec != "상품 전경 1장, 4:5" {
		t.Fatalf("spec = %q", payload.Spec)
	}
	// The bare draft enqueues nothing.
	if mj2, err := st.ClaimNextJob(JobMedia, "w"); err != nil || mj2 != nil {
		t.Fatalf("extra media job = %+v, %v", mj2, err)
	}
	// The review snapshot carries the shopping list for the chief.
	rj, err := st.ClaimNextJob(JobReview, "w")
	if err != nil || rj == nil {
		t.Fatalf("review job = %+v, %v", rj, err)
	}
	var rp struct {
		Contents []map[string]any `json:"contents"`
	}
	if err := json.Unmarshal([]byte(rj.Payload), &rp); err != nil {
		t.Fatal(err)
	}
	if len(rp.Contents) != 2 || rp.Contents[0]["media_spec"] != "상품 전경 1장, 4:5" {
		t.Fatalf("snapshot = %+v", rp.Contents)
	}
	if _, ok := rp.Contents[1]["media_spec"]; ok {
		t.Fatalf("bare snapshot = %+v", rp.Contents[1])
	}
}

func TestMediaEnqueueAIRoute(t *testing.T) {
	st := openTestStore(t)
	pid := seedProduct(t, st, "A")
	team, err := st.CreateTeam("w14t", "", "", 0, "auto")
	if err != nil {
		t.Fatal(err)
	}
	acc, err := st.CreateAccountRow(&store.ChannelAccount{Name: "w14vid", Platform: "gemini", Role: "worker", SessionState: "connected"})
	if err != nil {
		t.Fatal(err)
	}
	acc.PlanTier, acc.Capabilities = "Pro", `["text","video"]`
	if err := st.UpdateAccountRow(acc); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddTeamMember(team.ID, acc.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.SetProductTeam(pid, team.ID); err != nil {
		t.Fatal(err)
	}
	svc := NewService(st)
	submitW14Generate(t, svc, st, pid, []GenerateItem{
		{Channel: "shortform", Title: "t", Body: "b", Media: "15초 소개 영상"},
	})
	mj, err := st.ClaimNextJob(JobMedia, "w")
	if err != nil || mj == nil {
		t.Fatalf("media job = %+v, %v", mj, err)
	}
	var payload struct {
		Route     string `json:"route"`
		Required  string `json:"required_cap"`
		PaidFirst bool   `json:"paid_first"`
	}
	if err := json.Unmarshal([]byte(mj.Payload), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Route != "ai" || payload.Required != "video" || !payload.PaidFirst {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestMediaEnqueueOutputsOff(t *testing.T) {
	st := openTestStore(t)
	cfg, _ := store.GetAutomationSettings(st.DB())
	cfg.Outputs.Image = false
	if err := store.SetAutomationSettings(st.DB(), cfg); err != nil {
		t.Fatal(err)
	}
	pid := seedProduct(t, st, "A")
	svc := NewService(st)
	submitW14Generate(t, svc, st, pid, []GenerateItem{
		{Channel: "instagram", Title: "t", Body: "b", Media: "1장"},
	})
	if mj, err := st.ClaimNextJob(JobMedia, "w"); err != nil || mj != nil {
		t.Fatalf("disabled output enqueued = %+v, %v", mj, err)
	}
}

func TestSubmitMedia(t *testing.T) {
	st := openTestStore(t)
	pid := seedProduct(t, st, "A")
	svc := NewService(st)
	submitW14Generate(t, svc, st, pid, []GenerateItem{
		{Channel: "instagram", Title: "t", Body: "b", Media: "1장"},
	})
	mj, err := st.ClaimNextJob(JobMedia, "w")
	if err != nil || mj == nil {
		t.Fatal("no media job")
	}
	m, err := st.CreateMedia("card", "lib/c.png", 10, "image/png", "sha-m")
	if err != nil {
		t.Fatal(err)
	}
	done, err := svc.SubmitMedia(mj.ID, MediaResult{
		MediaIDs: []MediaResultItem{{MediaID: m.ID, Role: "image"}},
		Note:     "local render", Fallback: "local",
	})
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != StatusDone {
		t.Fatalf("status = %s", done.Status)
	}
	got, _ := st.GetMedia(m.ID)
	if got.ContentID == nil || got.ProductID == nil || *got.ProductID != pid {
		t.Fatalf("linked = %+v", got)
	}
	// Double submit is refused.
	if _, err := svc.SubmitMedia(mj.ID, MediaResult{}); !errors.Is(err, store.ErrAlreadyDone) {
		t.Fatalf("double = %v", err)
	}
	// Wrong job type.
	gj, _ := st.CreateJob(JobGenerate, pid, "{}")
	if _, err := st.ClaimNextJob(JobGenerate, "w"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SubmitMedia(gj.ID, MediaResult{}); !errors.Is(err, ErrJobNotMedia) {
		t.Fatalf("type = %v", err)
	}
	// Unknown media id on a fresh media job.
	submitW14Generate(t, svc, st, pid, []GenerateItem{
		{Channel: "instagram", Title: "t2", Body: "b2", Media: "1장"},
	})
	mj2, _ := st.ClaimNextJob(JobMedia, "w")
	if _, err := svc.SubmitMedia(mj2.ID, MediaResult{MediaIDs: []MediaResultItem{{MediaID: 99999}}}); !errors.Is(err, ErrBadMediaItem) {
		t.Fatalf("unknown media = %v", err)
	}
	// Media failure never touches the text chain: the review job survives.
	if _, err := st.SetJobFailed(mj2.ID, "render broke"); err != nil {
		t.Fatal(err)
	}
	if rj, err := st.ClaimNextJob(JobReview, "w"); err != nil || rj == nil {
		t.Fatalf("review after media fail = %+v, %v", rj, err)
	}
}
