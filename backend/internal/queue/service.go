package queue

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"somun/internal/content/policy"
	"somun/internal/store"
)

// Errors for result submission.
var (
	ErrJobNotAnalyze  = errors.New("job is not an analyze job")
	ErrJobNotGenerate = errors.New("job is not a generate job")
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

// SubmitAnalyze stores the reading, auto-creates the generate job, and marks
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
	// Auto-create the generate job once (guarded against mid-flight retries).
	open, err := s.store.CountOpenJobs(JobGenerate, job.RefID)
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
		if _, err := s.store.CreateContent(c); err != nil {
			return nil, 0, fmt.Errorf("save draft: %w", err)
		}
	}
	raw, _ := json.Marshal(res)
	done, err := s.store.SetJobResult(jobID, string(raw))
	if err != nil {
		return nil, 0, err
	}
	return done, len(items), nil
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
