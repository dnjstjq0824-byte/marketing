// Package works implements the S2/S3 work API over products (UI_DESIGN §7-2).
// A work IS a product; the brief carries the S2 form for workers (W3) and
// the W4 scheduler. Team assignment stays inert until W16.
package works

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"somun/internal/product"
	"somun/internal/store"
)

// jobAnalyze/jobGenerate mirror queue job types without importing queue.
const (
	jobAnalyze  = "analyze"
	jobGenerate = "generate"
	jobResearch = "research"
)

var (
	ErrNoInput      = errors.New("링크·사진·주제 중 하나는 필요합니다")
	ErrNoChannel    = errors.New("채널을 하나 이상 고르세요")
	ErrBadChannel   = errors.New("채널이 올바르지 않습니다")
	ErrBadVersions  = errors.New("버전은 1~4개입니다")
	ErrBadStart     = errors.New("시작 시간이 올바르지 않습니다")
	ErrNoDisclosure = errors.New("대가 표시 문구를 입력하세요")
	ErrBadMedia     = errors.New("업로드 파일을 확인하세요")
	ErrNoAccount    = errors.New("work not found")
)

// ChannelReq is one S2 channel chip with version count.
type ChannelReq struct {
	Channel  string `json:"channel"`
	Platform string `json:"platform"`
	Versions int    `json:"versions"`
}

// InputReq is one S2 input row.
type InputReq struct {
	Kind     string  `json:"kind"`
	URL      string  `json:"url"`
	Text     string  `json:"text"`
	MediaIDs []int64 `json:"mediaIds"`
}

// AffiliateReq is the S2 sponsorship block.
type AffiliateReq struct {
	IsSponsored bool   `json:"isSponsored"`
	Disclosure  string `json:"disclosure"`
}

// CreateReq is POST /api/works.
type CreateReq struct {
	Inputs    []InputReq  `json:"inputs"`
	Channels  []ChannelReq `json:"channels"`
	Business  string      `json:"business"`
	StartAt   string      `json:"startAt"`
	Affiliate AffiliateReq `json:"affiliate"`
	Team      string      `json:"team"`
}

var validChannels = map[string]bool{
	"blog": true, "instagram": true, "shortform": true, "cardnews": true,
}

// Service creates and projects works.
type Service struct {
	store *store.Store
}

// NewService builds the works service.
func NewService(st *store.Store) *Service { return &Service{store: st} }

// Create validates the S2 form, stores the product with its brief, attaches
// uploads, and auto-creates the analyze job.
func (s *Service) Create(req CreateReq) (int64, error) {
	if len(req.Inputs) == 0 {
		return 0, ErrNoInput
	}
	var sourceURL, topic string
	var mediaIDs []int64
	for _, in := range req.Inputs {
		switch in.Kind {
		case "url":
			u := strings.TrimSpace(in.URL)
			if err := product.ValidateURL(u); err != nil {
				return 0, err
			}
			if sourceURL == "" {
				sourceURL = u
			}
		case "topic":
			t := strings.TrimSpace(in.Text)
			if t == "" || len(t) > 200 {
				return 0, ErrNoInput
			}
			if topic == "" {
				topic = t
			}
		case "image":
			if len(in.MediaIDs) == 0 {
				return 0, ErrBadMedia
			}
			mediaIDs = append(mediaIDs, in.MediaIDs...)
		default:
			return 0, ErrNoInput
		}
	}
	if len(req.Channels) == 0 {
		return 0, ErrNoChannel
	}
	for _, c := range req.Channels {
		if !validChannels[c.Channel] || strings.TrimSpace(c.Platform) == "" || len(c.Platform) > 40 {
			return 0, ErrBadChannel
		}
		if c.Versions < 1 || c.Versions > 10 {
			return 0, ErrBadVersions
		}
	}
	for _, id := range mediaIDs {
		m, err := s.store.GetMedia(id)
		if err != nil {
			return 0, ErrBadMedia
		}
		if m.ProductID != nil {
			return 0, ErrBadMedia
		}
	}
	disclosure := strings.TrimSpace(req.Affiliate.Disclosure)
	if req.Affiliate.IsSponsored && disclosure == "" {
		return 0, ErrNoDisclosure
	}
	startAt, err := normalizeStart(req.StartAt)
	if err != nil {
		return 0, err
	}
	exit := "A"
	if req.Affiliate.IsSponsored {
		exit = "B"
	}
	p, err := s.store.CreateProduct(sourceURL, exit, 0, strings.TrimSpace(req.Business))
	if err != nil {
		return 0, err
	}
	if topic != "" {
		if err := s.store.SetProductName(p.ID, topic); err != nil {
			return 0, err
		}
	}
	if len(mediaIDs) > 0 {
		if err := s.store.AttachMedia(mediaIDs, p.ID); err != nil {
			return 0, err
		}
	}
	teamID, err := s.store.ResolveWorkTeam(req.Team, strings.TrimSpace(req.Business))
	if err != nil {
		return 0, err
	}
	channels := s.effectiveChannels(req.Channels, teamID)
	brief, _ := json.Marshal(map[string]any{
		"inputs": req.Inputs, "channels": channels,
		"business": strings.TrimSpace(req.Business), "startAt": startAt,
		"affiliate": map[string]any{"isSponsored": req.Affiliate.IsSponsored, "disclosure": disclosure},
		"team": strings.TrimSpace(req.Team),
	})
	if err := s.store.SetProductBrief(p.ID, string(brief)); err != nil {
		return 0, err
	}
	images := []string{}
	for _, id := range mediaIDs {
		if m, err := s.store.GetMedia(id); err == nil {
			images = append(images, m.Path)
		}
	}
	payload, _ := json.Marshal(map[string]any{
		"product_id": p.ID, "source_url": sourceURL, "topic": topic,
		"images": images, "channels": channels,
		"affiliate": map[string]any{"isSponsored": req.Affiliate.IsSponsored, "disclosure": disclosure},
	})
	// W1: typed inputs. Topic-only works research instead of parsing.
	kind := "topic"
	if sourceURL != "" {
		kind = "url"
	} else if len(mediaIDs) > 0 {
		kind = "image"
	}
	targets, _ := json.Marshal(targetList(channels))
	if err := s.store.SetProductInput(p.ID, kind, topic, string(targets)); err != nil {
		return 0, err
	}
	// W18-2: industry tag for the banned-word lists, auto-detected from
	// the business/topic text; the product API accepts an override.
	if industry := detectIndustry(strings.TrimSpace(req.Business) + " " + topic + " " + sourceURL); industry != "" {
		if err := s.store.SetProductIndustry(p.ID, industry); err != nil {
			return 0, err
		}
	}
	// W16: the work belongs to one team from birth; every downstream job
	// inherits it.
	if err := s.store.SetProductTeam(p.ID, teamID); err != nil {
		return 0, err
	}
	firstJob := jobAnalyze
	if kind == "topic" {
		firstJob = jobResearch
	}
	job, err := s.store.CreateJob(firstJob, p.ID, string(payload))
	if err != nil {
		return 0, fmt.Errorf("auto-create %s job: %w", firstJob, err)
	}
	if err := s.store.SetJobTeam(job.ID, teamID); err != nil {
		return 0, err
	}
	return p.ID, nil
}

// industryKeywords maps signal words to W18-2 industry tags.
var industryKeywords = []struct {
	tag   string
	words []string
}{
	{"cosmetics", []string{"화장품", "cosmetic", "스킨", "마스크팩", "선크림", "에센스", "앰플", "쿠션", "네일", "nail"}},
	{"health", []string{"건강", "health", "영양제", "supplement", "유산균", "프로바이오틱스", "비타민", "오메가", "관절", "혈압", "당뇨"}},
	{"food", []string{"식품", "food", "맛집", "다이어트", "diet", "꿀", "honey", "커피", "음료", "과자", "라면"}},
}

// detectIndustry tags a work for the banned-word lists. First match wins;
// no signal means common-only.
func detectIndustry(haystack string) string {
	lowered := strings.ToLower(haystack)
	for _, group := range industryKeywords {
		for _, w := range group.words {
			if strings.Contains(lowered, strings.ToLower(w)) {
				return group.tag
			}
		}
	}
	return ""
}

// targetList canonicalizes the brief's platform spellings for W1 queries.
func targetList(channels []ChannelReq) []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range channels {
		plat := canonicalPlatform(c.Platform)
		if plat == "" || seen[plat] {
			continue
		}
		seen[plat] = true
		out = append(out, plat)
	}
	if out == nil {
		out = []string{}
	}
	return out
}

// canonicalPlatform maps brief platform spellings to account platforms.
// It mirrors the scheduler's mapping; briefs predate the registry.
func canonicalPlatform(p string) string {
	if strings.TrimSpace(strings.ToLower(p)) == "naver" {
		return "naver_blog"
	}
	return strings.ToLower(strings.TrimSpace(p))
}

// effectiveChannels applies the team's W21-4 distribution: mode one pins
// every channel to a single version; mode all grows versions to cover the
// eligible accounts, bounded by the team cap and the hard ceiling.
func (s *Service) effectiveChannels(in []ChannelReq, teamID int64) []ChannelReq {
	out := make([]ChannelReq, 0, len(in))
	t, err := s.store.GetTeam(teamID)
	if err != nil {
		return in
	}
	if t.DistributionMode == "one" {
		for _, c := range in {
			c.Versions = 1
			out = append(out, c)
		}
		return out
	}
	cap := t.DistributionCap
	if cap < 1 {
		cap = 5
	}
	if cap > 10 {
		cap = 10
	}
	accounts, err := s.store.ListAccountRows()
	if err != nil {
		return in
	}
	linked := map[int64]bool{}
	if ids, err := s.store.ListTeamChannelIDs(teamID); err == nil && len(ids) > 0 {
		for _, id := range ids {
			linked[id] = true
		}
	}
	for _, c := range in {
		plat := canonicalPlatform(c.Platform)
		eligible := 0
		for _, a := range accounts {
			if a.Kind == "agent_service" || a.Platform != plat {
				continue
			}
			if a.SessionState != "connected" || a.PostingMode != "auto" {
				continue
			}
			if len(linked) > 0 && !linked[a.ID] {
				continue
			}
			eligible++
		}
		n := c.Versions
		if eligible > n {
			n = eligible
		}
		if n > cap {
			n = cap
		}
		if n < 1 {
			n = 1
		}
		c.Versions = n
		out = append(out, c)
	}
	return out
}

func normalizeStart(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC().Format(time.RFC3339), nil
	}
	if t, err := time.ParseInLocation("2006-01-02T15:04", raw, time.Local); err == nil {
		return t.UTC().Format(time.RFC3339), nil
	}
	return "", ErrBadStart
}

// HoldPost holds or releases one post of a work.
func (s *Service) HoldPost(workID, postID int64, held bool) error {
	c, err := s.store.GetContent(postID)
	if err != nil {
		return ErrNoAccount
	}
	if c.ProductID != workID {
		return ErrNoAccount
	}
	return s.store.SetContentHeld(postID, held)
}

// RegeneratePost queues a regeneration linked onto the post's revision chain (D21).
func (s *Service) RegeneratePost(workID, postID int64) (int64, error) {
	c, err := s.store.GetContent(postID)
	if err != nil {
		return 0, ErrNoAccount
	}
	if c.ProductID != workID {
		return 0, ErrNoAccount
	}
	payload, _ := json.Marshal(map[string]any{"product_id": workID, "revision_of": postID})
	job, err := s.store.CreateJob(jobGenerate, workID, string(payload))
	if err != nil {
		return 0, err
	}
	return job.ID, nil
}

// Delete removes a work with its subtree and returns stored file paths
// (legacy uploads + library files) for the handler to unlink.
func (s *Service) Delete(id int64) ([]string, error) {
	if _, err := s.store.GetProduct(id); err != nil {
		return nil, ErrNoAccount
	}
	var paths []string
	if images, err := s.store.ListImages(id); err == nil {
		for _, im := range images {
			paths = append(paths, im.Path)
		}
	}
	if lib, err := s.store.ListMedia(id, 1000); err == nil {
		for _, m := range lib {
			paths = append(paths, m.Path)
		}
	}
	if err := s.store.DeleteProduct(id); err != nil {
		return nil, err
	}
	return paths, nil
}
