// Package scheduler assigns approved contents to channel accounts (W4).
// Assignment is server-side: AI review decisions never carry accounts.
// The scheduler only reads automation settings; enforcement stays there.
package scheduler

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"somun/internal/content/policy"
	"somun/internal/guard"
	"somun/internal/media"
	"somun/internal/notify"
	"somun/internal/store"
)

// Service assigns approved contents to connected auto accounts.
type Service struct {
	store  *store.Store
	notify *notify.Service
	// FileKey signs image payload URLs (W5-3). Empty omits images.
	FileKey []byte
}

// NewService builds the scheduler over a store and the notify outbox.
// notify may be nil (notifications skipped, e.g. in unit tests).
func NewService(st *store.Store, ntf *notify.Service) *Service {
	return &Service{store: st, notify: ntf}
}

// Result reports one assignment attempt.
type Result struct {
	Assigned bool
	Account  string
	Platform string
	At       string // scheduled_at (RFC3339 UTC)
	Reason   string // why not assigned: not-approved|automation-off|no-platform|preparing|guard|dist-cap|no-account
}

// aiPlatforms mirrors the console AI list. Channel candidates must never be
// AI service accounts even if a row's kind is ever mislabeled.
var aiPlatforms = map[string]bool{
	"meta_ai": true, "claude_app": true, "chatgpt": true, "gemini": true, "muse": true,
}

// briefPlatform maps S2 brief platform values to account platforms.
func briefPlatform(p string) string {
	if p == "naver" {
		return "naver_blog"
	}
	return strings.ToLower(strings.TrimSpace(p))
}

// Assign moves one approved content to queued on the least-recently-used
// eligible account. It is a no-op (Assigned=false, nil error) when the
// content is not approved, automation is off, or no account qualifies.
func (s *Service) Assign(contentID int64) (Result, error) {
	c, err := s.store.GetContent(contentID)
	if err != nil {
		return Result{}, err
	}
	if c.Status != policy.StatusApproved || c.TargetAccount != "" || c.Held {
		return Result{Reason: "not-approved"}, nil
	}
	settings, err := store.GetAutomationSettings(s.store.DB())
	if err != nil {
		return Result{}, err
	}
	if !settings.Enabled {
		return Result{Reason: "automation-off"}, nil
	}
	platform, err := s.platformFor(c)
	if err != nil {
		s.notifyOnce(c.ID, "플랫폼 미정 — 대기 중")
		return Result{Reason: "no-platform"}, nil
	}
	plat, err := s.store.GetChannelPlatform(platform)
	if err != nil {
		s.notifyOnce(c.ID, "플랫폼 미정 — 대기 중")
		return Result{Reason: "no-platform"}, nil
	}
	if plat.Status != store.PlatformReady {
		s.notifyOnce(c.ID, plat.Name+" 준비 중 — 대기 중")
		return Result{Reason: "preparing"}, nil
	}
	// W18: server-side safety net after chief review, before any publish.
	// A flagged item returns for regeneration with a readable reason; past
	// the regen cap it waits for a human instead of looping.
	if f := guard.Check(s.store, c.ID, platform, time.Now()); f != nil {
		regen, rerr := guard.Reject(s.store, c.ID, f)
		if rerr == nil && regen {
			s.notifyOnce(c.ID, "안전망: "+f.Reason)
		} else {
			s.notifyOnce(c.ID, fmt.Sprintf("안전망: %s — 사람이 검수하세요", f.Reason))
		}
		return Result{Reason: "guard"}, nil
	}
	// W21-4: the team's distribution cap bounds versions per work+platform.
	// Over-cap drafts park as approved until a slot frees up.
	if limit, lok := s.distributionLimit(c.ProductID); lok {
		if n, cerr := s.store.CountProductPlatformQueued(c.ProductID, platform); cerr == nil && n >= limit {
			s.notifyOnce(c.ID, fmt.Sprintf("분배 상한(%d건) — 대기 중", limit))
			return Result{Reason: "dist-cap"}, nil
		}
	}
	now := time.Now()
	teamID, _ := s.store.EffectiveProductTeam(c.ProductID)
	pool, err := s.candidates(platform, teamID)
	if err != nil {
		return Result{}, err
	}
	var eligible []*store.ChannelAccount
	for _, a := range pool {
		limit := settings.DailyCap
		if a.DailyCap > 0 {
			limit = a.DailyCap // per-account override (W0), else global
		}
		ok, err := s.hasCapRoom(a.Name, limit, now)
		if err != nil {
			return Result{}, err
		}
		if ok {
			eligible = append(eligible, a)
		}
	}
	if len(eligible) == 0 {
		s.notifyOnce(c.ID, "연결된 "+platform+" 계정 없음 — 대기 중")
		return Result{Reason: "no-account"}, nil
	}
	best := s.pickLeastRecent(eligible)
	at := computeSlot(now, s.lastDone(best.Name), settings.PublishIntervalMin,
		settings.WindowStart, settings.WindowEnd)
	group := plat.Group
	if group == "" {
		group = policy.GroupManual
	}
	if existing, err := s.store.GetPublishLog(c.ID, platform, best.Name); err == nil && existing != nil {
		if existing.Status != "scheduled" {
			// A done/failed log holds this triplet: leave the row alone, stay approved.
			return Result{Reason: "no-account"}, nil
		}
		// A scheduled row already exists (earlier partial run): finish the
		// assignment idempotently instead of duplicating it.
		_ = s.store.UpdateContentStatus(c.ID, policy.StatusQueued)
		if group == policy.GroupManual {
			_ = s.ensurePublishJob(c.ID, best.Name, platform)
		}
		return Result{Assigned: true, Account: best.Name, Platform: platform, At: at}, nil
	}
	if err := s.store.ScheduleContent(c.ID, best.Name, platform, group, at); err != nil {
		return Result{}, err
	}
	if err := s.store.UpdateContentStatus(c.ID, policy.StatusQueued); err != nil {
		return Result{}, err
	}
	if _, err := s.store.CreatePublishLog(c.ID, platform, best.Name, "scheduled", "auto-assigned"); err != nil {
		_ = s.store.UpdateContentStatus(c.ID, policy.StatusApproved)
		return Result{}, err
	}
	if group == policy.GroupManual {
		if err := s.ensurePublishJob(c.ID, best.Name, platform); err != nil {
			return Result{}, err
		}
	}
	return Result{Assigned: true, Account: best.Name, Platform: platform, At: at}, nil
}

// ReassignPending assigns every approved-but-unassigned content.
// It is called when an account connects. Per-item errors are skipped so one
// bad row cannot block the rest; the return count is assigned items.
func (s *Service) ReassignPending() (int, error) {
	items, err := s.store.ListContents(policy.StatusApproved, "", 500)
	if err != nil {
		return 0, err
	}
	done := 0
	for _, c := range items {
		if c.TargetAccount != "" || c.Held {
			continue
		}
		res, err := s.Assign(c.ID)
		if err != nil {
			continue
		}
		if res.Assigned {
			done++
		}
	}
	return done, nil
}

// platformFor resolves the target platform from the work brief:
// the brief channel entry matching the content channel carries the platform.
// A pinned want_platform (W3-2) wins when it pairs with the channel in the
// brief; otherwise the first matching entry wins (legacy).
// The registry (W21-1) is authoritative; unknown keys stay unassigned.
func (s *Service) platformFor(c *store.Content) (string, error) {
	p, err := s.store.GetProduct(c.ProductID)
	if err != nil {
		return "", err
	}
	var brief struct {
		Channels []struct {
			Channel  string `json:"channel"`
			Platform string `json:"platform"`
		} `json:"channels"`
	}
	if err := json.Unmarshal([]byte(p.BriefJSON), &brief); err != nil {
		return "", errors.New("unreadable brief")
	}
	if want := strings.TrimSpace(c.WantPlatform); want != "" {
		for _, ch := range brief.Channels {
			if ch.Channel == c.Channel && briefPlatform(ch.Platform) == want {
				if _, err := s.store.GetChannelPlatform(want); err == nil {
					return want, nil
				}
			}
		}
	}
	for _, ch := range brief.Channels {
		if ch.Channel == c.Channel {
			plat := briefPlatform(ch.Platform)
			if _, err := s.store.GetChannelPlatform(plat); err == nil {
				return plat, nil
			}
		}
	}
	return "", errors.New("no platform for channel")
}

// candidates lists connected auto channel accounts for one platform.
// When the owning team links channels (W16-5), only those accounts qualify;
// teams without links publish through every connected channel.
func (s *Service) candidates(platform string, teamID int64) ([]*store.ChannelAccount, error) {
	rows, err := s.store.ListAccountRows()
	if err != nil {
		return nil, err
	}
	allowed := map[int64]bool{}
	if teamID > 0 {
		if ids, cerr := s.store.ListTeamChannelIDs(teamID); cerr == nil && len(ids) > 0 {
			for _, id := range ids {
				allowed[id] = true
			}
		}
	}
	var out []*store.ChannelAccount
	for _, a := range rows {
		if a.Kind == "agent_service" || aiPlatforms[a.Platform] {
			continue
		}
		if a.Platform != platform {
			continue
		}
		if a.SessionState != "connected" || a.PostingMode != "auto" {
			continue
		}
		if len(allowed) > 0 && !allowed[a.ID] {
			continue
		}
		out = append(out, a)
	}
	return out, nil
}

// hasCapRoom reports daily-cap headroom. A cap of 0 publishes nothing.
func (s *Service) hasCapRoom(account string, cap int, now time.Time) (bool, error) {
	if cap <= 0 {
		return false, nil
	}
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	done, err := s.store.CountAccountDoneSince(account, midnight.UTC().Format(time.RFC3339))
	if err != nil {
		return false, err
	}
	return done < cap, nil
}

func (s *Service) lastDone(account string) string {
	at, err := s.store.LastDoneAt(account)
	if err != nil {
		return ""
	}
	return at
}

// pickLeastRecent returns the candidate with the fewest queued items,
// then the oldest done time, so multi-version sets spread across the
// team's accounts instead of piling onto one. Never-published accounts
// ("") sort first; ties break by lower ID.
func (s *Service) pickLeastRecent(pool []*store.ChannelAccount) *store.ChannelAccount {
	best := pool[0]
	bestLoad, bestAt := s.queuedLoad(best.Name), s.lastDone(best.Name)
	for _, a := range pool[1:] {
		load, at := s.queuedLoad(a.Name), s.lastDone(a.Name)
		if load < bestLoad || (load == bestLoad && (at < bestAt || (at == bestAt && a.ID < best.ID))) {
			best, bestLoad, bestAt = a, load, at
		}
	}
	return best
}

// distributionLimit resolves the W21-4 cap: mode one pins 1, mode all
// uses the team cap (default 5). Unknown teams report no limit.
func (s *Service) distributionLimit(productID int64) (int, bool) {
	teamID, err := s.store.EffectiveProductTeam(productID)
	if err != nil {
		return 0, false
	}
	t, err := s.store.GetTeam(teamID)
	if err != nil {
		return 0, false
	}
	if t.DistributionMode == "one" {
		return 1, true
	}
	cap := t.DistributionCap
	if cap < 1 {
		cap = 5
	}
	return cap, true
}

// queuedLoad counts an account's waiting assignments.
func (s *Service) queuedLoad(account string) int {
	n, err := s.store.CountAccountQueued(account)
	if err != nil {
		return 0
	}
	return n
}

// computeSlot returns the earliest publish instant (RFC3339 UTC) honoring
// the per-account interval and the publish window. Overnight windows
// (start later than end) are supported.
func computeSlot(now time.Time, lastDoneUTC string, intervalMin int, startHHMM, endHHMM string) string {
	loc := now.Location()
	earliest := now.In(loc)
	if lastDoneUTC != "" {
		if done, err := time.Parse(time.RFC3339, lastDoneUTC); err == nil {
			if next := done.Add(time.Duration(intervalMin) * time.Minute); next.After(earliest) {
				earliest = next.In(loc)
			}
		}
	}
	sh, sm := splitHHMM(startHHMM)
	eh, em := splitHHMM(endHHMM)
	inWindow := func(t time.Time) bool {
		mins := t.Hour()*60 + t.Minute()
		start, end := sh*60+sm, eh*60+em
		if start <= end {
			return mins >= start && mins < end
		}
		return mins >= start || mins < end
	}
	atDay := func(day time.Time, h, m int) time.Time {
		return time.Date(day.Year(), day.Month(), day.Day(), h, m, 0, 0, loc)
	}
	t := earliest
	if !inWindow(t) {
		if sh*60+sm > eh*60+em {
			// Overnight window: outside means between end and start.
			t = atDay(t, sh, sm)
			if !t.After(earliest) {
				t = atDay(t.Add(24*time.Hour), sh, sm)
			}
		} else if t.Hour()*60+t.Minute() < sh*60+sm {
			t = atDay(t, sh, sm)
		} else {
			t = atDay(t.Add(24*time.Hour), sh, sm)
		}
	}
	return t.UTC().Format(time.RFC3339)
}

func splitHHMM(s string) (int, int) {
	var h, m int
	_, _ = time.Parse("15:04", s)
	if len(s) == 5 && s[2] == ':' {
		h = int(s[0]-'0')*10 + int(s[1]-'0')
		m = int(s[3]-'0')*10 + int(s[4]-'0')
	}
	return h, m
}

// ensurePublishJob enqueues one P-lane job per content+account pair.
// It mirrors content.ensurePublishJob without importing content
// (content -> queue -> agent -> session -> content would cycle).
func (s *Service) ensurePublishJob(contentID int64, account, platform string) error {
	c, err := s.store.GetContent(contentID)
	if err != nil {
		return err
	}
	active, err := s.store.HasActivePublishJob(c.ID, account)
	if err != nil || active {
		return err
	}
	business, priority := "", 0
	if p, err := s.store.GetProduct(c.ProductID); err == nil {
		business, priority = p.Business, p.Priority
	}
	var buyLinks []string
	if links, err := s.store.ListAffiliateLinks(c.ProductID); err == nil {
		for _, l := range links {
			buyLinks = append(buyLinks, l.URL)
		}
	}
	body := policy.AssembleBody(c.IsSponsored, c.Body, c.IsVirtualPersona, buyLinks)
	images := s.imagePayload(c.ProductID)
	payload, _ := json.Marshal(map[string]any{
		"content_id": c.ID, "account": account, "platform": platform,
		"title": c.Title, "body": body, "images": images,
	})
	// Literal "publish": importing queue here would cycle (scheduler is
	// called from queue, so queue -> scheduler -> queue is forbidden).
	pubJob, err := s.store.CreateJobFull("publish", c.ID, string(payload), business, priority, false)
	if err != nil {
		return err
	}
	if team, terr := s.store.EffectiveProductTeam(c.ProductID); terr == nil {
		_ = s.store.SetJobTeam(pubJob.ID, team)
	}
	return nil
}

// imagePayload lists attached media as signed download entries (W5-3):
// images for blog bodies plus W14 videos for reels/shorts. Without a
// FileKey the list stays empty and text still publishes.
func (s *Service) imagePayload(productID int64) []map[string]any {
	if len(s.FileKey) == 0 {
		return nil
	}
	items, err := s.store.ListMedia(productID, 50)
	if err != nil {
		return nil
	}
	exp := time.Now().Add(24 * time.Hour).Unix()
	var out []map[string]any
	for _, m := range items {
		if !strings.HasPrefix(m.Mime, "image/") && !strings.HasPrefix(m.Mime, "video/") {
			continue
		}
		out = append(out, map[string]any{
			"media_id": m.ID,
			"kind":     m.Kind,
			"mime":     m.Mime,
			"url":      media.SignFileURL(s.FileKey, m.ID, exp),
			"sha256":   m.SHA256,
		})
	}
	return out
}

func (s *Service) notifyOnce(contentID int64, message string) {
	if s.notify == nil {
		return
	}
	// publish_failed dedups per unread row: the "waiting" notice fires once.
	// A dedicated kind would need a notifications CHECK rebuild migration.
	_, _ = s.notify.Notify(notify.KindPublishFail, "content", contentID, message)
}
