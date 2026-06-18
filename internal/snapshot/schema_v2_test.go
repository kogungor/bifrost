package snapshot

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func validSnapshotV2() *SnapshotV2 {
	now := time.Date(2026, 6, 18, 10, 22, 33, 0, time.UTC)
	return &SnapshotV2{
		SchemaVersion: SnapshotSchemaV2,
		ID:            "snap_20260618_102233_abc123",
		Project:       ProjectRefV2{Name: "bifrost", Root: "/repo/bifrost"},
		CapturedAt:    now,
		Source:        SourceV2{Tool: "claude-code", Agent: "unknown", BifrostVersion: "0.9.0"},
		Session: SessionStateV2{
			Intent:   "implementing",
			Pressure: "medium",
			Task:     "Implement JSON schema foundation",
			NextStep: "Add validation tests",
		},
		Observed: ObservedV2{
			Git: &GitObservedV2{
				Branch:       "dev",
				Commit:       "abc123",
				Dirty:        true,
				ChangedFiles: []string{"internal/snapshot/schema_v2.go"},
			},
			Files: []FileObservedV2{
				{Path: "internal/snapshot/schema_v2.go", Exists: true, Size: 1234, MTime: now},
			},
			Commands: []CommandObservedV2{
				{ID: "cmd_001", Command: "go test ./...", ExitCode: 0, CapturedAt: now, Summary: "all packages pass"},
			},
		},
		Interpretation: InterpretationV2{
			StatusItems: []StatusItemV2{
				{
					ID:           "status_001",
					Text:         "Schema structs added",
					State:        "claimed_done",
					EvidenceRefs: []string{"ev_git_001"},
					Verification: &VerificationV2{State: "unverified", Reason: "not wired into CLI yet"},
				},
			},
			Decisions: []DecisionV2{
				{ID: "dec_001", Text: "Keep Markdown compatibility model", Scope: "phase_1"},
			},
			Assumptions: []AssumptionV2{
				{ID: "asm_001", Text: "Root-level unknown fields are enough for initial preservation", VerificationState: "unverified"},
			},
			OpenQuestions: []OpenQuestionV2{
				{ID: "q_001", Text: "When should session.json become default write output?", Severity: "medium"},
			},
			Risks: []RiskV2{
				{ID: "risk_001", Text: "Breaking session.md fallback", Severity: "high", Mitigation: "Do not change Read/Write yet"},
			},
		},
		ActiveFiles: []ActiveFileV2{
			{
				Path: "internal/snapshot/schema_v2.go",
				Note: "new JSON model",
				Trust: TrustV2{
					Implementation: "medium",
					Tests:          "medium",
					Security:       "unknown",
					Architecture:   "medium",
					Freshness:      "current",
					Evidence:       "low",
				},
				EvidenceRefs: []string{"ev_file_001"},
			},
		},
		ActivePlan: &ActivePlanRefV2{Name: "integrity-pack", Version: "v2"},
		Integrity:  SnapshotIntegrityV2{VerifyStatus: "not_run"},
		Evidence: []EvidenceV2{
			{
				ID:         "ev_git_001",
				Type:       "git_status",
				Source:     "collector.git",
				ObservedAt: now,
				Summary:    "Branch dev, dirty tree",
				Data:       map[string]any{"branch": "dev"},
			},
		},
	}
}

func validPlanV2() *PlanV2 {
	now := time.Date(2026, 6, 18, 10, 22, 33, 0, time.UTC)
	return &PlanV2{
		SchemaVersion: PlanSchemaV2,
		Name:          "integrity-pack",
		Title:         "Integrity Pack Foundation",
		Goal:          "Add JSON schema foundation without changing Markdown behavior.",
		Status:        PlanStatusActive,
		Version:       "v2",
		CreatedAt:     now.Add(-time.Hour),
		UpdatedAt:     now,
		Steps: []PlanStepV2{
			{
				ID:            "step_001",
				Title:         "Add schema structs",
				Status:        "claimed_done",
				ExpectedFiles: []string{"internal/snapshot/schema_v2.go"},
				Verification: &PlanStepVerificationV2{
					Required: true,
					Commands: []string{"go test ./internal/snapshot"},
					LastResult: &CommandResultRefV2{
						State:       "failed",
						Command:     "go test ./internal/snapshot",
						ExitCode:    0,
						CapturedAt:  now,
						EvidenceRef: "ev_cmd_001",
					},
				},
			},
		},
		Review: PlanReviewV2{Outcome: "approved", Notes: []string{"baseline safe"}},
	}
}

func TestValidateSnapshotV2Valid(t *testing.T) {
	if err := ValidateSnapshotV2(validSnapshotV2()); err != nil {
		t.Fatalf("ValidateSnapshotV2 returned error: %v", err)
	}
}

func TestValidateSnapshotV2Invalid(t *testing.T) {
	s := validSnapshotV2()
	s.SchemaVersion = "snapshot.v1"
	s.ID = ""
	s.Session.Intent = "coding"
	s.Interpretation.StatusItems[0].State = "done"
	s.ActiveFiles[0].Trust.Freshness = "fresh"

	err := ValidateSnapshotV2(s)
	if err == nil {
		t.Fatal("expected validation error")
	}
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	fields := validationFields(validationErr)
	for _, field := range []string{
		"schema_version",
		"id",
		"session.intent",
		"interpretation.status_items[0].state",
		"active_files[0].trust.freshness",
	} {
		if !fields[field] {
			t.Errorf("expected validation issue for %s; got %v", field, fields)
		}
	}
	if !strings.Contains(err.Error(), "invalid snapshot.v2 schema") {
		t.Errorf("error should include subject, got: %s", err.Error())
	}
}

func TestSnapshotV2ReadWriteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := validSnapshotV2()
	if err := WriteSnapshotV2(dir, s); err != nil {
		t.Fatal(err)
	}
	read, err := ReadSnapshotV2(dir)
	if err != nil {
		t.Fatal(err)
	}
	if read.ID != s.ID {
		t.Errorf("ID = %q, want %q", read.ID, s.ID)
	}
	if read.Session.Task != s.Session.Task {
		t.Errorf("task = %q, want %q", read.Session.Task, s.Session.Task)
	}
	if got := SnapshotJSONPath(dir); got != filepath.Join(dir, ".bifrost", "session.json") {
		t.Errorf("SnapshotJSONPath = %q", got)
	}
}

func TestSnapshotV2PreservesRootUnknownFields(t *testing.T) {
	raw := []byte(`{
		"schema_version": "snapshot.v2",
		"id": "snap_unknown",
		"project": {"name": "bifrost"},
		"captured_at": "2026-06-18T10:22:33Z",
		"source": {"tool": "claude-code"},
		"session": {"task": "test unknown preservation"},
		"x_future_field": {"enabled": true}
	}`)

	var s SnapshotV2
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Extra["x_future_field"]; !ok {
		t.Fatal("expected x_future_field in Extra")
	}
	encoded, err := json.Marshal(&s)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), "x_future_field") {
		t.Fatalf("expected encoded JSON to preserve unknown field, got: %s", string(encoded))
	}
}

func TestValidatePlanV2Valid(t *testing.T) {
	if err := ValidatePlanV2(validPlanV2()); err != nil {
		t.Fatalf("ValidatePlanV2 returned error: %v", err)
	}
}

func TestValidateSnapshotV2AllowsWeakEvidenceTrust(t *testing.T) {
	s := validSnapshotV2()
	s.ActiveFiles[0].Trust.Evidence = "weak"
	if err := ValidateSnapshotV2(s); err != nil {
		t.Fatalf("weak evidence trust should be valid: %v", err)
	}
}

func TestValidateSnapshotV2RejectsUnsafePaths(t *testing.T) {
	s := validSnapshotV2()
	s.ActiveFiles[0].Path = "../secret"
	s.Observed.Files[0].Path = "/abs/path"
	err := ValidateSnapshotV2(s)
	if err == nil {
		t.Fatal("expected unsafe path validation error")
	}
	fields := validationFields(err.(*ValidationError))
	if !fields["active_files[0].path"] || !fields["observed.files[0].path"] {
		t.Fatalf("expected unsafe path fields, got %v", fields)
	}
}

func TestValidateSnapshotV2AllowsDotsInsidePathSegment(t *testing.T) {
	s := validSnapshotV2()
	s.ActiveFiles[0].Path = "docs/v1..v2.md"
	s.Observed.Files[0].Path = "internal/snapshot/schema_v2.go"
	if err := ValidateSnapshotV2(s); err != nil {
		t.Fatalf("path with dots inside a segment should be valid: %v", err)
	}
}

func TestValidatePlanV2RejectsTraversalExpectedFiles(t *testing.T) {
	p := validPlanV2()
	p.Steps[0].ExpectedFiles = []string{"internal/../secret.go", `C:\secret.go`}
	err := ValidatePlanV2(p)
	if err == nil {
		t.Fatal("expected unsafe expected_files validation error")
	}
	fields := validationFields(err.(*ValidationError))
	if !fields["steps[0].expected_files[0]"] || !fields["steps[0].expected_files[1]"] {
		t.Fatalf("expected unsafe expected_files fields, got %v", fields)
	}
}

func TestValidateSnapshotV2RejectsUnsafeEvidenceID(t *testing.T) {
	s := validSnapshotV2()
	s.Evidence[0].ID = "../ev_bad"
	err := ValidateSnapshotV2(s)
	if err == nil {
		t.Fatal("expected unsafe evidence ID validation error")
	}
	fields := validationFields(err.(*ValidationError))
	if !fields["evidence[0].id"] {
		t.Fatalf("expected unsafe evidence ID field, got %v", fields)
	}
}

func TestValidatePlanV2Invalid(t *testing.T) {
	p := validPlanV2()
	p.SchemaVersion = "plan.v1"
	p.Name = "../bad"
	p.Status = "running"
	p.Steps[0].Status = "done"
	p.Steps[0].Verification.LastResult.State = "green"

	err := ValidatePlanV2(p)
	if err == nil {
		t.Fatal("expected validation error")
	}
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	fields := validationFields(validationErr)
	for _, field := range []string{
		"schema_version",
		"name",
		"status",
		"steps[0].status",
		"steps[0].verification.last_result.state",
	} {
		if !fields[field] {
			t.Errorf("expected validation issue for %s; got %v", field, fields)
		}
	}
}

func TestPlanV2ReadWriteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := validPlanV2()
	if err := WritePlanV2(dir, p); err != nil {
		t.Fatal(err)
	}
	read, err := ReadPlanV2(dir, p.Name)
	if err != nil {
		t.Fatal(err)
	}
	if read.Name != p.Name {
		t.Errorf("Name = %q, want %q", read.Name, p.Name)
	}
	if len(read.Steps) != 1 {
		t.Fatalf("Steps len = %d, want 1", len(read.Steps))
	}
	if got := PlanJSONPath(dir, p.Name); got != filepath.Join(dir, ".bifrost", "plans", "integrity-pack.json") {
		t.Errorf("PlanJSONPath = %q", got)
	}
	if got := EvidenceDir(dir); got != filepath.Join(dir, ".bifrost", "evidence") {
		t.Errorf("EvidenceDir = %q", got)
	}
}

func TestPlanV2PreservesRootUnknownFields(t *testing.T) {
	raw := []byte(`{
		"schema_version": "plan.v2",
		"name": "plan",
		"title": "Plan",
		"goal": "Test unknown preservation",
		"status": "draft",
		"version": "v2",
		"created_at": "2026-06-18T10:22:33Z",
		"updated_at": "2026-06-18T10:22:33Z",
		"x_future_field": "keep me"
	}`)

	var p PlanV2
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	if _, ok := p.Extra["x_future_field"]; !ok {
		t.Fatal("expected x_future_field in Extra")
	}
	encoded, err := json.Marshal(&p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), "x_future_field") {
		t.Fatalf("expected encoded JSON to preserve unknown field, got: %s", string(encoded))
	}
}

func validationFields(err *ValidationError) map[string]bool {
	fields := make(map[string]bool, len(err.Issues))
	for _, issue := range err.Issues {
		fields[issue.Field] = true
	}
	return fields
}
