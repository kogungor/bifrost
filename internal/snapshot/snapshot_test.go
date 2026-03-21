package snapshot

import (
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

	// Restore s1 (index 0 = newest in history, which is s1)
	if err := Restore(tmp, 0); err != nil {
		t.Fatal(err)
	}

	restored, _ := Read(tmp)
	if restored.CurrentTask != "first task" {
		t.Errorf("expected first task after restore, got %q", restored.CurrentTask)
	}
}
