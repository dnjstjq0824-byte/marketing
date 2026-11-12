// Package policy holds content-domain rules as code: status machine, channel
// groups, and mandatory notices (§5, §7). It depends on nothing.
package policy

import "fmt"

// Content statuses: draft -> review -> approved -> queued -> published.
const (
	StatusDraft     = "draft"
	StatusReview    = "review"
	StatusApproved  = "approved"
	StatusQueued    = "queued"
	StatusPublished = "published"
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

// ValidateTransition allows one forward step or one allowed regression.
// Skips and multi-step regressions are rejected.
func ValidateTransition(from, to string) error {
	if to == forward[from] {
		return nil
	}
	if to == backward[from] {
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
