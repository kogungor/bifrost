package snapshot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParsePlanRoundTrip(t *testing.T) {
	now := time.Date(2025, 3, 21, 14, 32, 17, 0, time.UTC)
	p := &Plan{
		BifrostVersion: 1,
		CreatedAt:      now,
		UpdatedAt:      now.Add(time.Hour),
		SourceTool:     "claude-code",
		Project:        "my-api",
		Status:         PlanStatusActive,
		Title:          "Auth Refactor",
		Goal:           "Refactor authentication to use JWT tokens.",
		Steps: []PlanStep{
			{Description: "Update auth middleware", Status: "done", Files: []string{"src/auth.go", "src/middleware.go"}},
			{Description: "Add token validation", Status: "pending", Files: []string{"src/handler.go"}},
			{Description: "Migrate database schema", Status: "blocked", Files: []string{"src/db.go"}},
		},
		Constraints: []string{
			"Must maintain backward compatibility",
			"No new dependencies",
		},
		ReviewNotes: []ReviewNote{
			{From: "opencode", Text: "Consider adding error handling for the auth timeout case"},
			{From: "claude-code", Text: "Good point, added step for that"},
		},
	}

	rendered := RenderPlan(p)
	parsed, err := ParsePlan([]byte(rendered))
	if err != nil {
		t.Fatalf("ParsePlan error: %v", err)
	}

	if parsed.BifrostVersion != p.BifrostVersion {
		t.Errorf("BifrostVersion = %d, want %d", parsed.BifrostVersion, p.BifrostVersion)
	}
	if parsed.SourceTool != p.SourceTool {
		t.Errorf("SourceTool = %q, want %q", parsed.SourceTool, p.SourceTool)
	}
	if parsed.Project != p.Project {
		t.Errorf("Project = %q, want %q", parsed.Project, p.Project)
	}
	if parsed.Status != p.Status {
		t.Errorf("Status = %q, want %q", parsed.Status, p.Status)
	}
	if !parsed.CreatedAt.Equal(p.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", parsed.CreatedAt, p.CreatedAt)
	}
	if !parsed.UpdatedAt.Equal(p.UpdatedAt) {
		t.Errorf("UpdatedAt = %v, want %v", parsed.UpdatedAt, p.UpdatedAt)
	}
	if parsed.Title != p.Title {
		t.Errorf("Title = %q, want %q", parsed.Title, p.Title)
	}
	if parsed.Goal != p.Goal {
		t.Errorf("Goal = %q, want %q", parsed.Goal, p.Goal)
	}
	if len(parsed.Steps) != len(p.Steps) {
		t.Fatalf("Steps count = %d, want %d", len(parsed.Steps), len(p.Steps))
	}
	for i, step := range parsed.Steps {
		if step.Description != p.Steps[i].Description {
			t.Errorf("Step[%d].Description = %q, want %q", i, step.Description, p.Steps[i].Description)
		}
		if step.Status != p.Steps[i].Status {
			t.Errorf("Step[%d].Status = %q, want %q", i, step.Status, p.Steps[i].Status)
		}
		if len(step.Files) != len(p.Steps[i].Files) {
			t.Errorf("Step[%d].Files count = %d, want %d", i, len(step.Files), len(p.Steps[i].Files))
		} else {
			for j, f := range step.Files {
				if f != p.Steps[i].Files[j] {
					t.Errorf("Step[%d].Files[%d] = %q, want %q", i, j, f, p.Steps[i].Files[j])
				}
			}
		}
	}
	if len(parsed.Constraints) != len(p.Constraints) {
		t.Fatalf("Constraints count = %d, want %d", len(parsed.Constraints), len(p.Constraints))
	}
	for i, c := range parsed.Constraints {
		if c != p.Constraints[i] {
			t.Errorf("Constraint[%d] = %q, want %q", i, c, p.Constraints[i])
		}
	}
	if len(parsed.ReviewNotes) != len(p.ReviewNotes) {
		t.Fatalf("ReviewNotes count = %d, want %d", len(parsed.ReviewNotes), len(p.ReviewNotes))
	}
	for i, rn := range parsed.ReviewNotes {
		if rn.From != p.ReviewNotes[i].From {
			t.Errorf("ReviewNote[%d].From = %q, want %q", i, rn.From, p.ReviewNotes[i].From)
		}
		if rn.Text != p.ReviewNotes[i].Text {
			t.Errorf("ReviewNote[%d].Text = %q, want %q", i, rn.Text, p.ReviewNotes[i].Text)
		}
	}
}

func TestParsePlanWithReviewNotes(t *testing.T) {
	md := `---
bifrost_version: 1
created_at: 2025-03-21T14:32:17Z
updated_at: 2025-03-21T14:32:17Z
source_tool: claude-code
project: test
status: active
---

# Test Plan

## Goal
Test goal.

## Steps
- [ ] Do something
  - ` + "`main.go`" + `

## Constraints
- None

## Review Notes
> [opencode] This looks good
> [claude-code] Thanks for the review
`
	p, err := ParsePlan([]byte(md))
	if err != nil {
		t.Fatalf("ParsePlan error: %v", err)
	}

	if len(p.ReviewNotes) != 2 {
		t.Fatalf("ReviewNotes count = %d, want 2", len(p.ReviewNotes))
	}
	if p.ReviewNotes[0].From != "opencode" {
		t.Errorf("ReviewNote[0].From = %q, want opencode", p.ReviewNotes[0].From)
	}
	if p.ReviewNotes[0].Text != "This looks good" {
		t.Errorf("ReviewNote[0].Text = %q, want 'This looks good'", p.ReviewNotes[0].Text)
	}
}

func TestParsePlanStepStatuses(t *testing.T) {
	md := `---
bifrost_version: 1
created_at: 2025-03-21T14:32:17Z
updated_at: 2025-03-21T14:32:17Z
source_tool: claude-code
project: test
status: draft
---

# Status Test

## Goal
Test step statuses.

## Steps
- [x] Completed step
- [ ] Pending step
- [!] Blocked step

## Constraints

## Review Notes
`
	p, err := ParsePlan([]byte(md))
	if err != nil {
		t.Fatalf("ParsePlan error: %v", err)
	}

	if len(p.Steps) != 3 {
		t.Fatalf("Steps count = %d, want 3", len(p.Steps))
	}

	tests := []struct {
		idx    int
		status string
		desc   string
	}{
		{0, "done", "Completed step"},
		{1, "pending", "Pending step"},
		{2, "blocked", "Blocked step"},
	}

	for _, tt := range tests {
		if p.Steps[tt.idx].Status != tt.status {
			t.Errorf("Step[%d].Status = %q, want %q", tt.idx, p.Steps[tt.idx].Status, tt.status)
		}
		if p.Steps[tt.idx].Description != tt.desc {
			t.Errorf("Step[%d].Description = %q, want %q", tt.idx, p.Steps[tt.idx].Description, tt.desc)
		}
	}
}

func TestParsePlanEmpty(t *testing.T) {
	md := `---
bifrost_version: 1
created_at: 2025-03-21T14:32:17Z
updated_at: 2025-03-21T14:32:17Z
source_tool: claude-code
project: test
status: draft
---

# Empty Plan

## Goal

## Steps

## Constraints

## Review Notes
`
	p, err := ParsePlan([]byte(md))
	if err != nil {
		t.Fatalf("ParsePlan error: %v", err)
	}

	if p.Title != "Empty Plan" {
		t.Errorf("Title = %q, want 'Empty Plan'", p.Title)
	}
	if len(p.Steps) != 0 {
		t.Errorf("Steps count = %d, want 0", len(p.Steps))
	}
	if len(p.Constraints) != 0 {
		t.Errorf("Constraints count = %d, want 0", len(p.Constraints))
	}
	if len(p.ReviewNotes) != 0 {
		t.Errorf("ReviewNotes count = %d, want 0", len(p.ReviewNotes))
	}
}

func TestParsePlanLegacyTimestamp(t *testing.T) {
	md := `---
bifrost_version: 1
timestamp: 2025-03-21T14:32:17Z
source_tool: claude-code
project: test
---

# Legacy Plan

## Goal
Test backward compat.

## Steps

## Constraints

## Review Notes
`
	p, err := ParsePlan([]byte(md))
	if err != nil {
		t.Fatalf("ParsePlan error: %v", err)
	}

	if p.Title != "Legacy Plan" {
		t.Errorf("Title = %q, want 'Legacy Plan'", p.Title)
	}
	if p.Status != PlanStatusDraft {
		t.Errorf("Status = %q, want %q (default)", p.Status, PlanStatusDraft)
	}
	expected := time.Date(2025, 3, 21, 14, 32, 17, 0, time.UTC)
	if !p.CreatedAt.Equal(expected) {
		t.Errorf("CreatedAt = %v, want %v", p.CreatedAt, expected)
	}
	if !p.UpdatedAt.Equal(expected) {
		t.Errorf("UpdatedAt = %v, want %v", p.UpdatedAt, expected)
	}
}

func TestParsePlanLegacyCommaFiles(t *testing.T) {
	md := `---
bifrost_version: 1
created_at: 2025-03-21T14:32:17Z
updated_at: 2025-03-21T14:32:17Z
source_tool: claude-code
project: test
status: draft
---

# Legacy Files

## Goal
Test legacy comma format.

## Steps
- [ ] Do something
  files: src/a.go, src/b.go

## Constraints

## Review Notes
`
	p, err := ParsePlan([]byte(md))
	if err != nil {
		t.Fatalf("ParsePlan error: %v", err)
	}

	if len(p.Steps) != 1 {
		t.Fatalf("Steps count = %d, want 1", len(p.Steps))
	}
	if len(p.Steps[0].Files) != 2 {
		t.Fatalf("Step[0].Files count = %d, want 2", len(p.Steps[0].Files))
	}
	if p.Steps[0].Files[0] != "src/a.go" {
		t.Errorf("Step[0].Files[0] = %q, want src/a.go", p.Steps[0].Files[0])
	}
}

func TestNamedPlans(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()

	plan1 := &Plan{
		BifrostVersion: 1,
		CreatedAt:      now,
		UpdatedAt:      now,
		SourceTool:     "claude-code",
		Project:        "test",
		Status:         PlanStatusDraft,
		Title:          "Plan One",
		Goal:           "First plan.",
	}
	plan2 := &Plan{
		BifrostVersion: 1,
		CreatedAt:      now,
		UpdatedAt:      now,
		SourceTool:     "opencode",
		Project:        "test",
		Status:         PlanStatusActive,
		Title:          "Plan Two",
		Goal:           "Second plan.",
	}

	if err := WritePlan(dir, "plan", plan1); err != nil {
		t.Fatalf("WritePlan plan: %v", err)
	}
	if err := WritePlan(dir, "auth-refactor", plan2); err != nil {
		t.Fatalf("WritePlan auth-refactor: %v", err)
	}

	p1, err := ReadPlan(dir, "plan")
	if err != nil {
		t.Fatalf("ReadPlan plan: %v", err)
	}
	if p1.Title != "Plan One" {
		t.Errorf("plan Title = %q, want 'Plan One'", p1.Title)
	}

	p2, err := ReadPlan(dir, "auth-refactor")
	if err != nil {
		t.Fatalf("ReadPlan auth-refactor: %v", err)
	}
	if p2.Title != "Plan Two" {
		t.Errorf("auth-refactor Title = %q, want 'Plan Two'", p2.Title)
	}

	if _, err := os.Stat(filepath.Join(dir, ".bifrost", "plan.plan.md")); err != nil {
		t.Errorf("plan.plan.md does not exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".bifrost", "auth-refactor.plan.md")); err != nil {
		t.Errorf("auth-refactor.plan.md does not exist: %v", err)
	}
}

func TestListPlans(t *testing.T) {
	dir := t.TempDir()

	names, err := ListPlans(dir)
	if err != nil {
		t.Fatalf("ListPlans error: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("expected 0 plans, got %d", len(names))
	}

	now := time.Now().UTC()
	for _, name := range []string{"plan", "auth-refactor", "db-migration"} {
		p := &Plan{
			BifrostVersion: 1,
			CreatedAt:      now,
			UpdatedAt:      now,
			SourceTool:     "test",
			Project:        "test",
			Status:         PlanStatusDraft,
			Title:          name,
			Goal:           "Test.",
		}
		if err := WritePlan(dir, name, p); err != nil {
			t.Fatalf("WritePlan %s: %v", name, err)
		}
	}

	names, err = ListPlans(dir)
	if err != nil {
		t.Fatalf("ListPlans error: %v", err)
	}
	if len(names) != 3 {
		t.Errorf("expected 3 plans, got %d", len(names))
	}

	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}
	for _, expected := range []string{"plan", "auth-refactor", "db-migration"} {
		if !nameSet[expected] {
			t.Errorf("missing plan name: %s", expected)
		}
	}
}

func TestReadPlanNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadPlan(dir, "nonexistent")
	if err != ErrNoPlan {
		t.Errorf("expected ErrNoPlan, got %v", err)
	}
}

func TestDeletePlan(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()

	p := &Plan{
		BifrostVersion: 1,
		CreatedAt:      now,
		UpdatedAt:      now,
		SourceTool:     "test",
		Project:        "test",
		Status:         PlanStatusDraft,
		Title:          "To Delete",
		Goal:           "Will be removed.",
	}
	if err := WritePlan(dir, "deleteme", p); err != nil {
		t.Fatalf("WritePlan: %v", err)
	}

	// Verify it exists
	if _, err := ReadPlan(dir, "deleteme"); err != nil {
		t.Fatalf("ReadPlan before delete: %v", err)
	}

	// Delete it
	if err := DeletePlan(dir, "deleteme"); err != nil {
		t.Fatalf("DeletePlan: %v", err)
	}

	// Verify it's gone
	_, err := ReadPlan(dir, "deleteme")
	if err != ErrNoPlan {
		t.Errorf("expected ErrNoPlan after delete, got %v", err)
	}
}

func TestDeletePlanNotFound(t *testing.T) {
	dir := t.TempDir()
	err := DeletePlan(dir, "nonexistent")
	if err != ErrNoPlan {
		t.Errorf("expected ErrNoPlan, got %v", err)
	}
}

func TestCompletionPct(t *testing.T) {
	tests := []struct {
		name  string
		steps []PlanStep
		pct   int
	}{
		{"no steps", nil, 0},
		{"all pending", []PlanStep{{Status: "pending"}, {Status: "pending"}}, 0},
		{"all done", []PlanStep{{Status: "done"}, {Status: "done"}}, 100},
		{"half done", []PlanStep{{Status: "done"}, {Status: "pending"}}, 50},
		{"one of three", []PlanStep{{Status: "done"}, {Status: "pending"}, {Status: "blocked"}}, 33},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Plan{Steps: tt.steps}
			if got := p.CompletionPct(); got != tt.pct {
				t.Errorf("CompletionPct() = %d, want %d", got, tt.pct)
			}
		})
	}
}

func TestStepSummary(t *testing.T) {
	p := &Plan{
		Steps: []PlanStep{
			{Status: "done"},
			{Status: "done"},
			{Status: "pending"},
			{Status: "blocked"},
		},
	}
	done, pending, blocked := p.StepSummary()
	if done != 2 {
		t.Errorf("done = %d, want 2", done)
	}
	if pending != 1 {
		t.Errorf("pending = %d, want 1", pending)
	}
	if blocked != 1 {
		t.Errorf("blocked = %d, want 1", blocked)
	}
}

func TestValidatePlanName(t *testing.T) {
	valid := []string{"plan", "auth-refactor", "db-migration", "v2", "my_plan"}
	for _, name := range valid {
		if err := ValidatePlanName(name); err != nil {
			t.Errorf("ValidatePlanName(%q) = %v, want nil", name, err)
		}
	}

	invalid := []struct {
		name string
		want string
	}{
		{"", "cannot be empty"},
		{"..", "invalid plan name"},
		{"../evil", "invalid plan name"},
		{"foo/bar", "invalid plan name"},
		{"foo\\bar", "invalid plan name"},
		{".", "invalid plan name"},
		{strings.Repeat("x", 101), "exceeds"},
	}
	for _, tt := range invalid {
		err := ValidatePlanName(tt.name)
		if err == nil {
			t.Errorf("ValidatePlanName(%q) = nil, want error containing %q", tt.name, tt.want)
			continue
		}
		if !strings.Contains(err.Error(), tt.want) {
			t.Errorf("ValidatePlanName(%q) = %q, want error containing %q", tt.name, err.Error(), tt.want)
		}
	}
}

func TestPlanFileFormat(t *testing.T) {
	// Verify the new backtick file format renders and parses correctly,
	// including filenames that would break with comma separation.
	now := time.Now().UTC()
	p := &Plan{
		BifrostVersion: 1,
		CreatedAt:      now,
		UpdatedAt:      now,
		SourceTool:     "test",
		Project:        "test",
		Status:         PlanStatusDraft,
		Title:          "File Format Test",
		Goal:           "Test backtick file format.",
		Steps: []PlanStep{
			{
				Description: "Handle tricky files",
				Status:      "pending",
				Files:       []string{"src/a.go", "src/b,c.go", "path with spaces/file.go"},
			},
		},
	}

	rendered := RenderPlan(p)
	parsed, err := ParsePlan([]byte(rendered))
	if err != nil {
		t.Fatalf("ParsePlan error: %v", err)
	}

	if len(parsed.Steps) != 1 {
		t.Fatalf("Steps count = %d, want 1", len(parsed.Steps))
	}
	if len(parsed.Steps[0].Files) != 3 {
		t.Fatalf("Files count = %d, want 3", len(parsed.Steps[0].Files))
	}
	for i, f := range parsed.Steps[0].Files {
		if f != p.Steps[0].Files[i] {
			t.Errorf("Files[%d] = %q, want %q", i, f, p.Steps[0].Files[i])
		}
	}
}

func TestPlanStatusValues(t *testing.T) {
	now := time.Now().UTC()
	for _, status := range []string{PlanStatusDraft, PlanStatusActive, PlanStatusCompleted, PlanStatusArchived} {
		p := &Plan{
			BifrostVersion: 1,
			CreatedAt:      now,
			UpdatedAt:      now,
			SourceTool:     "test",
			Project:        "test",
			Status:         status,
			Title:          "Status " + status,
			Goal:           "Test.",
		}
		rendered := RenderPlan(p)
		parsed, err := ParsePlan([]byte(rendered))
		if err != nil {
			t.Fatalf("ParsePlan error for status %q: %v", status, err)
		}
		if parsed.Status != status {
			t.Errorf("Status = %q, want %q", parsed.Status, status)
		}
	}
}

func TestUnicodeContent(t *testing.T) {
	now := time.Now().UTC()
	p := &Plan{
		BifrostVersion: 1,
		CreatedAt:      now,
		UpdatedAt:      now,
		SourceTool:     "test",
		Project:        "test",
		Status:         PlanStatusDraft,
		Title:          "Unicode Test",
		Goal:           "Support internationalization and emojis.",
		Steps: []PlanStep{
			{Description: "Add i18n support", Status: "pending", Files: []string{"src/i18n.go"}},
		},
		Constraints: []string{"Must support CJK characters"},
		ReviewNotes: []ReviewNote{
			{From: "reviewer", Text: "Looks good"},
		},
	}

	rendered := RenderPlan(p)
	parsed, err := ParsePlan([]byte(rendered))
	if err != nil {
		t.Fatalf("ParsePlan error: %v", err)
	}
	if parsed.Goal != p.Goal {
		t.Errorf("Goal = %q, want %q", parsed.Goal, p.Goal)
	}
	if parsed.Constraints[0] != p.Constraints[0] {
		t.Errorf("Constraint = %q, want %q", parsed.Constraints[0], p.Constraints[0])
	}
}

func TestFileLocking(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().UTC()

	// Write two plans concurrently — neither should fail
	done := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func(idx int) {
			p := &Plan{
				BifrostVersion: 1,
				CreatedAt:      now,
				UpdatedAt:      now,
				SourceTool:     "test",
				Project:        "test",
				Status:         PlanStatusDraft,
				Title:          "Concurrent",
				Goal:           "Test.",
			}
			done <- WritePlan(dir, "concurrent", p)
		}(i)
	}

	for i := 0; i < 2; i++ {
		if err := <-done; err != nil {
			t.Errorf("concurrent write %d failed: %v", i, err)
		}
	}

	// Verify the file is readable
	_, err := ReadPlan(dir, "concurrent")
	if err != nil {
		t.Errorf("ReadPlan after concurrent writes: %v", err)
	}
}

func TestStepIDGeneratedOnWrite(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2025, 3, 21, 14, 32, 17, 0, time.UTC)

	p := &Plan{
		BifrostVersion: 2,
		CreatedAt:      now,
		UpdatedAt:      now,
		SourceTool:     "claude-code",
		Project:        "test",
		Status:         PlanStatusDraft,
		Title:          "Test Plan",
		Goal:           "Test step IDs.",
		Steps: []PlanStep{
			{Description: "First step", Status: "pending"},
			{Description: "Second step", Status: "pending"},
		},
	}

	if err := WritePlan(dir, "test", p); err != nil {
		t.Fatal(err)
	}

	read, err := ReadPlan(dir, "test")
	if err != nil {
		t.Fatal(err)
	}

	if len(read.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(read.Steps))
	}
	if read.Steps[0].ID == "" {
		t.Error("expected step 0 to have an ID after write")
	}
	if read.Steps[1].ID == "" {
		t.Error("expected step 1 to have an ID after write")
	}
	if read.Steps[0].ID == read.Steps[1].ID {
		t.Error("expected different IDs for different steps")
	}
}

func TestStepIDPreservedOnUpdate(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2025, 3, 21, 14, 32, 17, 0, time.UTC)

	p := &Plan{
		BifrostVersion: 2,
		CreatedAt:      now,
		UpdatedAt:      now,
		SourceTool:     "claude-code",
		Project:        "test",
		Status:         PlanStatusDraft,
		Title:          "Test Plan",
		Goal:           "Test ID stability.",
		Steps: []PlanStep{
			{Description: "Stable step", Status: "pending"},
		},
	}

	if err := WritePlan(dir, "stable", p); err != nil {
		t.Fatal(err)
	}

	first, _ := ReadPlan(dir, "stable")
	firstID := first.Steps[0].ID

	// Update step status and write again
	first.Steps[0].Status = "done"
	if err := WritePlan(dir, "stable", first); err != nil {
		t.Fatal(err)
	}

	second, _ := ReadPlan(dir, "stable")
	if second.Steps[0].ID != firstID {
		t.Errorf("step ID changed after update: was %q, now %q", firstID, second.Steps[0].ID)
	}
}

func TestGenerateStepIDDeterministic(t *testing.T) {
	now := time.Date(2025, 3, 21, 14, 32, 17, 0, time.UTC)
	id1 := GenerateStepID("Write unit tests", now)
	id2 := GenerateStepID("Write unit tests", now)
	if id1 != id2 {
		t.Errorf("expected deterministic ID, got %q and %q", id1, id2)
	}
	if len(id1) != 8 {
		t.Errorf("expected 8-char ID, got %d chars: %q", len(id1), id1)
	}
	// Different description should produce different ID
	id3 := GenerateStepID("Write integration tests", now)
	if id1 == id3 {
		t.Error("expected different IDs for different descriptions")
	}
}

func TestConsensusFieldsRoundTrip(t *testing.T) {
	now := time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC)
	p := &Plan{
		BifrostVersion:   1,
		CreatedAt:        now,
		UpdatedAt:        now,
		SourceTool:       "claude-code",
		Project:          "my-api",
		Status:           PlanStatusActive,
		Title:            "Consensus Test",
		Goal:             "Test consensus fields.",
		PlanVersion:      3,
		ProposedBy:       "claude-code",
		MaxRevisions:     5,
		RevisionCount:    2,
		ConsensusState:   ConsensusReached,
		ActivationReason: ActivationConsensus,
	}

	rendered := RenderPlan(p)
	parsed, err := ParsePlan([]byte(rendered))
	if err != nil {
		t.Fatalf("ParsePlan error: %v", err)
	}

	if parsed.PlanVersion != 3 {
		t.Errorf("PlanVersion = %d, want 3", parsed.PlanVersion)
	}
	if parsed.ProposedBy != "claude-code" {
		t.Errorf("ProposedBy = %q, want claude-code", parsed.ProposedBy)
	}
	if parsed.MaxRevisions != 5 {
		t.Errorf("MaxRevisions = %d, want 5", parsed.MaxRevisions)
	}
	if parsed.RevisionCount != 2 {
		t.Errorf("RevisionCount = %d, want 2", parsed.RevisionCount)
	}
	if parsed.ConsensusState != ConsensusReached {
		t.Errorf("ConsensusState = %q, want %q", parsed.ConsensusState, ConsensusReached)
	}
	if parsed.ActivationReason != ActivationConsensus {
		t.Errorf("ActivationReason = %q, want %q", parsed.ActivationReason, ActivationConsensus)
	}
}

func TestReviewNoteNewFormatRoundTrip(t *testing.T) {
	at := time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC)
	now := at.Add(-time.Hour)
	p := &Plan{
		BifrostVersion: 1,
		CreatedAt:      now,
		UpdatedAt:      now,
		SourceTool:     "claude-code",
		Project:        "my-api",
		Status:         PlanStatusDraft,
		Title:          "Note Format Test",
		Goal:           "Test review note format.",
		ReviewNotes: []ReviewNote{
			{From: "opencode", At: at, PlanVersion: 2, Outcome: ReviewOutcomeNeedsRevision, Text: "auth flow missing"},
			{From: "opencode", At: at.Add(time.Hour), PlanVersion: 3, Outcome: ReviewOutcomeApproved, Text: "looks good"},
		},
	}

	rendered := RenderPlan(p)
	parsed, err := ParsePlan([]byte(rendered))
	if err != nil {
		t.Fatalf("ParsePlan error: %v", err)
	}

	if len(parsed.ReviewNotes) != 2 {
		t.Fatalf("ReviewNotes len = %d, want 2", len(parsed.ReviewNotes))
	}
	n := parsed.ReviewNotes[0]
	if n.From != "opencode" {
		t.Errorf("note[0].From = %q, want opencode", n.From)
	}
	if n.Outcome != ReviewOutcomeNeedsRevision {
		t.Errorf("note[0].Outcome = %q, want %q", n.Outcome, ReviewOutcomeNeedsRevision)
	}
	if n.PlanVersion != 2 {
		t.Errorf("note[0].PlanVersion = %d, want 2", n.PlanVersion)
	}
	if n.Text != "auth flow missing" {
		t.Errorf("note[0].Text = %q, want 'auth flow missing'", n.Text)
	}

	n2 := parsed.ReviewNotes[1]
	if n2.Outcome != ReviewOutcomeApproved {
		t.Errorf("note[1].Outcome = %q, want %q", n2.Outcome, ReviewOutcomeApproved)
	}
}

func TestLegacyReviewNoteStillParses(t *testing.T) {
	raw := `---
bifrost_version: 1
created_at: 2026-03-27T10:00:00Z
updated_at: 2026-03-27T10:00:00Z
source_tool: claude-code
project: my-api
status: draft
---

# Legacy Note Test

## Goal
Test legacy note parsing.

## Steps
No steps defined.

## Constraints
No constraints.

## Review Notes
> [opencode] This is a legacy note without outcome
`
	p, err := ParsePlan([]byte(raw))
	if err != nil {
		t.Fatalf("ParsePlan error: %v", err)
	}
	if len(p.ReviewNotes) != 1 {
		t.Fatalf("ReviewNotes len = %d, want 1", len(p.ReviewNotes))
	}
	n := p.ReviewNotes[0]
	if n.From != "opencode" {
		t.Errorf("From = %q, want opencode", n.From)
	}
	if n.Text != "This is a legacy note without outcome" {
		t.Errorf("Text = %q", n.Text)
	}
	if n.Outcome != "" {
		t.Errorf("Outcome should be empty for legacy notes, got %q", n.Outcome)
	}
}

func TestIsDeadlocked(t *testing.T) {
	p := &Plan{MaxRevisions: 3, RevisionCount: 3}

	// Not deadlocked without needs_revision outcome
	if p.IsDeadlocked() {
		t.Error("should not be deadlocked without review notes")
	}

	p.ReviewNotes = []ReviewNote{
		{Outcome: ReviewOutcomeNeedsRevision},
	}
	if !p.IsDeadlocked() {
		t.Error("should be deadlocked: revision_count >= max_revisions and latest is needs_revision")
	}

	// Approved overrides
	p.ReviewNotes = append(p.ReviewNotes, ReviewNote{Outcome: ReviewOutcomeApproved})
	if p.IsDeadlocked() {
		t.Error("should not be deadlocked when latest outcome is approved")
	}

	// Explicit flag
	p2 := &Plan{DeadlockDetected: true}
	if !p2.IsDeadlocked() {
		t.Error("should be deadlocked when DeadlockDetected is true")
	}
}

func TestLatestReviewOutcome(t *testing.T) {
	p := &Plan{}
	if p.LatestReviewOutcome() != "" {
		t.Error("expected empty outcome with no notes")
	}

	p.ReviewNotes = []ReviewNote{
		{Outcome: ReviewOutcomeNeedsRevision},
		{Outcome: ReviewOutcomeApproved},
	}
	if p.LatestReviewOutcome() != ReviewOutcomeApproved {
		t.Errorf("expected approved, got %q", p.LatestReviewOutcome())
	}
}

func TestAcquireLockSuccess(t *testing.T) {
	tmp := t.TempDir()
	lockPath := filepath.Join(tmp, "test.plan.lock")

	unlock, err := acquireLock(lockPath)
	if err != nil {
		t.Fatalf("acquireLock failed: %v", err)
	}

	// Lock file should exist while held
	if _, err := os.Stat(lockPath); err != nil {
		t.Error("lock file should exist while held")
	}

	unlock()

	// Lock file should be removed after unlock
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("lock file should be removed after unlock")
	}
}

func TestAcquireLockStaleLockIsCleared(t *testing.T) {
	tmp := t.TempDir()
	lockPath := filepath.Join(tmp, "test.plan.lock")

	// Create a stale lock file (mtime in the past)
	if err := os.WriteFile(lockPath, nil, 0600); err != nil {
		t.Fatal(err)
	}
	staleTime := time.Now().Add(-(staleLockAge + time.Second))
	if err := os.Chtimes(lockPath, staleTime, staleTime); err != nil {
		t.Fatal(err)
	}

	// acquireLock should remove the stale lock and succeed
	unlock, err := acquireLock(lockPath)
	if err != nil {
		t.Fatalf("acquireLock should succeed on stale lock, got: %v", err)
	}
	unlock()
}

func TestAcquireLockActiveLockReturnsError(t *testing.T) {
	tmp := t.TempDir()
	lockPath := filepath.Join(tmp, "test.plan.lock")

	// Hold the lock with a fresh mtime
	if err := os.WriteFile(lockPath, nil, 0600); err != nil {
		t.Fatal(err)
	}

	_, err := acquireLock(lockPath)
	if err == nil {
		t.Fatal("expected error when lock is actively held")
	}
	if !strings.Contains(err.Error(), "locked by another process") {
		t.Errorf("expected 'locked by another process' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), lockPath) {
		t.Errorf("expected lock path in error message, got: %v", err)
	}
}
