package cli

import (
	"fmt"
	"strconv"
	"time"

	"github.com/kogungor/bifrost/internal/snapshot"
	"github.com/kogungor/bifrost/internal/ui"
	"github.com/spf13/cobra"
)

var restoreCmd = &cobra.Command{
	Use:   "restore [number]",
	Short: "Restore a historical snapshot as the active one",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runRestore,
}

var restorePreview bool

func init() {
	restoreCmd.Flags().BoolVar(&restorePreview, "preview", false, "Preview the selected restore without modifying files")
	rootCmd.AddCommand(restoreCmd)
}

func runRestore(cmd *cobra.Command, args []string) error {
	root, err := resolveProject()
	if err != nil {
		ui.Error("No project root found.", "Run from inside a project directory.")
		return err
	}

	history, err := restoreHistory(root)
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
			task := snap.task()
			if task == "" {
				task = "(no task)"
			}
			ui.Plain(fmt.Sprintf("  %d  %s  %s  %s",
				i+1,
				snap.timestamp().Format("2006-01-02 15:04"),
				snap.sourceTool(),
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

	if restorePreview {
		printRestorePreview(root, n, history[index])
		return nil
	}

	if history[index].v2 != nil {
		if err := snapshot.RestoreSnapshotV2(root, history[index].v2); err != nil {
			ui.Error("Could not restore snapshot.", err.Error())
			return err
		}
	} else if err := snapshot.Restore(root, index); err != nil {
		ui.Error("Could not restore snapshot.", err.Error())
		return err
	}

	snap := history[index]
	ui.Blank()
	ui.Success("Snapshot restored.")
	ui.Section("Task", snap.task())
	ui.Section("From", fmt.Sprintf("%s  %s", snap.sourceTool(), snap.timestamp().Format("2006-01-02 15:04")))
	ui.Blank()
	ui.Dim("Run /handin in your AI coding tool to load this context.")

	return nil
}

type restoreEntry struct {
	legacy *snapshot.Snapshot
	v2     *snapshot.SnapshotV2
}

func restoreHistory(root string) ([]restoreEntry, error) {
	legacyHistory, err := snapshot.History(root)
	if err != nil {
		return nil, err
	}
	if len(legacyHistory) > 0 {
		entries := make([]restoreEntry, 0, len(legacyHistory))
		for _, item := range legacyHistory {
			entries = append(entries, restoreEntry{legacy: item})
		}
		return entries, nil
	}
	jsonHistory, err := snapshot.HistoryV2(root)
	if err != nil {
		return nil, err
	}
	entries := make([]restoreEntry, 0, len(jsonHistory))
	for _, item := range jsonHistory {
		entries = append(entries, restoreEntry{v2: item})
	}
	return entries, nil
}

func (e restoreEntry) task() string {
	if e.v2 != nil {
		return e.v2.Session.Task
	}
	if e.legacy != nil {
		return e.legacy.CurrentTask
	}
	return ""
}

func (e restoreEntry) sourceTool() string {
	if e.v2 != nil {
		return e.v2.Source.Tool
	}
	if e.legacy != nil {
		return e.legacy.SourceTool
	}
	return ""
}

func (e restoreEntry) timestamp() time.Time {
	if e.v2 != nil {
		return e.v2.CapturedAt
	}
	if e.legacy != nil {
		return e.legacy.Timestamp
	}
	return time.Time{}
}

func (e restoreEntry) snapshotV2(root string) *snapshot.SnapshotV2 {
	if e.v2 != nil {
		return e.v2
	}
	return snapshot.SnapshotToV2(root, e.legacy)
}

func printRestorePreview(root string, n int, selected restoreEntry) {
	ui.Blank()
	ui.Plain("Restore preview")
	ui.Blank()
	ui.Section("Snapshot", fmt.Sprintf("%d", n))
	ui.Section("Task", selected.task())
	ui.Section("From", fmt.Sprintf("%s  %s", selected.sourceTool(), selected.timestamp().Format("2006-01-02 15:04")))
	current, err := snapshot.SnapshotFromProject(root)
	if err != nil {
		ui.Dim("Current snapshot could not be read; restore preview is limited.")
		ui.Dim("No files were modified.")
		return
	}
	diff := sanitizeSnapshotDiff(root, snapshot.DiffSnapshots(current, selected.snapshotV2(root)))
	printSnapshotDiff(diff)
	ui.Blank()
	ui.Dim("Preview only. Run `bifrost restore <n>` without --preview to restore.")
}
