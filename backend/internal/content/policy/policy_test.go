package policy

import (
	"strings"
	"testing"
)

func TestArchivedTransitions(t *testing.T) {
	for _, from := range []string{StatusDraft, StatusReview, StatusApproved, StatusQueued} {
		if err := ValidateTransition(from, StatusArchived); err != nil {
			t.Fatalf("%s -> archived: %v", from, err)
		}
	}
	for _, bad := range [][2]string{
		{StatusPublished, StatusArchived},
		{StatusArchived, StatusDraft},
		{StatusArchived, StatusReview},
	} {
		if err := ValidateTransition(bad[0], bad[1]); err == nil {
			t.Fatalf("%s -> %s: want rejection", bad[0], bad[1])
		}
	}
}

func TestAssembleBody(t *testing.T) {
	plain := AssembleBody(false, "본문", false, nil)
	if plain != "본문" {
		t.Fatalf("plain = %q", plain)
	}
	full := AssembleBody(true, "본문", true, []string{"https://shop.test/1", "https://shop.test/2"})
	want := strings.Join([]string{
		DisclosureText, "본문", AILabelText,
		"구매 링크: https://shop.test/1", "구매 링크: https://shop.test/2",
	}, "\n\n")
	if full != want {
		t.Fatalf("full = %q", full)
	}
	raw := AssembleBody(false, "<b>&</b>", false, nil)
	if raw != "<b>&</b>" {
		t.Fatalf("escaped = %q", raw)
	}
	sponsored := AssembleBody(true, "본문", false, []string{""})
	if strings.Contains(sponsored, "구매 링크") {
		t.Fatalf("empty link emitted: %q", sponsored)
	}
	if !strings.HasPrefix(sponsored, DisclosureText) {
		t.Fatalf("disclosure not first: %q", sponsored)
	}
}
