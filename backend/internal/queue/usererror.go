package queue

import "strings"

// ---------------------------------------------------------------------------
// User-facing failure phrases (W20-5). Jobs store the screen phrase from
// UI_DESIGN §6 vocabulary ("페이지를 열 수 없었습니다" style) instead of
// the internal error string. Classification (W9-5) always runs on the raw
// reason first; translation is the last step before the row is written.
// ---------------------------------------------------------------------------

// userErrorRule maps one internal marker to its screen phrase. Rules run
// in order; the first case-insensitive substring hit wins.
var userErrorRule = []struct {
	markers []string
	phrase  string
}{
	{[]string{"uncertain_post"}, "올라갔는지 확인이 필요합니다"},
	{[]string{"pre_publish"}, "발행 준비 중 문제가 생겼습니다"},
	{[]string{"duplicate collect window"}, "수집이 겹쳐 건너뛰었습니다"},
	{[]string{"save a login session first", "needs login", "not connected", "session expired", "login required", "reauth"}, "로그인이 필요합니다"},
	{[]string{"selectors not pinned", "selector not fillable", "selector not clickable", "selector", "not clickable", "not fillable", "element not found", "no such element", "locator"}, "화면 구조가 바뀌어 진행하지 못했습니다"},
	{[]string{"takes no attachments", "takes no tags", "unsupported media", "attachment"}, "채널에서 지원하지 않는 형식입니다"},
	{[]string{"unknown platform", "preparing"}, "아직 준비 중인 채널입니다"},
	{[]string{"chat timed out", "empty reply", "no reply", "ai gave no answer"}, "인공지능이 답하지 않았습니다"},
	{[]string{"context deadline", "deadline exceeded", "timed out", "timeout"}, "시간이 초과되어 중단됐습니다"},
	{[]string{"connection refused", "connection reset", "network unreachable", "no such host", "dns", "eof", "broken pipe", "connection lost"}, "연결이 끊겼습니다"},
	{[]string{"429", "rate limit", "too many requests"}, "요청이 많습니다. 잠시 후 다시 시도됩니다"},
	{[]string{"forbidden", "403", "unauthorized", "401", "access denied"}, "접근이 거부됐습니다. 계정 상태를 확인하세요"},
	{[]string{"page blocked", "blocked", "captcha", "challenge"}, "페이지를 열 수 없었습니다"},
	{[]string{"disk", "no space", "out of memory"}, "저장 공간이 부족합니다"},
	{[]string{"probe fail"}, "확인용 실패입니다"},
}

// UserErrorMessage translates one internal failure reason into its screen
// phrase. Unrecognized reasons fall back to a generic phrase so raw
// internals never reach the screen or the stored row.
func UserErrorMessage(reason string) string {
	trimmed := strings.TrimSpace(reason)
	lowered := strings.ToLower(trimmed)
	for _, rule := range userErrorRule {
		for _, m := range rule.markers {
			if strings.Contains(lowered, m) {
				return rule.phrase
			}
		}
	}
	return "일시적인 문제로 처리하지 못했습니다"
}
