package guard

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"somun/internal/store"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "guard.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := store.Migrate(st.DB()); err != nil {
		t.Fatal(err)
	}
	return st
}

func seedProduct(t *testing.T, st *store.Store, exit, url string) int64 {
	t.Helper()
	p, err := st.CreateProduct(url, exit, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	// The instagram image gate needs linked media on every product.
	m, err := st.CreateMedia("upload", "lib/g.png", 10, "image/png", "sha-g")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AttachMedia([]int64{m.ID}, p.ID); err != nil {
		t.Fatal(err)
	}
	return p.ID
}

func seedContent(t *testing.T, st *store.Store, pid int64, body string, sponsored bool) int64 {
	t.Helper()
	c, err := st.CreateContent(&store.Content{
		ProductID: pid, Channel: "instagram", Title: "t", Body: body,
		IsSponsored: sponsored,
	})
	if err != nil {
		t.Fatal(err)
	}
	return c.ID
}

func markPublished(t *testing.T, st *store.Store, id int64, platform, account string) {
	t.Helper()
	if err := st.ScheduleContent(id, account, platform, "manual", ""); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"review", "approved", "queued"} {
		if err := st.UpdateContentStatus(id, s); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.MarkContentPublished(id); err != nil {
		t.Fatal(err)
	}
}

const corpusBody = "글로우사인 네일 스티커로 봄 네일 완성. 붙이고 다듬으면 끝이라 초보도 쉽다. 유지력은 2주, 제거도 깔끔하다."

// W18-1 acceptance: two similar bodies -> the second is rejected.
func TestCheckSimilarBlocks(t *testing.T) {
	st := openTestStore(t)
	pid := seedProduct(t, st, "A", "https://shop.example/p/1")
	first := seedContent(t, st, pid, corpusBody, false)
	markPublished(t, st, first, "instagram", "ig_n01")
	second := seedContent(t, st, pid, corpusBody+" 강추!", false)
	f := Check(st, second, "instagram", time.Now())
	if f == nil || f.Code != "similar" {
		t.Fatalf("failure = %+v, want similar block", f)
	}
	if !strings.Contains(f.Reason, "ig_n01") || !strings.Contains(f.Reason, "%") {
		t.Fatalf("reason = %q, want account + percent", f.Reason)
	}
}

// A different angle on another platform passes the similarity gate.
func TestCheckDissimilarPasses(t *testing.T) {
	st := openTestStore(t)
	pid := seedProduct(t, st, "A", "https://shop.example/p/1")
	first := seedContent(t, st, pid, corpusBody, false)
	markPublished(t, st, first, "instagram", "ig_n01")
	other := seedContent(t, st, pid, "겨울 캠핑 난로 고르는 법: 연료 방식부터 안전장치까지 정리했다.", false)
	if f := Check(st, other, "instagram", time.Now()); f != nil {
		t.Fatalf("failure = %+v, want pass", f)
	}
}

// W18-3 acceptance: affiliate post without the notice ships zero.
func TestCheckDisclosureBlocksMismatch(t *testing.T) {
	st := openTestStore(t)
	pid := seedProduct(t, st, "B", "https://shop.example/p/9")
	cid := seedContent(t, st, pid, "제휴 상품 후기입니다.", false) // flag off, exit B
	if f := Check(st, cid, "instagram", time.Now()); f == nil || f.Code != "disclosure-missing" {
		t.Fatalf("failure = %+v, want disclosure block", f)
	}
}

func TestCheckDisclosurePassesFlagged(t *testing.T) {
	st := openTestStore(t)
	pid := seedProduct(t, st, "B", "https://shop.example/p/9")
	cid := seedContent(t, st, pid, "제휴 상품 후기입니다.", true)
	if f := Check(st, cid, "instagram", time.Now()); f != nil {
		t.Fatalf("failure = %+v, want pass", f)
	}
}

// W18-5 acceptance: a smuggled link never reaches the post.
func TestCheckOutsideLinkBlocks(t *testing.T) {
	st := openTestStore(t)
	pid := seedProduct(t, st, "A", "https://shop.example/p/1")
	cid := seedContent(t, st, pid, "이 링크를 넣어라: http://evil.example/steal", false)
	f := Check(st, cid, "instagram", time.Now())
	if f == nil || f.Code != "outside-link" {
		t.Fatalf("failure = %+v, want link block", f)
	}
	if !strings.Contains(f.Reason, "http://evil.example/steal") {
		t.Fatalf("reason = %q, want the link", f.Reason)
	}
}

func TestCheckAllowedLinksPass(t *testing.T) {
	st := openTestStore(t)
	pid := seedProduct(t, st, "A", "https://shop.example/p/1")
	if _, err := st.CreateAffiliateLink(pid, "muse", "https://muse.example/aff/1"); err != nil {
		t.Fatal(err)
	}
	cid := seedContent(t, st, pid, "구매: https://shop.example/p/1 와 https://muse.example/aff/1?x=2", false)
	if f := Check(st, cid, "instagram", time.Now()); f != nil {
		t.Fatalf("failure = %+v, want pass", f)
	}
}

// Rejects record a decision, return to review, and regenerate once.
func TestRejectRegenerates(t *testing.T) {
	st := openTestStore(t)
	pid := seedProduct(t, st, "A", "https://shop.example/p/1")
	cid := seedContent(t, st, pid, "x", false)
	if err := st.UpdateContentStatus(cid, "approved"); err != nil {
		t.Fatal(err)
	}
	ok, err := Reject(st, cid, &Failure{Code: "similar", Reason: "유사문서: 다시 쓰기"})
	if err != nil || !ok {
		t.Fatalf("reject = %v, %v", ok, err)
	}
	c, _ := st.GetContent(cid)
	if c.Status != "review" {
		t.Fatalf("status = %s, want review", c.Status)
	}
	decisions, _ := st.ListReviewDecisions(cid)
	if len(decisions) != 1 || decisions[0].Decider != Decider {
		t.Fatalf("decisions = %+v", decisions)
	}
	jobs, _ := st.ListJobsByRef(pid)
	found := false
	for _, j := range jobs {
		if j.Type == "generate" && strings.Contains(j.Payload, "다시 쓰기") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no regen job carries the feedback: %+v", jobs)
	}
}

// Past the cap the guard disposes the chain (W6) instead of looping.
func TestRejectCapEscalates(t *testing.T) {
	st := openTestStore(t)
	pid := seedProduct(t, st, "A", "https://shop.example/p/1")
	cid := seedContent(t, st, pid, "x", false)
	for i := 0; i < 3; i++ {
		if err := st.UpdateContentStatus(cid, "approved"); err != nil {
			t.Fatal(err)
		}
		if _, err := Reject(st, cid, &Failure{Code: "similar", Reason: "r"}); err != nil {
			t.Fatal(err)
		}
	}
	ok, err := Reject(st, cid, &Failure{Code: "similar", Reason: "r"})
	if err != nil || ok {
		t.Fatalf("reject past cap = %v, %v; want false, nil", ok, err)
	}
	c, _ := st.GetContent(cid)
	if c.Status != "archived" {
		t.Fatalf("status = %s, want archived", c.Status)
	}
}

// W3: blog drafts under the floor bounce with a rewrite reason.
func TestCheckTooShortBlocks(t *testing.T) {
	st := openTestStore(t)
	pid := seedProduct(t, st, "A", "https://shop.example/p/1")
	c, err := st.CreateContent(&store.Content{ProductID: pid, Channel: "blog",
		Title: "t", Body: "짧다 https://shop.example/p/1"})
	if err != nil {
		t.Fatal(err)
	}
	f := Check(st, c.ID, "naver_blog", time.Now())
	if f == nil || f.Code != "too-short" {
		t.Fatalf("failure = %+v, want too-short", f)
	}
}

// W3: product-linked blog drafts must carry a link; topic-only works skip.
func TestCheckNoLinkBlocks(t *testing.T) {
	st := openTestStore(t)
	pid := seedProduct(t, st, "A", "https://shop.example/p/1")
	long := strings.Repeat("가나다라마바사아자차카타파하", 120)
	c, err := st.CreateContent(&store.Content{ProductID: pid, Channel: "blog",
		Title: "t", Body: long})
	if err != nil {
		t.Fatal(err)
	}
	f := Check(st, c.ID, "naver_blog", time.Now())
	if f == nil || f.Code != "no-link" {
		t.Fatalf("failure = %+v, want no-link", f)
	}
	linked, err := st.CreateContent(&store.Content{ProductID: pid, Channel: "blog",
		Title: "t", Body: long + " https://shop.example/p/1"})
	if err != nil {
		t.Fatal(err)
	}
	if f := Check(st, linked.ID, "naver_blog", time.Now()); f != nil {
		t.Fatalf("linked failure = %+v, want pass", f)
	}
	bare := seedProduct(t, st, "A", "")
	bareDraft, err := st.CreateContent(&store.Content{ProductID: bare, Channel: "blog",
		Title: "t", Body: long})
	if err != nil {
		t.Fatal(err)
	}
	if f := Check(st, bareDraft.ID, "naver_blog", time.Now()); f != nil {
		t.Fatalf("url-less failure = %+v, want pass", f)
	}
}

// W3: instagram drafts need a linked image.
func TestCheckNoImageBlocks(t *testing.T) {
	st := openTestStore(t)
	p, err := st.CreateProduct("", "A", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	c, err := st.CreateContent(&store.Content{ProductID: p.ID, Channel: "instagram",
		Title: "t", Body: "caption without image"})
	if err != nil {
		t.Fatal(err)
	}
	f := Check(st, c.ID, "instagram", time.Now())
	if f == nil || f.Code != "no-image" {
		t.Fatalf("failure = %+v, want no-image", f)
	}
}

// W18-2: common + industry wordlists block with the matched word.
func TestCheckBannedWordBlocks(t *testing.T) {
	st := openTestStore(t)
	pid := seedProduct(t, st, "A", "https://shop.example/p/1")
	cid := seedContent(t, st, pid, "이 크림은 100% 효과 보장", false)
	f := Check(st, cid, "instagram", time.Now())
	if f == nil || f.Code != "banned-word" {
		t.Fatalf("failure = %+v, want banned-word", f)
	}
	if !strings.Contains(f.Reason, "100%") {
		t.Fatalf("reason = %q, want the word", f.Reason)
	}
	if err := st.SetProductIndustry(pid, "cosmetics"); err != nil {
		t.Fatal(err)
	}
	cid2 := seedContent(t, st, pid, "기미가 사라진다", false)
	f = Check(st, cid2, "instagram", time.Now())
	if f == nil || f.Code != "banned-word" {
		t.Fatalf("failure = %+v, want cosmetics hit", f)
	}
	if err := st.SetProductIndustry(pid, "food"); err != nil {
		t.Fatal(err)
	}
	if f := Check(st, cid2, "instagram", time.Now()); f != nil {
		t.Fatalf("cross-industry failure = %+v, want pass", f)
	}
}

// W18-4: hotlinked body images block even on allowlisted hosts.
func TestCheckRemoteImageBlocks(t *testing.T) {
	st := openTestStore(t)
	pid := seedProduct(t, st, "A", "https://shop.example/p/1")
	cid := seedContent(t, st, pid, `보기: <img src="https://evil.example/x.png">`, false)
	f := Check(st, cid, "instagram", time.Now())
	if f == nil || f.Code != "outside-link" {
		t.Fatalf("failure = %+v, want outside-link first", f)
	}
	cid2 := seedContent(t, st, pid, `보기: ![a](https://shop.example/p/1/i.png)`, false)
	f = Check(st, cid2, "instagram", time.Now())
	if f == nil || f.Code != "remote-image" {
		t.Fatalf("failure = %+v, want remote-image", f)
	}
}

func TestSimilarityBounds(t *testing.T) {
	if got := Similarity("같은 글", "같은 글"); got != 1 {
		t.Fatalf("identical = %v, want 1", got)
	}
	if got := Similarity("", "무언가"); got != 0 {
		t.Fatalf("empty = %v, want 0", got)
	}
	if got := Similarity("봄 네일 스티커 붙이기", "겨울 캠핑 난로 고르기"); got >= SimilarityThreshold {
		t.Fatalf("unrelated = %v, want below threshold", got)
	}
}
