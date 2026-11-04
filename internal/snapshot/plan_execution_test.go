package snapshot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPlanExecutionSummaryHealthAndNext(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "existing.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatal(err)
	}
	plan := &PlanV2{
		SchemaVersion: PlanSchemaV2,
		Name:          "auth",
		Title:         "Auth",
		Goal:          "Ship auth",
		Status:        PlanStatusActive,
		Version:       "v1",
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
		Steps: []PlanStepV2{
			{ID: "step_1", Title: "Done", Status: "verified_done", ExpectedFiles: []string{"existing.go"}},
			{ID: "step_2", Title: "Claimed", Status: "claimed_done"},
			{ID: "step_3", Title: "Blocked", Status: "blocked", Verification: &PlanStepVerificationV2{LastResult: &CommandResultRefV2{State: "fail"}}},
			{ID: "step_4", Title: "Missing", Status: "not_started", ExpectedFiles: []string{"missing.go"}},
		},
	}

	summary := PlanExecutionSummaryFor(root, plan)
	if summary.VerifiedDone != 1 || summary.ClaimedDone != 1 || summary.Blocked != 1 || summary.NotStarted != 1 {
		t.Fatalf("unexpected counts: %+v", summary)
	}
	if summary.FailedVerify != 1 || summary.UnverifiedDone != 1 || summary.MissingFiles != 1 {
		t.Fatalf("unexpected health inputs: %+v", summary)
	}
	if summary.Health != 69 {
		t.Fatalf("health = %d, want 69", summary.Health)
	}
	if summary.NextStep == nil || summary.NextStep.ID != "step_3" || !strings.Contains(summary.NextAction, "failed verification") {
		t.Fatalf("unexpected next action: step=%+v action=%q", summary.NextStep, summary.NextAction)
	}
}

func TestWritePlanExecutionStatePreservesLegacyPlanMetadata(t *testing.T) {
	root := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	legacy := &Plan{
		BifrostVersion: 2,
		CreatedAt:      now,
		UpdatedAt:      now,
		SourceTool:     "claude-code",
		Project:        "bifrost",
		Status:         PlanStatusActive,
		PlanVersion:    1,
		Title:          "Legacy",
		Goal:           "Keep constraints",
		Steps: []PlanStep{
			{ID: "step_1", Description: "Do work", Status: "pending", Files: []string{"a.go"}},
		},
		Constraints: []string{"do not break markdown"},
		ReviewNotes: []ReviewNote{{
			From:        "reviewer",
			At:          now,
			PlanVersion: 1,
			Outcome:     ReviewOutcomeApproved,
			Text:        "approved",
		}},
	}
	if err := WritePlan(root, "legacy", legacy); err != nil {
		t.Fatal(err)
	}
	plan, err := LoadPlanForExecution(root, "legacy")
	if err != nil {
		t.Fatal(err)
	}
	if err := SetPlanStepStatus(plan, "step_1", "verified_done", ""); err != nil {
		t.Fatal(err)
	}
	if err := WritePlanExecutionState(root, plan); err != nil {
		t.Fatal(err)
	}
	readLegacy, err := ReadPlan(root, "legacy")
	if err != nil {
		t.Fatal(err)
	}
	if readLegacy.Steps[0].Status != "done" {
		t.Fatalf("legacy step status = %q, want done", readLegacy.Steps[0].Status)
	}
	if len(readLegacy.Constraints) != 1 || readLegacy.Constraints[0] != "do not break markdown" {
		t.Fatalf("constraints not preserved: %+v", readLegacy.Constraints)
	}
	if len(readLegacy.ReviewNotes) != 1 || readLegacy.ReviewNotes[0].Text != "approved" {
		t.Fatalf("review notes not preserved: %+v", readLegacy.ReviewNotes)
	}
	readJSON, err := ReadPlanV2(root, "legacy")
	if err != nil {
		t.Fatal(err)
	}
	if readJSON.Steps[0].Status != "verified_done" || readJSON.Steps[0].Verification.LastResult.State != "pass" {
		t.Fatalf("json step not verified: %+v", readJSON.Steps[0])
	}
}

func TestWritePlanSyncsExistingExecutionState(t *testing.T) {
	root := t.TempDir()
	now := time.Now().UTC().Truncate(time.Second)
	plan := &Plan{
		BifrostVersion: CurrentVersion,
		CreatedAt:      now,
		UpdatedAt:      now,
		SourceTool:     "test",
		Status:         PlanStatusActive,
		Title:          "Sync",
		Steps: []PlanStep{
			{Description: "Build", Status: "pending", Verify: []string{"go build ./..."}},
			{Description: "Test", Status: "pending", Verify: []string{"go test ./..."}},
		},
	}
	if err := WritePlan(root, "sync", plan); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(PlanJSONPath(root, "sync")); !os.IsNotExist(err) {
		t.Fatal("WritePlan should not create JSON state on its own")
	}

	// Simulate `bifrost plan verify` passing both steps.
	v2, err := LoadPlanForExecution(root, "sync")
	if err != nil {
		t.Fatal(err)
	}
	for i := range v2.Steps {
		v2.Steps[i].Status = "verified_done"
		v2.Steps[i].Verification.LastResult = &CommandResultRefV2{State: "pass", Command: v2.Steps[i].Verification.Commands[0], CapturedAt: now}
	}
	if err := WritePlanExecutionState(root, v2); err != nil {
		t.Fatal(err)
	}

	// An agent edits the Markdown plan: step 0 unchanged, step 1 gets a new command,
	// and a step is added.
	legacy, err := ReadPlan(root, "sync")
	if err != nil {
		t.Fatal(err)
	}
	legacy.Steps[1].Verify = []string{"go test -race ./..."}
	legacy.Steps = append(legacy.Steps, PlanStep{Description: "Release", Status: "pending"})
	if err := WritePlan(root, "sync", legacy); err != nil {
		t.Fatal(err)
	}

	synced, err := ReadPlanV2(root, "sync")
	if err != nil {
		t.Fatal(err)
	}
	if len(synced.Steps) != 3 {
		t.Fatalf("JSON state should gain the new step, got %d steps", len(synced.Steps))
	}
	if synced.Steps[0].Status != "verified_done" || synced.Steps[0].Verification.LastResult == nil {
		t.Fatalf("untouched step should keep its verification, got %+v", synced.Steps[0])
	}
	if synced.Steps[1].Status != "claimed_done" || synced.Steps[1].Verification.LastResult != nil {
		t.Fatalf("changed commands should drop the old result, got %+v", synced.Steps[1])
	}
	if synced.Steps[1].Verification.Commands[0] != "go test -race ./..." {
		t.Fatalf("new command not synced: %+v", synced.Steps[1].Verification)
	}

	if err := DeletePlan(root, "sync"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(PlanJSONPath(root, "sync")); !os.IsNotExist(err) {
		t.Fatal("DeletePlan should remove JSON state")
	}
}

func TestPlanWithoutGoalValidates(t *testing.T) {
	now := time.Now().UTC()
	v2 := PlanToV2(&Plan{CreatedAt: now, UpdatedAt: now, SourceTool: "test", Status: PlanStatusDraft, Title: "No goal"}, "nogoal")
	if err := ValidatePlanV2(v2); err != nil {
		t.Fatalf("a plan without a goal should validate: %v", err)
	}
}
