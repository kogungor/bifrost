package snapshot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildBriefGoldenModes(t *testing.T) {
	snap, verify, planSummary, now := briefFixture()
	for _, tt := range []struct {
		mode   string
		golden string
	}{
		{BriefModeImplement, "brief_implement.golden.md"},
		{BriefModeDebug, "brief_debug.golden.md"},
		{BriefModeReview, "brief_review.golden.md"},
		{BriefModePlan, "brief_plan.golden.md"},
	} {
		t.Run(tt.mode, func(t *testing.T) {
			got := BuildBrief(snap, BriefOptions{
				Mode:        tt.mode,
				Full:        true,
				Now:         now,
				Verify:      &verify,
				PlanSummary: &planSummary,
			}).Rendered
			want, err := os.ReadFile(filepath.Join("testdata", tt.golden))
			if err != nil {
				t.Fatal(err)
			}
			if got != string(want) {
				t.Fatalf("brief mismatch\n--- got ---\n%s\n--- want ---\n%s", got, string(want))
			}
		})
	}
}

func TestBuildBriefBudgetKeepsHighSeverityContext(t *testing.T) {
	snap, verify, planSummary, now := briefFixture()
	budget := 900
	result := BuildBrief(snap, BriefOptions{
		Mode:        BriefModeImplement,
		BudgetChars: budget,
		Now:         now,
		Verify:      &verify,
		PlanSummary: &planSummary,
	})
	for _, want := range []string{
		"High severity open questions",
		"Should refresh tokens be single-use?",
		"High severity risks",
		"Token revocation list is missing",
		"Omitted due to budget",
	} {
		if !strings.Contains(result.Rendered, want) {
			t.Fatalf("compacted brief missing %q:\n%s", want, result.Rendered)
		}
	}
	if len(result.Rendered) > budget {
		t.Fatalf("brief length = %d, want <= %d:\n%s", len(result.Rendered), budget, result.Rendered)
	}
}

func TestBuildBriefFutureTimestampDoesNotSayAgo(t *testing.T) {
	snap, verify, planSummary, now := briefFixture()
	snap.CapturedAt = now.Add(time.Hour)
	result := BuildBrief(snap, BriefOptions{
		Mode:        BriefModeImplement,
		Full:        true,
		Now:         now,
		Verify:      &verify,
		PlanSummary: &planSummary,
	})
	if strings.Contains(result.Rendered, "in the future ago") {
		t.Fatalf("future timestamp rendered with impossible age wording:\n%s", result.Rendered)
	}
	if !strings.Contains(result.Rendered, "Captured   in the future") {
		t.Fatalf("future timestamp not surfaced clearly:\n%s", result.Rendered)
	}
}

func briefFixture() (*SnapshotV2, VerifyResult, PlanExecutionSummary, time.Time) {
	captured := time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
	now := captured.Add(30 * time.Minute)
	snap := &SnapshotV2{
		SchemaVersion: SnapshotSchemaV2,
		ID:            "snap_brief",
		Project:       ProjectRefV2{Name: "bifrost", Root: "/repo/bifrost"},
		CapturedAt:    captured,
		Source:        SourceV2{Tool: "claude-code"},
		Session: SessionStateV2{
			Intent:   "implementing",
			Pressure: "high",
			Task:     "Implement refresh token rotation",
			NextStep: "Inspect tests/tokens.test.ts before coding",
		},
		Observed: ObservedV2{
			Git: &GitObservedV2{Branch: "feature/auth", Commit: "1234567890abcdef"},
			Commands: []CommandObservedV2{{
				ID:         "cmd_1",
				Command:    "go test ./...",
				ExitCode:   1,
				CapturedAt: captured,
				Summary:    "token tests failing",
			}},
		},
		Interpretation: InterpretationV2{
			StatusItems: []StatusItemV2{{ID: "status_1", Text: "refresh token code claimed done", State: "claimed_done"}},
			OpenQuestions: []OpenQuestionV2{
				{ID: "q_1", Text: "Should refresh tokens be single-use?", Severity: "high"},
				{ID: "q_2", Text: "Which cache should store revocation?", Severity: "medium"},
			},
			Risks: []RiskV2{{
				ID:         "risk_1",
				Text:       "Token revocation list is missing",
				Severity:   "high",
				Mitigation: "Add revocation before production use",
			}},
		},
		ActiveFiles: []ActiveFileV2{{
			Path: "src/tokens.ts",
			Note: "rotation logic partially implemented",
			Trust: TrustV2{
				Implementation: "medium",
				Tests:          "low",
				Security:       "low",
				Architecture:   "medium",
				Freshness:      "stale",
				Evidence:       "weak",
			},
		}},
	}
	verify := VerifyResult{
		Status:                VerifyWarn,
		GeneratedAt:           now,
		RecommendedNextAction: "Do not assume tests pass; inspect failing token tests first.",
		Checks: []VerifyCheck{
			{ID: "snapshot.age", Status: VerifyPass, Message: "Snapshot is 30 minutes old."},
			{ID: "commands.test_claims", Status: VerifyWarn, Message: "Test claim has no passing evidence.", SafeNextAction: "Run the relevant test command."},
		},
	}
	planSummary := PlanExecutionSummary{
		Total:        3,
		VerifiedDone: 1,
		ClaimedDone:  1,
		Blocked:      1,
		Health:       74,
		NextAction:   "Verify claimed step step_2 before marking it done.",
		NextStep:     &PlanStepV2{ID: "step_2", Title: "Add refresh token rotation"},
	}
	return snap, verify, planSummary, now
}
