package queue

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"somun/internal/content/policy"
	"somun/internal/scheduler"
	"somun/internal/store"
)

// Errors for result submission.
var (
	ErrJobNotAnalyze  = errors.New("job is not an analyze job")
	ErrJobNotStrategy = errors.New("job is not a strategy job")
	ErrJobNotGenerate = errors.New("job is not a generate job")
	ErrJobNotReview   = errors.New("job is not a review job")
	ErrJobNotPublish  = errors.New("job is not a publish job")
	ErrJobNotResearch = errors.New("job is not a research job")
	ErrNoURL          = errors.New("url is required")
	ErrNotQueued      = errors.New("content is not queued")
)

// Service implements the queue flow: claim, result/fail registration, and the
// automatic analyze->reading->generate chain (§5).
type Service struct {
	store *store.Store
	// OnFail, when set, fires after a failure is recorded (notify hook).
	OnFail func(job *store.Job)
	// OnReviewHuman, when set, fires when rejects pass the regen cap (D21).
	OnReviewHuman func(contentID int64, message string)
	// Scheduler, when set, assigns approved contents to accounts (W4).
	// Nil keeps the legacy bare-approve behavior (approved, unassigned).
	Scheduler *scheduler.Service
}

// NewService builds the queue service.
func NewService(st *store.Store) *Service { return &Service{store: st} }

// Next claims the oldest pending job of a type, or returns (nil, nil).
func (s *Service) Next(jobType, worker string) (*store.Job, error) {
	if err := ValidateJobType(jobType); err != nil {
		return nil, err
	}
	if worker == "" {
		worker = DefaultWorkerName
	}
	return s.store.ClaimNextJob(jobType, worker)
}

// SubmitResearch stores a topic study as the work's reading, auto-creates
// the strategy job, and marks the research job done (W1). Topic-only
// works enter the standard chain here instead of through page parsing.
func (s *Service) SubmitResearch(jobID int64, res ResearchResult) (*store.Job, error) {
	job, err := s.store.GetJob(jobID)
	if err != nil {
		return nil, err
	}
	if job.Type != JobResearch {
		return nil, ErrJobNotResearch
	}
	if job.Status == StatusDone || job.Status == StatusFailed {
		return nil, store.ErrAlreadyDone
	}
	if job.Status != StatusClaimed {
		return nil, store.ErrNotClaimed
	}
	product, err := s.store.GetProduct(job.RefID)
	if err != nil {
		return nil, fmt.Errorf("research ref product: %w", err)
	}
	detail := strings.TrimSpace(res.KeyPoints)
	if outline := strings.TrimSpace(res.Outline); outline != "" {
		if detail != "" {
			detail += "\n"
		}
		detail += outline
	}
	reading, err := s.store.UpsertReading(&store.Reading{
		ProductID:   job.RefID,
		ProductName: strings.TrimSpace(res.TopicTitle),
		DetailCore:  detail,
		Unknowns:    strings.TrimSpace(res.Unknowns),
	})
	if err != nil {
		return nil, fmt.Errorf("save research: %w", err)
	}
	if reading.ProductName != "" && product.Name == "" {
		_ = s.store.SetProductName(product.ID, reading.ProductName)
	}
	open, err := s.store.CountOpenJobs(JobStrategy, job.RefID)
	if err != nil {
		return nil, err
	}
	if open == 0 {
		payload, _ := json.Marshal(map[string]any{
			"product_id":   job.RefID,
			"product_name": reading.ProductName,
			"detail_core":  reading.DetailCore,
			"brand":        s.brandFor(job.RefID),
		})
		strategyJob, err := s.store.CreateJob(JobStrategy, job.RefID, string(payload))
		if err != nil {
			return nil, fmt.Errorf("auto-create strategy job: %w", err)
		}
		s.stampJobTeam(strategyJob.ID, job.RefID)
	}
	raw, _ := json.Marshal(res)
	return s.store.SetJobResult(jobID, string(raw))
}

// SubmitAnalyze stores the reading, auto-creates the strategy job, and marks
// the analyze job done. Non-idempotent: finished jobs return ErrAlreadyDone.
func (s *Service) SubmitAnalyze(jobID int64, res AnalyzeResult) (*store.Job, error) {
	job, err := s.store.GetJob(jobID)
	if err != nil {
		return nil, err
	}
	if job.Type != JobAnalyze {
		return nil, ErrJobNotAnalyze
	}
	if job.Status == StatusDone || job.Status == StatusFailed {
		return nil, store.ErrAlreadyDone
	}
	if job.Status != StatusClaimed {
		return nil, store.ErrNotClaimed
	}
	product, err := s.store.GetProduct(job.RefID)
	if err != nil {
		return nil, fmt.Errorf("analyze ref product: %w", err)
	}
	reading, err := s.store.UpsertReading(&store.Reading{
		ProductID:     job.RefID,
		ProductName:   strings.TrimSpace(res.ProductName),
		PriceSummary:  strings.TrimSpace(res.PriceSummary),
		OptionsText:   strings.TrimSpace(res.OptionsText),
		ShippingText:  strings.TrimSpace(res.ShippingText),
		DetailCore:    strings.TrimSpace(res.DetailCore),
		ReviewSummary: strings.TrimSpace(res.ReviewSummary),
		ImageDesc:     strings.TrimSpace(res.ImageDesc),
		Unknowns:      strings.TrimSpace(res.Unknowns),
	})
	if err != nil {
		return nil, fmt.Errorf("save reading: %w", err)
	}
	if reading.ProductName != "" && product.Name == "" {
		_ = s.store.SetProductName(product.ID, reading.ProductName)
	}
	// Auto-create the strategy job once (guarded against mid-flight retries).
	open, err := s.store.CountOpenJobs(JobStrategy, job.RefID)
	if err != nil {
		return nil, err
	}
	if open == 0 {
		payload, _ := json.Marshal(map[string]any{
			"product_id":     job.RefID,
			"product_name":   reading.ProductName,
			"detail_core":    reading.DetailCore,
			"review_summary": reading.ReviewSummary,
			"image_desc":     reading.ImageDesc,
			"brand":          s.brandFor(job.RefID),
		})
		strategyJob, err := s.store.CreateJob(JobStrategy, job.RefID, string(payload))
		if err != nil {
			return nil, fmt.Errorf("auto-create strategy job: %w", err)
		}
		s.stampJobTeam(strategyJob.ID, job.RefID)
	}
	raw, _ := json.Marshal(res)
	return s.store.SetJobResult(jobID, string(raw))
}

// SubmitStrategy stores the strategy, auto-creates the generate job, and marks
// the strategy job done. Non-idempotent like every result registration.
func (s *Service) SubmitStrategy(jobID int64, res StrategyResult) (*store.Job, error) {
	if err := ValidateStrategyResult(res); err != nil {
		return nil, err
	}
	job, err := s.store.GetJob(jobID)
	if err != nil {
		return nil, err
	}
	if job.Type != JobStrategy {
		return nil, ErrJobNotStrategy
	}
	if job.Status == StatusDone || job.Status == StatusFailed {
		return nil, store.ErrAlreadyDone
	}
	if job.Status != StatusClaimed {
		return nil, store.ErrNotClaimed
	}
	if _, err := s.store.GetProduct(job.RefID); err != nil {
		return nil, fmt.Errorf("strategy ref product: %w", err)
	}
	strategy, err := s.store.UpsertStrategy(&store.Strategy{
		ProductID:    job.RefID,
		Angle:        strings.TrimSpace(res.Angle),
		Channels:     strings.TrimSpace(res.Channels),
		KeyMessages:  strings.TrimSpace(res.KeyMessages),
		ScheduleNote: strings.TrimSpace(res.ScheduleNote),
	})
	if err != nil {
		return nil, fmt.Errorf("save strategy: %w", err)
	}
	open, err := s.store.CountOpenJobs(JobGenerate, job.RefID)
	if err != nil {
		return nil, err
	}
	if open == 0 {
		facts := map[string]any{}
		if reading, rerr := s.store.GetReadingByProduct(job.RefID); rerr == nil {
			facts["product_name"] = reading.ProductName
			facts["price_summary"] = reading.PriceSummary
			facts["options_text"] = reading.OptionsText
			facts["shipping_text"] = reading.ShippingText
			facts["detail_core"] = reading.DetailCore
			facts["review_summary"] = reading.ReviewSummary
			facts["image_desc"] = reading.ImageDesc
		}
		links := map[string]any{}
		if product, perr := s.store.GetProduct(job.RefID); perr == nil {
			links["product_url"] = product.SourceURL
		}
		if affs, aerr := s.store.ListAffiliateLinks(job.RefID); aerr == nil {
			var urls []string
			for _, l := range affs {
				urls = append(urls, l.URL)
			}
			if urls == nil {
				urls = []string{}
			}
			links["affiliate"] = urls
		}
		payload, _ := json.Marshal(map[string]any{
			"product_id":   job.RefID,
			"angle":        strategy.Angle,
			"channels":     strategy.Channels,
			"key_messages": strategy.KeyMessages,
			"brand":        s.brandFor(job.RefID),
			"versions":     s.briefVersions(job.RefID),
			"facts":        facts,
			"links":        links,
			"specs":        s.platformSpecs(job.RefID),
		})
		genJob, err := s.store.CreateJob(JobGenerate, job.RefID, string(payload))
		if err != nil {
			return nil, fmt.Errorf("auto-create generate job: %w", err)
		}
		s.stampJobTeam(genJob.ID, job.RefID)
	}
	raw, _ := json.Marshal(res)
	return s.store.SetJobResult(jobID, string(raw))
}

// SubmitGenerate stores every valid item as a draft and marks the job done.
// Sponsored products (exit B) get the compensation notice; the worker path
// never changes content status beyond creating drafts.
func (s *Service) SubmitGenerate(jobID int64, res GenerateResult) (*store.Job, int, error) {
	items, err := FilterGenerateItems(res.Contents)
	if err != nil {
		return nil, 0, err
	}
	job, err := s.store.GetJob(jobID)
	if err != nil {
		return nil, 0, err
	}
	if job.Type != JobGenerate {
		return nil, 0, ErrJobNotGenerate
	}
	if job.Status == StatusDone || job.Status == StatusFailed {
		return nil, 0, store.ErrAlreadyDone
	}
	if job.Status != StatusClaimed {
		return nil, 0, store.ErrNotClaimed
	}
	product, err := s.store.GetProduct(job.RefID)
	if err != nil {
		return nil, 0, fmt.Errorf("generate ref product: %w", err)
	}
	// D21: a regeneration payload links the new drafts onto the rejected
	// draft's revision chain. A missing parent unlinks silently.
	var regen struct {
		RevisionOf int64 `json:"revision_of"`
	}
	_ = json.Unmarshal([]byte(job.Payload), &regen)
	revOf, revRoot := int64(0), int64(0)
	if regen.RevisionOf > 0 {
		if parent, err := s.store.GetContent(regen.RevisionOf); err == nil {
			revOf = parent.ID
			revRoot = parent.RevisionRootID
			if revRoot == 0 {
				revRoot = parent.ID
			}
			// W6: the rejected original is disposed the moment its
			// revision is born; only the newest revision stays live.
			if parent.Status != policy.StatusPublished {
				_ = s.store.UpdateContentStatus(parent.ID, policy.StatusArchived)
			}
		}
	}
	sponsored := product.ExitType == "B"
	// W6: stamp the writer so review claims can exclude it. Unlinked
	// agents (legacy/token workers) leave authorship unknown.
	var writer *int64
	if job.AgentID != 0 {
		if agent, aerr := s.store.GetAgent(job.AgentID); aerr == nil && agent.AccountID != nil {
			v := *agent.AccountID
			writer = &v
		}
	}
	pairs := s.briefPairs(job.RefID)
	var draftIDs []int64
	for _, it := range items {
		c := &store.Content{
			ProductID:            job.RefID,
			Channel:              it.Channel,
			Title:                strings.TrimSpace(it.Title),
			Body:                 it.Body,
			IsSponsored:          sponsored,
			IsVirtualPersona:     it.VirtualPersona,
			RevisionOf:           revOf,
			RevisionRootID:       revRoot,
			GeneratedByAccountID: writer,
		}
		// W3-2: keep the platform pin only when the brief pairs it
		// with the channel; otherwise fall back to first match.
		if want := strings.ToLower(strings.TrimSpace(it.Platform)); want != "" {
			if want == "naver" {
				want = "naver_blog"
			}
			for _, pr := range pairs {
				if pr.Channel == c.Channel && pr.Platform == want {
					c.WantPlatform = want
					break
				}
			}
		}
		// W3-4: media shopping list verbatim, bounded.
		if media := strings.TrimSpace(it.Media); media != "" {
			if len(media) > 8000 {
				media = media[:8000]
			}
			c.MediaSpec = media
		}
		if sponsored {
			c.Disclosure = policy.DisclosureText
		}
		if it.VirtualPersona {
			c.AILabel = policy.AILabelText
		}
		created, err := s.store.CreateContent(c)
		if err != nil {
			return nil, 0, fmt.Errorf("save draft: %w", err)
		}
		draftIDs = append(draftIDs, created.ID)
	}
	// Auto-create one review job per generate batch (C chief gate).
	// The drafts ride along: reviewers judge text, not IDs.
	snapshot := make([]map[string]any, 0, len(items))
	for i, it := range items {
		snapshot = append(snapshot, map[string]any{
			"content_id": draftIDs[i],
			"channel":    it.Channel,
			"title":      strings.TrimSpace(it.Title),
			"body":       it.Body,
		})
	}
	facts := map[string]any{}
	if reading, rerr := s.store.GetReadingByProduct(job.RefID); rerr == nil {
		facts["product_name"] = reading.ProductName
		facts["price_summary"] = reading.PriceSummary
		facts["detail_core"] = reading.DetailCore
		facts["review_summary"] = reading.ReviewSummary
	}
	payload, _ := json.Marshal(map[string]any{
		"product_id": job.RefID, "content_ids": draftIDs,
		"contents": snapshot, "facts": facts,
		"brand": s.brandFor(job.RefID),
	})
	revJob, err := s.store.CreateJob(JobReview, job.RefID, string(payload))
	if err != nil {
		return nil, 0, fmt.Errorf("auto-create review job: %w", err)
	}
	s.stampJobTeam(revJob.ID, job.RefID)
	raw, _ := json.Marshal(res)
	done, err := s.store.SetJobResult(jobID, string(raw))
	if err != nil {
		return nil, 0, err
	}
	return done, len(items), nil
}

// MaxAutoRegen bounds chief-triggered regeneration: after this many rejects
// of one content, the item waits for a human instead of looping forever.
const MaxAutoRegen = 2

// SubmitReview applies chief judgments: approvals advance toward queued (with
// SubmitPublish records an adapter session post: the scheduled log closes
// as done and the queued item flips to published.
func (s *Service) SubmitPublish(jobID int64, res PublishResult) (*store.Job, error) {
	job, err := s.store.GetJob(jobID)
	if err != nil {
		return nil, err
	}
	if job.Type != JobPublish {
		return nil, store.ErrTypeMismatch
	}
	if job.Status != StatusClaimed {
		return nil, store.ErrNotClaimed
	}
	if strings.TrimSpace(res.URL) == "" {
		return nil, ErrNoURL
	}
	var payload struct {
		ContentID int64  `json:"content_id"`
		Account   string `json:"account"`
		Platform  string `json:"platform"`
	}
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return nil, fmt.Errorf("bad publish payload: %w", err)
	}
	c, err := s.store.GetContent(payload.ContentID)
	if err != nil {
		return nil, err
	}
	if c.Status != policy.StatusQueued {
		return nil, ErrNotQueued
	}
	note := strings.TrimSpace(res.URL)
	if ev := strings.TrimSpace(res.Evidence); ev != "" {
		note += " | evidence: " + ev
	}
	rec, err := s.store.GetPublishLog(c.ID, payload.Platform, payload.Account)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			return nil, err
		}
		if _, err := s.store.CreatePublishDone(c.ID, payload.Platform, payload.Account, nowStr(), note); err != nil {
			return nil, err
		}
	} else if err := s.store.MarkLogDone(rec.ID, nowStr(), note); err != nil {
		return nil, err
	}
	if err := s.store.MarkContentPublished(c.ID); err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(res)
	return s.store.SetJobResult(jobID, string(raw))
}

func nowStr() string { return time.Now().UTC().Format(time.RFC3339) }

// scheduling when a target rides along), rejections return to review and
// trigger bounded regeneration. Judgments outside the job's content scope
// are rejected. Every judgment is recorded with the deciding worker's name.
func (s *Service) SubmitReview(jobID int64, res ReviewResult) (*store.Job, error) {
	if err := ValidateReviewResult(res); err != nil {
		return nil, err
	}
	job, err := s.store.GetJob(jobID)
	if err != nil {
		return nil, err
	}
	if job.Type != JobReview {
		return nil, ErrJobNotReview
	}
	if job.Status == StatusDone || job.Status == StatusFailed {
		return nil, store.ErrAlreadyDone
	}
	if job.Status != StatusClaimed {
		return nil, store.ErrNotClaimed
	}
	allowed := map[int64]bool{}
	if len(res.Decisions) > 0 {
		var payload struct {
			ContentIDs []int64 `json:"content_ids"`
		}
		if err := json.Unmarshal([]byte(job.Payload), &payload); err == nil && len(payload.ContentIDs) > 0 {
			for _, id := range payload.ContentIDs {
				allowed[id] = true
			}
		}
	}
	inScope := func(id int64) bool {
		if len(allowed) == 0 {
			return true // legacy payload without scope: allow
		}
		return allowed[id]
	}
	decider := job.WorkerName
	if decider == "" {
		decider = DefaultWorkerName
	}
	// Validate everything before applying anything: a bad item fails the
	// whole submission and the job stays claimed for a corrected retry.
	// W4: AI decisions never carry accounts. Account/Platform/ScheduledAt
	// fields from the worker are ignored; the server scheduler assigns.
	for _, d := range res.Decisions {
		if !inScope(d.ContentID) {
			return nil, fmt.Errorf("%w: content %d outside job scope", ErrBadDecision, d.ContentID)
		}
		if _, err := s.store.GetContent(d.ContentID); err != nil {
			return nil, fmt.Errorf("%w: content %d", ErrBadDecision, d.ContentID)
		}
	}
	for _, d := range res.Decisions {
		verdict := "approved"
		if !d.Approve {
			verdict = "rejected"
		}
		if _, err := s.store.CreateReviewDecision(d.ContentID, decider, verdict,
			strings.TrimSpace(d.Reason)); err != nil {
			return nil, fmt.Errorf("record decision: %w", err)
		}
		c, _ := s.store.GetContent(d.ContentID)
		if c.Status == policy.StatusPublished {
			continue
		}
		if !d.Approve {
			switch c.Status {
			case policy.StatusDraft:
				_ = s.store.UpdateContentStatus(d.ContentID, policy.StatusReview)
			case policy.StatusApproved:
				_ = s.store.UpdateContentStatus(d.ContentID, policy.StatusReview)
			case policy.StatusQueued:
				_ = s.store.UpdateContentStatus(d.ContentID, policy.StatusApproved)
			}
			// D21: the cap counts rejects across the revision chain.
			n, err := s.store.CountRejectedChain(d.ContentID)
			if err == nil && n <= MaxAutoRegen {
				angle, keyMessages := "", ""
				if strategy, serr := s.store.GetStrategyByProduct(c.ProductID); serr == nil {
					angle, keyMessages = strategy.Angle, strategy.KeyMessages
				}
				retry, _ := json.Marshal(map[string]any{
					"product_id":   c.ProductID,
					"revision_of":  d.ContentID,
					"feedback":     strings.TrimSpace(d.Reason),
					"angle":        angle,
					"key_messages": keyMessages,
					"brand":        s.brandFor(c.ProductID),
				})
				if regenJob, rerr := s.store.CreateJob(JobGenerate, c.ProductID, string(retry)); rerr == nil {
					s.stampJobTeam(regenJob.ID, c.ProductID)
				}
			} else if err == nil {
				// W6: past the cap the chain is disposed, not handed to
				// a human; sibling platform items keep going. One info
				// notice only.
				_ = s.store.UpdateContentStatus(d.ContentID, policy.StatusArchived)
				if s.OnReviewHuman != nil {
					s.OnReviewHuman(d.ContentID, fmt.Sprintf("콘텐츠 #%d: 반려 %d회로 폐기(보관) — 다른 플랫폼 건은 계속 진행", d.ContentID, n))
				}
			}
			continue
		}
		switch c.Status {
		case policy.StatusDraft:
			_ = s.store.UpdateContentStatus(d.ContentID, policy.StatusReview)
			_ = s.store.UpdateContentStatus(d.ContentID, policy.StatusApproved)
		case policy.StatusReview:
			_ = s.store.UpdateContentStatus(d.ContentID, policy.StatusApproved)
		}
		// W4: the server assigns immediately on approval. A nil Scheduler
		// keeps legacy behavior (approved, unassigned).
		if s.Scheduler != nil {
			_, _ = s.Scheduler.Assign(d.ContentID)
		}
	}
	raw, _ := json.Marshal(res)
	return s.store.SetJobResult(jobID, string(raw))
}

// Fail records a worker failure. Failed jobs never requeue automatically;
// a human reviews them on the queue screen.
func (s *Service) Fail(jobID int64, reason string) (*store.Job, error) {
	if strings.TrimSpace(reason) == "" {
		return nil, errors.New("error reason is required")
	}
	job, err := s.store.SetJobFailed(jobID, strings.TrimSpace(reason))
	if err != nil {
		return nil, err
	}
	if s.OnFail != nil {
		s.OnFail(job)
	}
	return job, nil
}

// Recent returns the newest 100 jobs.
func (s *Service) Recent() ([]*store.Job, error) {
	return s.store.ListRecentJobs()
}

// brandFor loads the owning team's brand card for prompt injection (W16).
// Missing rows yield an empty map; callers always include the key.
func (s *Service) brandFor(productID int64) map[string]any {
	teamID, err := s.store.EffectiveProductTeam(productID)
	if err != nil {
		return map[string]any{}
	}
	b, err := s.store.GetTeamBrand(teamID)
	if err != nil {
		return map[string]any{}
	}
	return map[string]any{
		"intro": b.Intro, "tone": b.Tone, "banned": b.Banned, "link": b.BuyLink,
	}
}

// briefVersions reads the effective per-channel version counts (W21-4)
// for the generate worker. Missing briefs default every channel to 1.
func (s *Service) briefVersions(productID int64) map[string]int {
	out := map[string]int{}
	p, err := s.store.GetProduct(productID)
	if err != nil {
		return out
	}
	var brief struct {
		Channels []struct {
			Channel  string `json:"channel"`
			Versions int    `json:"versions"`
		} `json:"channels"`
	}
	if err := json.Unmarshal([]byte(p.BriefJSON), &brief); err != nil {
		return out
	}
	for _, c := range brief.Channels {
		n := c.Versions
		if n < 1 {
			n = 1
		}
		out[c.Channel] = n
	}
	return out
}

// platformSpecs reads the brief's target platforms and attaches each
// registry spec block (text_rules/media_rules) for the generate worker
// (W3-3). Unknown platforms keep an empty spec.
func (s *Service) platformSpecs(productID int64) map[string]any {
	out := map[string]any{}
	p, err := s.store.GetProduct(productID)
	if err != nil {
		return out
	}
	var brief struct {
		Channels []struct {
			Channel  string `json:"channel"`
			Platform string `json:"platform"`
		} `json:"channels"`
	}
	if err := json.Unmarshal([]byte(p.BriefJSON), &brief); err != nil {
		return out
	}
	for _, c := range brief.Channels {
		plat := strings.ToLower(strings.TrimSpace(c.Platform))
		if plat == "naver" {
			plat = "naver_blog"
		}
		spec := map[string]any{}
		if reg, err := s.store.GetChannelPlatform(plat); err == nil {
			var text, media any
			_ = json.Unmarshal([]byte(reg.TextRules), &text)
			_ = json.Unmarshal([]byte(reg.MediaRules), &media)
			spec["text"] = text
			spec["media"] = media
		}
		out[c.Channel+"/"+plat] = spec
	}
	return out
}

// briefPair is one brief channel entry, canonicalized.
type briefPair struct {
	Channel  string
	Platform string
}

// briefPairs lists a work's brief entries for platform-pin checks.
func (s *Service) briefPairs(productID int64) []briefPair {
	p, err := s.store.GetProduct(productID)
	if err != nil {
		return nil
	}
	var brief struct {
		Channels []struct {
			Channel  string `json:"channel"`
			Platform string `json:"platform"`
		} `json:"channels"`
	}
	if err := json.Unmarshal([]byte(p.BriefJSON), &brief); err != nil {
		return nil
	}
	var out []briefPair
	for _, c := range brief.Channels {
		plat := strings.ToLower(strings.TrimSpace(c.Platform))
		if plat == "naver" {
			plat = "naver_blog"
		}
		out = append(out, briefPair{Channel: c.Channel, Platform: plat})
	}
	return out
}

// stampJobTeam carries the owning work's team onto a follow-up job (W16).
func (s *Service) stampJobTeam(jobID, productID int64) {
	team, err := s.store.EffectiveProductTeam(productID)
	if err != nil {
		return
	}
	_ = s.store.SetJobTeam(jobID, team)
}

// BootCheck reports whether an AI account may boot a worker loop. Bound
// lanes spanning both B (generate) and C (review) refuse: one account
// cannot write and review (W6). Unknown accounts pass; the link guard at
// Register/Update is the config-time enforcement.
func (s *Service) BootCheck(accountID int64) (bool, []string, error) {
	lanes, err := s.store.LanesForAccount(accountID)
	if err != nil {
		return false, nil, err
	}
	hasB, hasC := false, false
	for _, l := range lanes {
		switch strings.ToUpper(strings.TrimSpace(l)) {
		case "B":
			hasB = true
		case "C":
			hasC = true
		}
	}
	return !(hasB && hasC), lanes, nil
}
