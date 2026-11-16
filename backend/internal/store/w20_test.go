package store

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func openW20Store(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "w20.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	if err := Migrate(st.DB()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if v, err := CurrentSchemaVersion(st.DB()); err != nil || v != "28" {
		t.Fatalf("schema version = %q, %v; want 28", v, err)
	}
	return st
}

func seedW20Account(t *testing.T, st *Store, name string) int64 {
	t.Helper()
	acc, err := st.CreateAccountRow(&ChannelAccount{Name: name, Platform: "meta_ai", Role: "worker", SessionState: "connected"})
	if err != nil {
		t.Fatalf("account: %v", err)
	}
	return acc.ID
}

// W20-1: one product thread's turns pair into full prompt/reply originals.
func TestW20TranscriptPairs(t *testing.T) {
	st := openW20Store(t)
	p, err := st.CreateProduct("https://example.com/w", "A", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	aid := seedW20Account(t, st, "ai-1")
	thread := "p" + strconv.FormatInt(p.ID, 10)
	if _, err := st.AppendChatMessageThread(aid, "user", "PROMPT-ONE", thread); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AppendChatMessageThread(aid, "assistant", "REPLY-ONE", thread); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AppendChatMessageThread(aid, "user", "PROMPT-TWO", thread); err != nil {
		t.Fatal(err)
	}
	// Noise on the default thread must not leak into the work log.
	if _, err := st.AppendChatMessage(aid, "user", "NOISE"); err != nil {
		t.Fatal(err)
	}
	got, err := st.WorkAILog(p.ID, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("entries = %d, want 2", len(got))
	}
	if got[0].Prompt != "PROMPT-ONE" || got[0].Reply != "REPLY-ONE" {
		t.Fatalf("pair0 = %+v", got[0])
	}
	if got[0].Account != "ai-1" {
		t.Fatalf("account = %q, want ai-1", got[0].Account)
	}
	if got[1].Prompt != "PROMPT-TWO" || got[1].Reply != "" {
		t.Fatalf("pair1 = %+v (failed run keeps empty reply)", got[1])
	}
}

// W20-1: works without transcript turns keep the job-summary fallback.
func TestW20AILogFallsBackToJobs(t *testing.T) {
	st := openW20Store(t)
	p, err := st.CreateProduct("https://example.com/j", "A", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	j, err := st.CreateJob("analyze", p.ID, "{}")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimNextJob("analyze", "w"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetJobResult(j.ID, `{"summary":"parsed"}`); err != nil {
		t.Fatal(err)
	}
	got, err := st.WorkAILog(p.ID, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !strings.Contains(got[0].Summary, "parsed") {
		t.Fatalf("fallback = %+v", got)
	}
}

// W20-1: AI originals older than 90 days purge; fresh turns survive.
func TestW20ChatRetention(t *testing.T) {
	st := openW20Store(t)
	aid := seedW20Account(t, st, "ai-1")
	old, err := st.AppendChatMessageThread(aid, "user", "old", "p9")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AppendChatMessageThread(aid, "user", "fresh", "p9"); err != nil {
		t.Fatal(err)
	}
	ancient := time.Now().UTC().AddDate(0, 0, -91).Format(time.RFC3339)
	if _, err := st.DB().Exec(`UPDATE chat_messages SET created_at=? WHERE id=?`, ancient, old.ID); err != nil {
		t.Fatal(err)
	}
	n, err := PurgeChat(st, 90)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("purged = %d, want 1", n)
	}
	var left int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM chat_messages`).Scan(&left); err != nil || left != 1 {
		t.Fatalf("left = %d, %v; want 1", left, err)
	}
}

// W20-2: audit rows record who/when/what per work, oldest first.
func TestW20AuditLog(t *testing.T) {
	st := openW20Store(t)
	if _, err := st.AppendAudit("admin", AuditWorkPause, "work", 7, 7, "멈추기 (게시물 2건)"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AppendAudit("", AuditConfirmPublished, "post", 3, 7, "올라가 있음 확인"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AppendAudit("admin", AuditTeamMembers, "team", 1, 0, "팀원 일괄 배치 (2명)"); err != nil {
		t.Fatal(err)
	}
	got, err := st.ListAuditByWork(7, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("work rows = %d, want 2", len(got))
	}
	if got[0].Action != AuditWorkPause || got[0].Actor != "admin" || got[0].CreatedAt == "" {
		t.Fatalf("row0 = %+v", got[0])
	}
	if got[1].Actor != "system" {
		t.Fatalf("empty actor = %q, want system", got[1].Actor)
	}
	recent, err := st.ListAuditRecent(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 3 || recent[0].Action != AuditTeamMembers {
		t.Fatalf("recent = %+v", recent)
	}
}

// W20-3: sessions zip, seal with the vault key, and come back byte-equal.
func TestW20SessionBackupRoundtrip(t *testing.T) {
	dir := t.TempDir()
	profiles := filepath.Join(dir, "profiles", "ai-1")
	if err := os.MkdirAll(profiles, 0o755); err != nil {
		t.Fatal(err)
	}
	want := []byte(`{"cookies":["a","b"]}`)
	if err := os.WriteFile(filepath.Join(profiles, "storage_state.json"), want, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profiles, "thread.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// Volatile browser weight stays out of the bundle.
	if err := os.MkdirAll(filepath.Join(profiles, "profile", "Cache"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profiles, "profile", "Cache", "bulk"), []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	gen := filepath.Join(dir, "gen")
	if err := os.MkdirAll(gen, 0o755); err != nil {
		t.Fatal(err)
	}
	bundle, err := BackupProfilesFile(gen, filepath.Join(dir, "profiles"), key)
	if err != nil || bundle == "" {
		t.Fatalf("bundle = %q, %v", bundle, err)
	}
	enc, err := os.ReadFile(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(enc), "cookies") {
		t.Fatal("bundle leaks plaintext")
	}
	restored := filepath.Join(dir, "profiles2")
	if err := DecryptProfiles(enc, key, restored); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(restored, "ai-1", "storage_state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("restored = %q", got)
	}
	if _, err := os.Stat(filepath.Join(restored, "ai-1", "thread.json")); err != nil {
		t.Fatalf("thread continuation missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(restored, "ai-1", "profile")); !os.IsNotExist(err) {
		t.Fatal("browser cache dir must stay out of the bundle")
	}
	// Wrong key and tampered bytes both fail closed.
	badKey := make([]byte, 32)
	if err := DecryptProfiles(enc, badKey, filepath.Join(dir, "bad")); err == nil {
		t.Fatal("wrong key decrypted")
	}
	enc[len(enc)-1] ^= 0xff
	if err := DecryptProfiles(enc, key, filepath.Join(dir, "tampered")); err == nil {
		t.Fatal("tampered bundle decrypted")
	}
	// Nothing to bundle is a silent no-op.
	if out, err := BackupProfilesFile(gen, filepath.Join(dir, "nobody"), key); err != nil || out != "" {
		t.Fatalf("missing dir = %q, %v; want silent no-op", out, err)
	}
}

// W20-3: restore brings the bundle back only when live sessions are gone.
func TestW20RestoreProfilesIfMissing(t *testing.T) {
	dir := t.TempDir()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(9)
	}
	profiles := filepath.Join(dir, "profiles")
	if err := os.MkdirAll(filepath.Join(profiles, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profiles, "a", "storage_state.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	enc, err := EncryptProfiles(profiles, key)
	if err != nil || enc == nil {
		t.Fatalf("enc = %d bytes, %v", len(enc), err)
	}
	// Live sessions present: no restore.
	data := t.TempDir()
	live := filepath.Join(data, "profiles", "a")
	if err := os.MkdirAll(live, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(live, "storage_state.json"), []byte(`{"live":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, ProfilesFileName), enc, 0o644); err != nil {
		t.Fatal(err)
	}
	if restored, err := RestoreProfilesIfMissing(data, key); err != nil || restored {
		t.Fatalf("restored = %v, %v; want false (live wins)", restored, err)
	}
	// Sessions wiped: the bundle comes back.
	if err := os.RemoveAll(filepath.Join(data, "profiles")); err != nil {
		t.Fatal(err)
	}
	restored, err := RestoreProfilesIfMissing(data, key)
	if err != nil || !restored {
		t.Fatalf("restored = %v, %v; want true", restored, err)
	}
	if _, err := os.Stat(filepath.Join(data, "profiles", "a", "storage_state.json")); err != nil {
		t.Fatalf("session file missing after restore: %v", err)
	}
	// No bundle at all: quiet no-op.
	if restored, err := RestoreProfilesIfMissing(t.TempDir(), key); err != nil || restored {
		t.Fatalf("restored = %v, %v; want false", restored, err)
	}
}

// W20-3: the backup option defaults on, including for pre-W20 rows.
func TestW20SessionsIncludedDefault(t *testing.T) {
	st := openW20Store(t)
	if !SessionsIncluded(st.DB()) {
		t.Fatal("fresh DB must include sessions")
	}
	s, err := GetAutomationSettings(st.DB())
	if err != nil {
		t.Fatal(err)
	}
	if !s.BackupIncludeSessions {
		t.Fatal("default settings must include sessions")
	}
	s.BackupIncludeSessions = false
	if err := SetAutomationSettings(st.DB(), s); err != nil {
		t.Fatal(err)
	}
	if SessionsIncluded(st.DB()) {
		t.Fatal("operator opt-out must stick")
	}
	// Pre-W20 rows lack the key: they read back as included.
	if _, err := st.DB().Exec(`UPDATE app_meta SET v='{"enabled":true}' WHERE k=?`, KeyAutomationSettings); err != nil {
		t.Fatal(err)
	}
	back, err := GetAutomationSettings(st.DB())
	if err != nil {
		t.Fatal(err)
	}
	if !back.BackupIncludeSessions {
		t.Fatal("pre-W20 row must default to included")
	}
}

// W20-4: executors, per-PC load, per-team backlog, and the error tail.
func TestW20HealthQueries(t *testing.T) {
	st := openW20Store(t)
	if db := st.ProbeDB(); !db.OK {
		t.Fatal("probe must pass on a live store")
	}
	if _, _, err := st.CreateAgent("ex-1", "T", "pc-1", "", "", 3, nil); err != nil {
		t.Fatal(err)
	}
	agents, err := st.ListAgents(AgentFilter{}, 10, 0)
	if err != nil || len(agents) != 1 {
		t.Fatalf("agents = %+v, %v", agents, err)
	}
	if err := st.TouchAgent(agents[0].ID); err != nil {
		t.Fatal(err)
	}
	p, err := st.CreateProduct("https://example.com/h", "A", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	j, err := st.CreateJob("analyze", p.ID, "{}")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClaimNextJob("analyze", "ex-1"); err != nil {
		t.Fatal(err)
	}
	// Bind the claim to the agent so the load counts it.
	if _, err := st.DB().Exec(`UPDATE jobs SET agent_id=? WHERE id=?`, agents[0].ID, j.ID); err != nil {
		t.Fatal(err)
	}
	health, err := st.AgentHealth(time.Now())
	if err != nil || len(health) != 1 {
		t.Fatalf("health = %+v, %v", health, err)
	}
	if !health[0].Online || health[0].Active != 1 || health[0].Capacity != 3 {
		t.Fatalf("agent = %+v", health[0])
	}
	load := PCLoad(health)
	if len(load) != 1 || load[0].Host != "pc-1" || load[0].Active != 1 {
		t.Fatalf("pc load = %+v", load)
	}
	// One more pending job lands in the team backlog; the claimed one
	// does not count as waiting.
	def, err := st.GetDefaultTeam()
	if err != nil {
		t.Fatal(err)
	}
	j2, err := st.CreateJob("strategy", p.ID, "{}")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetJobTeam(j2.ID, def.ID); err != nil {
		t.Fatal(err)
	}
	queues, err := st.TeamPendingCounts()
	if err != nil {
		t.Fatal(err)
	}
	if len(queues) != 1 || queues[0].Pending != 1 || queues[0].TeamID != def.ID {
		t.Fatalf("queues = %+v", queues)
	}
	if _, err := st.SetJobFailed(j2.ID, "시간이 초과되어 중단됐습니다"); err != nil {
		t.Fatal(err)
	}
	errs, err := st.RecentErrors(10)
	if err != nil || len(errs) != 1 || errs[0].Error == "" {
		t.Fatalf("errors = %+v, %v", errs, err)
	}
	disk := DiskUsage(t.TempDir())
	if disk.TotalBytes == 0 {
		t.Fatal("disk totals must read on this platform")
	}
	if disk.Low {
		t.Fatalf("test volume must not read low: %+v", disk)
	}
}

// W20: the notifications CHECK admits the disk_low kind with old rows kept.
func TestW20DiskLowKind(t *testing.T) {
	st := openW20Store(t)
	made, err := st.CreateNotificationDedup("disk_low", "error", "system", 0, "disk_low:system:0", "디스크 여유 9.0% — 미디어 정리가 필요합니다")
	if err != nil || !made {
		t.Fatalf("disk_low insert = %v, %v", made, err)
	}
	madeAgain, err := st.CreateNotificationDedup("disk_low", "error", "system", 0, "disk_low:system:0", "again")
	if err != nil || madeAgain {
		t.Fatalf("dedup = %v, %v; want single live alert", madeAgain, err)
	}
}
