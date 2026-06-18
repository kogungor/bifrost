package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/kogungor/bifrost/internal/project"
	"github.com/kogungor/bifrost/internal/snapshot"
	"github.com/kogungor/bifrost/internal/ui"
	"github.com/spf13/cobra"
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Analyze and update durable BIFROST.md project context",
}

var contextCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check BIFROST.md for missing durable project context",
	RunE:  runContextCheck,
}

var contextUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Preview or apply BIFROST.md context updates",
	RunE:  runContextUpdate,
}

var (
	contextJSON   bool
	contextDryRun bool
	contextAccept string
)

func init() {
	contextCheckCmd.Flags().BoolVar(&contextJSON, "json", false, "Print machine-readable JSON")
	contextUpdateCmd.Flags().BoolVar(&contextDryRun, "dry-run", false, "Preview BIFROST.md patch without writing")
	contextUpdateCmd.Flags().StringVar(&contextAccept, "accept", "", "Apply candidate ID or all")
	contextCmd.AddCommand(contextCheckCmd, contextUpdateCmd)
	rootCmd.AddCommand(contextCmd)
}

func runContextCheck(cmd *cobra.Command, args []string) error {
	root, snap, report, err := loadContextReport()
	if err != nil {
		return err
	}
	_ = root
	_ = snap
	if contextJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	printContextReport(report)
	return nil
}

func runContextUpdate(cmd *cobra.Command, args []string) error {
	root, _, report, err := loadContextReport()
	if err != nil {
		return err
	}
	ids := splitAcceptIDs(contextAccept)
	if contextAccept != "" {
		if err := validateAcceptedCandidateIDs(report.Candidates, ids); err != nil {
			if contextDryRun {
				ui.Error("Could not preview BIFROST.md patch.", err.Error())
			} else {
				ui.Error("Could not update BIFROST.md.", err.Error())
			}
			return err
		}
	}
	if contextDryRun || contextAccept == "" {
		fmt.Fprint(os.Stdout, project.ContextPatch(report, ids))
		if contextAccept == "" && !contextDryRun {
			ui.Dim("Dry-run only. Apply with `bifrost context update --accept <id|all>`.")
		}
		return nil
	}
	if len(ids) == 0 {
		return fmt.Errorf("missing accepted candidate id")
	}
	if err := project.ApplyContextCandidates(root, report, ids); err != nil {
		ui.Error("Could not update BIFROST.md.", err.Error())
		return err
	}
	ui.Success("BIFROST.md updated.")
	return nil
}

func validateAcceptedCandidateIDs(candidates []project.ContextCandidate, ids []string) error {
	if len(ids) == 0 {
		return fmt.Errorf("missing accepted candidate id")
	}
	if missing := project.MissingAcceptedCandidateIDs(candidates, ids); len(missing) > 0 {
		return fmt.Errorf("unknown promotion candidate ID(s): %s", strings.Join(missing, ", "))
	}
	return nil
}

func loadContextReport() (string, *snapshot.SnapshotV2, project.ContextReport, error) {
	root, err := resolveProject()
	if err != nil {
		ui.Error("Could not determine project root.", err.Error())
		return "", nil, project.ContextReport{}, err
	}
	var snap *snapshot.SnapshotV2
	if current, err := snapshot.SnapshotFromProject(root); err == nil {
		snap = current
	}
	report, err := project.AnalyzeContext(root, snap)
	if err != nil {
		ui.Error("Could not analyze BIFROST.md.", err.Error())
		return "", nil, project.ContextReport{}, err
	}
	return root, snap, report, nil
}

func printContextReport(report project.ContextReport) {
	ui.Section("BIFROST.md", report.Path)
	if report.Exists {
		ui.Success("BIFROST.md exists")
	} else {
		ui.Warning("BIFROST.md is missing")
	}
	if len(report.MissingSections) > 0 {
		ui.Warning("Missing sections: " + strings.Join(report.MissingSections, ", "))
	}
	if len(report.Placeholders) > 0 {
		ui.Warning("Placeholder sections: " + strings.Join(report.Placeholders, ", "))
	}
	if len(report.Contradictions) > 0 {
		ui.Warning("Possible stale or contradictory entries:")
		for _, item := range report.Contradictions {
			ui.Dim("  " + item)
		}
	}
	if len(report.Ignored) > 0 {
		ui.Dim(fmt.Sprintf("%d ignored promotion candidate(s)", len(report.Ignored)))
	}
	if len(report.Candidates) == 0 {
		ui.Success("No promotion candidates.")
		return
	}
	ui.Section("Promotion candidates", fmt.Sprintf("%d candidate(s)", len(report.Candidates)))
	for _, candidate := range report.Candidates {
		ui.Plain(fmt.Sprintf("%s  [%s -> %s] %s", candidate.ID, candidate.Type, candidate.RecommendedSection, candidate.Text))
		ui.Dim("  " + candidate.Reason)
	}
	ui.Dim("Preview with `bifrost context update --dry-run`; apply with `bifrost context update --accept <id|all>`.")
}

func splitAcceptIDs(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
