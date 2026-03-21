package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kog/bifrost/internal/snapshot"
	"github.com/kog/bifrost/internal/ui"
	"github.com/spf13/cobra"
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "List archived snapshots",
	RunE:  runHistory,
}

var historyLimit int

func init() {
	historyCmd.Flags().IntVar(&historyLimit, "limit", 10, "Maximum number of entries to show")
	rootCmd.AddCommand(historyCmd)
}

func runHistory(cmd *cobra.Command, args []string) error {
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
		ui.Blank()
		ui.Warning("No archived snapshots found.")
		ui.Dim("Snapshots are archived automatically each time you run /handoff.")
		return nil
	}

	projectName := filepath.Base(root)

	ui.Blank()
	ui.Plain(fmt.Sprintf("Snapshot history — %s", projectName))
	ui.Blank()
	ui.Plain(fmt.Sprintf("  %-3s %-22s %-13s %-14s %s", "#", "Timestamp", "Age", "Source", "Task"))
	ui.Plain(fmt.Sprintf("  %-3s %-22s %-13s %-14s %s", "─", strings.Repeat("─", 21), strings.Repeat("─", 12), strings.Repeat("─", 13), strings.Repeat("─", 30)))

	limit := historyLimit
	if limit > len(history) {
		limit = len(history)
	}

	for i, snap := range history[:limit] {
		ts := snap.Timestamp.Format("2006-01-02 15:04:05")
		age := formatAge(snap.Timestamp)
		task := snap.CurrentTask
		if len(task) > 40 {
			task = task[:37] + "..."
		}
		if task == "" {
			task = "(no task)"
		}

		ui.Plain(fmt.Sprintf("  %-3d %-22s %-13s %-14s %s", i+1, ts, age, snap.SourceTool, task))
	}

	if len(history) > limit {
		ui.Blank()
		ui.Dim(fmt.Sprintf("  Showing %d of %d. Use --limit to see more.", limit, len(history)))
	}

	ui.Blank()
	ui.Dim("Run 'bifrost restore <n>' to restore a snapshot.")

	// Also check for handoff note
	notePath := filepath.Join(root, ".bifrost", "handoff.md")
	if _, err := os.Stat(notePath); err == nil {
		data, _ := os.ReadFile(notePath)
		if len(data) > 0 {
			// Extract note text (skip frontmatter)
			note := extractNoteText(string(data))
			if note != "" {
				ui.Blank()
				ui.Dim(fmt.Sprintf("  Latest handoff note: %s", truncate(note, 60)))
			}
		}
	}

	return nil
}

func extractNoteText(content string) string {
	// Skip YAML frontmatter
	if strings.HasPrefix(content, "---\n") {
		rest := content[4:]
		idx := strings.Index(rest, "\n---\n")
		if idx >= 0 {
			content = strings.TrimSpace(rest[idx+4:])
		}
	}
	// Return first non-empty line
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
