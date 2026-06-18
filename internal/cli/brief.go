package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/kogungor/bifrost/internal/snapshot"
	"github.com/kogungor/bifrost/internal/ui"
	"github.com/spf13/cobra"
)

var briefCmd = &cobra.Command{
	Use:   "brief",
	Short: "Print a compact mode-aware Bifrost briefing",
	Long:  "Builds a deterministic briefing from the current snapshot, verification checks, trust signals, active files, risks, questions, and active plan health.",
	RunE:  runBrief,
}

var (
	briefMode   string
	briefBudget int
	briefFull   bool
	briefJSON   bool
)

func init() {
	briefCmd.Flags().StringVar(&briefMode, "mode", snapshot.BriefModeImplement, "Briefing mode: implement, debug, review, plan, or handoff")
	briefCmd.Flags().IntVar(&briefBudget, "budget", 5000, "Maximum briefing size in characters before compacting")
	briefCmd.Flags().BoolVar(&briefFull, "full", false, "Disable compaction and print all available briefing context")
	briefCmd.Flags().BoolVar(&briefJSON, "json", false, "Print machine-readable JSON")
	rootCmd.AddCommand(briefCmd)
}

func runBrief(cmd *cobra.Command, args []string) error {
	if !validBriefMode(briefMode) {
		err := fmt.Errorf("invalid brief mode %q", briefMode)
		ui.Error("Invalid brief mode.", "Use one of: implement, debug, review, plan, handoff.")
		return err
	}
	root, err := resolveProject()
	if err != nil {
		ui.Error("Could not determine project root.", err.Error())
		return err
	}
	snap, err := snapshot.SnapshotFromProject(root)
	if err != nil {
		ui.Error("Could not read snapshot.", err.Error())
		return err
	}
	verify := snapshot.VerifySnapshotV2(root, snap, snapshot.VerifyOptions{
		LoadActivePlan: func(name string) (string, error) {
			return snapshot.LoadPlanStatus(root, name)
		},
	})
	var planSummary *snapshot.PlanExecutionSummary
	if snap.ActivePlan != nil && snap.ActivePlan.Name != "" {
		if plan, err := snapshot.LoadPlanForExecution(root, snap.ActivePlan.Name); err == nil {
			summary := snapshot.PlanExecutionSummaryFor(root, plan)
			planSummary = &summary
		}
	}
	result := snapshot.BuildBrief(snap, snapshot.BriefOptions{
		Mode:        briefMode,
		BudgetChars: briefBudget,
		Full:        briefFull,
		Verify:      &verify,
		PlanSummary: planSummary,
	})
	if briefJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}
	fmt.Fprint(os.Stdout, result.Rendered)
	return nil
}

func validBriefMode(mode string) bool {
	switch mode {
	case snapshot.BriefModeImplement, snapshot.BriefModeDebug, snapshot.BriefModeReview, snapshot.BriefModePlan, snapshot.BriefModeHandoff:
		return true
	default:
		return false
	}
}
