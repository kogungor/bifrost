package cli

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/kogungor/bifrost/internal/snapshot"
	"github.com/kogungor/bifrost/internal/ui"
	"github.com/spf13/cobra"
)

const planVerifyTimeout = 2 * time.Minute

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Inspect and update Bifrost plans",
}

var planStatusCmd = &cobra.Command{
	Use:   "status <name>",
	Short: "Show plan execution status and health",
	Args:  cobra.ExactArgs(1),
	RunE:  runPlanStatus,
}

var planNextCmd = &cobra.Command{
	Use:   "next <name>",
	Short: "Show the safest next plan action",
	Args:  cobra.ExactArgs(1),
	RunE:  runPlanNext,
}

var planVerifyCmd = &cobra.Command{
	Use:   "verify <name>",
	Short: "Run configured plan step verification commands",
	Args:  cobra.ExactArgs(1),
	RunE:  runPlanVerify,
}

var planStepCmd = &cobra.Command{
	Use:   "step <name> <step-id>",
	Short: "Update a plan step status",
	Args:  cobra.ExactArgs(2),
	RunE:  runPlanStep,
}

var (
	planStepClaimedDone  bool
	planStepVerifiedDone bool
	planStepBlocked      string
)

func init() {
	planStepCmd.Flags().BoolVar(&planStepClaimedDone, "claimed-done", false, "Mark the step claimed done")
	planStepCmd.Flags().BoolVar(&planStepVerifiedDone, "verified-done", false, "Mark the step verified done")
	planStepCmd.Flags().StringVar(&planStepBlocked, "blocked", "", "Mark the step blocked with a reason")
	planCmd.AddCommand(planStatusCmd, planNextCmd, planVerifyCmd, planStepCmd)
	rootCmd.AddCommand(planCmd)
}

func runPlanStatus(cmd *cobra.Command, args []string) error {
	root, plan, err := loadPlanForCLI(args[0])
	if err != nil {
		return err
	}
	printPlanStatus(root, plan)
	return nil
}

func runPlanNext(cmd *cobra.Command, args []string) error {
	root, plan, err := loadPlanForCLI(args[0])
	if err != nil {
		return err
	}
	summary := snapshot.PlanExecutionSummaryFor(root, plan)
	ui.Section("Plan next action", plan.Title)
	ui.Plain(summary.NextAction)
	if summary.NextStep != nil {
		ui.Plain(fmt.Sprintf("Step        %s", summary.NextStep.ID))
		ui.Plain(fmt.Sprintf("Title       %s", summary.NextStep.Title))
		if len(summary.NextStep.ExpectedFiles) > 0 {
			ui.Plain(fmt.Sprintf("Files       %s", strings.Join(summary.NextStep.ExpectedFiles, ", ")))
		}
	}
	return nil
}

func runPlanStep(cmd *cobra.Command, args []string) error {
	selected := 0
	status := ""
	reason := ""
	if planStepClaimedDone {
		selected++
		status = "claimed_done"
	}
	if planStepVerifiedDone {
		selected++
		status = "verified_done"
	}
	if planStepBlocked != "" {
		selected++
		status = "blocked"
		reason = planStepBlocked
	}
	if selected != 1 {
		err := fmt.Errorf("select exactly one step status flag")
		ui.Error("Invalid step update.", "Use one of --claimed-done, --verified-done, or --blocked.")
		return err
	}

	root, plan, err := loadPlanForCLI(args[0])
	if err != nil {
		return err
	}
	if err := snapshot.SetPlanStepStatus(plan, args[1], status, reason); err != nil {
		ui.Error("Could not update step.", err.Error())
		return err
	}
	if err := snapshot.WritePlanExecutionState(root, plan); err != nil {
		ui.Error("Could not write plan.", err.Error())
		return err
	}
	_ = snapshot.AppendTimelineEvent(root, snapshot.TimelineEvent{
		Type:   "plan.step.update",
		Plan:   plan.Name,
		Step:   args[1],
		Status: status,
	})
	ui.Success(fmt.Sprintf("Updated %s step %s to %s.", plan.Name, args[1], status))
	return nil
}

func runPlanVerify(cmd *cobra.Command, args []string) error {
	root, plan, err := loadPlanForCLI(args[0])
	if err != nil {
		return err
	}
	var evidence []snapshot.EvidenceV2
	ran := 0
	failed := 0
	now := time.Now().UTC()
	for i := range plan.Steps {
		step := &plan.Steps[i]
		if step.Verification == nil || len(step.Verification.Commands) == 0 {
			continue
		}
		for _, command := range step.Verification.Commands {
			ran++
			result := runPlanVerificationCommand(root, command)
			if result.exitCode != 0 {
				failed++
			}
			_, ev := snapshot.NewCommandEvidence(snapshot.ReportedCommand{
				Command:    command,
				ExitCode:   result.exitCode,
				CapturedAt: now,
				Summary:    result.summary,
				TestResult: strings.Contains(strings.ToLower(command), "test"),
			}, now)
			evidence = append(evidence, ev)
			state := "pass"
			if result.exitCode != 0 {
				state = "fail"
			}
			step.Verification.LastResult = &snapshot.CommandResultRefV2{
				State:       state,
				Command:     command,
				ExitCode:    result.exitCode,
				CapturedAt:  now,
				EvidenceRef: ev.ID,
			}
			if result.exitCode != 0 {
				break
			}
		}
		if step.Verification.LastResult != nil && step.Verification.LastResult.State == "pass" {
			step.Status = "verified_done"
		} else if step.Verification.LastResult != nil && step.Verification.LastResult.State == "fail" {
			step.Status = "invalidated"
		}
	}
	if ran == 0 {
		ui.Warning("No plan verification commands configured.")
		ui.Dim("Add verification.commands to the plan JSON or mark steps manually with `bifrost plan step`.")
		printPlanStatus(root, plan)
		return nil
	}
	if err := snapshot.WriteEvidenceRecordsV2(root, evidence); err != nil {
		ui.Error("Could not write verification evidence.", err.Error())
		return err
	}
	if err := snapshot.WritePlanExecutionState(root, plan); err != nil {
		ui.Error("Could not write plan verification result.", err.Error())
		return err
	}
	verifyStatus := "pass"
	if failed > 0 {
		verifyStatus = "fail"
	}
	_ = snapshot.AppendTimelineEvent(root, snapshot.TimelineEvent{
		Type:   "plan.verify." + verifyStatus,
		Plan:   plan.Name,
		Status: verifyStatus,
	})
	if failed > 0 {
		ui.Warning(fmt.Sprintf("Plan verification completed with %d failed command(s).", failed))
		printPlanStatus(root, plan)
		return verifyExitError{code: 2}
	}
	ui.Success(fmt.Sprintf("Plan verification passed (%d command(s)).", ran))
	printPlanStatus(root, plan)
	return nil
}

func loadPlanForCLI(name string) (string, *snapshot.PlanV2, error) {
	root, err := resolveProject()
	if err != nil {
		ui.Error("Could not determine project root.", err.Error())
		return "", nil, err
	}
	plan, err := snapshot.LoadPlanForExecution(root, name)
	if err != nil {
		ui.Error("Could not read plan.", err.Error())
		return "", nil, err
	}
	return root, plan, nil
}

func printPlanStatus(root string, plan *snapshot.PlanV2) {
	summary := snapshot.PlanExecutionSummaryFor(root, plan)
	ui.Section("Plan", plan.Title)
	ui.Plain(fmt.Sprintf("Name         %s", plan.Name))
	ui.Plain(fmt.Sprintf("Status       %s", plan.Status))
	ui.Plain(fmt.Sprintf("Health       %d/100", summary.Health))
	ui.Plain(fmt.Sprintf("Steps        %d total", summary.Total))
	ui.Plain(fmt.Sprintf("Verified     %d", summary.VerifiedDone))
	ui.Plain(fmt.Sprintf("Claimed      %d", summary.ClaimedDone))
	ui.Plain(fmt.Sprintf("In progress  %d", summary.InProgress))
	ui.Plain(fmt.Sprintf("Blocked      %d", summary.Blocked))
	ui.Plain(fmt.Sprintf("Not started  %d", summary.NotStarted))
	if summary.FailedVerify > 0 {
		ui.Plain(fmt.Sprintf("Failed verify %d", summary.FailedVerify))
	}
	if summary.MissingFiles > 0 {
		ui.Plain(fmt.Sprintf("Missing files %d", summary.MissingFiles))
	}
	ui.Section("Next safest action", summary.NextAction)
}

type commandRunResult struct {
	exitCode int
	summary  string
}

func runPlanVerificationCommand(projectRoot, command string) commandRunResult {
	ctx, cancel := context.WithTimeout(context.Background(), planVerifyTimeout)
	defer cancel()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()
	summary := truncateCommandOutput(string(output))
	if ctx.Err() == context.DeadlineExceeded {
		return commandRunResult{exitCode: -1, summary: "timed out after " + planVerifyTimeout.String() + ": " + summary}
	}
	if err == nil {
		return commandRunResult{exitCode: 0, summary: summary}
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return commandRunResult{exitCode: exitErr.ExitCode(), summary: summary}
	}
	return commandRunResult{exitCode: -1, summary: err.Error() + ": " + summary}
}

func truncateCommandOutput(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return "no output"
	}
	const limit = 500
	if len(output) <= limit {
		return output
	}
	return output[:limit] + "... (truncated)"
}
