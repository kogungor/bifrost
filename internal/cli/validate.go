package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kogungor/bifrost/internal/snapshot"
	"github.com/kogungor/bifrost/internal/ui"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate Bifrost snapshot and plan files",
	Long:  "Validates canonical JSON v2 snapshots/plans or legacy Markdown snapshots/plans without changing files.",
	RunE:  runValidate,
}

var validateSnapshotPath string
var validatePlanPath string

func init() {
	validateCmd.Flags().StringVar(&validateSnapshotPath, "snapshot", "", "Validate a specific snapshot file")
	validateCmd.Flags().StringVar(&validatePlanPath, "plan", "", "Validate a specific plan file")
	rootCmd.AddCommand(validateCmd)
}

func runValidate(cmd *cobra.Command, args []string) error {
	if validateSnapshotPath != "" && validatePlanPath != "" {
		err := fmt.Errorf("use --snapshot or --plan, not both")
		ui.Error("Invalid validate arguments.", err.Error())
		return err
	}
	if validateSnapshotPath != "" {
		return validateSnapshotFile(validateSnapshotPath)
	}
	if validatePlanPath != "" {
		return validatePlanFile(validatePlanPath)
	}

	root, err := resolveProject()
	if err != nil {
		ui.Error("Could not determine project root.", err.Error())
		return err
	}

	validated := 0
	if fileExists(snapshot.SnapshotJSONPath(root)) {
		if err := validateSnapshotFile(snapshot.SnapshotJSONPath(root)); err != nil {
			return err
		}
		validated++
	} else if fileExists(snapshot.SessionPath(root)) {
		if err := validateSnapshotFile(snapshot.SessionPath(root)); err != nil {
			return err
		}
		validated++
	}

	for _, path := range planValidationPaths(root) {
		if err := validatePlanFile(path); err != nil {
			return err
		}
		validated++
	}

	if validated == 0 {
		ui.Warning("No Bifrost snapshot or plan files found.")
		return nil
	}
	ui.Success(fmt.Sprintf("Validated %d Bifrost file(s).", validated))
	return nil
}

func validateSnapshotFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		ui.Error("Could not read snapshot.", err.Error())
		return err
	}
	if isJSONPath(path) {
		var snap snapshot.SnapshotV2
		if err := json.Unmarshal(data, &snap); err != nil {
			ui.Error("Invalid snapshot JSON.", err.Error())
			return err
		}
		if err := snapshot.ValidateSnapshotV2(&snap); err != nil {
			ui.Error("Invalid snapshot schema.", err.Error())
			return err
		}
		ui.Success(fmt.Sprintf("Snapshot valid: %s", path))
		return nil
	}
	if _, err := snapshot.Parse(data); err != nil {
		ui.Error("Invalid legacy snapshot Markdown.", err.Error())
		return err
	}
	ui.Success(fmt.Sprintf("Snapshot valid: %s", path))
	return nil
}

func validatePlanFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		ui.Error("Could not read plan.", err.Error())
		return err
	}
	if isJSONPath(path) {
		var plan snapshot.PlanV2
		if err := json.Unmarshal(data, &plan); err != nil {
			ui.Error("Invalid plan JSON.", err.Error())
			return err
		}
		if err := snapshot.ValidatePlanV2(&plan); err != nil {
			ui.Error("Invalid plan schema.", err.Error())
			return err
		}
		ui.Success(fmt.Sprintf("Plan valid: %s", path))
		return nil
	}
	if _, err := snapshot.ParsePlan(data); err != nil {
		ui.Error("Invalid legacy plan Markdown.", err.Error())
		return err
	}
	ui.Success(fmt.Sprintf("Plan valid: %s", path))
	return nil
}

func planValidationPaths(root string) []string {
	var paths []string
	jsonPlans, _ := filepath.Glob(filepath.Join(snapshot.PlansDir(root), "*.json"))
	paths = append(paths, jsonPlans...)
	markdownPlans, _ := filepath.Glob(filepath.Join(snapshot.Dir(root), "*.plan.md"))
	paths = append(paths, markdownPlans...)
	return paths
}

func isJSONPath(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".json")
}
