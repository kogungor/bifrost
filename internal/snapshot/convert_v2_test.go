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
		SessionStart:   "2026-06-18T09:00:00Z",
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
	if v2.Session.SessionStart != "2026-06-18T09:00:00Z" {
		t.Errorf("session_start not preserved in v2: %q", v2.Session.SessionStart)
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
	if roundTrip.SessionStart != "2026-06-18T09:00:00Z" {
		t.Errorf("SessionStart = %q", roundTrip.SessionStart)
	}
	if roundTrip.Status[0] != "- [x?] Audit complete — claimed done, not verified" {
		t.Errorf("Status[0] = %q", roundTrip.Status[0])
	}
}

func TestStatusV2RenderDistinguishesClaimedAndVerified(t *testing.T) {
	items := []StatusItemV2{
		{ID: "status_1", Text: "validateToken implemented", State: "verified_done", Verification: &VerificationV2{State: "pass", Reason: "`go test ./...`"}},
		{ID: "status_2", Text: "refreshToken implemented", State: "claimed_done", Verification: &VerificationV2{State: "unverified"}},
		{ID: "status_3", Text: "token revocation", State: "blocked", Verification: &VerificationV2{State: "fail", Reason: "q_001"}},
		{ID: "status_4", Text: "auth tests pass", State: "claimed_done", Verification: &VerificationV2{State: "fail", Reason: "`go test` failed"}},
	}

	got := statusItemsFromV2(items)
	want := []string{
		"- [x] validateToken implemented — verified by `go test ./...`",
		"- [x?] refreshToken implemented — claimed done, not verified",
		"- [!] token revocation — blocked by q_001",
		"- [x?] auth tests pass — verification failed: `go test` failed",
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("status %d = %q, want %q", i, got[i], want[i])
		}
	}

	for i, rendered := range got {
		text, state := parseChecklistState(rendered)
		if text != items[i].Text {
			t.Fatalf("parsed text %d = %q, want %q", i, text, items[i].Text)
		}
		if state != items[i].State {
			t.Fatalf("parsed state %d = %q, want %q", i, state, items[i].State)
		}
	}
}

func TestPlanToV2AndBackPreservesSteps(t *testing.T) {
	legacy := &Plan{
		BifrostVersion:   2,
		CreatedAt:        time.Date(2026, 6, 18, 9, 0, 0, 0, time.UTC),
		UpdatedAt:        time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC),
		SourceTool:       "claude-code",
		Project:          "bifrost",
		Status:           PlanStatusActive,
		PlanVersion:      3,
		ProposedBy:       "claude-code",
		MaxRevisions:     5,
		RevisionCount:    2,
		ConsensusState:   ConsensusReached,
		ActivationReason: ActivationConsensus,
		Title:            "Integrity Pack",
		Goal:             "Add migration foundation.",
		Steps: []PlanStep{
			{ID: "step_done", Description: "Done step", Status: "done", Files: []string{"a.go"}},
			{ID: "step_pending", Description: "Pending step", Status: "pending", Files: []string{"b.go"}},
			{ID: "step_blocked", Description: "Blocked step", Status: "blocked", Files: []string{"c.go"}},
		},
		ReviewNotes: []ReviewNote{{
			From:        "opencode",
			At:          time.Date(2026, 6, 18, 10, 30, 0, 0, time.UTC),
			PlanVersion: 3,
			Outcome:     ReviewOutcomeApproved,
			Text:        "looks good",
		}},
	}

	v2 := PlanToV2(legacy, "integrity-pack")
	if err := ValidatePlanV2(v2); err != nil {
		t.Fatal(err)
	}
	if v2.Version != "v3" {
		t.Errorf("Version = %q, want v3", v2.Version)
	}
	if v2.Source.Tool != "claude-code" || v2.Project.Name != "bifrost" {
		t.Errorf("plan source/project not preserved: source=%+v project=%+v", v2.Source, v2.Project)
	}
	if v2.Consensus.PlanVersion != 3 || v2.Consensus.ProposedBy != "claude-code" || v2.Consensus.MaxRevisions != 5 {
		t.Errorf("consensus not preserved: %+v", v2.Consensus)
	}
	if len(v2.Review.Details) != 1 || v2.Review.Details[0].From != "opencode" {
		t.Errorf("review details not preserved: %+v", v2.Review.Details)
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
	if roundTrip.SourceTool != "claude-code" || roundTrip.Project != "bifrost" {
		t.Errorf("roundTrip source/project = %q/%q", roundTrip.SourceTool, roundTrip.Project)
	}
	if roundTrip.PlanVersion != 3 || roundTrip.ProposedBy != "claude-code" || roundTrip.MaxRevisions != 5 {
		t.Errorf("roundTrip consensus not preserved: %+v", roundTrip)
	}
	if len(roundTrip.ReviewNotes) != 1 || roundTrip.ReviewNotes[0].From != "opencode" || roundTrip.ReviewNotes[0].PlanVersion != 3 {
		t.Errorf("roundTrip review notes not preserved: %+v", roundTrip.ReviewNotes)
	}
}
