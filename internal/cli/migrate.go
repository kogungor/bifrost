package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kogungor/bifrost/internal/snapshot"
	"github.com/kogungor/bifrost/internal/ui"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate legacy Markdown state to JSON v2",
	Long:  "Migrates existing Markdown snapshots and plans into canonical JSON v2 files. Existing JSON files are not overwritten.",
	RunE:  runMigrate,
}

var migrateDryRun bool

func init() {
	migrateCmd.Flags().BoolVar(&migrateDryRun, "dry-run", false, "Show generated JSON without writing files")
	rootCmd.AddCommand(migrateCmd)
}

func runMigrate(cmd *cobra.Command, args []string) error {
	root, err := resolveProject()
	if err != nil {
		ui.Error("Could not determine project root.", err.Error())
		return err
	}

	migrated := 0
	if fileExists(snapshot.SessionPath(root)) {
		snap, err := snapshot.Read(root)
		if err != nil {
			ui.Error("Could not read legacy snapshot.", err.Error())
			return err
		}
		v2 := snapshot.SnapshotToV2(root, snap)
		if err := snapshot.ValidateSnapshotV2(v2); err != nil {
			ui.Error("Generated snapshot JSON is invalid.", err.Error())
			return err
		}
		if migrateDryRun {
			printMigrationPreview(snapshot.SnapshotJSONPath(root), v2)
		} else if fileExists(snapshot.SnapshotJSONPath(root)) {
			ui.Warning(fmt.Sprintf("Skipping existing %s", snapshot.SnapshotJSONPath(root)))
		} else {
			if err := snapshot.WriteSnapshotV2(root, v2); err != nil {
				ui.Error("Could not write snapshot JSON.", err.Error())
				return err
			}
			ui.Success(fmt.Sprintf("Wrote %s", snapshot.SnapshotJSONPath(root)))
		}
		migrated++
	}

	names, err := snapshot.ListPlans(root)
	if err != nil {
		ui.Error("Could not list legacy plans.", err.Error())
		return err
	}
	for _, name := range names {
		plan, err := snapshot.ReadPlan(root, name)
		if err != nil {
			ui.Warning(fmt.Sprintf("Skipping unreadable plan %s: %s", name, err.Error()))
			continue
		}
		v2 := snapshot.PlanToV2(plan, name)
		if err := snapshot.ValidatePlanV2(v2); err != nil {
			ui.Error(fmt.Sprintf("Generated plan JSON is invalid for %s.", name), err.Error())
			return err
		}
		target := snapshot.PlanJSONPath(root, name)
		if migrateDryRun {
			printMigrationPreview(target, v2)
		} else if fileExists(target) {
			ui.Warning(fmt.Sprintf("Skipping existing %s", target))
		} else {
			if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
				ui.Error("Could not create plans directory.", err.Error())
				return err
			}
			if err := snapshot.WritePlanV2(root, v2); err != nil {
				ui.Error(fmt.Sprintf("Could not write plan JSON for %s.", name), err.Error())
				return err
			}
			ui.Success(fmt.Sprintf("Wrote %s", target))
		}
		migrated++
	}

	if migrated == 0 {
		ui.Warning("No legacy Markdown snapshot or plans found.")
		return nil
	}
	if migrateDryRun {
		ui.Success(fmt.Sprintf("Dry-run generated %d JSON file(s).", migrated))
	} else {
		ui.Success(fmt.Sprintf("Migration processed %d file(s).", migrated))
	}
	return nil
}

func printMigrationPreview(path string, v any) {
	ui.Plain(fmt.Sprintf("Would write %s", path))
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		ui.Warning(fmt.Sprintf("Could not render preview for %s: %s", path, err.Error()))
		return
	}
	fmt.Fprintln(os.Stdout, string(data))
}
