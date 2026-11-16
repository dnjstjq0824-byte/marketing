package store

import (
	"encoding/json"
	"strings"
)

// Media capability tokens (W15-9 account capabilities, W16 routing).
const (
	MediaCapImage = "image"
	MediaCapVideo = "video"
)

// AccountHasCap reports whether a capabilities JSON array carries a token.
// Unparseable values fail closed (no capability).
func AccountHasCap(capabilitiesJSON, cap string) bool {
	var arr []string
	if err := json.Unmarshal([]byte(capabilitiesJSON), &arr); err != nil {
		return false
	}
	for _, c := range arr {
		if strings.TrimSpace(c) == cap {
			return true
		}
	}
	return false
}

// AccountPaidTier reports whether a plan tier label names a paid plan.
// Empty/unknown labels count as free: only an explicit non-free label
// wins the paid-first routing (W16).
func AccountPaidTier(planTier string) bool {
	t := strings.ToLower(strings.TrimSpace(planTier))
	if t == "" {
		return false
	}
	for _, free := range []string{"free", "무료", "basic", "베이직"} {
		if t == free {
			return false
		}
	}
	return true
}

// TeamMediaCap describes one team's generation coverage for a capability:
// whether any connected account can do it, and whether a paid one can
// (paid-first ordering applies only then).
type TeamMediaCap struct {
	Any  bool
	Paid bool
}

// TeamMediaCoverage scans a team's member accounts for a capability.
// Only connected chat-linked AI accounts count; channel accounts and
// unlinked rows never match.
func (s *Store) TeamMediaCoverage(teamID int64, cap string) (TeamMediaCap, error) {
	var out TeamMediaCap
	members, err := s.TeamMembers(teamID)
	if err != nil {
		return out, err
	}
	for _, m := range members {
		row, err := s.GetAccountRow(m.AccountID)
		if err != nil {
			continue
		}
		if row.SessionState != "connected" {
			continue
		}
		p, err := s.GetAIPlatform(strings.TrimSpace(row.Platform))
		if err != nil || p.ChatStatus != AIChatOK {
			continue
		}
		if !AccountHasCap(row.Capabilities, cap) {
			continue
		}
		out.Any = true
		if AccountPaidTier(row.PlanTier) {
			out.Paid = true
		}
	}
	return out, nil
}
