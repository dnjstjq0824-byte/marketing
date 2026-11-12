package queue

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"somun/internal/content/policy"
	"somun/internal/store"
)

// Errors for result submission.
var (
	ErrJobNotAnalyze  = errors.New("job is not an analyze job")
	ErrJobNotStrategy = errors.New("job is not a strategy job")
	ErrJobNotGenerate = errors.New("job is not a generate job")
	ErrJobNotReview   = errors.New("job is not a review job")
)

// Service implements the queue flow: claim, result/fail registration, and the
// automatic analyze->reading->generate chain (§5).
type Service struct {
	store *store.Store
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
		})
		if _, err := s.store.CreateJob(JobStrategy, job.RefID, string(payload)); err != nil {
			return nil, fmt.Errorf("auto-create strategy job: %w", err)
		}
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
		payload, _ := json.Marshal(map[string]any{
			"product_id":   job.RefID,
			"angle":        strategy.Angle,
			"channels":     strategy.Channels,
			"key_messages": strategy.KeyMessages,
		})
		if _, err := s.store.CreateJob(JobGenerate, job.RefID, string(payload)); err != nil {
			return nil, fmt.Errorf("auto-create generate job: %w", err)
		}
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
	sponsored := product.ExitType == "B"
	var draftIDs []int64
	for _, it := range items {
		c := &store.Content{
			ProductID:        job.RefID,
			Channel:          it.Channel,
			Title:            strings.TrimSpace(it.Title),
			Body:             it.Body,
			IsSponsored:      sponsored,
			IsVirtualPersona: it.VirtualPersona,
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
	payload, _ := json.Marshal(map[string]any{"product_id": job.RefID, "content_ids": draftIDs})
	if _, err := s.store.CreateJob(JobReview, job.RefID, string(payload)); err != nil {
		return nil, 0, fmt.Errorf("auto-create review job: %w", err)
	}
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
	type planned struct {
		item     ReviewDecisionItem
		account  string
		platform string
		at       string
	}
	var plans []planned
	for _, d := range res.Decisions {
		if !inScope(d.ContentID) {
			return nil, fmt.Errorf("%w: content %d outside job scope", ErrBadDecision, d.ContentID)
		}
		c, err := s.store.GetContent(d.ContentID)
		if err != nil {
			return nil, fmt.Errorf("%w: content %d", ErrBadDecision, d.ContentID)
		}
		if c.Status == policy.StatusPublished {
			continue // terminal: recorded below, status untouched
		}
		p := planned{item: d}
		if d.Approve {
			account := strings.TrimSpace(d.Account)
			platform := strings.ToLower(strings.TrimSpace(d.Platform))
			if account != "" {
				if _, err := s.store.GetAccountRowByName(account); err != nil {
					return nil, fmt.Errorf("%w: unknown account %q", ErrBadDecision, account)
				}
				group, ok := policy.GroupOf(platform)
				if !ok {
					return nil, fmt.Errorf("%w: unknown platform %q", ErrBadDecision, platform)
				}
				_ = group
				at := strings.TrimSpace(d.ScheduledAt)
				if at != "" {
					t, err := time.Parse(time.RFC3339, at)
					if err != nil || !t.After(time.Now()) {
						return nil, fmt.Errorf("%w: bad scheduledAt", ErrBadDecision)
					}
					at = t.UTC().Format(time.RFC3339)
				}
				p.account, p.platform, p.at = account, platform, at
			}
		}
		plans = append(plans, p)
	}
	for _, p := range plans {
		d := p.item
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
			if n, err := s.store.CountRejected(d.ContentID); err == nil && n <= MaxAutoRegen {
				retry, _ := json.Marshal(map[string]any{
					"product_id":  c.ProductID,
					"revision_of": d.ContentID,
					"feedback":    strings.TrimSpace(d.Reason),
				})
				_, _ = s.store.CreateJob(JobGenerate, c.ProductID, string(retry))
			}
			continue
		}
		if p.account != "" {
			group, _ := policy.GroupOf(p.platform)
			_ = s.store.UpdateContentStatus(d.ContentID, policy.StatusQueued)
			_ = s.store.ScheduleContent(d.ContentID, p.account, p.platform, group, p.at)
			_, _ = s.store.CreatePublishLog(d.ContentID, p.platform, p.account, "scheduled", "chief-approved")
			continue
		}
		switch c.Status {
		case policy.StatusDraft:
			_ = s.store.UpdateContentStatus(d.ContentID, policy.StatusReview)
			_ = s.store.UpdateContentStatus(d.ContentID, policy.StatusApproved)
		case policy.StatusReview:
			_ = s.store.UpdateContentStatus(d.ContentID, policy.StatusApproved)
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
	return s.store.SetJobFailed(jobID, strings.TrimSpace(reason))
}

// Recent returns the newest 100 jobs.
func (s *Service) Recent() ([]*store.Job, error) {
	return s.store.ListRecentJobs()
}
