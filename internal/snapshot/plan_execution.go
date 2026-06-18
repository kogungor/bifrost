package snapshot

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type PlanExecutionSummary struct {
	Total          int
	VerifiedDone   int
	ClaimedDone    int
	InProgress     int
	Blocked        int
	NotStarted     int
	Invalidated    int
	FailedVerify   int
	UnverifiedDone int
	MissingFiles   int
	Health         int
	NextStep       *PlanStepV2
	NextAction     string
}

func LoadPlanForExecution(projectRoot, name string) (*PlanV2, error) {
	if err := ValidatePlanName(name); err != nil {
		return nil, err
	}
	if _, err := os.Stat(PlanJSONPath(projectRoot, name)); err == nil {
		return ReadPlanV2(projectRoot, name)
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	legacy, err := ReadPlan(projectRoot, name)
	if err != nil {
		return nil, err
	}
	return PlanToV2(legacy, name), nil
}

func WritePlanExecutionState(projectRoot string, plan *PlanV2) error {
	if plan == nil {
		return ValidatePlanV2(nil)
	}
	plan.UpdatedAt = time.Now().UTC()
	if err := WritePlanV2(projectRoot, plan); err != nil {
		return err
	}
	legacy, err := ReadPlan(projectRoot, plan.Name)
	if err != nil && !errors.Is(err, ErrNoPlan) {
		return err
	}
	if legacy == nil {
		legacy = PlanFromV2(plan)
	} else {
		applyPlanV2ToLegacy(legacy, plan)
	}
	return WritePlan(projectRoot, plan.Name, legacy)
}

func PlanExecutionSummaryFor(projectRoot string, plan *PlanV2) PlanExecutionSummary {
	summary := PlanExecutionSummary{Health: 100}
	if plan == nil {
		summary.Health = 0
		summary.NextAction = "Plan is missing."
		return summary
	}
	summary.Total = len(plan.Steps)
	for i := range plan.Steps {
		step := &plan.Steps[i]
		switch step.Status {
		case "verified_done":
			summary.VerifiedDone++
		case "claimed_done":
			summary.ClaimedDone++
			if step.Verification == nil || step.Verification.LastResult == nil || step.Verification.LastResult.State != "pass" {
				summary.UnverifiedDone++
			}
		case "in_progress":
			summary.InProgress++
		case "blocked":
			summary.Blocked++
		case "invalidated":
			summary.Invalidated++
		default:
			summary.NotStarted++
		}
		if step.Verification != nil && step.Verification.LastResult != nil && isFailedVerificationState(step.Verification.LastResult.State) {
			summary.FailedVerify++
		}
		summary.MissingFiles += missingExpectedFiles(projectRoot, step.ExpectedFiles)
	}
	summary.Health = planHealth(summary)
	step, action := nextPlanAction(plan)
	summary.NextStep = step
	summary.NextAction = action
	return summary
}

func SetPlanStepStatus(plan *PlanV2, stepID, status, blockedReason string) error {
	if plan == nil {
		return fmt.Errorf("plan is nil")
	}
	if !validWorkStates[status] {
		return fmt.Errorf("invalid step status: %s", status)
	}
	for i := range plan.Steps {
		if plan.Steps[i].ID != stepID {
			continue
		}
		plan.Steps[i].Status = status
		switch status {
		case "claimed_done":
			if plan.Steps[i].Verification == nil {
				plan.Steps[i].Verification = &PlanStepVerificationV2{}
			}
			plan.Steps[i].Verification.LastResult = &CommandResultRefV2{
				State:      "unverified",
				Command:    "manual claimed_done",
				CapturedAt: time.Now().UTC(),
			}
		case "verified_done":
			if plan.Steps[i].Verification == nil {
				plan.Steps[i].Verification = &PlanStepVerificationV2{}
			}
			plan.Steps[i].Verification.LastResult = &CommandResultRefV2{
				State:      "pass",
				Command:    "manual verified_done",
				CapturedAt: time.Now().UTC(),
			}
		case "blocked":
			if plan.Steps[i].Verification == nil {
				plan.Steps[i].Verification = &PlanStepVerificationV2{}
			}
			plan.Steps[i].Verification.LastResult = &CommandResultRefV2{
				State:      "fail",
				Command:    blockedReason,
				CapturedAt: time.Now().UTC(),
			}
		}
		return nil
	}
	return fmt.Errorf("step %q not found", stepID)
}

func applyPlanV2ToLegacy(legacy *Plan, plan *PlanV2) {
	legacy.Status = plan.Status
	legacy.UpdatedAt = plan.UpdatedAt
	legacy.PlanVersion = planVersionFromV2(plan)
	byID := map[string]PlanStepV2{}
	for _, step := range plan.Steps {
		byID[step.ID] = step
	}
	for i := range legacy.Steps {
		if step, ok := byID[legacy.Steps[i].ID]; ok {
			legacy.Steps[i].Status = planStepStatusFromV2(step.Status)
			legacy.Steps[i].Description = step.Title
			legacy.Steps[i].Files = step.ExpectedFiles
		}
	}
}

func planHealth(summary PlanExecutionSummary) int {
	score := 100
	score -= summary.Blocked * 10
	score -= summary.Invalidated * 10
	score -= summary.UnverifiedDone * 8
	score -= summary.FailedVerify * 8
	score -= summary.MissingFiles * 5
	if score < 0 {
		return 0
	}
	return score
}

func nextPlanAction(plan *PlanV2) (*PlanStepV2, string) {
	for i := range plan.Steps {
		step := &plan.Steps[i]
		if step.Verification != nil && step.Verification.LastResult != nil && isFailedVerificationState(step.Verification.LastResult.State) {
			return step, "Inspect failed verification for " + step.ID + " before continuing."
		}
	}
	for i := range plan.Steps {
		step := &plan.Steps[i]
		if step.Status == "claimed_done" {
			return step, "Verify claimed step " + step.ID + " before marking it done."
		}
	}
	for i := range plan.Steps {
		step := &plan.Steps[i]
		switch step.Status {
		case "not_started", "in_progress":
			return step, "Continue with step " + step.ID + "."
		case "blocked":
			return step, "Resolve blocked step " + step.ID + " before continuing."
		}
	}
	return nil, "All plan steps are verified or complete."
}

func missingExpectedFiles(projectRoot string, files []string) int {
	missing := 0
	for _, path := range files {
		if !isSafeRelativePath(path) {
			missing++
			continue
		}
		if _, err := os.Stat(projectPath(projectRoot, path)); err != nil {
			if os.IsNotExist(err) {
				missing++
			}
		}
	}
	return missing
}

func projectPath(projectRoot, rel string) string {
	rel = strings.ReplaceAll(rel, "\\", "/")
	return filepath.Join(projectRoot, filepath.FromSlash(rel))
}

func isFailedVerificationState(state string) bool {
	return state == "fail" || state == "failed"
}
