package queue

import (
	"errors"
	"strings"
)

// Validation errors for worker result payloads.
var (
	ErrEmptyContents = errors.New("contents must not be empty")
	ErrNoValidItems  = errors.New("no contents with an allowed channel")
	ErrBadJobType    = errors.New("type must be analyze or generate")
)

// ValidateJobType accepts only the two specified job types.
func ValidateJobType(t string) error {
	if t != JobAnalyze && t != JobGenerate {
		return ErrBadJobType
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
