package product

import (
	"errors"
	"net/url"
	"strings"
)

// Exit types (§6): A = partner mall link, B = affiliate link.
const (
	ExitA = "A"
	ExitB = "B"
)

// Validation errors.
var (
	ErrBadExitType = errors.New("exitType must be A or B")
	ErrBadURL      = errors.New("sourceUrl must be an http(s) URL")
	ErrBadPriority = errors.New("priority must be >= 0")
	ErrBadIndustry = errors.New("industry must be cosmetics, health, food, general, or empty")
)

// NormalizeExitType defaults an empty exit type to A (own/partner first).
func NormalizeExitType(exit string) (string, error) {
	exit = strings.ToUpper(strings.TrimSpace(exit))
	if exit == "" {
		return ExitA, nil
	}
	if exit != ExitA && exit != ExitB {
		return "", ErrBadExitType
	}
	return exit, nil
}

// ValidateURL requires a non-empty http(s) URL.
func ValidateURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ErrBadURL
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ErrBadURL
	}
	return nil
}
