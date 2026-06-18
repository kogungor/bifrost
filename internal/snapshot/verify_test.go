package snapshot

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVerifySnapshotV2DetectsStaleCommit(t *testing.T) {
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

	snap := verifyBaseSnapshot()
	if err := EnrichSnapshotV2(root, snap); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("two\n"), 0600); err != nil {
		t.Fatal(err)
	}
	runGitForTest(t, root, "add", "tracked.txt")
	runGitForTest(t, root, "commit", "-m", "second")

	result := VerifySnapshotV2(root, snap, VerifyOptions{Now: snap.CapturedAt.Add(time.Minute)})
	if !hasVerifyCheck(result, "git.commit_match", VerifyWarn) {
		t.Fatalf("expected stale commit warning: %+v", result.Checks)
	}
}

func TestVerifySnapshotV2DetectsChangedAndMissingActiveFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "changed.go"), []byte("one\n"), 0600); err != nil {
		t.Fatal(err)
	}
	snap := verifyBaseSnapshot()
	snap.ActiveFiles = []ActiveFileV2{{Path: "changed.go"}, {Path: "missing.go"}}
	if err := EnrichSnapshotV2(root, snap); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "changed.go"), []byte("two\n"), 0600); err != nil {
		t.Fatal(err)
	}

	result := VerifySnapshotV2(root, snap, VerifyOptions{Now: snap.CapturedAt.Add(time.Minute)})
	if result.Status != VerifyFail {
		t.Fatalf("status = %s, want fail; checks=%+v", result.Status, result.Checks)
	}
	if !hasVerifyCheck(result, "files.active_exist", VerifyFail) {
		t.Fatalf("missing file check not failed: %+v", result.Checks)
	}
	if !hasVerifyCheck(result, "files.active_changed", VerifyWarn) {
		t.Fatalf("changed file check not warned: %+v", result.Checks)
	}
}

func TestVerifySnapshotV2WarnsWhenActiveFileMetadataMissing(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "active.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	snap := verifyBaseSnapshot()
	snap.ActiveFiles = []ActiveFileV2{{Path: "active.go"}}

	result := VerifySnapshotV2(root, snap, VerifyOptions{Now: snap.CapturedAt.Add(time.Minute)})
	if !hasVerifyCheck(result, "files.active_changed", VerifyWarn) {
		t.Fatalf("expected missing metadata warning: %+v", result.Checks)
	}
	if !checkMessageContains(result, "files.active_changed", "No snapshot metadata") {
		t.Fatalf("expected missing metadata message: %+v", result.Checks)
	}
}

func TestVerifySnapshotV2WarnsWhenPreviouslyMissingFileAppears(t *testing.T) {
	root := t.TempDir()
	snap := verifyBaseSnapshot()
	snap.ActiveFiles = []ActiveFileV2{{Path: "new.go"}}
	snap.Observed.Files = []FileObservedV2{{Path: "new.go", Exists: false}}
	if err := os.WriteFile(filepath.Join(root, "new.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}

	result := VerifySnapshotV2(root, snap, VerifyOptions{Now: snap.CapturedAt.Add(time.Minute)})
	if !hasVerifyCheck(result, "files.active_changed", VerifyWarn) {
		t.Fatalf("expected appeared file warning: %+v", result.Checks)
	}
}

func TestVerifySnapshotV2DetectsUnevidencedTestClaim(t *testing.T) {
	root := t.TempDir()
	snap := verifyBaseSnapshot()
	snap.Interpretation.StatusItems = []StatusItemV2{{
		ID:    "status_tests",
		Text:  "Tests pass",
		State: "claimed_done",
	}}

	result := VerifySnapshotV2(root, snap, VerifyOptions{Now: snap.CapturedAt.Add(time.Minute)})
	if !hasVerifyCheck(result, "commands.test_claims", VerifyFail) {
		t.Fatalf("expected test claim failure: %+v", result.Checks)
	}
}

func TestVerifySnapshotV2AcceptsPassingTestEvidence(t *testing.T) {
	root := t.TempDir()
	snap := verifyBaseSnapshot()
	cmd, ev := NewCommandEvidence(ReportedCommand{Command: "go test ./...", ExitCode: 0, Summary: "all tests pass", TestResult: true}, snap.CapturedAt)
	snap.Observed.Commands = []CommandObservedV2{cmd}
	snap.Evidence = []EvidenceV2{ev}
	snap.Interpretation.StatusItems = []StatusItemV2{{
		ID:           "status_tests",
		Text:         "Tests pass",
		State:        "claimed_done",
		EvidenceRefs: []string{ev.ID},
	}}

	result := VerifySnapshotV2(root, snap, VerifyOptions{Now: snap.CapturedAt.Add(time.Minute)})
	if hasVerifyCheck(result, "commands.test_claims", VerifyFail) {
		t.Fatalf("test claim should be verified: %+v", result.Checks)
	}
}

func TestVerifySnapshotV2AcceptsPassingCommandResultForTestCommand(t *testing.T) {
	root := t.TempDir()
	snap := verifyBaseSnapshot()
	cmd, ev := NewCommandEvidence(ReportedCommand{Command: "go test ./...", ExitCode: 0, Summary: "all packages pass"}, snap.CapturedAt)
	snap.Observed.Commands = []CommandObservedV2{cmd}
	snap.Evidence = []EvidenceV2{ev}
	snap.Interpretation.StatusItems = []StatusItemV2{{
		ID:           "status_tests",
		Text:         "Tests pass",
		State:        "claimed_done",
		EvidenceRefs: []string{ev.ID},
	}}

	result := VerifySnapshotV2(root, snap, VerifyOptions{Now: snap.CapturedAt.Add(time.Minute)})
	if hasVerifyCheck(result, "commands.test_claims", VerifyFail) {
		t.Fatalf("go test command_result should verify test claim: %+v", result.Checks)
	}
}

func TestVerifySnapshotV2DetectsHighQuestionsAndRisks(t *testing.T) {
	root := t.TempDir()
	snap := verifyBaseSnapshot()
	snap.Interpretation.OpenQuestions = []OpenQuestionV2{{ID: "q1", Text: "Decide token reuse", Severity: "high"}}
	snap.Interpretation.Risks = []RiskV2{{ID: "r1", Text: "Secret could leak", Severity: "high"}}

	result := VerifySnapshotV2(root, snap, VerifyOptions{Now: snap.CapturedAt.Add(time.Minute)})
	if !hasVerifyCheck(result, "questions.unresolved_high", VerifyFail) || !hasVerifyCheck(result, "risks.unresolved_high", VerifyFail) {
		t.Fatalf("expected high question/risk failures: %+v", result.Checks)
	}
}

func TestVerifySnapshotV2PlanStatusLoader(t *testing.T) {
	root := t.TempDir()
	snap := verifyBaseSnapshot()
	snap.ActivePlan = &ActivePlanRefV2{Name: "draft-plan", Version: "v2"}

	result := VerifySnapshotV2(root, snap, VerifyOptions{
		Now: snap.CapturedAt.Add(time.Minute),
		LoadActivePlan: func(name string) (string, error) {
			return PlanStatusDraft, nil
		},
	})
	if !hasVerifyCheck(result, "plans.status", VerifyWarn) {
		t.Fatalf("expected draft plan warning: %+v", result.Checks)
	}
}

func TestVerifySnapshotV2DetectsSecretLikeValues(t *testing.T) {
	root := t.TempDir()
	snap := verifyBaseSnapshot()
	snap.Evidence = []EvidenceV2{{
		ID:         "ev_secret",
		Type:       EvidenceTypeManualNote,
		Source:     "test",
		ObservedAt: snap.CapturedAt,
		Summary:    "Bearer abcdefghijklmnopqrstuvwxyz123456",
	}}

	result := VerifySnapshotV2(root, snap, VerifyOptions{Now: snap.CapturedAt.Add(time.Minute)})
	if !hasVerifyCheck(result, "security.secrets", VerifyFail) {
		t.Fatalf("expected secret failure: %+v", result.Checks)
	}
}

func verifyBaseSnapshot() *SnapshotV2 {
	now := time.Now().UTC().Truncate(time.Second)
	return &SnapshotV2{
		SchemaVersion: SnapshotSchemaV2,
		ID:            "snap_verify",
		Project:       ProjectRefV2{Name: "verify", Root: "/tmp/verify"},
		CapturedAt:    now,
		Source:        SourceV2{Tool: "test"},
		Session:       SessionStateV2{Task: "verify snapshot"},
		Integrity:     SnapshotIntegrityV2{VerifyStatus: "not_run"},
	}
}

func hasVerifyCheck(result VerifyResult, id, status string) bool {
	for _, check := range result.Checks {
		if check.ID == id && check.Status == status {
			return true
		}
	}
	return false
}

func checkMessageContains(result VerifyResult, id, want string) bool {
	for _, check := range result.Checks {
		if check.ID == id && strings.Contains(check.Message, want) {
			return true
		}
	}
	return false
}
