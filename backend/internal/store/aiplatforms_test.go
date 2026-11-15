package store

import (
	"path/filepath"
	"testing"
)

func openAIStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "ai.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := Migrate(st.DB()); err != nil {
		t.Fatal(err)
	}
	return st
}

// W15-1: the builtin five seed with proven chat values.
func TestAIPlatformSeed(t *testing.T) {
	st := openAIStore(t)
	items, err := st.ListAIPlatforms()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 5 {
		t.Fatalf("seeded = %d, want 5", len(items))
	}
	byKey := map[string]*AIPlatform{}
	for _, p := range items {
		byKey[p.Key] = p
	}
	meta := byKey["meta_ai"]
	if meta == nil || !meta.Builtin || meta.ChatStatus != AIChatOK {
		t.Fatalf("meta_ai = %+v", meta)
	}
	if meta.LoginURL != "https://www.meta.ai/" || meta.ComposerSelector == "" || meta.ReplySelector == "" {
		t.Fatalf("meta_ai selectors = %+v", meta)
	}
	claude := byKey["claude_app"]
	if claude == nil || claude.ReplyBodySelector != "[data-cds='Prose']" {
		t.Fatalf("claude body selector = %+v", claude)
	}
}

// W15-3/7: custom AI registers with https only; duplicates refuse.
func TestCustomAICRUD(t *testing.T) {
	st := openAIStore(t)
	p, err := st.CreateCustomAIPlatform("perplexity", "Perplexity", "https://www.perplexity.ai/")
	if err != nil {
		t.Fatal(err)
	}
	if p.Builtin || p.ChatStatus != AIChatUnlinked {
		t.Fatalf("custom = %+v", p)
	}
	if _, err := st.CreateCustomAIPlatform("perplexity", "P", "https://x.test/"); err != ErrDuplicate {
		t.Fatalf("duplicate err = %v", err)
	}
	if _, err := st.CreateCustomAIPlatform("plain", "P", "http://plain.test/"); err == nil {
		t.Fatal("http login accepted")
	}
	if _, err := st.CreateCustomAIPlatform("bad key", "P", "https://x.test/"); err == nil {
		t.Fatal("spaced key accepted")
	}
	p.ComposerSelector = "textarea"
	p.ChatStatus = AIChatOK
	if err := st.UpdateAIPlatform(p); err != nil {
		t.Fatal(err)
	}
	got, _ := st.GetAIPlatform("perplexity")
	if got.ComposerSelector != "textarea" || got.ChatStatus != AIChatOK {
		t.Fatalf("updated = %+v", got)
	}
	got.ChatStatus = "bogus"
	if err := st.UpdateAIPlatform(got); err == nil {
		t.Fatal("bad chat status accepted")
	}
	if err := st.DeleteCustomAIPlatform("meta_ai"); err == nil {
		t.Fatal("builtin delete accepted")
	}
	acc, err := st.CreateAccountRow(&ChannelAccount{Name: "px1", Platform: "perplexity", Kind: "agent_service"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteCustomAIPlatform("perplexity"); err == nil {
		t.Fatal("delete with accounts accepted")
	}
	if _, err := st.DB().Exec(`DELETE FROM channel_accounts WHERE id=?`, acc.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteCustomAIPlatform("perplexity"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := st.GetAIPlatform("perplexity"); err != ErrNotFound {
		t.Fatalf("get after delete = %v", err)
	}
}

// W15-2: only custom rows materialize adapter JSON.
func TestAIPlatformJSON(t *testing.T) {
	st := openAIStore(t)
	if path, err := st.AIPlatformJSON("meta_ai"); err != nil || path != "" {
		t.Fatalf("builtin json = %q, %v", path, err)
	}
	if path, err := st.AIPlatformJSON("nope"); err != nil || path != "" {
		t.Fatalf("unknown json = %q, %v", path, err)
	}
	if _, err := st.CreateCustomAIPlatform("ppx", "P", "https://x.test/"); err != nil {
		t.Fatal(err)
	}
	path, err := st.AIPlatformJSON("ppx")
	if err != nil || path == "" {
		t.Fatalf("custom json = %q, %v", path, err)
	}
}

// W15-5: AI lanes skip agents on unlinked platforms.
func TestClaimSkipsUnlinkedAI(t *testing.T) {
	st := openAIStore(t)
	if _, err := st.CreateCustomAIPlatform("ppx", "P", "https://x.test/"); err != nil {
		t.Fatal(err)
	}
	acc, err := st.CreateAccountRow(&ChannelAccount{Name: "px1", Platform: "ppx", Kind: "agent_service"})
	if err != nil {
		t.Fatal(err)
	}
	acc.Kind = "agent_service"
	if err := st.UpdateAccountRow(acc); err != nil {
		t.Fatal(err)
	}
	ag, _, err := st.CreateAgent("wB", "B", "", "", "", 1, &acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	p, err := st.CreateProduct("https://example.com/w15", "A", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateJob("generate", p.ID, "{}"); err != nil {
		t.Fatal(err)
	}
	if j, err := st.ClaimRoutedJob(ag, []string{"generate"}, 600); err != nil || j != nil {
		t.Fatalf("unlinked claim = %+v, %v; want nil", j, err)
	}
	row, _ := st.GetAIPlatform("ppx")
	row.ChatStatus = AIChatOK
	if err := st.UpdateAIPlatform(row); err != nil {
		t.Fatal(err)
	}
	if j, err := st.ClaimRoutedJob(ag, []string{"generate"}, 600); err != nil || j == nil {
		t.Fatalf("linked claim = %+v, %v; want job", j, err)
	}
}
