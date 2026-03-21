package cli

import (
	"fmt"
	"strconv"

	"github.com/kog/bifrost/internal/snapshot"
	"github.com/kog/bifrost/internal/ui"
	"github.com/spf13/cobra"
)

var restoreCmd = &cobra.Command{
	Use:   "restore [number]",
	Short: "Restore a historical snapshot as the active one",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runRestore,
}

func init() {
	rootCmd.AddCommand(restoreCmd)
}

func runRestore(cmd *cobra.Command, args []string) error {
	root, err := resolveProject()
	if err != nil {
		ui.Error("No project root found.", "Run from inside a project directory.")
		return err
	}

	history, err := snapshot.History(root)
	if err != nil {
		ui.Error("Could not read history.", err.Error())
		return err
	}

	if len(history) == 0 {
		ui.Warning("No archived snapshots found.")
		ui.Dim("Snapshots are archived automatically each time you run /handoff.")
		return nil
	}

	if len(args) == 0 {
		// Show list and ask user to re-run with a number
		ui.Blank()
		ui.Plain("Available snapshots:")
		ui.Blank()
		for i, snap := range history {
			task := snap.CurrentTask
			if task == "" {
				task = "(no task)"
			}
			ui.Plain(fmt.Sprintf("  %d  %s  %s  %s",
				i+1,
				snap.Timestamp.Format("2006-01-02 15:04"),
				snap.SourceTool,
				truncate(task, 50)))
		}
		ui.Blank()
		ui.Dim("Run 'bifrost restore <n>' to restore a snapshot.")
		return nil
	}

	n, err := strconv.Atoi(args[0])
	if err != nil || n < 1 {
		ui.Error(fmt.Sprintf("Invalid snapshot number: %s", args[0]), "Run 'bifrost history' to see available snapshots.")
		return fmt.Errorf("invalid snapshot number")
	}

	index := n - 1 // user-facing is 1-based
	if index >= len(history) {
		ui.Error(fmt.Sprintf("Snapshot %d not found.", n), "Run 'bifrost history' to see available snapshots.")
		return fmt.Errorf("snapshot not found")
	}

	if err := snapshot.Restore(root, index); err != nil {
		ui.Error("Could not restore snapshot.", err.Error())
		return err
	}

	snap := history[index]
	ui.Blank()
	ui.Success("Snapshot restored.")
	ui.Section("Task", snap.CurrentTask)
	ui.Section("From", fmt.Sprintf("%s  %s", snap.SourceTool, snap.Timestamp.Format("2006-01-02 15:04")))
	ui.Blank()
	ui.Dim("Run /handin in your AI coding tool to load this context.")

	return nil
}
