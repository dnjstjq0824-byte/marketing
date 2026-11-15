// Package guard is the W18 server-side safety net: rule checks that run
// after the C-lane chief review and before any publish, independent of
// what the AI judged. Anything flagged never publishes; it returns for
// regeneration with a human-readable reason.
//
// Shipped checks (real auto-publish prerequisites):
//  1. Similar already-published text on the same platform (W18-1).
//  3. Compensation disclosure present on exit-B (affiliate) posts (W18-3).
//  5. No outside links smuggled in through source material (W18-5).
//
// Deferred to the input-quality step: trade word lists (W18-2), image
// rights (W18-4), platform AI-label switches (W18-6).
package guard

import (
	"regexp"
	"strings"
	"time"
	"unicode"
)

// SimilarityWindowDays bounds the W18-1 corpus: same-platform posts
// published within this window. SimilarityThreshold is the reject line.
var (
	SimilarityWindowDays = 90
	SimilarityThreshold  = 0.6
)

// Published is one corpus row for the similarity check.
type Published struct {
	Account   string
	Body      string
	UpdatedAt string
}

// Failure is one blocked publish with a human-readable reason.
type Failure struct {
	Code   string
	Reason string
}

func (f *Failure) Error() string { return f.Code + ": " + f.Reason }

// bigrams builds the character-bigram set of normalized text. Whitespace
// and punctuation collapse so spacing tricks do not dodge the check.
func bigrams(s string) map[string]bool {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			continue
		}
		b.WriteRune(r)
	}
	runes := []rune(b.String())
	out := map[string]bool{}
	for i := 0; i+1 < len(runes); i++ {
		out[string(runes[i:i+2])] = true
	}
	return out
}

// Similarity is the char-bigram Jaccard index of two texts in [0,1].
// Empty inputs score 0 (nothing to compare), identical texts score 1.
func Similarity(a, b string) float64 {
	ga, gb := bigrams(a), bigrams(b)
	if len(ga) == 0 || len(gb) == 0 {
		return 0
	}
	inter := 0
	for g := range ga {
		if gb[g] {
			inter++
		}
	}
	union := len(ga) + len(gb) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// Similar finds the first corpus row at or above the threshold.
func Similar(body string, corpus []Published, threshold float64) (Published, float64, bool) {
	for _, p := range corpus {
		if s := Similarity(body, p.Body); s >= threshold {
			return p, s, true
		}
	}
	return Published{}, 0, false
}

// daysAgo renders how long before now an RFC3339 stamp was.
func daysAgo(stamp string, now time.Time) string {
	t, err := time.Parse(time.RFC3339, stamp)
	if err != nil {
		return "최근"
	}
	d := int(now.Sub(t).Hours() / 24)
	if d < 0 {
		d = 0
	}
	if d == 0 {
		return "오늘"
	}
	if d == 1 {
		return "1일 전"
	}
	return strings.TrimSpace(strings.Join([]string{itoa(d), "일 전"}, ""))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

var linkRe = regexp.MustCompile(`https?://[^\s<>"')\]]+`)

// OutsideLinks returns body links not covered by the allowlist. Matching is
// prefix-based on the trimmed URL so tracking parameters do not false-fire,
// but a different host or path always fails closed.
func OutsideLinks(body string, allowed []string) []string {
	var out []string
	for _, raw := range linkRe.FindAllString(body, -1) {
		link := strings.TrimRight(strings.TrimSpace(raw), ".,;:!?")
		ok := false
		for _, a := range allowed {
			a = strings.TrimSpace(a)
			if a != "" && strings.HasPrefix(link, a) {
				ok = true
				break
			}
		}
		if !ok {
			out = append(out, link)
		}
	}
	return out
}
