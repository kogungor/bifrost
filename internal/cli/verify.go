package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/kogungor/bifrost/internal/snapshot"
	"github.com/kogungor/bifrost/internal/ui"
	"github.com/spf13/cobra"
)

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify snapshot freshness, evidence, and local repo integrity",
	Long:  "Checks whether the current Bifrost snapshot still matches local git, active files, evidence, risks, questions, and plan status. The command is non-destructive.",
	RunE:  runVerify,
}

var verifyJSON bool
var verifyStrict bool
var verifyFix bool

func init() {
	verifyCmd.Flags().BoolVar(&verifyJSON, "json", false, "Print machine-readable JSON")
	verifyCmd.Flags().BoolVar(&verifyStrict, "strict", false, "Exit non-zero for warnings")
	verifyCmd.Flags().BoolVar(&verifyFix, "fix", false, "Apply safe non-destructive fixes when available")
	rootCmd.AddCommand(verifyCmd)
}

func runVerify(cmd *cobra.Command, args []string) error {
	root, err := resolveProject()
	if err != nil {
		ui.Error("Could not determine project root.", err.Error())
		return verifyExitError{code: 2}
	}
	snap, err := snapshot.SnapshotFromProject(root)
	if err != nil {
		if errors.Is(err, snapshot.ErrNoSnapshot) {
			result := snapshot.VerifySnapshotV2(root, nil, snapshot.VerifyOptions{Strict: verifyStrict})
			return printVerifyResult(result)
		}
		ui.Error("Could not read snapshot.", err.Error())
		return verifyExitError{code: 2}
	}
	result := snapshot.VerifySnapshotV2(root, snap, snapshot.VerifyOptions{
		Strict: verifyStrict,
		LoadActivePlan: func(name string) (string, error) {
			return snapshot.LoadPlanStatus(root, name)
		},
	})
	if verifyFix {
		result.Checks = append(result.Checks, snapshot.VerifyCheck{
			ID:             "fix",
			Status:         snapshot.VerifyInfo,
			Message:        "No safe automatic fixes are available yet.",
			SafeNextAction: "Review verify findings and refresh the snapshot manually if needed.",
		})
	}
	_ = snapshot.AppendTimelineEvent(root, snapshot.TimelineEvent{
		Timestamp: result.GeneratedAt,
		Type:      "verify." + result.Status,
		Snapshot:  snap.ID,
		Status:    result.Status,
	})
	return printVerifyResult(result)
}

func printVerifyResult(result snapshot.VerifyResult) error {
	code := verifyExitCode(result, verifyStrict)
	if verifyJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			return err
		}
		if code == 0 {
			return nil
		}
		return verifyExitError{code: code}
	}
	printVerifyHuman(result)
	if code == 0 {
		return nil
	}
	return verifyExitError{code: code}
}

func printVerifyHuman(result snapshot.VerifyResult) {
	ui.Blank()
	ui.Plain("Bifrost Verify")
	ui.Blank()
	ui.Section("Status", result.Status)
	if !result.GeneratedAt.IsZero() {
		ui.Section("Generated", result.GeneratedAt.UTC().Format(time.RFC3339))
	}
	ui.Blank()
	for _, check := range result.Checks {
		line := fmt.Sprintf("%s %s: %s", verifySymbol(check.Status), check.ID, check.Message)
		switch check.Status {
		case snapshot.VerifyFail:
			ui.Error(line, check.SafeNextAction)
		case snapshot.VerifyWarn:
			ui.Warning(line)
			if check.SafeNextAction != "" {
				ui.Dim("  " + check.SafeNextAction)
			}
		default:
			ui.Plain(line)
		}
	}
	ui.Blank()
	ui.Section("Recommended next action", result.RecommendedNextAction)
}

func verifySymbol(status string) string {
	switch status {
	case snapshot.VerifyPass:
		return "[OK]"
	case snapshot.VerifyWarn:
		return "[WARN]"
	case snapshot.VerifyFail:
		return "[FAIL]"
	default:
		return "[INFO]"
	}
}

func verifyExitCode(result snapshot.VerifyResult, strict bool) int {
	switch result.Status {
	case snapshot.VerifyFail:
		return 2
	case snapshot.VerifyWarn:
		if strict {
			return 1
		}
	}
	return 0
}

type verifyExitError struct {
	code int
}

func (e verifyExitError) Error() string {
	if e.code == 0 {
		return ""
	}
	return fmt.Sprintf("verify exited with code %d", e.code)
}

func (e verifyExitError) ExitCode() int {
	return e.code
}
