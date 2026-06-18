package snapshot

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const validSnapshot = `---
bifrost_version: 1
timestamp: 2025-03-21T14:32:17Z
source_tool: claude-code
project: my-api
token_pressure: high
---

# Session Snapshot

## Current Task
Implement JWT refresh token rotation

## Status
- [x] validateToken()
- [-] refreshToken() — stub written, logic incomplete
- [ ] revokeToken()

## Active Files
- ` + "`src/auth.ts`" + ` — middleware stubbed, not wired
- ` + "`src/tokens.ts`" + ` — new file, refresh logic

## Decisions Made
- **Using jsonwebtoken not jose**: already installed, fewer deps
- **Refresh tokens in Redis**: faster lookup than Postgres for this

## Environment Notes
- AUTH_SECRET must be in .env, not .env.local
- Run with: npm run dev -- --port 3001

## Next Step
Write unit tests for validateToken() using pattern from crypto.test.ts.
Focus on the expiry edge case — it returns null, does not throw.
`

func TestParseValidSnapshot(t *testing.T) {
	s, err := Parse([]byte(validSnapshot))
	if err != nil {
		t.Fatal(err)
	}

	if s.BifrostVersion != 1 {
		t.Errorf("expected version 1, got %d", s.BifrostVersion)
	}
	if s.SourceTool != "claude-code" {
		t.Errorf("expected claude-code, got %s", s.SourceTool)
	}
	if s.Project != "my-api" {
		t.Errorf("expected my-api, got %s", s.Project)
	}
	if s.TokenPressure != "high" {
		t.Errorf("expected high, got %s", s.TokenPressure)
	}
	if s.CurrentTask != "Implement JWT refresh token rotation" {
		t.Errorf("unexpected current task: %q", s.CurrentTask)
	}
	if len(s.Status) != 3 {
		t.Errorf("expected 3 status items, got %d", len(s.Status))
	}
	if len(s.ActiveFiles) != 2 {
		t.Errorf("expected 2 active files, got %d", len(s.ActiveFiles))
	}
	if s.ActiveFiles[0].Path != "src/auth.ts" {
		t.Errorf("unexpected file path: %s", s.ActiveFiles[0].Path)
	}
	if s.ActiveFiles[0].Note != "middleware stubbed, not wired" {
		t.Errorf("unexpected file note: %q", s.ActiveFiles[0].Note)
	}
	if len(s.Decisions) != 2 {
		t.Errorf("expected 2 decisions, got %d", len(s.Decisions))
	}
	if len(s.EnvNotes) != 2 {
		t.Errorf("expected 2 env notes, got %d", len(s.EnvNotes))
	}
	if !strings.HasPrefix(s.NextStep, "Write unit tests") {
		t.Errorf("unexpected next step: %q", s.NextStep)
	}
}

func TestParseMissingSections(t *testing.T) {
	minimal := `---
bifrost_version: 1
timestamp: 2025-03-21T14:32:17Z
source_tool: opencode
project: test
token_pressure: low
---

# Session Snapshot

## Current Task
Do something

## Next Step
Continue doing something
`
	s, err := Parse([]byte(minimal))
	if err != nil {
		t.Fatal(err)
	}

	if s.CurrentTask != "Do something" {
		t.Errorf("unexpected task: %q", s.CurrentTask)
	}
	if len(s.Status) != 0 {
		t.Errorf("expected empty status, got %d items", len(s.Status))
	}
	if len(s.ActiveFiles) != 0 {
		t.Errorf("expected empty files, got %d items", len(s.ActiveFiles))
	}
	if len(s.Decisions) != 0 {
		t.Errorf("expected empty decisions, got %d items", len(s.Decisions))
	}
	if len(s.EnvNotes) != 0 {
		t.Errorf("expected empty env notes, got %d items", len(s.EnvNotes))
	}
}

func TestParseMissingFrontmatter(t *testing.T) {
	bad := "# No frontmatter\nJust content"
	_, err := Parse([]byte(bad))
	if err == nil {
		t.Error("expected error for missing frontmatter")
	}
}

func TestParseMissingTimestamp(t *testing.T) {
	bad := `---
bifrost_version: 1
source_tool: claude-code
project: test
token_pressure: low
---

# Content
`
	_, err := Parse([]byte(bad))
	if err == nil {
		t.Error("expected error for missing timestamp")
	}
}

func TestWriteAndReadRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)

	original := &Snapshot{
		BifrostVersion: 1,
		Timestamp:      now,
		SourceTool:     "claude-code",
		Project:        "test-project",
		TokenPressure:  "medium",
		CurrentTask:    "Build the widget",
		Status:         []string{"- [x] design", "- [ ] implement"},
		ActiveFiles: []ActiveFile{
			{Path: "src/widget.go", Note: "new file"},
		},
		Decisions: []string{"- **Go over Rust**: team familiarity"},
		EnvNotes:  []string{"- Run: go test ./..."},
		NextStep:  "Implement the core logic in widget.go",
	}

	if err := Write(tmp, original); err != nil {
		t.Fatal(err)
	}

	read, err := Read(tmp)
	if err != nil {
		t.Fatal(err)
	}

	if read.Project != original.Project {
		t.Errorf("project: expected %q, got %q", original.Project, read.Project)
	}
	if read.CurrentTask != original.CurrentTask {
		t.Errorf("task: expected %q, got %q", original.CurrentTask, read.CurrentTask)
	}
	if read.SourceTool != original.SourceTool {
		t.Errorf("source: expected %q, got %q", original.SourceTool, read.SourceTool)
	}
	if !read.Timestamp.Equal(original.Timestamp) {
		t.Errorf("timestamp: expected %v, got %v", original.Timestamp, read.Timestamp)
	}
	if len(read.Status) != len(original.Status) {
		t.Errorf("status count: expected %d, got %d", len(original.Status), len(read.Status))
	}
	if len(read.ActiveFiles) != 1 || read.ActiveFiles[0].Path != "src/widget.go" {
		t.Errorf("active files mismatch: %+v", read.ActiveFiles)
	}
	if read.NextStep != original.NextStep {
		t.Errorf("next step: expected %q, got %q", original.NextStep, read.NextStep)
	}
}

func TestReadNoSnapshot(t *testing.T) {
	tmp := t.TempDir()
	_, err := Read(tmp)
	if err != ErrNoSnapshot {
		t.Errorf("expected ErrNoSnapshot, got %v", err)
	}
}

func TestArchiveCreatesHistoryFile(t *testing.T) {
	tmp := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)

	s := &Snapshot{
		BifrostVersion: 1,
		Timestamp:      now,
		SourceTool:     "claude-code",
		Project:        "test",
		TokenPressure:  "low",
		CurrentTask:    "task one",
		NextStep:       "next",
	}

	if err := Write(tmp, s); err != nil {
		t.Fatal(err)
	}

	// Write again to trigger archive
	s.CurrentTask = "task two"
	s.Timestamp = now.Add(time.Minute)
	if err := Write(tmp, s); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(HistoryDir(tmp))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 archive entry, got %d", len(entries))
	}
}

func TestHistorySortedNewestFirst(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(HistoryDir(tmp), 0755)

	base := time.Date(2025, 3, 20, 10, 0, 0, 0, time.UTC)

	for i := 0; i < 3; i++ {
		s := &Snapshot{
			BifrostVersion: 1,
			Timestamp:      base.Add(time.Duration(i) * time.Hour),
			SourceTool:     "claude-code",
			Project:        "test",
			TokenPressure:  "low",
			CurrentTask:    "task",
			NextStep:       "next",
		}
		ts := s.Timestamp.UTC().Format("2006-01-02T15-04-05Z")
		data := Render(s)
		os.WriteFile(filepath.Join(HistoryDir(tmp), ts+".md"), []byte(data), 0644)
	}

	history, err := History(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 3 {
		t.Fatalf("expected 3 history entries, got %d", len(history))
	}

	// Should be newest first
	if !history[0].Timestamp.After(history[1].Timestamp) {
		t.Error("history not sorted newest-first")
	}
	if !history[1].Timestamp.After(history[2].Timestamp) {
		t.Error("history not sorted newest-first")
	}
}

func TestNewSemanticFieldsRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)

	original := &Snapshot{
		BifrostVersion: 2,
		Timestamp:      now,
		SourceTool:     "claude-code",
		Project:        "test-project",
		TokenPressure:  "high",
		SessionIntent:  "implementing",
		ActivePlanName: "auth-refactor",
		GitSHA:         "abc1234def567890",
		SessionStart:   "2025-03-21T12:00:00Z",
		CurrentTask:    "Implement refresh token rotation",
		NextStep:       "Write tests for validateToken()",
		Assumptions:    []string{"- Redis is on localhost:6379"},
		OpenQuestions:  []string{"- Should tokens be single-use?"},
		Risks:          []string{"- Token revocation list not yet built"},
		ActiveFiles: []ActiveFile{
			{Path: "src/auth.ts", Note: "stub written", Confidence: "medium"},
			{Path: "src/tokens.ts", Note: "new file", Confidence: "low"},
		},
	}

	if err := Write(tmp, original); err != nil {
		t.Fatal(err)
	}

	read, err := Read(tmp)
	if err != nil {
		t.Fatal(err)
	}

	if read.SessionIntent != "implementing" {
		t.Errorf("session_intent: expected %q, got %q", "implementing", read.SessionIntent)
	}
	if read.ActivePlanName != "auth-refactor" {
		t.Errorf("active_plan_name: expected %q, got %q", "auth-refactor", read.ActivePlanName)
	}
	if read.GitSHA != "abc1234def567890" {
		t.Errorf("git_sha: expected %q, got %q", "abc1234def567890", read.GitSHA)
	}
	if read.SessionStart != "2025-03-21T12:00:00Z" {
		t.Errorf("session_start: expected %q, got %q", "2025-03-21T12:00:00Z", read.SessionStart)
	}
	if len(read.Assumptions) != 1 {
		t.Errorf("assumptions: expected 1, got %d", len(read.Assumptions))
	}
	if len(read.OpenQuestions) != 1 {
		t.Errorf("open_questions: expected 1, got %d", len(read.OpenQuestions))
	}
	if len(read.Risks) != 1 {
		t.Errorf("risks: expected 1, got %d", len(read.Risks))
	}
	if len(read.ActiveFiles) != 2 {
		t.Fatalf("active_files: expected 2, got %d", len(read.ActiveFiles))
	}
	if read.ActiveFiles[0].Confidence != "medium" {
		t.Errorf("confidence[0]: expected %q, got %q", "medium", read.ActiveFiles[0].Confidence)
	}
	if read.ActiveFiles[1].Confidence != "low" {
		t.Errorf("confidence[1]: expected %q, got %q", "low", read.ActiveFiles[1].Confidence)
	}
}

func TestNewFieldsAbsentInV1SnapshotParsesCleanly(t *testing.T) {
	// v1 snapshot has no new fields — should parse without error and leave them empty
	s, err := Parse([]byte(validSnapshot))
	if err != nil {
		t.Fatal(err)
	}
	if s.SessionIntent != "" {
		t.Errorf("expected empty session_intent, got %q", s.SessionIntent)
	}
	if s.ActivePlanName != "" {
		t.Errorf("expected empty active_plan_name, got %q", s.ActivePlanName)
	}
	if s.GitSHA != "" {
		t.Errorf("expected empty git_sha, got %q", s.GitSHA)
	}
	if len(s.Assumptions) != 0 {
		t.Errorf("expected empty assumptions, got %d", len(s.Assumptions))
	}
	if len(s.OpenQuestions) != 0 {
		t.Errorf("expected empty open_questions, got %d", len(s.OpenQuestions))
	}
	if len(s.Risks) != 0 {
		t.Errorf("expected empty risks, got %d", len(s.Risks))
	}
}

func TestActiveFileConfidenceRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)

	original := &Snapshot{
		BifrostVersion: 2,
		Timestamp:      now,
		SourceTool:     "claude-code",
		Project:        "test",
		TokenPressure:  "low",
		CurrentTask:    "task",
		NextStep:       "next",
		ActiveFiles: []ActiveFile{
			{Path: "a.go", Note: "done", Confidence: "high"},
			{Path: "b.go", Note: "partial"}, // no confidence
			{Path: "c.go", Note: "stub", Confidence: "low"},
		},
	}

	if err := Write(tmp, original); err != nil {
		t.Fatal(err)
	}

	read, err := Read(tmp)
	if err != nil {
		t.Fatal(err)
	}

	if read.ActiveFiles[0].Confidence != "high" {
		t.Errorf("expected high, got %q", read.ActiveFiles[0].Confidence)
	}
	if read.ActiveFiles[1].Confidence != "" {
		t.Errorf("expected empty confidence, got %q", read.ActiveFiles[1].Confidence)
	}
	if read.ActiveFiles[2].Confidence != "low" {
		t.Errorf("expected low, got %q", read.ActiveFiles[2].Confidence)
	}
}

func TestRenderSnapshotGolden(t *testing.T) {
	s := &Snapshot{
		BifrostVersion: 2,
		Timestamp:      time.Date(2026, 6, 18, 10, 22, 33, 0, time.UTC),
		SourceTool:     "claude-code",
		Project:        "bifrost",
		TokenPressure:  "high",
		SessionIntent:  "implementing",
		ActivePlanName: "integrity-pack",
		GitSHA:         "abc123def456",
		SessionStart:   "2026-06-18T09:00:00Z",
		CurrentTask:    "Implement JSON-backed integrity foundation",
		Status: []string{
			"- [x] Repo audit complete",
			"- [-] Golden tests in progress",
			"- [ ] JSON schema not started",
		},
		ActiveFiles: []ActiveFile{
			{Path: "internal/snapshot/snapshot.go", Note: "compatibility model", Confidence: "high"},
			{Path: "internal/snapshot/parse.go", Note: "Markdown parser and renderer", Confidence: "medium"},
		},
		Decisions: []string{
			"- Keep Markdown readable during JSON migration",
			"- Treat snapshot.v2 schema separately from bifrost_version",
		},
		EnvNotes:      []string{"- Run go test ./... from repo root"},
		NextStep:      "Add JSON schema structs without changing current Markdown behavior.",
		Assumptions:   []string{"- Existing slash commands continue to prefer MCP when available"},
		OpenQuestions: []string{"- Should session.json be written by default in the first JSON phase?"},
		Risks:         []string{"- Breaking existing session.md parsing would break current handin fallback"},
	}

	got := Render(s)
	wantBytes, err := os.ReadFile(filepath.Join("testdata", "snapshot_render.golden.md"))
	if err != nil {
		t.Fatal(err)
	}
	if got != string(wantBytes) {
		t.Fatalf("snapshot render mismatch\n--- got ---\n%s\n--- want ---\n%s", got, string(wantBytes))
	}
}

func TestRestoreRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	base := time.Date(2025, 3, 20, 10, 0, 0, 0, time.UTC)

	// Write first snapshot
	s1 := &Snapshot{
		BifrostVersion: 1,
		Timestamp:      base,
		SourceTool:     "claude-code",
		Project:        "test",
		TokenPressure:  "low",
		CurrentTask:    "first task",
		NextStep:       "first next",
	}
	if err := Write(tmp, s1); err != nil {
		t.Fatal(err)
	}

	// Write second snapshot (archives first)
	s2 := &Snapshot{
		BifrostVersion: 1,
		Timestamp:      base.Add(time.Hour),
		SourceTool:     "opencode",
		Project:        "test",
		TokenPressure:  "medium",
		CurrentTask:    "second task",
		NextStep:       "second next",
	}
	if err := Write(tmp, s2); err != nil {
		t.Fatal(err)
	}

	// Current should be s2
	current, _ := Read(tmp)
	if current.CurrentTask != "second task" {
		t.Errorf("expected second task, got %q", current.CurrentTask)
	}
	if err := WriteSnapshotV2(tmp, SnapshotToV2(tmp, s2)); err != nil {
		t.Fatal(err)
	}

	// Restore s1 (index 0 = newest in history, which is s1)
	if err := Restore(tmp, 0); err != nil {
		t.Fatal(err)
	}

	restored, _ := Read(tmp)
	if restored.CurrentTask != "first task" {
		t.Errorf("expected first task after restore, got %q", restored.CurrentTask)
	}
	restoredV2, err := SnapshotFromProject(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if restoredV2.Session.Task != "first task" {
		t.Errorf("expected JSON-backed restore to expose first task, got %q", restoredV2.Session.Task)
	}
}

func TestPruneNoop(t *testing.T) {
	tmp := t.TempDir()

	// Prune on empty / non-existent history dir should not error
	if err := Prune(tmp, 10); err != nil {
		t.Fatalf("Prune on empty dir: %v", err)
	}

	// maxKeep <= 0 should always be a no-op
	if err := Prune(tmp, 0); err != nil {
		t.Fatalf("Prune(0): %v", err)
	}
	if err := Prune(tmp, -1); err != nil {
		t.Fatalf("Prune(-1): %v", err)
	}
}

func TestPruneKeepsNewest(t *testing.T) {
	tmp := t.TempDir()
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// Write 5 snapshots so history ends up with 4 archived entries
	// (each Write archives the previous one)
	for i := 0; i < 5; i++ {
		s := &Snapshot{
			BifrostVersion: 1,
			Timestamp:      base.Add(time.Duration(i) * time.Hour),
			SourceTool:     "claude-code",
			Project:        "test",
			TokenPressure:  "low",
			CurrentTask:    fmt.Sprintf("task-%d", i),
			NextStep:       "next",
		}
		if err := Write(tmp, s); err != nil {
			t.Fatalf("Write %d: %v", i, err)
		}
	}

	history, err := History(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 4 {
		t.Fatalf("expected 4 history entries before prune, got %d", len(history))
	}

	// Prune to 2 — should remove 2 oldest, keep 2 newest
	if err := Prune(tmp, 2); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	history, err = History(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries after prune, got %d", len(history))
	}

	// Newest entry should be task-3 (index 0 = newest-first)
	if history[0].CurrentTask != "task-3" {
		t.Errorf("expected task-3 as newest after prune, got %q", history[0].CurrentTask)
	}
	if history[1].CurrentTask != "task-2" {
		t.Errorf("expected task-2 as second after prune, got %q", history[1].CurrentTask)
	}
}

func TestPruneNoopWhenUnderLimit(t *testing.T) {
	tmp := t.TempDir()
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// Write 3 snapshots → 2 in history
	for i := 0; i < 3; i++ {
		s := &Snapshot{
			BifrostVersion: 1,
			Timestamp:      base.Add(time.Duration(i) * time.Hour),
			SourceTool:     "claude-code",
			Project:        "test",
			TokenPressure:  "low",
			CurrentTask:    fmt.Sprintf("task-%d", i),
			NextStep:       "next",
		}
		if err := Write(tmp, s); err != nil {
			t.Fatalf("Write %d: %v", i, err)
		}
	}

	// Prune with a limit larger than history — nothing should be removed
	if err := Prune(tmp, 10); err != nil {
		t.Fatalf("Prune: %v", err)
	}

	history, err := History(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries unchanged, got %d", len(history))
	}
}

func TestWritePrunesAutomatically(t *testing.T) {
	tmp := t.TempDir()
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// Write DefaultMaxHistory+2 snapshots so history would exceed the limit
	for i := 0; i <= DefaultMaxHistory+1; i++ {
		s := &Snapshot{
			BifrostVersion: 1,
			Timestamp:      base.Add(time.Duration(i) * time.Hour),
			SourceTool:     "claude-code",
			Project:        "test",
			TokenPressure:  "low",
			CurrentTask:    fmt.Sprintf("task-%d", i),
			NextStep:       "next",
		}
		if err := Write(tmp, s); err != nil {
			t.Fatalf("Write %d: %v", i, err)
		}
	}

	history, err := History(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) > DefaultMaxHistory {
		t.Errorf("expected at most %d history entries after auto-prune, got %d", DefaultMaxHistory, len(history))
	}
}

func TestWriteRedactsSecretsBeforeSessionMarkdownWrite(t *testing.T) {
	tmp := t.TempDir()
	s := &Snapshot{
		BifrostVersion: 1,
		Timestamp:      time.Now().UTC(),
		SourceTool:     "claude-code",
		Project:        "test",
		TokenPressure:  "low",
		CurrentTask:    "Do not leak OPENAI_API_KEY=sk-proj-abcdefghijklmnopqrstuvwxyz123456",
		NextStep:       "next",
	}
	if err := Write(tmp, s); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(SessionPath(tmp))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "sk-proj-abcdefghijklmnopqrstuvwxyz123456") {
		t.Fatalf("session.md contains raw secret: %s", string(data))
	}
	if !strings.Contains(string(data), "[REDACTED:env_secret]") {
		t.Fatalf("session.md does not contain redaction marker: %s", string(data))
	}
}

func TestWriteFailsWhenSecurityStrictFindsSecret(t *testing.T) {
	tmp := t.TempDir()
	clean := &Snapshot{
		BifrostVersion: 1,
		Timestamp:      time.Now().UTC(),
		SourceTool:     "claude-code",
		Project:        "test",
		TokenPressure:  "low",
		CurrentTask:    "clean",
		NextStep:       "next",
	}
	if err := Write(tmp, clean); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(Dir(tmp), "config.json"), []byte(`{"security":{"strict":true}}`), 0600); err != nil {
		t.Fatal(err)
	}
	s := &Snapshot{
		BifrostVersion: 1,
		Timestamp:      time.Now().UTC(),
		SourceTool:     "claude-code",
		Project:        "test",
		TokenPressure:  "low",
		CurrentTask:    "Use Bearer abcdefghijklmnopqrstuvwxyz123456",
		NextStep:       "next",
	}
	if err := Write(tmp, s); err == nil {
		t.Fatal("expected strict security write failure")
	}
	data, err := os.ReadFile(SessionPath(tmp))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "abcdefghijklmnopqrstuvwxyz123456") || !strings.Contains(string(data), "clean") {
		t.Fatalf("strict failure should preserve existing clean session: %s", string(data))
	}
	entries, err := os.ReadDir(HistoryDir(tmp))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("strict failure should not archive current session, got %d entries", len(entries))
	}
}
