// Package policy holds content-domain rules as code: status machine, channel
// groups, and mandatory notices (§5, §7). It depends on nothing.
package policy

import (
	"fmt"
	"strings"
)

// Content statuses: draft -> review -> approved -> queued -> published.
// archived is terminal: reject-cap disposal (W6) with no return.
const (
	StatusDraft     = "draft"
	StatusReview    = "review"
	StatusApproved  = "approved"
	StatusQueued    = "queued"
	StatusPublished = "published"
	StatusArchived  = "archived"
)

// forward is the single allowed forward step from each status.
var forward = map[string]string{
	StatusDraft:    StatusReview,
	StatusReview:   StatusApproved,
	StatusApproved: StatusQueued,
	StatusQueued:   StatusPublished,
}

// backward holds the only allowed regressions: one step back each.
var backward = map[string]string{
	StatusReview:   StatusDraft,
	StatusApproved: StatusReview,
	StatusQueued:   StatusApproved,
}

// archivable lists the non-terminal statuses that may be disposed to
// archived (W6): reject-cap chains and superseded rejected originals.
var archivable = map[string]bool{
	StatusDraft:    true,
	StatusReview:   true,
	StatusApproved: true,
	StatusQueued:   true,
}

// ValidateTransition allows one forward step, one allowed regression, or
// disposal to archived. Skips and multi-step regressions are rejected.
func ValidateTransition(from, to string) error {
	if to == forward[from] {
		return nil
	}
	if to == backward[from] {
		return nil
	}
	if to == StatusArchived && archivable[from] {
		return nil
	}
	return fmt.Errorf("transition %s -> %s is not allowed", from, to)
}

// Generate channels: the only 4 kinds a worker may produce.
var GenerateChannels = map[string]bool{
	"instagram": true,
	"shortform": true,
	"cardnews":  true,
	"blog":      true,
}

// Channel groups (§7). The two groups are stored separately and never mixed
// in one queue listing.
const (
	GroupOfficialAPI = "official_api"
	GroupManual      = "manual"
)

// officialPlatforms have a real publish API; manual ones need session posting.
var officialPlatforms = map[string]bool{
	"instagram": true,
	"facebook":  true,
	"threads":   true,
	"tiktok":    true,
	"youtube":   true,
	"blogger":   true,
}

var manualPlatforms = map[string]bool{
	"naver_blog": true,
	"tistory":    true,
}

// GroupOf returns the channel group for a platform, or false when unknown.
//
// Deprecated: new code reads the channel_platforms registry (W21-1) instead.
// Kept for the frozen manual Schedule path and its tests.
func GroupOf(platform string) (string, bool) {
	if officialPlatforms[platform] {
		return GroupOfficialAPI, true
	}
	if manualPlatforms[platform] {
		return GroupManual, true
	}
	return "", false
}

// KnownPlatform reports whether a platform belongs to either group.
func KnownPlatform(platform string) bool {
	_, ok := GroupOf(platform)
	return ok
}

// DisclosureText is the generic compensation notice auto-inserted at the top
// of affiliate/sponsored contents. It is intentionally generic: no platform's
// recommended wording is confirmed, so none is hardcoded here.
const DisclosureText = "이 콘텐츠는 경제적 대가를 받을 수 있는 제휴·협찬 내용을 포함합니다."

// AILabelText marks contents where an AI virtual persona recommends or
// endorses. It is attached only in that case, never blanket-applied.
const AILabelText = "이 콘텐츠에는 AI 가상인물이 추천·보증 주체로 등장합니다."

// UnclassifiedCause labels zero/missing metrics without interpretation.
const UnclassifiedCause = "원인 미분류"

// AssembleBody is the single post assembler (W5-2). Every publish path
// (P-lane payload, API executor) sends exactly this body:
// sponsored disclosure first, raw body, AI label when a persona endorses,
// buy-link CTA lines last. Nothing is escaped or trimmed.
func AssembleBody(sponsored bool, body string, persona bool, buyLinks []string) string {
	parts := make([]string, 0, 3+len(buyLinks))
	if sponsored {
		parts = append(parts, DisclosureText)
	}
	parts = append(parts, body)
	if persona {
		parts = append(parts, AILabelText)
	}
	for _, link := range buyLinks {
		if link != "" {
			parts = append(parts, "구매 링크: "+link)
		}
	}
	return strings.Join(parts, "\n\n")
}
