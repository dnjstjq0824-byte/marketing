package queue

import (
	"strings"
	"testing"
)

// W20-5: every known internal marker maps to a screen phrase; unknown
// internals fall back to the generic phrase, never the raw string.
func TestW20UserErrorMessage(t *testing.T) {
	cases := []struct{ in, want string }{
		{"UncertainPostFailed: UNCERTAIN_POST landing /x lacks /post/", "올라갔는지 확인이 필요합니다"},
		{"PrePublishFailed: PRE_PUBLISH selector not clickable: input.submit", "발행 준비 중 문제가 생겼습니다"},
		{"duplicate collect window (sweep race)", "수집이 겹쳐 건너뛰었습니다"},
		{"save a login session first", "로그인이 필요합니다"},
		{"adapter: session expired mid-run", "로그인이 필요합니다"},
		{"selectors not pinned yet", "화면 구조가 바뀌어 진행하지 못했습니다"},
		{"no such element: .submit", "화면 구조가 바뀌어 진행하지 못했습니다"},
		{"channel takes no attachments", "채널에서 지원하지 않는 형식입니다"},
		{"unknown platform: x", "아직 준비 중인 채널입니다"},
		{"chat timed out waiting for the reply", "인공지능이 답하지 않았습니다"},
		{"context deadline exceeded", "시간이 초과되어 중단됐습니다"},
		{"Post https://x: connection refused", "연결이 끊겼습니다"},
		{"429 too many requests", "요청이 많습니다. 잠시 후 다시 시도됩니다"},
		{"403 forbidden", "접근이 거부됐습니다. 계정 상태를 확인하세요"},
		{"page blocked by captcha", "페이지를 열 수 없었습니다"},
		{"boom: something odd happened", "일시적인 문제로 처리하지 못했습니다"},
		{"", "일시적인 문제로 처리하지 못했습니다"},
	}
	for _, c := range cases {
		if got := UserErrorMessage(c.in); got != c.want {
			t.Errorf("UserErrorMessage(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// W20-5: terminal failures store the screen phrase; the transient W9-5
// retry keeps the raw reason for its own bookkeeping.
func TestW20FailStoresScreenPhrase(t *testing.T) {
	st := openTestStore(t)
	pid := seedProduct(t, st, "A")
	j, err := st.CreateJob(JobAnalyze, pid, "{}")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimNextJob(JobAnalyze, "w"); err != nil {
		t.Fatal(err)
	}
	svc := NewService(st)
	got, err := svc.Fail(j.ID, "Post https://x: connection refused")
	if err != nil {
		t.Fatal(err)
	}
	if got.Error != "연결이 끊겼습니다" {
		t.Fatalf("stored error = %q", got.Error)
	}
	if strings.Contains(got.Error, "connection refused") {
		t.Fatalf("raw internal leaked: %q", got.Error)
	}
}
