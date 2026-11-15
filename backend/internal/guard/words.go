package guard

import (
	"embed"
	"regexp"
	"strings"
)

//go:embed wordlists/*.txt
var wordlists embed.FS

// bannedByIndustry holds the W18-2 lists: common always applies, plus the
// work's industry list when set. Parsed once at startup.
var bannedByIndustry = loadWordlists()

func loadWordlists() map[string][]string {
	out := map[string][]string{}
	for _, name := range []string{"common", "cosmetics", "health", "food"} {
		raw, err := wordlists.ReadFile("wordlists/" + name + ".txt")
		if err != nil {
			continue
		}
		var words []string
		for _, line := range strings.Split(string(raw), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			words = append(words, line)
		}
		out[name] = words
	}
	return out
}

// BannedHit returns the first banned expression in text for the industry
// ("" = common list only). Matching is case-insensitive for ASCII.
func BannedHit(text, industry string) string {
	lowered := strings.ToLower(text)
	check := func(words []string) string {
		for _, w := range words {
			if strings.Contains(lowered, strings.ToLower(w)) {
				return w
			}
		}
		return ""
	}
	if hit := check(bannedByIndustry["common"]); hit != "" {
		return hit
	}
	if industry = strings.ToLower(strings.TrimSpace(industry)); industry != "" {
		if hit := check(bannedByIndustry[industry]); hit != "" {
			return hit
		}
	}
	return ""
}

var imgRe = regexp.MustCompile(`(?i)<img[^>]+src=["']?(https?://[^"'\s>]+)|!\[[^\]]*\]\((https?://[^)\s]+)\)`)

// RemoteBodyImages returns every http(s) image URL embedded in the body
// (W18-4). Publish images must come from the media library as attached
// files; even same-host embeds are refused, with no allowlist.
func RemoteBodyImages(body string) []string {
	var out []string
	for _, m := range imgRe.FindAllStringSubmatch(body, -1) {
		link := m[1]
		if link == "" {
			link = m[2]
		}
		if link != "" {
			out = append(out, link)
		}
	}
	return out
}
