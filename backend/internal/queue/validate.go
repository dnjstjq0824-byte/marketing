package queue

import (
	"errors"
	"strings"
)

// Validation errors for worker result payloads.
var (
	ErrEmptyContents  = errors.New("contents must not be empty")
	ErrNoValidItems   = errors.New("no contents with an allowed channel")
	ErrBadJobType     = errors.New("type must be analyze, strategy, generate, review, or publish")
	ErrEmptyAngle     = errors.New("angle must not be empty")
	ErrEmptyDecisions = errors.New("decisions must not be empty")
	ErrBadDecision    = errors.New("invalid review decision")
)

// ValidateJobType accepts only the five specified job types.
func ValidateJobType(t string) error {
	if t != JobAnalyze && t != JobStrategy && t != JobGenerate && t != JobReview && t != JobPublish {
		return ErrBadJobType
	}
	return nil
}

// ValidateReviewResult checks decision shapes. Target fields travel together:
// account without platform (or vice versa) is rejected here; existence and
// timing are checked at apply time.
func ValidateReviewResult(res ReviewResult) error {
	if len(res.Decisions) == 0 {
		return ErrEmptyDecisions
	}
	for _, d := range res.Decisions {
		if d.ContentID <= 0 {
			return ErrBadDecision
		}
		account := strings.TrimSpace(d.Account)
		platform := strings.ToLower(strings.TrimSpace(d.Platform))
		if (account == "") != (platform == "") {
			return ErrBadDecision
		}
	}
	return nil
}

// ValidateStrategyResult requires a non-empty angle.
func ValidateStrategyResult(res StrategyResult) error {
	if strings.TrimSpace(res.Angle) == "" {
		return ErrEmptyAngle
	}
	return nil
}

// FilterGenerateItems drops items with disallowed channels or empty bodies.
// It returns ErrEmptyContents when the input is empty and ErrNoValidItems when
// nothing valid remains. No error is raised for dropped items: dropping is the
// specified behavior.
func FilterGenerateItems(items []GenerateItem) ([]GenerateItem, error) {
	if len(items) == 0 {
		return nil, ErrEmptyContents
	}
	var kept []GenerateItem
	for _, it := range items {
		ch := strings.ToLower(strings.TrimSpace(it.Channel))
		if !AllowedChannels[ch] {
			continue
		}
		if strings.TrimSpace(it.Body) == "" {
			continue
		}
		it.Channel = ch
		kept = append(kept, it)
	}
	if len(kept) == 0 {
		return nil, ErrNoValidItems
	}
	return kept, nil
}
