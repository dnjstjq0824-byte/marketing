package product

import (
	"path/filepath"
	"testing"

	"somun/internal/store"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "product.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := store.Migrate(st.DB()); err != nil {
		t.Fatal(err)
	}
	return st
}

// W18-2: industry set / keep / clear through product Update.
func TestUpdateIndustry(t *testing.T) {
	st := openTestStore(t)
	svc := NewService(st)
	p, err := st.CreateProduct("https://shop.example/p/1", "A", 0, "b1")
	if err != nil {
		t.Fatal(err)
	}
	base := UpdateInput{Name: "n", SourceURL: "https://shop.example/p/1", ExitType: "A", Business: "b1"}
	base.Industry = "cosmetics"
	got, err := svc.Update(p.ID, base)
	if err != nil || got.Industry != "cosmetics" {
		t.Fatalf("set = %+v, %v", got, err)
	}
	// Empty keeps the stored tag.
	base.Industry = ""
	got, err = svc.Update(p.ID, base)
	if err != nil || got.Industry != "cosmetics" {
		t.Fatalf("keep = %+v, %v", got, err)
	}
	// "general" clears back to common-only.
	base.Industry = "general"
	got, err = svc.Update(p.ID, base)
	if err != nil || got.Industry != "" {
		t.Fatalf("clear = %+v, %v", got, err)
	}
	// Unknown tags reject.
	base.Industry = "bogus"
	if _, err := svc.Update(p.ID, base); err != ErrBadIndustry {
		t.Fatalf("bogus err = %v, want ErrBadIndustry", err)
	}
}

// W1: URL-less works (topic/image input) stay patchable.
func TestUpdateWithoutURL(t *testing.T) {
	st := openTestStore(t)
	svc := NewService(st)
	p, err := st.CreateProduct("", "A", 0, "b1")
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.Update(p.ID, UpdateInput{Business: "b1", Industry: "food"})
	if err != nil {
		t.Fatalf("update = %v", err)
	}
	if got.Industry != "food" || got.SourceURL != "" {
		t.Fatalf("got = %+v", got)
	}
	// A malformed URL still rejects.
	if _, err := svc.Update(p.ID, UpdateInput{SourceURL: "notaurl"}); err != ErrBadURL {
		t.Fatalf("bad url err = %v, want ErrBadURL", err)
	}
}
