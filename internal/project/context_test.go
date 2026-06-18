package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kogungor/bifrost/internal/snapshot"
)

func TestAnalyzeContextFindsCandidatesAndPlaceholders(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n")
	writeFile(t, filepath.Join(root, "BIFROST.md"), `# Project: app

## Stack
- [Runtime, framework, database]

## Commands
- existing command
`)
	snap := &snapshot.SnapshotV2{
		ID: "snap_001",
		Interpretation: snapshot.InterpretationV2{
			Decisions: []snapshot.DecisionV2{{ID: "dec_001", Text: "Use PostgreSQL for metadata"}},
		},
	}

	report, err := AnalyzeContext(root, snap)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Exists {
		t.Fatal("expected BIFROST.md to exist")
	}
	if !contains(report.Placeholders, "Stack") {
		t.Fatalf("expected Stack placeholder, got %#v", report.Placeholders)
	}
	if !hasCandidate(report.Candidates, "command", "go test ./...") {
		t.Fatalf("expected go test candidate: %#v", report.Candidates)
	}
	if !hasCandidate(report.Candidates, "decision", "Use PostgreSQL for metadata") {
		t.Fatalf("expected decision candidate: %#v", report.Candidates)
	}
}

func TestAnalyzeContextReportsContradictoryPackageManager(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n")
	writeFile(t, filepath.Join(root, "BIFROST.md"), `# Project: app

## Stack
- Package manager: npm
`)

	report, err := AnalyzeContext(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Contradictions) != 1 || !strings.Contains(report.Contradictions[0], "npm") {
		t.Fatalf("expected npm contradiction, got %#v", report.Contradictions)
	}
}

func TestApplyContextCandidatesRequiresSelectedIDs(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n")
	report, err := AnalyzeContext(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyContextCandidates(root, report, []string{"all"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "BIFROST.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "Package manager: go") || !strings.Contains(string(data), "go test ./...") {
		t.Fatalf("BIFROST.md missing applied candidates:\n%s", string(data))
	}
	if !strings.Contains(string(data), "detected from project files") {
		t.Fatalf("applied candidate should include source reason:\n%s", string(data))
	}
}

func TestApplyContextCandidatesRejectsUnknownExplicitID(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n")
	report, err := AnalyzeContext(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyContextCandidates(root, report, []string{"prom_missing"}); err == nil {
		t.Fatal("expected unknown explicit accept id to fail")
	}
	if err := ApplyContextCandidates(root, report, []string{report.Candidates[0].ID, "prom_missing"}); err == nil {
		t.Fatal("expected partially unknown explicit accept id to fail")
	}
}

func TestApplyContextCandidatesDoesNotMatchSectionPrefix(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n")
	writeFile(t, filepath.Join(root, "BIFROST.md"), "# Project\n\n## Commands Old\n- legacy\n")
	report, err := AnalyzeContext(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyContextCandidates(root, report, []string{"all"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "BIFROST.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "## Commands\n- go test ./...") {
		t.Fatalf("expected exact Commands section to be created:\n%s", string(data))
	}
}

func TestApplyContextCandidatesRedactsSecrets(t *testing.T) {
	root := t.TempDir()
	rawSecret := "sk-proj-abcdefghijklmnopqrstuvwxyz123456"
	snap := &snapshot.SnapshotV2{
		ID: "snap_001",
		Interpretation: snapshot.InterpretationV2{
			Decisions: []snapshot.DecisionV2{{ID: "dec_001", Text: "Use API key " + rawSecret}},
		},
	}
	report, err := AnalyzeContext(root, snap)
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyContextCandidates(root, report, []string{"all"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "BIFROST.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), rawSecret) {
		t.Fatalf("BIFROST.md leaked raw secret:\n%s", string(data))
	}
	if !strings.Contains(string(data), "[REDACTED:openai_key]") {
		t.Fatalf("BIFROST.md missing redaction marker:\n%s", string(data))
	}
}

func TestApplyContextCandidatesStrictSecretFailsWithoutWriting(t *testing.T) {
	root := t.TempDir()
	rawSecret := "sk-proj-abcdefghijklmnopqrstuvwxyz123456"
	writeFile(t, filepath.Join(root, ".bifrost", "config.json"), `{"security":{"strict":true}}`)
	snap := &snapshot.SnapshotV2{
		ID: "snap_001",
		Interpretation: snapshot.InterpretationV2{
			Decisions: []snapshot.DecisionV2{{ID: "dec_001", Text: "Use API key " + rawSecret}},
		},
	}
	report, err := AnalyzeContext(root, snap)
	if err != nil {
		t.Fatal(err)
	}
	err = ApplyContextCandidates(root, report, []string{"all"})
	if err == nil {
		t.Fatal("expected strict security mode to reject secret-like promotion")
	}
	if _, statErr := os.Stat(filepath.Join(root, "BIFROST.md")); !os.IsNotExist(statErr) {
		t.Fatalf("BIFROST.md should not be written in strict mode, stat error=%v", statErr)
	}
}

func TestIgnorePromotionIDsFiltersCandidates(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/app\n")
	report, err := AnalyzeContext(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Candidates) == 0 {
		t.Fatal("expected candidates")
	}
	ignored := report.Candidates[0].ID
	if err := IgnorePromotionIDs(root, []string{ignored}); err != nil {
		t.Fatal(err)
	}
	report, err = AnalyzeContext(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range report.Candidates {
		if candidate.ID == ignored {
			t.Fatalf("ignored candidate still present: %+v", candidate)
		}
	}
	if !contains(report.Ignored, ignored) {
		t.Fatalf("ignored id not reported: %#v", report.Ignored)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func hasCandidate(candidates []ContextCandidate, kind, text string) bool {
	for _, candidate := range candidates {
		if candidate.Type == kind && candidate.Text == text {
			return true
		}
	}
	return false
}

func contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}
