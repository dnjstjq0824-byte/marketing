package guard

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"somun/internal/content/policy"
	"somun/internal/store"
)

// Check runs the shipped W18 gates over one approved content for one
// platform. It returns nil when the item may publish, or a *Failure with
// a human-readable Korean reason. Checks never mutate; the caller owns
// the reject path (decision record + bounded regeneration).
func Check(st *store.Store, contentID int64, platform string, now time.Time) *Failure {
	c, err := st.GetContent(contentID)
	if err != nil {
		return &Failure{Code: "guard-error", Reason: "게시물을 읽지 못했습니다"}
	}
	p, err := st.GetProduct(c.ProductID)
	if err != nil {
		return &Failure{Code: "guard-error", Reason: "작업을 읽지 못했습니다"}
	}
	// W18-5: outside links smuggled through source material. Allowed =
	// the product URL, registered affiliate links, and the team brand's
	// base buy link.
	allowed := []string{strings.TrimSpace(p.SourceURL)}
	if links, err := st.ListAffiliateLinks(p.ID); err == nil {
		for _, l := range links {
			allowed = append(allowed, l.URL)
		}
	}
	if teamID, terr := st.EffectiveProductTeam(p.ID); terr == nil {
		if brand, berr := st.GetTeamBrand(teamID); berr == nil && strings.TrimSpace(brand.BuyLink) != "" {
			allowed = append(allowed, strings.TrimSpace(brand.BuyLink))
		}
	}
	if bad := OutsideLinks(c.Body, allowed); len(bad) > 0 {
		return &Failure{Code: "outside-link",
			Reason: fmt.Sprintf("외부 링크 차단: %s (자료에 없던 링크는 발행 불가)", bad[0])}
	}
	// W18-2: banned expressions (common + industry wordlists).
	if hit := BannedHit(c.Title+" "+c.Body, p.Industry); hit != "" {
		return &Failure{Code: "banned-word",
			Reason: fmt.Sprintf("금칙어: %s — 다른 표현으로 다시 씁니다", hit)}
	}
	// W18-4: no hotlinked images in the body; publish images come from
	// the media library (uploads or AI-generated), never remote embeds.
	if bad := RemoteBodyImages(c.Body); len(bad) > 0 {
		return &Failure{Code: "remote-image",
			Reason: fmt.Sprintf("외부 이미지 차단: %s (이미지는 파일로 첨부)", bad[0])}
	}
	// W18-3: exit-B posts must carry the compensation notice in the exact
	// assembled body that ships (double-check over W5-2). The exit type is
	// authoritative: a missing sponsored flag on the content is itself the
	// block, so legacy mismatches fail closed instead of shipping bare.
	sponsored := c.IsSponsored || p.ExitType == "B"
	if sponsored {
		var buyLinks []string
		if links, err := st.ListAffiliateLinks(p.ID); err == nil {
			for _, l := range links {
				buyLinks = append(buyLinks, l.URL)
			}
		}
		assembled := policy.AssembleBody(c.IsSponsored, c.Body, c.IsVirtualPersona, buyLinks)
		if !strings.Contains(assembled, policy.DisclosureText) {
			return &Failure{Code: "disclosure-missing",
				Reason: "대가 표시 누락: 제휴 글에 보상 고지가 없습니다"}
		}
	}
	// W3: blog drafts meet the length floor and carry a purchase link;
	// instagram drafts need a linked image. Failures return for rewrite
	// with the same bounded loop as the W18 gates.
	if c.Channel == "blog" {
		if cfg, serr := store.GetAutomationSettings(st.DB()); serr == nil && cfg.BlogMinLen > 0 {
			if n := utf8.RuneCountInString(c.Body); n < cfg.BlogMinLen {
				return &Failure{Code: "too-short",
					Reason: fmt.Sprintf("분량 미달: %d자 (최소 %d자) — 보강해서 다시 씁니다", n, cfg.BlogMinLen)}
			}
		}
		// Purchase links are required only when the work has URLs to
		// include: topic/image-only works without any URL skip the rule
		// instead of deadlocking against the outside-link gate.
		if hasLinkableURL(st, p) && len(linkRe.FindAllString(c.Body, -1)) == 0 {
			return &Failure{Code: "no-link",
				Reason: "구매 링크 없음: 블로그 글에는 링크를 1개 이상 넣습니다"}
		}
	}
	if c.Channel == "instagram" && !hasProductImage(st, c.ProductID) {
		return &Failure{Code: "no-image",
			Reason: "이미지 없음: 인스타 글에는 이미지를 1장 이상 연결합니다"}
	}
	// W18-1: near-duplicate of a recent same-platform post. Threshold and
	// window are package vars so tests and operators can tune them.
	since := now.AddDate(0, 0, -SimilarityWindowDays).Format(time.RFC3339)
	corpus, err := st.RecentPublished(platform, since)
	if err != nil {
		return &Failure{Code: "guard-error", Reason: "최근 발행 글을 읽지 못했습니다"}
	}
	rows := make([]Published, 0, len(corpus))
	for _, r := range corpus {
		if r.ID == c.ID {
			continue
		}
		rows = append(rows, Published{Account: r.TargetAccount, Body: r.Body, UpdatedAt: r.UpdatedAt})
	}
	if hit, score, ok := Similar(c.Body, rows, SimilarityThreshold); ok {
		return &Failure{Code: "similar",
			Reason: fmt.Sprintf("유사문서: %s %s 글과 %d%% 유사 — 다른 각도로 다시 씁니다",
				daysAgo(hit.UpdatedAt, now), accountName(hit.Account), int(score*100+0.5))}
	}
	return nil
}

// hasLinkableURL reports whether the work offers URLs the draft must use.
func hasLinkableURL(st *store.Store, p *store.Product) bool {
	if strings.TrimSpace(p.SourceURL) != "" {
		return true
	}
	if links, err := st.ListAffiliateLinks(p.ID); err == nil && len(links) > 0 {
		return true
	}
	return false
}

// hasProductImage reports any library or legacy image on the work.
func hasProductImage(st *store.Store, productID int64) bool {
	if media, err := st.ListMedia(productID, 1); err == nil && len(media) > 0 {
		return true
	}
	if imgs, err := st.ListImages(productID); err == nil && len(imgs) > 0 {
		return true
	}
	return false
}

func accountName(a string) string {
	if strings.TrimSpace(a) == "" {
		return "같은 플랫폼"
	}
	return a
}

// brandFor loads the owning team's brand card for regen prompts (W16).
func brandFor(st *store.Store, productID int64) map[string]any {
	teamID, err := st.EffectiveProductTeam(productID)
	if err != nil {
		return map[string]any{}
	}
	b, err := st.GetTeamBrand(teamID)
	if err != nil {
		return map[string]any{}
	}
	return map[string]any{
		"intro": b.Intro, "tone": b.Tone, "banned": b.Banned, "link": b.BuyLink,
	}
}

// MaxAutoRegen bounds guard-triggered regeneration, mirroring the chief
// path (queue.MaxAutoRegen lives across an import cycle, so the bound is
// repeated here and asserted equal in tests).
const MaxAutoRegen = 2

// Decider labels guard records in the review history.
const Decider = "안전망"

// Reject records a guard block as a rejected review decision, returns the
// content to review, and queues one bounded regeneration carrying the
// reason as feedback. Past the cap it reports false: the caller escalates
// to a human instead of looping forever.
func Reject(st *store.Store, contentID int64, f *Failure) (regenerated bool, err error) {
	c, err := st.GetContent(contentID)
	if err != nil {
		return false, err
	}
	if _, err := st.CreateReviewDecision(contentID, Decider, "rejected", f.Reason); err != nil {
		return false, err
	}
	switch c.Status {
	case policy.StatusApproved:
		_ = st.UpdateContentStatus(contentID, policy.StatusReview)
	case policy.StatusQueued:
		_ = st.UpdateContentStatus(contentID, policy.StatusApproved)
	}
	n, err := st.CountRejectedChain(contentID)
	if err != nil || n > MaxAutoRegen {
		// W6 mirror: past the cap the chain is disposed, not looped.
		if err == nil {
			_ = st.UpdateContentStatus(contentID, policy.StatusArchived)
		}
		return false, err
	}
	angle, keyMessages := "", ""
	if strategy, serr := st.GetStrategyByProduct(c.ProductID); serr == nil {
		angle, keyMessages = strategy.Angle, strategy.KeyMessages
	}
	retry, _ := json.Marshal(map[string]any{
		"product_id":   c.ProductID,
		"revision_of":  contentID,
		"feedback":     f.Reason,
		"angle":        angle,
		"key_messages": keyMessages,
		"brand":        brandFor(st, c.ProductID),
	})
	// Literal "generate": queue imports the scheduler, which imports this
	// package, so the constant is out of reach here by design.
	regenJob, err := st.CreateJob("generate", c.ProductID, string(retry))
	if err != nil {
		return false, err
	}
	if team, terr := st.EffectiveProductTeam(c.ProductID); terr == nil {
		_ = st.SetJobTeam(regenJob.ID, team)
	}
	return true, nil
}
