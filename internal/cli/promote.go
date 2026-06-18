package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/kogungor/bifrost/internal/project"
	"github.com/kogungor/bifrost/internal/ui"
	"github.com/spf13/cobra"
)

var promoteCmd = &cobra.Command{
	Use:   "promote [type]",
	Short: "Promote session knowledge into BIFROST.md with explicit acceptance",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runPromote,
}

var (
	promoteDryRun        bool
	promoteAccept        string
	promoteIgnore        string
	promoteIgnoreForever string
)

func init() {
	promoteCmd.Flags().BoolVar(&promoteDryRun, "dry-run", false, "Preview promotion patch without writing")
	promoteCmd.Flags().StringVar(&promoteAccept, "accept", "", "Apply candidate ID or all")
	promoteCmd.Flags().StringVar(&promoteIgnore, "ignore", "", "Ignore candidate ID(s) for this run")
	promoteCmd.Flags().StringVar(&promoteIgnoreForever, "ignore-forever", "", "Persistently ignore candidate ID(s)")
	rootCmd.AddCommand(promoteCmd)
}

func runPromote(cmd *cobra.Command, args []string) error {
	root, _, report, err := loadContextReport()
	if err != nil {
		return err
	}
	if err := validatePromotionType(args); err != nil {
		ui.Error("Invalid promotion type.", err.Error())
		return err
	}
	if promoteIgnoreForever != "" {
		if err := project.IgnorePromotionIDs(root, splitAcceptIDs(promoteIgnoreForever)); err != nil {
			ui.Error("Could not persist ignored promotion IDs.", err.Error())
			return err
		}
		ui.Success("Promotion candidate(s) ignored forever.")
	}
	report.Candidates = filterPromotionType(report.Candidates, args)
	report.Candidates = filterPromotionIgnoredForRun(report.Candidates, append(splitAcceptIDs(promoteIgnore), splitAcceptIDs(promoteIgnoreForever)...))
	ids := splitAcceptIDs(promoteAccept)
	if promoteAccept != "" {
		if err := validateAcceptedCandidateIDs(report.Candidates, ids); err != nil {
			if promoteDryRun {
				ui.Error("Could not preview promotion patch.", err.Error())
			} else {
				ui.Error("Could not promote candidate(s).", err.Error())
			}
			return err
		}
	}
	if promoteDryRun || promoteAccept == "" {
		fmt.Fprint(os.Stdout, project.ContextPatch(report, ids))
		if promoteAccept == "" && !promoteDryRun {
			ui.Dim("Dry-run only. Apply with `bifrost promote --accept <id|all>`.")
		}
		return nil
	}
	if err := project.ApplyContextCandidates(root, report, ids); err != nil {
		ui.Error("Could not promote candidate(s).", err.Error())
		return err
	}
	ui.Success("Promotion applied to BIFROST.md.")
	return nil
}

func validatePromotionType(args []string) error {
	if len(args) == 0 || args[0] == "" {
		return nil
	}
	if supportedPromotionTypes()[args[0]] {
		return nil
	}
	var types []string
	for kind := range supportedPromotionTypes() {
		types = append(types, kind)
	}
	sort.Strings(types)
	return fmt.Errorf("expected one of: %s", strings.Join(types, ", "))
}

func supportedPromotionTypes() map[string]bool {
	return map[string]bool{
		"command":          true,
		"convention":       true,
		"decision":         true,
		"env_example":      true,
		"framework":        true,
		"important_config": true,
		"package_manager":  true,
		"risk":             true,
	}
}

func filterPromotionType(candidates []project.ContextCandidate, args []string) []project.ContextCandidate {
	if len(args) == 0 || args[0] == "" {
		return candidates
	}
	kind := args[0]
	out := make([]project.ContextCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Type == kind {
			out = append(out, candidate)
		}
	}
	return out
}

func filterPromotionIgnoredForRun(candidates []project.ContextCandidate, ignored []string) []project.ContextCandidate {
	if len(ignored) == 0 {
		return candidates
	}
	set := map[string]bool{}
	for _, id := range ignored {
		set[id] = true
	}
	out := make([]project.ContextCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if !set[candidate.ID] {
			out = append(out, candidate)
		}
	}
	return out
}
