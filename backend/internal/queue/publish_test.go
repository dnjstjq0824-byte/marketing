package queue

import (
	"testing"

	"somun/internal/store"
)

// P1-2: adapter session posts close through SubmitPublish.
func TestSubmitPublishClosesLogAndContent(t *testing.T) {
	st := openTestStore(t)
	pid := seedProduct(t, st, "A")
	c, err := st.CreateContent(&store.Content{ProductID: pid, Channel: "blog", Title: "t", Body: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`UPDATE contents SET status='queued',channel_group='manual',target_account='a',target_platform='naver_blog' WHERE id=?`, c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreatePublishLog(c.ID, "naver_blog", "a", "scheduled", ""); err != nil {
		t.Fatal(err)
	}
	j, err := st.CreateJobFull("publish", c.ID, `{"content_id":`+itoa(c.ID)+`,"account":"a","platform":"naver_blog"}`, "", 0, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimNextJob("publish", "w"); err != nil {
		t.Fatal(err)
	}
	svc := NewService(st)
	done, err := svc.SubmitPublish(j.ID, PublishResult{URL: "https://e.com/post/9", Evidence: "uploads/evidence/x.png"})
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != "done" {
		t.Fatalf("job = %s", done.Status)
	}
	got, _ := st.GetContent(c.ID)
	if got.Status != "published" {
		t.Fatalf("content = %s, want published", got.Status)
	}
	rec, _ := st.GetPublishLog(c.ID, "naver_blog", "a")
	if rec.Status != "done" || rec.Note == "" {
		t.Fatalf("log = %+v", rec)
	}
}
