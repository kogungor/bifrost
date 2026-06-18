package snapshot

import (
	"testing"
	"time"
)

func TestSnapshotToV2AndBackPreservesLegacyFields(t *testing.T) {
	legacy := &Snapshot{
		BifrostVersion: 2,
		Timestamp:      time.Date(2026, 6, 18, 10, 22, 33, 0, time.UTC),
		SourceTool:     "claude-code",
		Project:        "bifrost",
		TokenPressure:  "high",
		SessionIntent:  "implementing",
		ActivePlanName: "integrity-pack",
		GitSHA:         "abc123",
		CurrentTask:    "Add migration foundation",
		Status:         []string{"- [x] Audit complete", "- [-] Migration in progress", "- [ ] CLI tests"},
		ActiveFiles:    []ActiveFile{{Path: "internal/snapshot/convert_v2.go", Note: "conversion helpers", Confidence: "medium"}},
		Decisions:      []string{"- Keep Markdown compatibility"},
		EnvNotes:       []string{"- Run go test ./..."},
		NextStep:       "Wire validate command.",
		Assumptions:    []string{"- JSON write remains opt-in"},
		OpenQuestions:  []string{"- Should migrate overwrite existing JSON?"},
		Risks:          []string{"- Data loss during migration"},
	}

	v2 := SnapshotToV2("/repo/bifrost", legacy)
	if err := ValidateSnapshotV2(v2); err != nil {
		t.Fatal(err)
	}
	if v2.SchemaVersion != SnapshotSchemaV2 {
		t.Errorf("schema = %q", v2.SchemaVersion)
	}
	if v2.Observed.Git == nil || v2.Observed.Git.Commit != "abc123" {
		t.Fatalf("git SHA not preserved: %+v", v2.Observed.Git)
	}
	if got := v2.Interpretation.EnvironmentNotes[0]; got != "Run go test ./..." {
		t.Errorf("environment note = %q", got)
	}

	roundTrip := SnapshotFromV2(v2)
	if roundTrip.CurrentTask != legacy.CurrentTask {
		t.Errorf("CurrentTask = %q, want %q", roundTrip.CurrentTask, legacy.CurrentTask)
	}
	if roundTrip.ActiveFiles[0].Confidence != "medium" {
		t.Errorf("confidence = %q", roundTrip.ActiveFiles[0].Confidence)
	}
	if len(roundTrip.EnvNotes) != 1 || roundTrip.EnvNotes[0] != "- Run go test ./..." {
		t.Errorf("EnvNotes = %#v", roundTrip.EnvNotes)
	}
	if roundTrip.Status[0] != "- [x] Audit complete" {
		t.Errorf("Status[0] = %q", roundTrip.Status[0])
	}
}

func TestPlanToV2AndBackPreservesSteps(t *testing.T) {
	legacy := &Plan{
		BifrostVersion: 2,
		CreatedAt:      time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC),
		UpdatedAt:      time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC),
		SourceTool:     "claude-code",
		Project:        "bifrost",
		Status:         PlanStatusActive,
		PlanVersion:    3,
		Title:          "Integrity Pack",
		Goal:           "Add migration foundation.",
		Steps: []PlanStep{
			{ID: "step_done", Description: "Done step", Status: "done", Files: []string{"a.go"}},
			{ID: "step_pending", Description: "Pending step", Status: "pending", Files: []string{"b.go"}},
			{ID: "step_blocked", Description: "Blocked step", Status: "blocked", Files: []string{"c.go"}},
		},
		ReviewNotes: []ReviewNote{{From: "opencode", Outcome: ReviewOutcomeApproved, Text: "looks good"}},
	}

	v2 := PlanToV2(legacy, "integrity-pack")
	if err := ValidatePlanV2(v2); err != nil {
		t.Fatal(err)
	}
	if v2.Version != "v3" {
		t.Errorf("Version = %q, want v3", v2.Version)
	}
	if v2.Steps[0].Status != "claimed_done" {
		t.Errorf("done status mapped to %q", v2.Steps[0].Status)
	}
	if v2.Steps[2].Status != "blocked" {
		t.Errorf("blocked status mapped to %q", v2.Steps[2].Status)
	}

	roundTrip := PlanFromV2(v2)
	if roundTrip.Steps[0].Status != "done" {
		t.Errorf("Steps[0].Status = %q", roundTrip.Steps[0].Status)
	}
	if roundTrip.Steps[1].Status != "pending" {
		t.Errorf("Steps[1].Status = %q", roundTrip.Steps[1].Status)
	}
	if roundTrip.Steps[2].Status != "blocked" {
		t.Errorf("Steps[2].Status = %q", roundTrip.Steps[2].Status)
	}
	if roundTrip.Steps[0].ID != "step_done" {
		t.Errorf("step ID not preserved: %q", roundTrip.Steps[0].ID)
	}
}
