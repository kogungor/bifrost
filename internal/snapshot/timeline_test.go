package snapshot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppendTimelineEventRedactsSecrets(t *testing.T) {
	root := t.TempDir()
	raw := "sk-proj-abcdefghijklmnopqrstuvwxyz123456"
	if err := AppendTimelineEvent(root, TimelineEvent{
		Type: "snapshot.write",
		Task: "Use " + raw,
	}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(TimelinePath(root))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), raw) {
		t.Fatalf("timeline leaked raw secret:\n%s", string(data))
	}
	if !strings.Contains(string(data), "[REDACTED:openai_key]") {
		t.Fatalf("timeline missing redaction marker:\n%s", string(data))
	}
}

func TestDiffSnapshotsTracksKeyChanges(t *testing.T) {
	before := &SnapshotV2{
		ID:      "snap_before",
		Session: SessionStateV2{Task: "Implement auth", NextStep: "Write code"},
		Interpretation: InterpretationV2{
			Risks:         []RiskV2{{Text: "Old risk"}},
			OpenQuestions: []OpenQuestionV2{{Text: "Old question"}},
		},
		ActiveFiles: []ActiveFileV2{{
			Path:  "src/auth.go",
			Trust: TrustV2{Tests: "low"},
		}},
	}
	after := &SnapshotV2{
		ID:      "snap_after",
		Session: SessionStateV2{Task: "Fix tests", NextStep: "Run tests"},
		Interpretation: InterpretationV2{
			Risks:         []RiskV2{{Text: "New risk"}},
			OpenQuestions: []OpenQuestionV2{{Text: "New question"}},
		},
		ActiveFiles: []ActiveFileV2{{
			Path:  "src/auth.go",
			Trust: TrustV2{Tests: "medium"},
		}, {
			Path: "tests/auth_test.go",
		}},
	}
	diff := DiffSnapshots(before, after)
	if diff.TaskChanged == nil || diff.TaskChanged.After != "Fix tests" {
		t.Fatalf("missing task change: %+v", diff.TaskChanged)
	}
	if len(diff.NewRisks) != 1 || diff.NewRisks[0] != "New risk" {
		t.Fatalf("missing new risk: %#v", diff.NewRisks)
	}
	if len(diff.ResolvedQs) != 1 || diff.ResolvedQs[0] != "Old question" {
		t.Fatalf("missing resolved question: %#v", diff.ResolvedQs)
	}
	if len(diff.ActiveFiles.Added) != 1 || diff.ActiveFiles.Added[0] != "tests/auth_test.go" {
		t.Fatalf("missing added file: %#v", diff.ActiveFiles.Added)
	}
	if len(diff.TrustChanges) != 1 || diff.TrustChanges[0].Dimension != "tests" {
		t.Fatalf("missing trust change: %#v", diff.TrustChanges)
	}
}

func TestWriteSnapshotV2ArchivesPreviousJSON(t *testing.T) {
	root := t.TempDir()
	first := &SnapshotV2{
		SchemaVersion: SnapshotSchemaV2,
		ID:            "snap_first",
		Project:       ProjectRefV2{Name: "test", Root: root},
		CapturedAt:    time.Now().UTC().Truncate(time.Second).Add(-time.Hour),
		Source:        SourceV2{Tool: "test"},
		Session:       SessionStateV2{Task: "first"},
		Integrity:     SnapshotIntegrityV2{VerifyStatus: "not_run"},
	}
	second := *first
	second.ID = "snap_second"
	second.CapturedAt = first.CapturedAt.Add(time.Hour)
	second.Session.Task = "second"
	if err := WriteSnapshotV2(root, first); err != nil {
		t.Fatal(err)
	}
	if err := WriteSnapshotV2(root, &second); err != nil {
		t.Fatal(err)
	}
	history, err := HistoryV2(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].ID != "snap_first" {
		t.Fatalf("unexpected JSON history: %+v", history)
	}
	matches, err := filepath.Glob(filepath.Join(HistoryDir(root), "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one archived JSON snapshot, got %d", len(matches))
	}
}

func TestWriteSnapshotV2PrunesJSONHistory(t *testing.T) {
	root := t.TempDir()
	base := time.Now().UTC().Truncate(time.Second).Add(-2 * time.Hour)
	for i := 0; i < DefaultMaxHistory+3; i++ {
		snap := &SnapshotV2{
			SchemaVersion: SnapshotSchemaV2,
			ID:            stableID("snap_test", time.Duration(i).String()),
			Project:       ProjectRefV2{Name: "test", Root: root},
			CapturedAt:    base.Add(time.Duration(i) * time.Minute),
			Source:        SourceV2{Tool: "test"},
			Session:       SessionStateV2{Task: "task"},
			Integrity:     SnapshotIntegrityV2{VerifyStatus: "not_run"},
		}
		if err := WriteSnapshotV2(root, snap); err != nil {
			t.Fatal(err)
		}
	}
	history, err := HistoryV2(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) > DefaultMaxHistory {
		t.Fatalf("expected at most %d JSON history entries, got %d", DefaultMaxHistory, len(history))
	}
}

func TestWriteSnapshotV2StrictFailureDoesNotArchivePreviousJSON(t *testing.T) {
	root := t.TempDir()
	first := &SnapshotV2{
		SchemaVersion: SnapshotSchemaV2,
		ID:            "snap_first",
		Project:       ProjectRefV2{Name: "test", Root: root},
		CapturedAt:    time.Now().UTC().Truncate(time.Second).Add(-time.Hour),
		Source:        SourceV2{Tool: "test"},
		Session:       SessionStateV2{Task: "first"},
		Integrity:     SnapshotIntegrityV2{VerifyStatus: "not_run"},
	}
	if err := WriteSnapshotV2(root, first); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, ".bifrost", "config.json"), `{"security":{"strict":true}}`)
	bad := *first
	bad.ID = "snap_bad"
	bad.CapturedAt = first.CapturedAt.Add(time.Hour)
	bad.Session.Task = "Use sk-proj-abcdefghijklmnopqrstuvwxyz123456"
	if err := WriteSnapshotV2(root, &bad); err == nil {
		t.Fatal("expected strict secret detection to fail")
	}
	history, err := HistoryV2(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 0 {
		t.Fatalf("failed write should not archive previous JSON snapshot: %+v", history)
	}
	current, err := ReadSnapshotV2(root)
	if err != nil {
		t.Fatal(err)
	}
	if current.ID != "snap_first" {
		t.Fatalf("failed write changed current snapshot: %s", current.ID)
	}
}

func TestWritePlanExecutionStateEmitsSinglePlanWriteEvent(t *testing.T) {
	root := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	plan := &PlanV2{
		SchemaVersion: PlanSchemaV2,
		Name:          "timeline-plan",
		Title:         "Timeline Plan",
		Goal:          "Avoid duplicate events",
		Status:        PlanStatusActive,
		Version:       "v1",
		CreatedAt:     now,
		UpdatedAt:     now,
		Steps:         []PlanStepV2{{ID: "step_1", Title: "Do work", Status: "not_started"}},
	}
	if err := WritePlanExecutionState(root, plan); err != nil {
		t.Fatal(err)
	}
	events, err := ReadTimeline(root, 0)
	if err != nil {
		t.Fatal(err)
	}
	planWrites := 0
	for _, event := range events {
		if event.Type == "plan.write" && event.Plan == "timeline-plan" {
			planWrites++
		}
	}
	if planWrites != 1 {
		t.Fatalf("expected one plan.write event, got %d: %+v", planWrites, events)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}
