package product

import (
	"encoding/json"
	"fmt"

	"somun/internal/store"
)

// Job types shared with the queue spec (§5). The queue package owns the
// canonical constants; these literals stay in sync and are covered by tests.
const (
	jobAnalyze = "analyze"
)

// Service implements product intake and reading storage linkage.
type Service struct {
	store *store.Store
}

// NewService builds the product service.
func NewService(st *store.Store) *Service { return &Service{store: st} }

// AnalyzePayload is the worker context embedded in auto-created analyze jobs.
type AnalyzePayload struct {
	ProductID int64    `json:"product_id"`
	SourceURL string   `json:"source_url"`
	Images    []string `json:"images"`
}

// CreateInput is a validated product intake request.
type CreateInput struct {
	SourceURL string
	ExitType  string
	Priority  int
	Images    []string // stored relative paths, already saved by the handler
}

// CreateResult is the intake outcome: product plus its auto-created analyze job.
type CreateResult struct {
	Product *store.Product
	Job     *store.Job
}

// Create stores the product, its images, and auto-creates the analyze job.
func (s *Service) Create(in CreateInput) (*CreateResult, error) {
	exit, err := NormalizeExitType(in.ExitType)
	if err != nil {
		return nil, err
	}
	if err := ValidateURL(in.SourceURL); err != nil {
		return nil, err
	}
	if in.Priority < 0 {
		return nil, ErrBadPriority
	}
	p, err := s.store.CreateProduct(in.SourceURL, exit, in.Priority)
	if err != nil {
		return nil, err
	}
	for i, path := range in.Images {
		if _, err := s.store.AddImage(p.ID, path, i); err != nil {
			return nil, fmt.Errorf("save image: %w", err)
		}
	}
	payload, _ := json.Marshal(AnalyzePayload{ProductID: p.ID, SourceURL: p.SourceURL, Images: in.Images})
	job, err := s.store.CreateJob(jobAnalyze, p.ID, string(payload))
	if err != nil {
		return nil, fmt.Errorf("auto-create analyze job: %w", err)
	}
	return &CreateResult{Product: p, Job: job}, nil
}

// Detail bundles a product with its images, reading, jobs, and affiliate links.
type Detail struct {
	Product    *store.Product         `json:"product"`
	Images     []*store.ProductImage  `json:"images"`
	Reading    *store.Reading         `json:"reading"`
	Jobs       []*store.Job           `json:"jobs"`
	Affiliates []*store.AffiliateLink `json:"affiliateLinks"`
}

// GetDetail loads the full product view. Reading may be nil before analysis.
func (s *Service) GetDetail(id int64) (*Detail, error) {
	p, err := s.store.GetProduct(id)
	if err != nil {
		return nil, err
	}
	images, err := s.store.ListImages(id)
	if err != nil {
		return nil, err
	}
	reading, err := s.store.GetReadingByProduct(id)
	if err != nil && err != store.ErrNotFound {
		return nil, err
	}
	jobs, err := s.store.ListJobsByRef(id)
	if err != nil {
		return nil, err
	}
	links, err := s.store.ListAffiliateLinks(id)
	if err != nil {
		return nil, err
	}
	return &Detail{Product: p, Images: images, Reading: reading, Jobs: jobs, Affiliates: links}, nil
}
