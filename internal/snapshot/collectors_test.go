package snapshot

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestParseGitPorcelain(t *testing.T) {
	changed, staged, untracked := parseGitPorcelain(" M changed.go\nA  staged.go\n?? new.go\nR  old.go -> renamed.go\n")
	if got, want := changed, []string{"changed.go", "renamed.go", "staged.go"}; !sameStrings(got, want) {
		t.Fatalf("changed = %#v, want %#v", got, want)
	}
	if got, want := staged, []string{"renamed.go", "staged.go"}; !sameStrings(got, want) {
		t.Fatalf("staged = %#v, want %#v", got, want)
	}
	if got, want := untracked, []string{"new.go"}; !sameStrings(got, want) {
		t.Fatalf("untracked = %#v, want %#v", got, want)
	}
}

func TestEnrichSnapshotV2CollectsFileProjectAndClaimEvidence(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/bifrost\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "BIFROST.md"), []byte("# Project Context\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "internal"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "file.go"), []byte("package internal\n"), 0600); err != nil {
		t.Fatal(err)
	}

	snap := validSnapshotV2()
	snap.Project.Root = ""
	snap.ActiveFiles = []ActiveFileV2{{Path: "internal/file.go", Note: "active"}}
	snap.Observed = ObservedV2{}
	snap.Evidence = nil
	snap.Interpretation.StatusItems = []StatusItemV2{{ID: "status_001", Text: "Implemented file", State: "claimed_done"}}

	if err := EnrichSnapshotV2(root, snap); err != nil {
		t.Fatal(err)
	}
	if snap.Project.Root != root {
		t.Fatalf("Project.Root = %q, want %q", snap.Project.Root, root)
	}
	if len(snap.Observed.Files) != 1 || !snap.Observed.Files[0].Exists || snap.Observed.Files[0].SHA256 == "" {
		t.Fatalf("file metadata not collected: %+v", snap.Observed.Files)
	}
	if snap.Observed.Project == nil || !snap.Observed.Project.BifrostMDExists {
		t.Fatalf("project metadata not collected: %+v", snap.Observed.Project)
	}
	if !sameStrings(snap.Observed.Project.PackageManagerCandidates, []string{"go"}) {
		t.Fatalf("package manager candidates = %#v", snap.Observed.Project.PackageManagerCandidates)
	}
	if !containsString(snap.Observed.Project.CommandCandidates, "go test ./...") {
		t.Fatalf("command candidates = %#v", snap.Observed.Project.CommandCandidates)
	}
	if len(snap.ActiveFiles[0].EvidenceRefs) == 0 {
		t.Fatalf("active file evidence refs not attached: %+v", snap.ActiveFiles[0])
	}
	if snap.Interpretation.StatusItems[0].Verification == nil || snap.Interpretation.StatusItems[0].Verification.State != "unverified" {
		t.Fatalf("model claim verification not marked unverified: %+v", snap.Interpretation.StatusItems[0])
	}
	if !hasEvidenceType(snap.Evidence, EvidenceTypeFileMetadata) || !hasEvidenceType(snap.Evidence, EvidenceTypeProjectMetadata) || !hasEvidenceType(snap.Evidence, EvidenceTypeModelClaim) {
		t.Fatalf("expected file/project/model evidence, got %+v", snap.Evidence)
	}
}

func TestEnrichSnapshotV2WithReportedCommandAndManualEvidence(t *testing.T) {
	root := t.TempDir()
	snap := validSnapshotV2()
	snap.Observed = ObservedV2{}
	snap.ActiveFiles = nil
	snap.Evidence = nil

	if err := EnrichSnapshotV2WithOptions(root, snap, EnrichOptions{
		ReportedCommands: []ReportedCommand{{
			Command:    "go test ./...",
			ExitCode:   1,
			Summary:    "one failing package",
			TestResult: true,
		}},
		ManualEvidence: []ManualEvidence{{
			Text:   "User confirmed Redis is not required.",
			Source: "user",
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if len(snap.Observed.Commands) != 1 || snap.Observed.Commands[0].Command != "go test ./..." {
		t.Fatalf("reported command not observed: %+v", snap.Observed.Commands)
	}
	if !hasEvidenceType(snap.Evidence, EvidenceTypeTestResult) || !hasEvidenceType(snap.Evidence, EvidenceTypeManualNote) {
		t.Fatalf("expected test/manual evidence, got %+v", snap.Evidence)
	}
}

func TestCollectGitEvidenceFromTempRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	runGitForTest(t, root, "init")
	runGitForTest(t, root, "config", "user.email", "test@example.com")
	runGitForTest(t, root, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("one\n"), 0600); err != nil {
		t.Fatal(err)
	}
	runGitForTest(t, root, "add", "tracked.txt")
	runGitForTest(t, root, "commit", "-m", "initial")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("two\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "staged.txt"), []byte("staged\n"), 0600); err != nil {
		t.Fatal(err)
	}
	runGitForTest(t, root, "add", "staged.txt")
	if err := os.WriteFile(filepath.Join(root, "untracked.txt"), []byte("new\n"), 0600); err != nil {
		t.Fatal(err)
	}

	git, ev, err := collectGitEvidence(root, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if git.Branch == "" || git.Commit == "" || !git.Dirty {
		t.Fatalf("unexpected git observation: %+v", git)
	}
	if !containsString(git.ChangedFiles, "tracked.txt") || !containsString(git.StagedFiles, "staged.txt") || !containsString(git.UntrackedFiles, "untracked.txt") {
		t.Fatalf("git status not parsed as expected: %+v", git)
	}
	if ev.Type != EvidenceTypeGitStatus || ev.Source != "collector.git" {
		t.Fatalf("unexpected git evidence: %+v", ev)
	}
}

func TestEnrichSnapshotV2CollectsDiffSummaryEvidence(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	runGitForTest(t, root, "init")
	runGitForTest(t, root, "config", "user.email", "test@example.com")
	runGitForTest(t, root, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("one\n"), 0600); err != nil {
		t.Fatal(err)
	}
	runGitForTest(t, root, "add", "tracked.txt")
	runGitForTest(t, root, "commit", "-m", "initial")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("two\n"), 0600); err != nil {
		t.Fatal(err)
	}

	snap := validSnapshotV2()
	snap.ActiveFiles = nil
	snap.Evidence = nil
	if err := EnrichSnapshotV2(root, snap); err != nil {
		t.Fatal(err)
	}
	if !hasEvidenceType(snap.Evidence, EvidenceTypeDiffSummary) {
		t.Fatalf("expected diff summary evidence, got %+v", snap.Evidence)
	}
}

func TestEvidenceV2StoreRoundTrip(t *testing.T) {
	root := t.TempDir()
	ev := EvidenceV2{
		ID:         "ev_test_001",
		Type:       EvidenceTypeProjectMetadata,
		Source:     "collector.project",
		ObservedAt: time.Now().UTC(),
		Summary:    "test evidence",
		Data:       map[string]any{"ok": true},
	}
	if err := WriteEvidenceV2(root, ev); err != nil {
		t.Fatal(err)
	}
	read, err := ReadEvidenceV2(root, ev.ID)
	if err != nil {
		t.Fatal(err)
	}
	if read.ID != ev.ID || read.Type != ev.Type {
		t.Fatalf("read evidence = %+v, want %+v", read, ev)
	}
	list, err := ListEvidenceV2(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != ev.ID {
		t.Fatalf("list evidence = %+v", list)
	}
}

func runGitForTest(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func hasEvidenceType(evidence []EvidenceV2, typ string) bool {
	for _, ev := range evidence {
		if ev.Type == typ {
			return true
		}
	}
	return false
}
