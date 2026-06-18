package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/kogungor/bifrost/internal/snapshot"
	"github.com/kogungor/bifrost/internal/ui"
	"github.com/spf13/cobra"
)

var renderCmd = &cobra.Command{
	Use:   "render",
	Short: "Render JSON v2 state as Markdown",
	Long:  "Renders a canonical JSON v2 snapshot or plan into the existing human-readable Markdown format.",
	RunE:  runRender,
}

var renderSnapshotPath string
var renderPlanPath string
var renderOutputPath string

func init() {
	renderCmd.Flags().StringVar(&renderSnapshotPath, "snapshot", "", "Render a snapshot JSON file")
	renderCmd.Flags().StringVar(&renderPlanPath, "plan", "", "Render a plan JSON file")
	renderCmd.Flags().StringVarP(&renderOutputPath, "output", "o", "", "Write rendered Markdown to a file instead of stdout")
	rootCmd.AddCommand(renderCmd)
}

func runRender(cmd *cobra.Command, args []string) error {
	if renderSnapshotPath != "" && renderPlanPath != "" {
		err := fmt.Errorf("use --snapshot or --plan, not both")
		ui.Error("Invalid render arguments.", err.Error())
		return err
	}
	root, err := resolveProject()
	if err != nil {
		ui.Error("Could not determine project root.", err.Error())
		return err
	}

	var rendered string
	if renderPlanPath != "" {
		rendered, err = renderPlanJSON(renderPlanPath)
	} else {
		path := renderSnapshotPath
		if path == "" {
			path = snapshot.SnapshotJSONPath(root)
		}
		rendered, err = renderSnapshotJSON(path)
	}
	if err != nil {
		return err
	}
	if renderOutputPath != "" {
		if err := os.WriteFile(renderOutputPath, []byte(rendered), 0600); err != nil {
			ui.Error("Could not write rendered Markdown.", err.Error())
			return err
		}
		ui.Success(fmt.Sprintf("Rendered Markdown written to %s", renderOutputPath))
		return nil
	}
	fmt.Fprint(os.Stdout, rendered)
	return nil
}

func renderSnapshotJSON(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		ui.Error("Could not read snapshot JSON.", err.Error())
		return "", err
	}
	var snap snapshot.SnapshotV2
	if err := json.Unmarshal(data, &snap); err != nil {
		ui.Error("Invalid snapshot JSON.", err.Error())
		return "", err
	}
	if err := snapshot.ValidateSnapshotV2(&snap); err != nil {
		ui.Error("Invalid snapshot schema.", err.Error())
		return "", err
	}
	return snapshot.Render(snapshot.SnapshotFromV2(&snap)), nil
}

func renderPlanJSON(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		ui.Error("Could not read plan JSON.", err.Error())
		return "", err
	}
	var plan snapshot.PlanV2
	if err := json.Unmarshal(data, &plan); err != nil {
		ui.Error("Invalid plan JSON.", err.Error())
		return "", err
	}
	if err := snapshot.ValidatePlanV2(&plan); err != nil {
		ui.Error("Invalid plan schema.", err.Error())
		return "", err
	}
	return snapshot.RenderPlan(snapshot.PlanFromV2(&plan)), nil
}
