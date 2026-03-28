package cli_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kogungor/bifrost/internal/snapshot"
)

// bifrost builds and returns the path to the bifrost binary.
// The binary is built once per test run and cached.
var binaryPath string

func TestMain(m *testing.M) {
	// Build bifrost binary into a temp dir
	tmp, err := os.MkdirTemp("", "bifrost-test-*")
	if err != nil {
		panic(err)
	}
	binaryPath = filepath.Join(tmp, "bifrost")
	cmd := exec.Command("go", "build", "-o", binaryPath, "../../cmd/bifrost")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("failed to build bifrost: " + err.Error())
	}

	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}

func runBifrost(t *testing.T, projectDir string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = projectDir
	cmd.Env = append(os.Environ(),
		"HOME="+projectDir, // isolate home directory
		"NO_COLOR=1",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}
	return stdout.String(), stderr.String(), exitCode
}

// --- 8.1 Integration test: install on fresh temp home ---

func TestInstallFreshHome(t *testing.T) {
	home := t.TempDir()

	// Create .claude dir so adapter detects as installed
	os.MkdirAll(filepath.Join(home, ".claude"), 0755)

	out, _, code := runBifrost(t, home, "install", "--adapter", "claude-code")
	if code != 0 {
		t.Fatalf("install failed (exit %d): %s", code, out)
	}

	if !strings.Contains(out, "commands registered") {
		t.Errorf("expected 'commands registered' in output, got:\n%s", out)
	}

	// Verify command files exist
	for _, name := range []string{"handoff.md", "handin.md"} {
		path := filepath.Join(home, ".claude", "commands", name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected %s to exist", path)
		}
	}
}

func TestInstallDryRun(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".claude"), 0755)

	out, _, code := runBifrost(t, home, "install", "--adapter", "claude-code", "--dry-run")
	if code != 0 {
		t.Fatalf("install --dry-run failed (exit %d): %s", code, out)
	}

	if !strings.Contains(out, "would install") {
		t.Errorf("expected dry-run message, got:\n%s", out)
	}

	// Verify no files were actually created
	path := filepath.Join(home, ".claude", "commands", "handoff.md")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("dry-run should not create files")
	}
}

func TestInstallMCPFlag(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".claude"), 0755)

	out, _, code := runBifrost(t, home, "install", "--adapter", "claude-code", "--mcp")
	if code != 0 {
		t.Fatalf("install --mcp failed (exit %d): %s", code, out)
	}

	if !strings.Contains(out, "MCP server registered") {
		t.Errorf("expected 'MCP server registered' in output, got:\n%s", out)
	}

	mcpPath := filepath.Join(home, ".claude", "mcp.json")
	data, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("mcp.json not created: %v", err)
	}
	if !strings.Contains(string(data), "bifrost") {
		t.Errorf("mcp.json missing bifrost entry: %s", string(data))
	}
	if !strings.Contains(string(data), "mcp-serve") {
		t.Errorf("mcp.json missing mcp-serve arg: %s", string(data))
	}
}

// --- 8.2 Integration test: doctor healthy + unhealthy states ---

func TestDoctorHealthy(t *testing.T) {
	home := t.TempDir()

	// Set up a healthy state: .claude with commands, .git, .gitignore, snapshot
	os.MkdirAll(filepath.Join(home, ".claude", "commands"), 0755)
	os.WriteFile(filepath.Join(home, ".claude", "commands", "handoff.md"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(home, ".claude", "commands", "handin.md"), []byte("test"), 0644)
	// Write MCP config with bifrost entry
	os.MkdirAll(filepath.Join(home, ".claude"), 0755)
	os.WriteFile(filepath.Join(home, ".claude", "mcp.json"), []byte(`{"mcpServers":{"bifrost":{"command":"bifrost","args":["mcp-serve"]}}}`), 0644)
	os.MkdirAll(filepath.Join(home, ".git"), 0755)
	os.WriteFile(filepath.Join(home, ".gitignore"), []byte(".bifrost/\n"), 0644)

	// Write a snapshot
	snap := &snapshot.Snapshot{
		BifrostVersion: snapshot.CurrentVersion,
		Timestamp:      time.Now().UTC().Truncate(time.Second),
		SourceTool:     "claude-code",
		Project:        "test",
		TokenPressure:  "low",
		CurrentTask:    "testing",
		NextStep:       "continue",
	}
	snapshot.Write(home, snap)

	// Write BIFROST.md
	os.WriteFile(filepath.Join(home, "BIFROST.md"), []byte("---\nproject: test\n---\n"), 0644)

	out, _, code := runBifrost(t, home, "doctor", "--project", home)
	if code != 0 {
		t.Fatalf("doctor failed (exit %d): %s", code, out)
	}

	if !strings.Contains(out, "All checks passed") {
		t.Errorf("expected 'All checks passed', got:\n%s", out)
	}
}

func TestDoctorUnhealthy(t *testing.T) {
	home := t.TempDir()

	// Set up unhealthy: .claude exists but no command files, no snapshot, no gitignore
	os.MkdirAll(filepath.Join(home, ".claude"), 0755)
	os.MkdirAll(filepath.Join(home, ".git"), 0755)
	os.WriteFile(filepath.Join(home, ".gitignore"), []byte("node_modules/\n"), 0644)

	out, _, _ := runBifrost(t, home, "doctor", "--project", home)

	if !strings.Contains(out, "missing") {
		t.Errorf("expected 'missing' for unregistered commands, got:\n%s", out)
	}
	if !strings.Contains(out, "issue(s) found") {
		t.Errorf("expected issue count, got:\n%s", out)
	}
}

func TestDoctorFixRegistersCommands(t *testing.T) {
	home := t.TempDir()

	// .claude exists but no commands registered, .git present, gitignore missing .bifrost/
	os.MkdirAll(filepath.Join(home, ".claude"), 0755)
	os.MkdirAll(filepath.Join(home, ".git"), 0755)
	os.WriteFile(filepath.Join(home, ".gitignore"), []byte("node_modules/\n"), 0644)

	out, _, code := runBifrost(t, home, "doctor", "--fix", "--project", home)
	if code != 0 {
		t.Fatalf("doctor --fix failed (exit %d): %s", code, out)
	}

	// Commands should now be registered
	handoff := filepath.Join(home, ".claude", "commands", "handoff.md")
	handin := filepath.Join(home, ".claude", "commands", "handin.md")
	if _, err := os.Stat(handoff); err != nil {
		t.Errorf("handoff.md not created by --fix: %v", err)
	}
	if _, err := os.Stat(handin); err != nil {
		t.Errorf("handin.md not created by --fix: %v", err)
	}

	if !strings.Contains(out, "[fixed]") {
		t.Errorf("expected '[fixed]' in output, got:\n%s", out)
	}
}

func TestDoctorFixGitignore(t *testing.T) {
	home := t.TempDir()

	// Healthy commands, but .bifrost/ not in gitignore
	os.MkdirAll(filepath.Join(home, ".claude", "commands"), 0755)
	os.WriteFile(filepath.Join(home, ".claude", "commands", "handoff.md"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(home, ".claude", "commands", "handin.md"), []byte("test"), 0644)
	os.MkdirAll(filepath.Join(home, ".git"), 0755)
	os.WriteFile(filepath.Join(home, ".gitignore"), []byte("node_modules/\n"), 0644)

	out, _, code := runBifrost(t, home, "doctor", "--fix", "--project", home)
	if code != 0 {
		t.Fatalf("doctor --fix failed (exit %d): %s", code, out)
	}

	// .gitignore should now contain .bifrost/
	data, err := os.ReadFile(filepath.Join(home, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), ".bifrost/") {
		t.Errorf(".gitignore should contain .bifrost/ after --fix, got:\n%s", string(data))
	}

	if !strings.Contains(out, "[fixed]") {
		t.Errorf("expected '[fixed]' in output, got:\n%s", out)
	}
}

func TestDoctorFixCreatesBifrostMd(t *testing.T) {
	home := t.TempDir()

	// Healthy commands and gitignore, but no BIFROST.md
	os.MkdirAll(filepath.Join(home, ".claude", "commands"), 0755)
	os.WriteFile(filepath.Join(home, ".claude", "commands", "handoff.md"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(home, ".claude", "commands", "handin.md"), []byte("test"), 0644)
	os.MkdirAll(filepath.Join(home, ".git"), 0755)
	os.WriteFile(filepath.Join(home, ".gitignore"), []byte(".bifrost/\n"), 0644)

	out, _, code := runBifrost(t, home, "doctor", "--fix", "--project", home)
	if code != 0 {
		t.Fatalf("doctor --fix failed (exit %d): %s", code, out)
	}

	if _, err := os.Stat(filepath.Join(home, "BIFROST.md")); err != nil {
		t.Errorf("BIFROST.md not created by --fix: %v", err)
	}

	if !strings.Contains(out, "[fixed]") {
		t.Errorf("expected '[fixed]' in output, got:\n%s", out)
	}
}

// --- 8.3 Integration test: status with/without snapshot ---

func TestStatusNoSnapshot(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".git"), 0755)

	out, _, code := runBifrost(t, home, "status", "--project", home)
	if code != 0 {
		t.Fatalf("status failed (exit %d): %s", code, out)
	}

	if !strings.Contains(out, "none") || !strings.Contains(out, "No snapshot found") {
		t.Errorf("expected 'No snapshot found' message, got:\n%s", out)
	}
}

func TestStatusWithSnapshot(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".git"), 0755)
	os.WriteFile(filepath.Join(home, "BIFROST.md"), []byte("---\nproject: test\n---\n"), 0644)

	snap := &snapshot.Snapshot{
		BifrostVersion: snapshot.CurrentVersion,
		Timestamp:      time.Now().UTC().Truncate(time.Second),
		SourceTool:     "claude-code",
		Project:        "test",
		TokenPressure:  "medium",
		CurrentTask:    "integration testing",
		NextStep:       "verify output",
	}
	snapshot.Write(home, snap)

	out, _, code := runBifrost(t, home, "status", "--project", home)
	if code != 0 {
		t.Fatalf("status failed (exit %d): %s", code, out)
	}

	if !strings.Contains(out, "session.md") {
		t.Errorf("expected session.md in output, got:\n%s", out)
	}
	if !strings.Contains(out, "claude-code") {
		t.Errorf("expected source tool in output, got:\n%s", out)
	}
}

func TestStatusShowsSemanticFields(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".git"), 0755)

	snap := &snapshot.Snapshot{
		BifrostVersion: snapshot.CurrentVersion,
		Timestamp:      time.Now().UTC().Truncate(time.Second),
		SourceTool:     "claude-code",
		Project:        "test",
		TokenPressure:  "high",
		SessionIntent:  "implementing",
		ActivePlanName: "auth-refactor",
		OpenQuestions:  []string{"- Q1?", "- Q2?"},
		CurrentTask:    "build auth",
		NextStep:       "write tests",
	}
	snapshot.Write(home, snap)

	out, _, code := runBifrost(t, home, "status", "--project", home)
	if code != 0 {
		t.Fatalf("status failed (exit %d): %s", code, out)
	}

	for _, want := range []string{"implementing", "auth-refactor", "2 unresolved"} {
		if !strings.Contains(out, want) {
			t.Errorf("status output missing %q\ngot:\n%s", want, out)
		}
	}
}

func TestStatusWithHandoffNote(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".git"), 0755)

	snap := &snapshot.Snapshot{
		BifrostVersion: 1,
		Timestamp:      time.Now().UTC().Truncate(time.Second),
		SourceTool:     "opencode",
		Project:        "test",
		TokenPressure:  "low",
		CurrentTask:    "testing notes",
		NextStep:       "verify",
	}
	snapshot.Write(home, snap)

	note := &snapshot.HandoffNote{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		From:      "opencode",
		Text:      "auth module half done",
	}
	snapshot.WriteNote(home, note)

	out, _, code := runBifrost(t, home, "status", "--project", home)
	if code != 0 {
		t.Fatalf("status failed (exit %d): %s", code, out)
	}

	if !strings.Contains(out, "handoff.md") {
		t.Errorf("expected handoff.md reference in output, got:\n%s", out)
	}
}

// --- 8.4 Integration test: history + restore round-trip ---

func TestHistoryEmpty(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".git"), 0755)

	out, _, code := runBifrost(t, home, "history", "--project", home)
	if code != 0 {
		t.Fatalf("history failed (exit %d): %s", code, out)
	}

	if !strings.Contains(out, "No archived snapshots") {
		t.Errorf("expected empty history message, got:\n%s", out)
	}
}

func TestHistoryWithEntries(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".git"), 0755)
	base := time.Now().UTC().Truncate(time.Second).Add(-2 * time.Hour)

	// Write two snapshots (first gets archived when second is written)
	s1 := &snapshot.Snapshot{
		BifrostVersion: 1,
		Timestamp:      base,
		SourceTool:     "claude-code",
		Project:        "test",
		TokenPressure:  "low",
		CurrentTask:    "first task",
		NextStep:       "next",
	}
	snapshot.Write(home, s1)

	s2 := &snapshot.Snapshot{
		BifrostVersion: 1,
		Timestamp:      base.Add(time.Hour),
		SourceTool:     "opencode",
		Project:        "test",
		TokenPressure:  "medium",
		CurrentTask:    "second task",
		NextStep:       "continue",
	}
	snapshot.Write(home, s2)

	out, _, code := runBifrost(t, home, "history", "--project", home)
	if code != 0 {
		t.Fatalf("history failed (exit %d): %s", code, out)
	}

	if !strings.Contains(out, "first task") {
		t.Errorf("expected 'first task' in history, got:\n%s", out)
	}
	if !strings.Contains(out, "claude-code") {
		t.Errorf("expected 'claude-code' in history, got:\n%s", out)
	}
}

func TestHistoryRestoreRoundTrip(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".git"), 0755)
	base := time.Now().UTC().Truncate(time.Second).Add(-2 * time.Hour)

	// Create two snapshots
	s1 := &snapshot.Snapshot{
		BifrostVersion: 1,
		Timestamp:      base,
		SourceTool:     "claude-code",
		Project:        "test",
		TokenPressure:  "low",
		CurrentTask:    "original task",
		NextStep:       "original next",
	}
	snapshot.Write(home, s1)

	s2 := &snapshot.Snapshot{
		BifrostVersion: 1,
		Timestamp:      base.Add(time.Hour),
		SourceTool:     "opencode",
		Project:        "test",
		TokenPressure:  "high",
		CurrentTask:    "newer task",
		NextStep:       "newer next",
	}
	snapshot.Write(home, s2)

	// Current should be s2
	current, _ := snapshot.Read(home)
	if current.CurrentTask != "newer task" {
		t.Fatalf("expected current task 'newer task', got %q", current.CurrentTask)
	}

	// Restore snapshot #1 (the archived one)
	out, _, code := runBifrost(t, home, "restore", "1", "--project", home)
	if code != 0 {
		t.Fatalf("restore failed (exit %d): %s", code, out)
	}

	if !strings.Contains(out, "Snapshot restored") {
		t.Errorf("expected 'Snapshot restored' in output, got:\n%s", out)
	}

	// Verify restored snapshot is now active
	restored, err := snapshot.Read(home)
	if err != nil {
		t.Fatalf("could not read restored snapshot: %v", err)
	}
	if restored.CurrentTask != "original task" {
		t.Errorf("expected restored task 'original task', got %q", restored.CurrentTask)
	}
}

func TestRestoreInvalidNumber(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".git"), 0755)
	base := time.Now().UTC().Truncate(time.Second).Add(-time.Hour)

	// Need at least one archived snapshot for restore to check the number
	s1 := &snapshot.Snapshot{
		BifrostVersion: 1, Timestamp: base, SourceTool: "test",
		Project: "test", TokenPressure: "low", CurrentTask: "task1", NextStep: "next",
	}
	snapshot.Write(home, s1)
	s2 := &snapshot.Snapshot{
		BifrostVersion: 1, Timestamp: base.Add(30 * time.Minute), SourceTool: "test",
		Project: "test", TokenPressure: "low", CurrentTask: "task2", NextStep: "next",
	}
	snapshot.Write(home, s2)

	_, _, code := runBifrost(t, home, "restore", "abc", "--project", home)
	if code == 0 {
		t.Error("expected non-zero exit for invalid restore number")
	}
}

// --- Export command tests ---

func TestExportSnapshotJSON(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".git"), 0755)

	snap := &snapshot.Snapshot{
		BifrostVersion: snapshot.CurrentVersion,
		Timestamp:      time.Now().UTC().Truncate(time.Second),
		SourceTool:     "claude-code",
		Project:        "test",
		TokenPressure:  "medium",
		SessionIntent:  "implementing",
		ActivePlanName: "auth-refactor",
		CurrentTask:    "Build JWT refresh",
		NextStep:       "Write tests",
		Assumptions:    []string{"- Redis is running"},
		OpenQuestions:  []string{"- Single-use tokens?"},
		Risks:          []string{"- Revocation not done"},
		ActiveFiles: []snapshot.ActiveFile{
			{Path: "src/auth.ts", Note: "stub written", Confidence: "medium"},
		},
	}
	if err := snapshot.Write(home, snap); err != nil {
		t.Fatal(err)
	}

	stdout, _, code := runBifrost(t, home, "export", "--format", "snapshot", "--project", home)
	if code != 0 {
		t.Fatalf("export exited %d", code)
	}

	for _, want := range []string{
		`"session_intent": "implementing"`,
		`"active_plan_name": "auth-refactor"`,
		`"confidence": "medium"`,
		`"assumptions"`,
		`"open_questions"`,
		`"risks"`,
		`"found": true`,
		`"age_seconds"`,
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("export output missing %q", want)
		}
	}
}

func TestExportPlansJSON(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".git"), 0755)

	plan := &snapshot.Plan{
		BifrostVersion: 1,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
		SourceTool:     "claude-code",
		Project:        "test",
		Status:         "active",
		Title:          "Auth Refactor",
		Goal:           "Rotate JWT tokens",
		Steps: []snapshot.PlanStep{
			{Description: "Set up Redis", Status: "done"},
			{Description: "Write refresh logic", Status: "pending"},
		},
	}
	if err := snapshot.WritePlan(home, "auth-refactor", plan); err != nil {
		t.Fatal(err)
	}

	stdout, _, code := runBifrost(t, home, "export", "--format", "plans", "--project", home)
	if code != 0 {
		t.Fatalf("export plans exited %d", code)
	}

	for _, want := range []string{
		`"plans"`,
		`"Auth Refactor"`,
		`"completion_pct"`,
		`"id"`,
		`"steps_done"`,
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("export plans output missing %q", want)
		}
	}
}

func TestExportNoSnapshot(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".git"), 0755)

	// No snapshot written — should warn but exit 0
	_, _, code := runBifrost(t, home, "export", "--format", "snapshot", "--project", home)
	if code != 0 {
		t.Errorf("export with no snapshot should exit 0, got %d", code)
	}
}

func TestExportInvalidFormat(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".git"), 0755)

	_, _, code := runBifrost(t, home, "export", "--format", "invalid", "--project", home)
	if code == 0 {
		t.Error("export with invalid format should exit non-zero")
	}
}

func TestStatusSnapshotSizeNormal(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".git"), 0755)

	snap := &snapshot.Snapshot{
		BifrostVersion: snapshot.CurrentVersion,
		Timestamp:      time.Now().UTC().Truncate(time.Second),
		SourceTool:     "claude-code",
		Project:        "test",
		TokenPressure:  "low",
		CurrentTask:    "small task",
		NextStep:       "next",
	}
	snapshot.Write(home, snap)

	out, _, code := runBifrost(t, home, "status", "--project", home)
	if code != 0 {
		t.Fatalf("status failed (exit %d): %s", code, out)
	}

	if !strings.Contains(out, "KB") {
		t.Errorf("expected size in KB in output, got:\n%s", out)
	}
}

func TestStatusSnapshotSizeWarning(t *testing.T) {
	home := t.TempDir()
	os.MkdirAll(filepath.Join(home, ".git"), 0755)

	// Build a snapshot large enough to trigger the >10KB warning
	decisions := make([]string, 200)
	for i := range decisions {
		decisions[i] = strings.Repeat("x", 60)
	}
	snap := &snapshot.Snapshot{
		BifrostVersion: snapshot.CurrentVersion,
		Timestamp:      time.Now().UTC().Truncate(time.Second),
		SourceTool:     "claude-code",
		Project:        "test",
		TokenPressure:  "high",
		CurrentTask:    "large task",
		NextStep:       "next",
		Decisions:      decisions,
	}
	snapshot.Write(home, snap)

	out, _, code := runBifrost(t, home, "status", "--project", home)
	if code != 0 {
		t.Fatalf("status failed (exit %d): %s", code, out)
	}

	if !strings.Contains(out, "large snapshot") {
		t.Errorf("expected large snapshot warning in output, got:\n%s", out)
	}
}
