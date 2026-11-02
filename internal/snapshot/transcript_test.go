package snapshot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// transcriptLines builds Claude Code transcript entries in the shape the
// harness writes: assistant tool_use blocks and user tool_result blocks with a
// toolUseResult payload.
type transcriptBuilder struct {
	root  string
	lines []string
	at    time.Time
}

func newTranscript(root string) *transcriptBuilder {
	return &transcriptBuilder{root: root, at: time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)}
}

func (b *transcriptBuilder) add(entry map[string]any) {
	b.at = b.at.Add(time.Minute)
	entry["timestamp"] = b.at.Format(time.RFC3339)
	if _, ok := entry["cwd"]; !ok {
		entry["cwd"] = b.root
	}
	data, _ := json.Marshal(entry)
	b.lines = append(b.lines, string(data))
}

func (b *transcriptBuilder) toolUse(id, name string, input map[string]any) {
	b.add(map[string]any{"type": "assistant", "message": map[string]any{"role": "assistant",
		"content": []any{map[string]any{"type": "tool_use", "id": id, "name": name, "input": input}}}})
}

func (b *transcriptBuilder) toolResult(id string, isError bool, toolUseResult any) {
	b.add(map[string]any{"type": "user", "toolUseResult": toolUseResult, "message": map[string]any{"role": "user",
		"content": []any{map[string]any{"type": "tool_result", "tool_use_id": id, "is_error": isError, "content": "output"}}}})
}

func (b *transcriptBuilder) write(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(b.lines, "\n")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func bashOK() map[string]any {
	return map[string]any{"stdout": "ok", "stderr": "", "interrupted": false, "isImage": false}
}

func TestReadClaudeTranscript(t *testing.T) {
	root := t.TempDir()
	b := newTranscript(root)
	b.add(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": "fix the auth bug"}})
	b.toolUse("t1", "Bash", map[string]any{"command": "go test ./..."})
	b.toolResult("t1", false, bashOK())
	b.toolUse("t0", "Bash", map[string]any{"command": "go build ./..."})
	b.toolResult("t0", false, bashOK()) // superseded by the later failing run
	b.toolUse("t2", "Bash", map[string]any{"command": "go build ./..."})
	b.toolResult("t2", true, "Error: Exit code 2\n---\nbuild failed")
	b.toolUse("t3", "Bash", map[string]any{"command": "npm run dev", "run_in_background": true})
	b.toolResult("t3", false, map[string]any{"stdout": "", "backgroundTaskId": "x"})
	b.toolUse("t4", "Bash", map[string]any{"command": "sleep 100"})
	b.toolResult("t4", false, map[string]any{"stdout": "", "interrupted": true})
	b.toolUse("t5", "Bash", map[string]any{"command": "rm -rf build"})
	b.toolResult("t5", true, "Permission to use Bash with command rm -rf build has been denied.")
	b.add(map[string]any{"type": "assistant", "cwd": "/elsewhere", "message": map[string]any{"content": []any{
		map[string]any{"type": "tool_use", "id": "t6", "name": "Bash", "input": map[string]any{"command": "make"}}}}})
	b.add(map[string]any{"type": "user", "cwd": "/elsewhere", "toolUseResult": bashOK(), "message": map[string]any{"content": []any{
		map[string]any{"type": "tool_result", "tool_use_id": "t6"}}}})
	b.toolUse("e1", "Edit", map[string]any{"file_path": filepath.Join(root, "src", "auth.go")})
	b.toolResult("e1", false, map[string]any{"filePath": filepath.Join(root, "src", "auth.go")})
	b.toolUse("e2", "Write", map[string]any{"file_path": "/etc/hosts"})
	b.toolResult("e2", false, map[string]any{})
	b.toolUse("e3", "Write", map[string]any{"file_path": "relative.go"})
	b.toolResult("e3", false, map[string]any{})
	b.toolUse("e4", "Edit", map[string]any{"file_path": filepath.Join(root, "denied.go")})
	b.toolResult("e4", true, "The user doesn't want to proceed with this tool use.")
	b.toolUse("t7", "Bash", map[string]any{"command": "grep -r missing ."})
	b.toolResult("t7", false, map[string]any{"stdout": "", "interrupted": false, "returnCodeInterpretation": "No matches found"})
	b.toolUse("t8", "Bash", map[string]any{"command": "npm run build"})
	b.toolResult("t8", false, map[string]any{"stdout": "", "interrupted": false, "backgroundTaskId": "b1", "timedOutAfterMs": 120000})
	b.lines = append(b.lines, `{"type":"assistant","message":{"content":[{"type":"tool_use"`) // still being written

	activity, err := ReadClaudeTranscript(b.write(t), root)
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 11, 2, 10, 1, 0, 0, time.UTC); !activity.SessionStart.Equal(want) {
		t.Errorf("SessionStart = %v, want %v", activity.SessionStart, want)
	}
	if len(activity.Commands) != 2 {
		t.Fatalf("commands = %+v, want go test and go build only", activity.Commands)
	}
	if c := activity.Commands[0]; c.Command != "go test ./..." || c.ExitCode != 0 || !c.TestResult {
		t.Errorf("first command = %+v", c)
	}
	if c := activity.Commands[1]; c.Command != "go build ./..." || c.ExitCode != 2 {
		t.Errorf("second command = %+v", c)
	}
	if len(activity.EditedFiles) != 1 || activity.EditedFiles[0] != "src/auth.go" {
		t.Errorf("edited files = %v, want [src/auth.go]", activity.EditedFiles)
	}
}

func TestCaptureSessionCreatesSnapshotFromTranscript(t *testing.T) {
	root := t.TempDir()
	activity := TranscriptActivity{
		SessionStart: time.Now().UTC().Add(-time.Hour),
		Commands:     []ReportedCommand{{Command: "go test ./...", ExitCode: 0, CapturedAt: time.Now().UTC().Add(-time.Minute), TestResult: true}},
		EditedFiles:  []string{"src/auth.go"},
	}
	result, err := CaptureSession(root, activity, CaptureOptions{SourceTool: "claude-code", Pressure: "high"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Written || !result.Created || result.Commands != 1 || result.Files != 1 {
		t.Fatalf("result = %+v", result)
	}
	snap, err := ReadSnapshotV2(root)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Session.Pressure != "high" || !strings.Contains(snap.Session.Task, "captured automatically") {
		t.Errorf("session = %+v", snap.Session)
	}
	if len(snap.ActiveFiles) != 1 || snap.ActiveFiles[0].Path != "src/auth.go" {
		t.Errorf("active files = %+v", snap.ActiveFiles)
	}
	if !hasCollectedTestEvidence(snap.Evidence) {
		t.Errorf("transcript test result should be collected evidence: %+v", snap.Evidence)
	}
	for _, ev := range snap.Evidence {
		if ev.Type == EvidenceTypeTestResult && ev.Source != EvidenceSourceCollectorTranscript {
			t.Errorf("test evidence source = %q", ev.Source)
		}
	}
	if _, err := Read(root); err != nil {
		t.Errorf("session.md should be written for /handin without MCP: %v", err)
	}
}

func TestCaptureSessionMergesIntoThisSessionsHandoff(t *testing.T) {
	root := t.TempDir()
	start := time.Now().UTC().Add(-time.Hour)
	handoff := &Snapshot{BifrostVersion: CurrentVersion, Timestamp: time.Now().UTC().Add(-10 * time.Minute).Truncate(time.Second),
		SourceTool: "claude-code", Project: "app", CurrentTask: "Implement refresh tokens", NextStep: "Write tests"}
	if err := WriteSnapshotV2(root, SnapshotToV2(root, handoff)); err != nil {
		t.Fatal(err)
	}
	activity := TranscriptActivity{SessionStart: start, Commands: []ReportedCommand{{Command: "go vet ./...", ExitCode: 1, CapturedAt: time.Now().UTC()}}}

	result, err := CaptureSession(root, activity, CaptureOptions{SourceTool: "claude-code"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Created {
		t.Fatal("a handoff written in this session must be merged into, not replaced")
	}
	snap, _ := ReadSnapshotV2(root)
	if snap.Session.Task != "Implement refresh tokens" || snap.Session.NextStep != "Write tests" {
		t.Errorf("handoff interpretation was lost: %+v", snap.Session)
	}
	if len(snap.Observed.Commands) != 1 || snap.Observed.Commands[0].ExitCode != 1 {
		t.Errorf("observed commands = %+v", snap.Observed.Commands)
	}
}

func TestCaptureSessionKeepsAnEarlierHandoff(t *testing.T) {
	// A handoff from another tool must survive a Claude session that only
	// read it (/handin runs `bifrost brief`, which is a command).
	root := t.TempDir()
	old := &Snapshot{BifrostVersion: CurrentVersion, Timestamp: time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second),
		SourceTool: "codex", Project: "app", CurrentTask: "Old task"}
	if err := WriteSnapshotV2(root, SnapshotToV2(root, old)); err != nil {
		t.Fatal(err)
	}
	activity := TranscriptActivity{SessionStart: time.Now().UTC().Add(-time.Hour),
		Commands: []ReportedCommand{{Command: "bifrost brief --mode implement", ExitCode: 0, CapturedAt: time.Now().UTC()}}}

	result, err := CaptureSession(root, activity, CaptureOptions{SourceTool: "claude-code"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Written {
		t.Fatalf("an earlier agent handoff must be left untouched: %+v", result)
	}
	snap, _ := SnapshotFromProject(root)
	if snap.Source.Tool != "codex" || snap.Session.Task != "Old task" || len(snap.Observed.Commands) != 0 {
		t.Errorf("handoff changed: %+v %+v", snap.Session, snap.Observed.Commands)
	}
}

func TestCaptureSessionReplacesAnEarlierAutomaticSnapshot(t *testing.T) {
	root := t.TempDir()
	first := TranscriptActivity{SessionStart: time.Now().UTC().Add(-48 * time.Hour), EditedFiles: []string{"old.go"}}
	if _, err := CaptureSession(root, first, CaptureOptions{SourceTool: "claude-code", Now: time.Now().UTC().Add(-47 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	second := TranscriptActivity{SessionStart: time.Now().UTC().Add(-time.Hour), EditedFiles: []string{"new.go"}}
	result, err := CaptureSession(root, second, CaptureOptions{SourceTool: "claude-code"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Created {
		t.Fatal("a hook-made snapshot from an earlier session should be replaced")
	}
	snap, _ := SnapshotFromProject(root)
	if len(snap.ActiveFiles) != 1 || snap.ActiveFiles[0].Path != "new.go" || snap.Source.Agent != AutoCaptureAgent {
		t.Errorf("snapshot not replaced: %+v", snap.ActiveFiles)
	}
}

func TestCaptureSessionMergeKeepsDriftBaseline(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "auth.go")
	os.WriteFile(path, []byte("package auth\n"), 0600)
	handoff := SnapshotToV2(root, &Snapshot{BifrostVersion: CurrentVersion, Timestamp: time.Now().UTC().Add(-10 * time.Minute).Truncate(time.Second),
		SourceTool: "claude-code", Project: "app", CurrentTask: "Auth", ActiveFiles: []ActiveFile{{Path: "auth.go", Note: "wip"}}})
	if err := EnrichSnapshotV2(root, handoff); err != nil {
		t.Fatal(err)
	}
	if err := WriteSnapshotV2(root, handoff); err != nil {
		t.Fatal(err)
	}
	// The file changes after the /handoff, then the session's capture runs.
	time.Sleep(10 * time.Millisecond)
	os.WriteFile(path, []byte("package auth\n\nfunc changed() {}\n"), 0600)
	activity := TranscriptActivity{SessionStart: time.Now().UTC().Add(-time.Hour),
		Commands: []ReportedCommand{{Command: "go vet ./...", ExitCode: 0, CapturedAt: time.Now().UTC()}}}
	if _, err := CaptureSession(root, activity, CaptureOptions{SourceTool: "claude-code"}); err != nil {
		t.Fatal(err)
	}

	snap, _ := SnapshotFromProject(root)
	result := VerifySnapshotV2(root, snap, VerifyOptions{})
	if !hasVerifyCheck(result, "files.active_changed", VerifyWarn) {
		t.Fatalf("a merge must not hide changes made after the handoff: %+v", result.Checks)
	}
}

func TestCaptureSessionSkipsEmptyActivityAndRedactsCommands(t *testing.T) {
	root := t.TempDir()
	if result, err := CaptureSession(root, TranscriptActivity{}, CaptureOptions{SourceTool: "claude-code"}); err != nil || result.Written {
		t.Fatalf("empty activity should write nothing: %+v %v", result, err)
	}
	if _, err := os.Stat(SnapshotJSONPath(root)); !os.IsNotExist(err) {
		t.Fatal("no snapshot should exist")
	}

	secret := "sk_" + "live_" + strings.Repeat("a1B2c3D4", 4)
	activity := TranscriptActivity{Commands: []ReportedCommand{{Command: "curl -u " + secret + ": https://api.example.com", ExitCode: 0, CapturedAt: time.Now().UTC()}}}
	if _, err := CaptureSession(root, activity, CaptureOptions{SourceTool: "claude-code"}); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{SnapshotJSONPath(root), EvidenceDir(root)} {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				if data, _ := os.ReadFile(path); strings.Contains(string(data), secret) {
					t.Errorf("secret written to %s", path)
				}
			}
			return nil
		})
	}
}

func TestCaptureSessionReplacesEarlierTranscriptResults(t *testing.T) {
	root := t.TempDir()
	start := time.Now().UTC().Add(-time.Hour)
	first := TranscriptActivity{SessionStart: start, Commands: []ReportedCommand{{Command: "go test ./...", ExitCode: 1, CapturedAt: start.Add(10 * time.Minute)}}}
	if _, err := CaptureSession(root, first, CaptureOptions{SourceTool: "claude-code"}); err != nil {
		t.Fatal(err)
	}
	// The second capture reads the whole session again, now with a later passing run.
	second := TranscriptActivity{SessionStart: start, Commands: []ReportedCommand{{Command: "go test ./...", ExitCode: 0, CapturedAt: start.Add(20 * time.Minute)}}}
	if _, err := CaptureSession(root, second, CaptureOptions{SourceTool: "claude-code"}); err != nil {
		t.Fatal(err)
	}
	snap, _ := ReadSnapshotV2(root)
	if len(snap.Observed.Commands) != 1 || snap.Observed.Commands[0].ExitCode != 0 {
		t.Fatalf("superseded run kept: %+v", snap.Observed.Commands)
	}
	transcriptEvidence := 0
	for _, ev := range snap.Evidence {
		if ev.Source == EvidenceSourceCollectorTranscript {
			transcriptEvidence++
		}
	}
	if transcriptEvidence != 1 {
		t.Fatalf("transcript evidence = %d, want 1", transcriptEvidence)
	}
}

func TestSnapshotFromProjectPrefersNewerMarkdownHandoff(t *testing.T) {
	root := t.TempDir()
	if _, err := CaptureSession(root, TranscriptActivity{EditedFiles: []string{"a.go"}},
		CaptureOptions{SourceTool: "claude-code", Now: time.Now().UTC().Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	// An agent without MCP writes only session.md, after the hook's session.json.
	md := &Snapshot{BifrostVersion: CurrentVersion, Timestamp: time.Now().UTC().Truncate(time.Second),
		SourceTool: "opencode", Project: "app", CurrentTask: "Markdown handoff"}
	if err := Write(root, md); err != nil {
		t.Fatal(err)
	}
	snap, err := SnapshotFromProject(root)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Session.Task != "Markdown handoff" {
		t.Fatalf("newer session.md ignored; got task %q", snap.Session.Task)
	}
}

func TestLooksLikeTestCommand(t *testing.T) {
	for command, want := range map[string]bool{
		"go test ./...":                         true,
		"cd api && go test -run TestAuth ./...": true,
		"CI=1 npm test":                         true,
		"npm run test:unit":                     true,
		"pnpm test":                             true,
		"npx vitest run":                        true,
		"python -m pytest -q":                   true,
		"uv run pytest":                         true,
		"cargo test --workspace":                true,
		"make test-integration":                 true,
		"bundle exec rspec":                     true,
		"./gradlew test":                        true,
		"cat auth_test.go":                      false,
		"ls tests/":                             false,
		"go build ./... && echo testing":        false,
		"grep -r TestAuth .":                    false,
		"go run ./cmd/test-server":              false,
	} {
		if got := LooksLikeTestCommand(command); got != want {
			t.Errorf("LooksLikeTestCommand(%q) = %v, want %v", command, got, want)
		}
	}
}
