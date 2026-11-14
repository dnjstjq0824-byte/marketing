package works

import (
	"errors"
	"path/filepath"
	"testing"

	"somun/internal/store"
)

func openWorksStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "w.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := store.Migrate(st.DB()); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestCreateURL(t *testing.T) {
	st := openWorksStore(t)
	svc := NewService(st)
	id, err := svc.Create(CreateReq{
		Inputs:   []InputReq{{Kind: "url", URL: "https://shop.test/p1"}},
		Channels: []ChannelReq{{Channel: "blog", Platform: "naver", Versions: 2}},
		Business: "LUNO",
		Affiliate: AffiliateReq{IsSponsored: true, Disclosure: "협찬 고지"},
		Team:     "마케팅1팀",
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := st.GetProduct(id)
	if err != nil {
		t.Fatal(err)
	}
	if p.SourceURL != "https://shop.test/p1" || p.ExitType != "B" || p.BriefJSON == "" {
		t.Fatalf("product = %+v", p)
	}
	d, err := st.WorkDetailByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if d.Stage != "intake" {
		t.Fatalf("stage = %q", d.Stage)
	}
	if len(d.Contents) != 0 {
		t.Fatalf("contents = %d", len(d.Contents))
	}
}

func TestCreateTopicAndFiles(t *testing.T) {
	st := openWorksStore(t)
	svc := NewService(st)
	m, err := st.CreateMedia("upload", "lib/x.png", 10, "image/png", "sha-x")
	if err != nil {
		t.Fatal(err)
	}
	id, err := svc.Create(CreateReq{
		Inputs: []InputReq{
			{Kind: "topic", Text: "여름 신상"},
			{Kind: "image", MediaIDs: []int64{m.ID}},
		},
		Channels: []ChannelReq{{Channel: "instagram", Platform: "instagram", Versions: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	p, err := st.GetProduct(id)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "여름 신상" || p.SourceURL != "" {
		t.Fatalf("product = %+v", p)
	}
	got, err := st.GetMedia(m.ID)
	if err != nil || got.ProductID == nil || *got.ProductID != id {
		t.Fatalf("media = %+v %v", got, err)
	}
}

func TestCreateValidation(t *testing.T) {
	st := openWorksStore(t)
	svc := NewService(st)
	base := CreateReq{
		Inputs:   []InputReq{{Kind: "url", URL: "https://shop.test/p1"}},
		Channels: []ChannelReq{{Channel: "blog", Platform: "naver", Versions: 1}},
	}
	cases := []struct {
		name string
		mut  func(*CreateReq)
		want error
	}{
		{"no input", func(r *CreateReq) { r.Inputs = nil }, ErrNoInput},
		{"bad url", func(r *CreateReq) { r.Inputs[0].URL = "notaurl" }, nil},
		{"no channel", func(r *CreateReq) { r.Channels = nil }, ErrNoChannel},
		{"bad channel", func(r *CreateReq) { r.Channels[0].Channel = "tv" }, ErrBadChannel},
		{"bad versions", func(r *CreateReq) { r.Channels[0].Versions = 9 }, ErrBadVersions},
		{"bad start", func(r *CreateReq) { r.StartAt = "어제" }, ErrBadStart},
		{"no disclosure", func(r *CreateReq) { r.Affiliate.IsSponsored = true }, ErrNoDisclosure},
		{"bad media", func(r *CreateReq) {
			r.Inputs = []InputReq{{Kind: "image", MediaIDs: []int64{9999}}}
		}, ErrBadMedia},
	}
	for _, c := range cases {
		req := base
		req.Inputs = append([]InputReq(nil), base.Inputs...)
		req.Channels = append([]ChannelReq(nil), base.Channels...)
		c.mut(&req)
		_, err := svc.Create(req)
		if c.want != nil {
			if !errors.Is(err, c.want) {
				t.Fatalf("%s: err = %v", c.name, err)
			}
			continue
		}
		if err == nil {
			t.Fatalf("%s: expected error", c.name)
		}
	}
}

func TestWorksListDetailControls(t *testing.T) {
	st := openWorksStore(t)
	svc := NewService(st)
	id, err := svc.Create(CreateReq{
		Inputs:   []InputReq{{Kind: "url", URL: "https://shop.test/p9"}},
		Channels: []ChannelReq{{Channel: "blog", Platform: "naver", Versions: 1}},
		Business: "LUNO",
	})
	if err != nil {
		t.Fatal(err)
	}
	c, err := st.CreateContent(&store.Content{ProductID: id, Channel: "blog", Title: "d1"})
	if err != nil {
		t.Fatal(err)
	}
	items, err := st.ListWorks("running", "", "", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != id || items[0].Stage != "generate" {
		t.Fatalf("items = %+v", items)
	}
	items, err = st.ListWorks("running", "nomatch-zzz", "", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("search = %+v", items)
	}
	// A fresh work (analyze pending, no contents yet) still lists as running.
	fresh, err := st.CreateProduct("https://shop.test/fresh", "A", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateJob("analyze", fresh.ID, "{}"); err != nil {
		t.Fatal(err)
	}
	items, err = st.ListWorks("running", "fresh", "", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Stage != "intake" || items[0].Detail != "분석 대기 중" {
		t.Fatalf("fresh = %+v", items)
	}
	n, err := st.SetWorkHeld(id, true)
	if err != nil || n != 1 {
		t.Fatalf("held = %d %v", n, err)
	}
	d, err := st.WorkDetailByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if !d.Contents[0].Held {
		t.Fatalf("detail = %+v", d.Contents[0])
	}
	if _, err := st.SetWorkHeld(id, false); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Delete(id); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetProduct(id); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("deleted product still there: %v", err)
	}
	_ = c
}

func TestHoldAndRegeneratePost(t *testing.T) {
	st := openWorksStore(t)
	svc := NewService(st)
	id, err := svc.Create(CreateReq{
		Inputs:   []InputReq{{Kind: "url", URL: "https://shop.test/hold"}},
		Channels: []ChannelReq{{Channel: "blog", Platform: "naver", Versions: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	c, err := st.CreateContent(&store.Content{ProductID: id, Channel: "blog", Title: "h"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.HoldPost(id, c.ID, true); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetContent(c.ID)
	if err != nil || !got.Held {
		t.Fatalf("held = %+v %v", got, err)
	}
	if err := svc.HoldPost(9999, c.ID, true); !errors.Is(err, ErrNoAccount) {
		t.Fatalf("wrong work: %v", err)
	}
	jobID, err := svc.RegeneratePost(id, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if jobID <= 0 {
		t.Fatal("no job")
	}
	if _, err := svc.RegeneratePost(id, 9999); !errors.Is(err, ErrNoAccount) {
		t.Fatalf("wrong post: %v", err)
	}
}

func TestWorkAILog(t *testing.T) {
	st := openWorksStore(t)
	svc := NewService(st)
	id, err := svc.Create(CreateReq{
		Inputs:   []InputReq{{Kind: "url", URL: "https://shop.test/p5"}},
		Channels: []ChannelReq{{Channel: "blog", Platform: "naver", Versions: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	log, err := st.WorkAILog(id, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(log) != 1 || log[0].Type != "analyze" || log[0].Stage != "analyze" {
		t.Fatalf("log = %+v", log)
	}
}
