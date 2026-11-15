package scheduler

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"somun/internal/content/policy"
	"somun/internal/media"
	"somun/internal/notify"
	"somun/internal/store"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "sched.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := store.Migrate(st.DB()); err != nil {
		t.Fatal(err)
	}
	// Registry seeds preparing; most tests need ready platforms.
	for _, p := range []string{"naver_blog", "instagram"} {
		if err := st.SetChannelPlatformStatus(p, store.PlatformReady); err != nil {
			t.Fatal(err)
		}
	}
	// Scheduler tests exercise assignment, not the W3 length floor.
	cfg := store.DefaultAutomationSettings()
	cfg.BlogMinLen = 1
	if err := store.SetAutomationSettings(st.DB(), cfg); err != nil {
		t.Fatal(err)
	}
	return st
}

// W18: a guard-flagged item returns to review with a regen job, never queued.
func TestAssignGuardBlocks(t *testing.T) {
	st := openTestStore(t)
	svc := NewService(st, notify.NewService(st))
	seedAccount(t, st, "nb1", "naver_blog", "connected", "auto")
	body := "글로우사인 네일 스티커로 봄 네일 완성. 붙이고 다듬으면 끝이라 초보도 쉽다. https://example.com/g"
	p, err := st.CreateProduct("https://example.com/g", "A", 0, "G")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetProductBrief(p.ID, blogBrief); err != nil {
		t.Fatal(err)
	}
	mk := func() int64 {
		c, err := st.CreateContent(&store.Content{ProductID: p.ID, Channel: "blog", Title: "t", Body: body})
		if err != nil {
			t.Fatal(err)
		}
		return c.ID
	}
	first, second := mk(), mk()
	if err := st.ScheduleContent(first, "nb1", "naver_blog", "manual", ""); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"review", "approved", "queued"} {
		if err := st.UpdateContentStatus(first, s); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.MarkContentPublished(first); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateContentStatus(second, policy.StatusApproved); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Assign(second)
	if err != nil {
		t.Fatal(err)
	}
	if res.Assigned || res.Reason != "guard" {
		t.Fatalf("result = %+v, want guard block", res)
	}
	c, _ := st.GetContent(second)
	if c.Status != policy.StatusReview {
		t.Fatalf("status = %s, want review", c.Status)
	}
	jobs, _ := st.ListJobsByRef(p.ID)
	found := false
	for _, j := range jobs {
		if j.Type == "generate" {
			found = true
		}
	}
	if !found {
		t.Fatalf("no regen job queued: %+v", jobs)
	}
}

// W16-5: linked team channels restrict assignment; unlinked teams use all.
func TestAssignTeamChannels(t *testing.T) {
	st := openTestStore(t)
	svc := NewService(st, notify.NewService(st))
	seedAccount(t, st, "nb1", "naver_blog", "connected", "auto")
	seedAccount(t, st, "nb2", "naver_blog", "connected", "auto")
	tm, err := st.CreateTeam("TCH", "", "", 0, "auto")
	if err != nil {
		t.Fatal(err)
	}
	nb2, _ := st.GetAccountRowByName("nb2")
	if _, err := st.DB().Exec(`INSERT INTO team_channels(team_id,account_id) VALUES(?,?)`, tm.ID, nb2.ID); err != nil {
		t.Fatal(err)
	}
	p, _ := st.CreateProduct("https://example.com/tch", "A", 0, "")
	st.SetProductBrief(p.ID, blogBrief)
	st.SetProductTeam(p.ID, tm.ID)
	c, _ := st.CreateContent(&store.Content{ProductID: p.ID, Channel: "blog", Title: "t", Body: "b https://example.com/tch"})
	st.UpdateContentStatus(c.ID, policy.StatusApproved)
	res, err := svc.Assign(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Assigned || res.Account != "nb2" {
		t.Fatalf("result = %+v, want nb2 only", res)
	}
}

// W21-4: two versions spread across two accounts, never piling up.
func TestAssignSpreadsVersions(t *testing.T) {
	st := openTestStore(t)
	svc := NewService(st, notify.NewService(st))
	seedAccount(t, st, "nb1", "naver_blog", "connected", "auto")
	seedAccount(t, st, "nb2", "naver_blog", "connected", "auto")
	p, _ := st.CreateProduct("https://example.com/spread", "A", 0, "")
	st.SetProductBrief(p.ID, blogBrief)
	mk := func(body string) int64 {
		c, err := st.CreateContent(&store.Content{ProductID: p.ID, Channel: "blog", Title: "t", Body: body})
		if err != nil {
			t.Fatal(err)
		}
		if err := st.UpdateContentStatus(c.ID, policy.StatusApproved); err != nil {
			t.Fatal(err)
		}
		return c.ID
	}
	first := mk("첫 번째 각도의 글이다 https://example.com/spread")
	second := mk("완전히 다른 두 번째 각도의 글이다 https://example.com/spread")
	r1, err := svc.Assign(first)
	if err != nil || !r1.Assigned {
		t.Fatalf("first = %+v, %v", r1, err)
	}
	r2, err := svc.Assign(second)
	if err != nil || !r2.Assigned {
		t.Fatalf("second = %+v, %v", r2, err)
	}
	if r1.Account == r2.Account {
		t.Fatalf("both on %s, want spread", r1.Account)
	}
}

// W21-4: mode one pins a single assignment; the rest park as approved.
func TestAssignDistCapOne(t *testing.T) {
	st := openTestStore(t)
	svc := NewService(st, notify.NewService(st))
	seedAccount(t, st, "nb1", "naver_blog", "connected", "auto")
	seedAccount(t, st, "nb2", "naver_blog", "connected", "auto")
	tm, err := st.CreateTeam("ONE", "", "", 0, "auto")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`UPDATE teams SET distribution_mode='one' WHERE id=?`, tm.ID); err != nil {
		t.Fatal(err)
	}
	p, _ := st.CreateProduct("https://example.com/one", "A", 0, "")
	st.SetProductBrief(p.ID, blogBrief)
	st.SetProductTeam(p.ID, tm.ID)
	mk := func(body string) int64 {
		c, _ := st.CreateContent(&store.Content{ProductID: p.ID, Channel: "blog", Title: "t", Body: body})
		st.UpdateContentStatus(c.ID, policy.StatusApproved)
		return c.ID
	}
	first, second := mk("버전 하나 https://example.com/one"), mk("버전 둘인데 내용 다름 https://example.com/one")
	r1, _ := svc.Assign(first)
	if !r1.Assigned {
		t.Fatalf("first = %+v", r1)
	}
	r2, err := svc.Assign(second)
	if err != nil {
		t.Fatal(err)
	}
	if r2.Assigned || r2.Reason != "dist-cap" {
		t.Fatalf("second = %+v, want dist-cap hold", r2)
	}
	c, _ := st.GetContent(second)
	if c.Status != policy.StatusApproved {
		t.Fatalf("status = %s, want approved", c.Status)
	}
}

// W21-1: preparing platforms hold assignment with a notice.
func TestAssignPreparingBlocked(t *testing.T) {
	st := openTestStore(t)
	svc := NewService(st, notify.NewService(st))
	if err := st.SetChannelPlatformStatus("naver_blog", store.PlatformPreparing); err != nil {
		t.Fatal(err)
	}
	seedAccount(t, st, "nb1", "naver_blog", "connected", "auto")
	cid := seedApproved(t, st, "LUNO", blogBrief, "blog")
	res, err := svc.Assign(cid)
	if err != nil {
		t.Fatal(err)
	}
	if res.Assigned || res.Reason != "preparing" {
		t.Fatalf("result = %+v", res)
	}
	n, _ := st.CountUnreadNotifications()
	if n != 1 {
		t.Fatalf("unread = %d", n)
	}
}

func seedAccount(t *testing.T, st *store.Store, name, platform, state, mode string) {
	t.Helper()
	_, err := st.CreateAccountRow(&store.ChannelAccount{Name: name, Platform: platform,
		SessionState: state, PostingMode: mode})
	if err != nil {
		t.Fatal(err)
	}
}

func seedAIAccount(t *testing.T, st *store.Store, name, platform string) {
	t.Helper()
	a, err := st.CreateAccountRow(&store.ChannelAccount{Name: name, Platform: platform,
		SessionState: "connected", PostingMode: "auto"})
	if err != nil {
		t.Fatal(err)
	}
	a.Kind = "agent_service"
	if err := st.UpdateAccountRow(a); err != nil {
		t.Fatal(err)
	}
}

func seedApproved(t *testing.T, st *store.Store, business, brief, channel string) int64 {
	t.Helper()
	p, err := st.CreateProduct("https://example.com/"+business, "A", 0, business)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetProductBrief(p.ID, brief); err != nil {
		t.Fatal(err)
	}
	c, err := st.CreateContent(&store.Content{ProductID: p.ID, Channel: channel, Title: "t", Body: "b https://example.com/" + business})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateContentStatus(c.ID, policy.StatusApproved); err != nil {
		t.Fatal(err)
	}
	return c.ID
}

const blogBrief = `{"channels":[{"channel":"blog","platform":"naver","versions":1}]}`

// Approve moves to queued on the eligible account with a publish job.
func TestAssignHappy(t *testing.T) {
	st := openTestStore(t)
	svc := NewService(st, notify.NewService(st))
	seedAccount(t, st, "nb1", "naver_blog", "connected", "auto")
	cid := seedApproved(t, st, "LUNO", blogBrief, "blog")
	res, err := svc.Assign(cid)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Assigned || res.Account != "nb1" || res.Platform != "naver_blog" {
		t.Fatalf("result = %+v", res)
	}
	if _, err := time.Parse(time.RFC3339, res.At); err != nil {
		t.Fatalf("bad slot %q: %v", res.At, err)
	}
	c, _ := st.GetContent(cid)
	if c.Status != policy.StatusQueued || c.TargetAccount != "nb1" || c.ChannelGroup != policy.GroupManual {
		t.Fatalf("content = %+v", c)
	}
	log, err := st.GetPublishLog(cid, "naver_blog", "nb1")
	if err != nil || log.Status != "scheduled" {
		t.Fatalf("log = %+v err = %v", log, err)
	}
	jobs, err := st.ListJobsByRef(cid)
	if err != nil || len(jobs) != 1 || jobs[0].Type != "publish" {
		t.Fatalf("jobs = %+v err = %v", jobs, err)
	}
	if !strings.Contains(jobs[0].Payload, `"account":"nb1"`) {
		t.Fatalf("payload = %s", jobs[0].Payload)
	}
}

// Least-recently-published account wins; AI accounts never qualify.
func TestAssignPicksLeastRecent(t *testing.T) {
	st := openTestStore(t)
	svc := NewService(st, notify.NewService(st))
	seedAccount(t, st, "old", "naver_blog", "connected", "auto")
	seedAccount(t, st, "new", "naver_blog", "connected", "auto")
	seedAIAccount(t, st, "ai1", "meta_ai")
	cid := seedApproved(t, st, "LUNO", blogBrief, "blog")
	old := seedApproved(t, st, "LUNO", blogBrief, "blog")
	doneAt := time.Now().Add(-48 * time.Hour).UTC().Format(time.RFC3339)
	if _, err := st.CreatePublishDone(old, "naver_blog", "old", doneAt, "u"); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Assign(cid)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Assigned || res.Account != "new" {
		t.Fatalf("result = %+v, want new", res)
	}
}

// No eligible account: stays approved, one notice, second attempt silent.
func TestAssignNoAccountNotifiesOnce(t *testing.T) {
	st := openTestStore(t)
	svc := NewService(st, notify.NewService(st))
	cid := seedApproved(t, st, "LUNO", blogBrief, "blog")
	res, err := svc.Assign(cid)
	if err != nil {
		t.Fatal(err)
	}
	if res.Assigned || res.Reason != "no-account" {
		t.Fatalf("result = %+v", res)
	}
	c, _ := st.GetContent(cid)
	if c.Status != policy.StatusApproved || c.TargetAccount != "" {
		t.Fatalf("content = %+v", c)
	}
	n, err := st.CountUnreadNotifications()
	if err != nil || n != 1 {
		t.Fatalf("unread = %d err = %v", n, err)
	}
	if _, err := svc.Assign(cid); err != nil {
		t.Fatal(err)
	}
	n, _ = st.CountUnreadNotifications()
	if n != 1 {
		t.Fatalf("dedup failed, unread = %d", n)
	}
}

// W5-3: payload carries assembled body + signed image entries.
func TestAssignPayloadBodyAndImages(t *testing.T) {
	st := openTestStore(t)
	svc := NewService(st, notify.NewService(st))
	svc.FileKey = []byte("0123456789abcdef0123456789abcdef")
	seedAccount(t, st, "nb1", "naver_blog", "connected", "auto")
	p, err := st.CreateProduct("https://example.com/m", "A", 0, "LUNO")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetProductBrief(p.ID, blogBrief); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateAffiliateLink(p.ID, "shop", "https://shop.test/9"); err != nil {
		t.Fatal(err)
	}
	png := append([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, bytes.Repeat([]byte{0}, 32)...)
	msvc := media.NewService(st, t.TempDir())
	uid, _, err := msvc.Init("m.png", int64(len(png)), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := msvc.PutPart(uid, 1, bytes.NewReader(png)); err != nil {
		t.Fatal(err)
	}
	m, err := msvc.Complete(uid)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AttachMedia([]int64{m.ID}, p.ID); err != nil {
		t.Fatal(err)
	}
	c, err := st.CreateContent(&store.Content{ProductID: p.ID, Channel: "blog", Title: "t", Body: "hello https://example.com/m"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`UPDATE contents SET status='approved',is_sponsored=1 WHERE id=?`, c.ID); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Assign(c.ID)
	if err != nil || !res.Assigned {
		t.Fatalf("assign = %+v err = %v", res, err)
	}
	jobs, err := st.ListJobsByRef(c.ID)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("jobs = %+v err = %v", jobs, err)
	}
	var payload struct {
		Body   string `json:"body"`
		Images []struct {
			MediaID int64  `json:"media_id"`
			URL     string `json:"url"`
			SHA256  string `json:"sha256"`
		} `json:"images"`
	}
	if err := json.Unmarshal([]byte(jobs[0].Payload), &payload); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(payload.Body, policy.DisclosureText) {
		t.Fatalf("body = %q", payload.Body)
	}
	if !strings.Contains(payload.Body, "https://shop.test/9") {
		t.Fatalf("no buy link: %q", payload.Body)
	}
	if len(payload.Images) != 1 || payload.Images[0].MediaID != m.ID {
		t.Fatalf("images = %+v", payload.Images)
	}
	if payload.Images[0].SHA256 != m.SHA256 || !strings.Contains(payload.Images[0].URL, "&sig=") {
		t.Fatalf("image entry = %+v", payload.Images[0])
	}
	if !media.VerifyFileURL(svc.FileKey, m.ID,
		payload.Images[0].URL[strings.Index(payload.Images[0].URL, "exp=")+4:strings.Index(payload.Images[0].URL, "&sig=")],
		payload.Images[0].URL[strings.Index(payload.Images[0].URL, "sig=")+4:]) {
		t.Fatal("payload URL does not verify")
	}
}

// W0: per-account cap overrides the global cap both directions.
func TestAssignCapOverride(t *testing.T) {
	st := openTestStore(t)
	svc := NewService(st, notify.NewService(st))
	s := store.DefaultAutomationSettings()
	s.DailyCap = 1
	s.BlogMinLen = 1
	if err := store.SetAutomationSettings(st.DB(), s); err != nil {
		t.Fatal(err)
	}
	seedAccount(t, st, "strict", "naver_blog", "connected", "auto")
	seedAccount(t, st, "roomy", "naver_blog", "connected", "auto")
	now := time.Now().UTC().Format(time.RFC3339)
	first := seedApproved(t, st, "LUNO", blogBrief, "blog")
	if _, err := st.CreatePublishDone(first, "naver_blog", "strict", now, "u"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreatePublishDone(first, "naver_blog", "roomy", now, "u"); err != nil {
		t.Fatal(err)
	}
	// Global cap 1 reached for both. Loosen roomy to 5 via override.
	row, _ := st.GetAccountRowByName("roomy")
	row.DailyCap = 5
	if err := st.UpdateAccountRow(row); err != nil {
		t.Fatal(err)
	}
	res, err := svc.Assign(seedApproved(t, st, "LUNO", blogBrief, "blog"))
	if err != nil {
		t.Fatal(err)
	}
	if !res.Assigned || res.Account != "roomy" {
		t.Fatalf("override = %+v", res)
	}
}

// Daily cap reached: no assignment until tomorrow.
func TestAssignCapBlocks(t *testing.T) {
	st := openTestStore(t)
	svc := NewService(st, notify.NewService(st))
	s := store.DefaultAutomationSettings()
	s.DailyCap = 1
	s.BlogMinLen = 1
	if err := store.SetAutomationSettings(st.DB(), s); err != nil {
		t.Fatal(err)
	}
	seedAccount(t, st, "nb1", "naver_blog", "connected", "auto")
	done := seedApproved(t, st, "LUNO", blogBrief, "blog")
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := st.CreatePublishDone(done, "naver_blog", "nb1", now, "u"); err != nil {
		t.Fatal(err)
	}
	cid := seedApproved(t, st, "LUNO", blogBrief, "blog")
	res, err := svc.Assign(cid)
	if err != nil {
		t.Fatal(err)
	}
	if res.Assigned {
		t.Fatalf("assigned despite cap: %+v", res)
	}
}

// Automation off: silent no-op, no notice.
func TestAssignAutomationOff(t *testing.T) {
	st := openTestStore(t)
	svc := NewService(st, notify.NewService(st))
	s := store.DefaultAutomationSettings()
	s.Enabled = false
	if err := store.SetAutomationSettings(st.DB(), s); err != nil {
		t.Fatal(err)
	}
	seedAccount(t, st, "nb1", "naver_blog", "connected", "auto")
	cid := seedApproved(t, st, "LUNO", blogBrief, "blog")
	res, err := svc.Assign(cid)
	if err != nil {
		t.Fatal(err)
	}
	if res.Assigned || res.Reason != "automation-off" {
		t.Fatalf("result = %+v", res)
	}
	n, _ := st.CountUnreadNotifications()
	if n != 0 {
		t.Fatalf("unread = %d, want silent", n)
	}
}

// ReassignPending assigns waiting items once an account qualifies.
func TestReassignPending(t *testing.T) {
	st := openTestStore(t)
	svc := NewService(st, notify.NewService(st))
	seedAccount(t, st, "nb1", "naver_blog", "expired", "auto")
	cid := seedApproved(t, st, "LUNO", blogBrief, "blog")
	n, err := svc.ReassignPending()
	if err != nil || n != 0 {
		t.Fatalf("reassign = %d err = %v", n, err)
	}
	row, _ := st.GetAccountRowByName("nb1")
	row.SessionState = "connected"
	if err := st.UpdateAccountRow(row); err != nil {
		t.Fatal(err)
	}
	n, err = svc.ReassignPending()
	if err != nil || n != 1 {
		t.Fatalf("reassign = %d err = %v", n, err)
	}
	c, _ := st.GetContent(cid)
	if c.Status != policy.StatusQueued {
		t.Fatalf("status = %s", c.Status)
	}
}

func TestComputeSlot(t *testing.T) {
	loc := time.FixedZone("KST", 9*3600)
	at := func(h, m int) time.Time { return time.Date(2026, 5, 4, h, m, 0, 0, loc) }
	mustParse := func(t *testing.T, s string) time.Time {
		t.Helper()
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return v.In(loc)
	}
	// Inside window, no interval push.
	got := mustParse(t, computeSlot(at(10, 0), "", 120, "09:00", "22:00"))
	if got.Hour() != 10 {
		t.Fatalf("inside = %v", got)
	}
	// Interval pushes past now.
	last := at(9, 30).UTC().Format(time.RFC3339)
	got = mustParse(t, computeSlot(at(10, 0), last, 120, "09:00", "22:00"))
	if got.Hour() != 11 || got.Minute() != 30 {
		t.Fatalf("interval = %v", got)
	}
	// Outside window (late) -> tomorrow start.
	got = mustParse(t, computeSlot(at(23, 0), "", 0, "09:00", "22:00"))
	if got.Day() != 5 || got.Hour() != 9 {
		t.Fatalf("late = %v", got)
	}
	// Outside window (early) -> today start.
	got = mustParse(t, computeSlot(at(7, 0), "", 0, "09:00", "22:00"))
	if got.Day() != 4 || got.Hour() != 9 {
		t.Fatalf("early = %v", got)
	}
	// Overnight window: inside.
	got = mustParse(t, computeSlot(at(23, 30), "", 0, "22:00", "06:00"))
	if got.Hour() != 23 {
		t.Fatalf("overnight-in = %v", got)
	}
	// Overnight window: outside -> today 22:00.
	got = mustParse(t, computeSlot(at(10, 0), "", 0, "22:00", "06:00"))
	if got.Day() != 4 || got.Hour() != 22 {
		t.Fatalf("overnight-out = %v", got)
	}
}
