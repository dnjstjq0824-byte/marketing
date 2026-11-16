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
	ErrJobNotMedia    = errors.New("job is not a media job")
	ErrJobNotCollect  = errors.New("job is not a collect job")
	ErrBadCollect     = errors.New("invalid collect result")
	ErrBadMediaItem   = errors.New("invalid media item")
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
			"past_top":     s.pastTop(job.RefID),
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
			"past_top":       s.pastTop(job.RefID),
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
	// The drafts ride along: reviewers judge text, not IDs. Each item
	// also carries its planned media (W14) so the chief judges the
	// generation plan — and any already-linked files — with the text.
	snapshot := make([]map[string]any, 0, len(items))
	for i, it := range items {
		entry := map[string]any{
			"content_id": draftIDs[i],
			"channel":    it.Channel,
			"title":      strings.TrimSpace(it.Title),
			"body":       it.Body,
		}
		if media := strings.TrimSpace(it.Media); media != "" {
			entry["media_spec"] = media
		}
		if linked, lerr := s.store.MediaForContent(draftIDs[i]); lerr == nil && len(linked) > 0 {
			seen := make([]map[string]any, 0, len(linked))
			for _, m := range linked {
				seen = append(seen, map[string]any{
					"media_id": m.ID, "kind": m.Kind, "prompt": m.Prompt,
				})
			}
			entry["media"] = seen
		}
		snapshot = append(snapshot, entry)
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
	// W14: one media job per draft that asks for generated files. Text
	// never waits for these: they are leaves, and their failure falls
	// back to original images (recorded on the job, not the post).
	for i, it := range items {
		if strings.TrimSpace(it.Media) == "" {
			continue
		}
		if _, err := s.enqueueMedia(job.RefID, draftIDs[i], it.Channel, it.Media); err != nil {
			return nil, 0, err
		}
	}
	raw, _ := json.Marshal(res)
	done, err := s.store.SetJobResult(jobID, string(raw))
	if err != nil {
		return nil, 0, err
	}
	return done, len(items), nil
}

// enqueueMedia creates one generation job for a post's shopping list.
// The required capability follows the channel (shortform needs video,
// everything else images). Route ai needs a capable connected account on
// the team; otherwise the worker takes the local cost-0 path (W14/W16).
// Operator-disabled outputs skip silently (nil job, nil error).
func (s *Service) enqueueMedia(productID, contentID int64, channel, spec string) (*store.Job, error) {
	cap := store.MediaCapImage
	if channel == "shortform" || wantsVideo(spec) {
		cap = store.MediaCapVideo
	}
	if settings, err := store.GetAutomationSettings(s.store.DB()); err == nil {
		if cap == store.MediaCapVideo && !settings.Outputs.Video {
			return nil, nil
		}
		if cap == store.MediaCapImage && !settings.Outputs.Image {
			return nil, nil
		}
	}
	route, paidFirst := "ai", false
	if teamID, err := s.store.EffectiveProductTeam(productID); err == nil {
		if cov, cerr := s.store.TeamMediaCoverage(teamID, cap); cerr == nil {
			if !cov.Any {
				route = "local"
			} else {
				paidFirst = cov.Paid
			}
		}
	}
	payload, _ := json.Marshal(map[string]any{
		"product_id": productID, "content_id": contentID,
		"channel": channel, "media_spec": strings.TrimSpace(spec),
		"route": route, "required_cap": cap, "paid_first": paidFirst,
		"brand": s.brandFor(productID), "specs": s.platformSpecs(productID),
	})
	business, priority := "", 0
	if p, err := s.store.GetProduct(productID); err == nil {
		business, priority = p.Business, p.Priority
	}
	job, err := s.store.CreateJobFull(JobMedia, contentID, string(payload), business, priority, false)
	if err != nil {
		return nil, fmt.Errorf("auto-create media job: %w", err)
	}
	if teamID, terr := s.store.EffectiveProductTeam(productID); terr == nil {
		_ = s.store.SetJobTeam(job.ID, teamID)
	}
	return job, nil
}

// wantsVideo spots a video ask inside a free-text shopping list.
func wantsVideo(spec string) bool {
	low := strings.ToLower(spec)
	for _, token := range []string{"video", "영상", "숏폼", "shortform", "릴스", "reel"} {
		if strings.Contains(low, token) {
			return true
		}
	}
	return false
}

// SubmitMedia links finished files to the job's post and closes the job.
// Media jobs are leaves: nothing chains after them, and their failure
// (via Fail) never touches the text pipeline.
func (s *Service) SubmitMedia(jobID int64, res MediaResult) (*store.Job, error) {
	job, err := s.store.GetJob(jobID)
	if err != nil {
		return nil, err
	}
	if job.Type != JobMedia {
		return nil, ErrJobNotMedia
	}
	if job.Status == StatusDone || job.Status == StatusFailed {
		return nil, store.ErrAlreadyDone
	}
	if job.Status != StatusClaimed {
		return nil, store.ErrNotClaimed
	}
	var scope struct {
		ProductID int64 `json:"product_id"`
		ContentID int64 `json:"content_id"`
	}
	if err := json.Unmarshal([]byte(job.Payload), &scope); err != nil || scope.ContentID <= 0 || scope.ProductID <= 0 {
		return nil, fmt.Errorf("%w: job scope", ErrBadMediaItem)
	}
	if _, err := s.store.GetContent(scope.ContentID); err != nil {
		return nil, fmt.Errorf("%w: content %d", ErrBadMediaItem, scope.ContentID)
	}
	for _, it := range res.MediaIDs {
		if it.MediaID <= 0 || len(it.Role) > 40 {
			return nil, fmt.Errorf("%w: media %d", ErrBadMediaItem, it.MediaID)
		}
		if _, err := s.store.GetMedia(it.MediaID); err != nil {
			return nil, fmt.Errorf("%w: media %d", ErrBadMediaItem, it.MediaID)
		}
	}
	for _, it := range res.MediaIDs {
		if err := s.store.LinkMediaContent(it.MediaID, scope.ContentID, scope.ProductID); err != nil {
			return nil, fmt.Errorf("%w: media %d", ErrBadMediaItem, it.MediaID)
		}
	}
	raw, _ := json.Marshal(res)
	return s.store.SetJobResult(jobID, string(raw))
}

// MaxAutoRegen bounds chief-triggered regeneration: after this many rejects
// of one content, the item waits for a human instead of looping forever.
const MaxAutoRegen = 2

// SubmitReview applies chief judgments: approvals advance toward queued (with
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

// SubmitCollect stores one window's performance row and closes the job.
// Collect jobs are leaves: the row lands even when every figure is missing
// (NULLs with cause '원인 미분류'), and nothing chains after it.
func (s *Service) SubmitCollect(jobID int64, res CollectResult) (*store.Job, error) {
	job, err := s.store.GetJob(jobID)
	if err != nil {
		return nil, err
	}
	if job.Type != JobCollect {
		return nil, ErrJobNotCollect
	}
	if job.Status == StatusDone || job.Status == StatusFailed {
		return nil, store.ErrAlreadyDone
	}
	if job.Status != StatusClaimed {
		return nil, store.ErrNotClaimed
	}
	var scope struct {
		ContentID int64  `json:"content_id"`
		Window    string `json:"window"`
	}
	if err := json.Unmarshal([]byte(job.Payload), &scope); err != nil || scope.ContentID <= 0 {
		return nil, fmt.Errorf("%w: job scope", ErrBadCollect)
	}
	c, err := s.store.GetContent(scope.ContentID)
	if err != nil {
		return nil, fmt.Errorf("%w: content %d", ErrBadCollect, scope.ContentID)
	}
	for _, v := range []*int64{res.Views, res.Likes, res.Comments, res.Exposures, res.Inflows, res.Orders, res.Inquiries} {
		if v != nil && *v < 0 {
			return nil, fmt.Errorf("%w: negative figure", ErrBadCollect)
		}
	}
	cause := ""
	if collectAllZero(res) {
		cause = policy.UnclassifiedCause
	}
	note := strings.TrimSpace(res.Note)
	if len(note) > 2000 {
		note = note[:2000]
	}
	window := strings.TrimSpace(scope.Window)
	if window == "" {
		window = "24h"
	}
	if _, err := s.store.CreateMetric(&store.Metric{
		ContentID: &c.ID, Channel: c.Channel,
		Exposures: res.Exposures, Inflows: res.Inflows, Orders: res.Orders, Inquiries: res.Inquiries,
		Views: res.Views, Likes: res.Likes, Comments: res.Comments,
		Origin: store.CollectOrigin(window), CauseClass: cause, Note: note,
	}); err != nil {
		return nil, fmt.Errorf("save metric: %w", err)
	}
	raw, _ := json.Marshal(res)
	return s.store.SetJobResult(jobID, string(raw))
}

func collectAllZero(res CollectResult) bool {
	for _, v := range []*int64{res.Views, res.Likes, res.Comments, res.Exposures, res.Inflows, res.Orders, res.Inquiries} {
		if v != nil && *v != 0 {
			return false
		}
	}
	return true
}

func nowStr() string { return time.Now().UTC().Format(time.RFC3339) }

// Publish failure codes (W9-5). The P-lane adapter stamps every explicit
// publish failure with one of these B18-style fixed codes; legacy messages
// without a code are classified by substring, fail-closed toward uncertain.
const (
	// PublishFailPrePublish marks failures before anything could post
	// (login entry, missing selectors): safe to retry once.
	PublishFailPrePublish = "PRE_PUBLISH"
	// PublishFailUncertain marks failures where the post may have landed
	// (submit click, post-submit navigation): the content is held so no
	// duplicate goes out, and the operator resolves it with
	// confirm-published.
	PublishFailUncertain = "UNCERTAIN_POST"
)

// legacyPrePublish matches adapter reasons from before the W9-5 codes.
// Every one of them fails before the submit click, so a retry is safe.
var legacyPrePublish = []string{
	"save a login session first",
	"selectors not pinned yet",
	"selector not fillable",
	"selector not clickable",
	"takes no attachments",
	"takes no tags",
	"unknown platform",
}

// classifyPublishFailure reports whether a publish failure is a safe
// pre-publish retry. Uncertain wins over pre-publish; anything
// unrecognized is uncertain (fail closed: hold, never repost blindly).
func classifyPublishFailure(reason string) (prePublish bool) {
	if strings.Contains(reason, PublishFailUncertain) {
		return false
	}
	if strings.Contains(reason, PublishFailPrePublish) {
		return true
	}
	for _, legacy := range legacyPrePublish {
		if strings.Contains(reason, legacy) {
			return true
		}
	}
	return false
}

// Fail records a worker failure. Failed jobs never requeue automatically;
// a human reviews them on the queue screen. Collect jobs are the exception:
// their ledger row is released so the next sweep re-enqueues the window
// (a missed scrape round must not silence that window forever).
// W9-5 publish policy: a pre-publish failure (login entry, selector miss)
// returns the job to pending once for an automatic retry on the next poll;
// the second failure, like any uncertain one, closes the job — and only
// uncertain failures hold the content (duplicate prevention: the post may
// have landed, so the operator confirms via confirm-published).
func (s *Service) Fail(jobID int64, reason string) (*store.Job, error) {
	if strings.TrimSpace(reason) == "" {
		return nil, errors.New("error reason is required")
	}
	reason = strings.TrimSpace(reason)
	cur, err := s.store.GetJob(jobID)
	if err != nil {
		return nil, err
	}
	if cur.Type == JobPublish && classifyPublishFailure(reason) && cur.Attempts == 0 &&
		(cur.Status == StatusPending || cur.Status == StatusClaimed) {
		return s.store.MarkJobPendingRetry(jobID, reason)
	}
	// W20-5: the stored row keeps the screen phrase, never the raw
	// internal string. Classification above already ran on the raw
	// reason, so no retry/hold decision is affected.
	job, err := s.store.SetJobFailed(jobID, UserErrorMessage(reason))
	if err != nil {
		return nil, err
	}
	if job.Type == JobPublish && !classifyPublishFailure(reason) {
		var payload struct {
			ContentID int64 `json:"content_id"`
		}
		if jerr := json.Unmarshal([]byte(job.Payload), &payload); jerr == nil && payload.ContentID > 0 {
			_ = s.store.HoldContent(payload.ContentID, "발행 실패 — 게시 여부 사람 확인")
		}
	}
	if job.Type == JobCollect {
		var scope struct {
			ContentID int64  `json:"content_id"`
			Window    string `json:"window"`
			Account   string `json:"account"`
		}
		if jerr := json.Unmarshal([]byte(job.Payload), &scope); jerr == nil && scope.ContentID > 0 {
			_ = s.store.ReleaseCollectWindow(scope.ContentID, scope.Window, scope.Account)
		}
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

// pastTop loads the same business's recent top-3 performance lines for
// strategy prompts (W7 feedback). Missing data yields an empty list;
// callers always include the key.
func (s *Service) pastTop(productID int64) []*store.TopMetric {
	empty := []*store.TopMetric{}
	p, err := s.store.GetProduct(productID)
	if err != nil || strings.TrimSpace(p.Business) == "" {
		return empty
	}
	top, err := s.store.TopMetricsByBusiness(p.Business, 3)
	if err != nil {
		return empty
	}
	return top
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
