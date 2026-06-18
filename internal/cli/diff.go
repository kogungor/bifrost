package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/kogungor/bifrost/internal/security"
	"github.com/kogungor/bifrost/internal/snapshot"
	"github.com/kogungor/bifrost/internal/ui"
	"github.com/spf13/cobra"
)

var diffCmd = &cobra.Command{
	Use:   "diff [latest~1..latest]",
	Short: "Show changes between the latest archived snapshot and current snapshot",
	Long:  "Shows a compact snapshot diff for task, next step, risks, open questions, active files, and trust changes. Supports human-readable output and --json.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runDiff,
}

var diffJSON bool

func init() {
	diffCmd.Flags().BoolVar(&diffJSON, "json", false, "Print machine-readable JSON")
	rootCmd.AddCommand(diffCmd)
}

func runDiff(cmd *cobra.Command, args []string) error {
	if len(args) == 1 && args[0] != "latest~1..latest" {
		err := fmt.Errorf("unsupported diff range %q", args[0])
		ui.Error("Invalid diff range.", "Supported range: latest~1..latest")
		return err
	}
	root, err := resolveProject()
	if err != nil {
		ui.Error("Could not determine project root.", err.Error())
		return err
	}
	from, to, err := loadLatestSnapshotPair(root)
	if err != nil {
		ui.Error("Could not prepare snapshot diff.", err.Error())
		return err
	}
	diff := sanitizeSnapshotDiff(root, snapshot.DiffSnapshots(from, to))
	if diffJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(diff)
	}
	printSnapshotDiff(diff)
	return nil
}

func loadLatestSnapshotPair(root string) (*snapshot.SnapshotV2, *snapshot.SnapshotV2, error) {
	current, err := snapshot.SnapshotFromProject(root)
	if err != nil {
		return nil, nil, err
	}
	if jsonHistory, err := snapshot.HistoryV2(root); err == nil && len(jsonHistory) > 0 {
		return jsonHistory[0], current, nil
	}
	history, err := snapshot.History(root)
	if err != nil {
		return nil, nil, err
	}
	if len(history) == 0 {
		return nil, nil, fmt.Errorf("no archived snapshots found")
	}
	return snapshot.SnapshotToV2(root, history[0]), current, nil
}

func printSnapshotDiff(diff snapshot.SnapshotDiff) {
	ui.Blank()
	ui.Plain("Snapshot diff")
	ui.Blank()
	ui.Section("From", diff.From)
	ui.Section("To", diff.To)
	if diff.TaskChanged != nil {
		ui.Blank()
		ui.Plain("Task changed:")
		ui.Plain("- " + emptyText(diff.TaskChanged.Before))
		ui.Plain("+ " + emptyText(diff.TaskChanged.After))
	}
	if diff.NextChanged != nil {
		ui.Blank()
		ui.Plain("Next step changed:")
		ui.Plain("- " + emptyText(diff.NextChanged.Before))
		ui.Plain("+ " + emptyText(diff.NextChanged.After))
	}
	printDiffList("New risks", diff.NewRisks, "+")
	printDiffList("Resolved risks", diff.ResolvedRisks, "-")
	printDiffList("New open questions", diff.NewQuestions, "+")
	printDiffList("Resolved open questions", diff.ResolvedQs, "-")
	printDiffList("Active files added", diff.ActiveFiles.Added, "+")
	printDiffList("Active files removed", diff.ActiveFiles.Removed, "-")
	if len(diff.TrustChanges) > 0 {
		ui.Blank()
		ui.Plain("Trust changes:")
		for _, change := range diff.TrustChanges {
			ui.Plain(fmt.Sprintf("- %s %s: %s -> %s", change.Path, change.Dimension, emptyText(change.Before), emptyText(change.After)))
		}
	}
	if diff.TaskChanged == nil && diff.NextChanged == nil && len(diff.NewRisks) == 0 && len(diff.ResolvedRisks) == 0 && len(diff.NewQuestions) == 0 && len(diff.ResolvedQs) == 0 && len(diff.ActiveFiles.Added) == 0 && len(diff.ActiveFiles.Removed) == 0 && len(diff.TrustChanges) == 0 {
		ui.Plain("No meaningful snapshot changes detected.")
	}
}

func printDiffList(title string, items []string, prefix string) {
	if len(items) == 0 {
		return
	}
	ui.Blank()
	ui.Plain(title + ":")
	for _, item := range items {
		ui.Plain(prefix + " " + item)
	}
}

func emptyText(value string) string {
	if strings.TrimSpace(value) == "" {
		return "(empty)"
	}
	return value
}

func sanitizeSnapshotDiff(root string, diff snapshot.SnapshotDiff) snapshot.SnapshotDiff {
	cfg := security.LoadConfig(root)
	diff.From = redactCLIString(diff.From, cfg)
	diff.To = redactCLIString(diff.To, cfg)
	if diff.TaskChanged != nil {
		diff.TaskChanged.Before = redactCLIString(diff.TaskChanged.Before, cfg)
		diff.TaskChanged.After = redactCLIString(diff.TaskChanged.After, cfg)
	}
	if diff.NextChanged != nil {
		diff.NextChanged.Before = redactCLIString(diff.NextChanged.Before, cfg)
		diff.NextChanged.After = redactCLIString(diff.NextChanged.After, cfg)
	}
	diff.NewRisks = redactCLIStrings(diff.NewRisks, cfg)
	diff.ResolvedRisks = redactCLIStrings(diff.ResolvedRisks, cfg)
	diff.NewQuestions = redactCLIStrings(diff.NewQuestions, cfg)
	diff.ResolvedQs = redactCLIStrings(diff.ResolvedQs, cfg)
	diff.ActiveFiles.Added = redactCLIStrings(diff.ActiveFiles.Added, cfg)
	diff.ActiveFiles.Removed = redactCLIStrings(diff.ActiveFiles.Removed, cfg)
	diff.ActiveFiles.Common = redactCLIStrings(diff.ActiveFiles.Common, cfg)
	for i := range diff.TrustChanges {
		diff.TrustChanges[i].Path = redactCLIString(diff.TrustChanges[i].Path, cfg)
		diff.TrustChanges[i].Dimension = redactCLIString(diff.TrustChanges[i].Dimension, cfg)
		diff.TrustChanges[i].Before = redactCLIString(diff.TrustChanges[i].Before, cfg)
		diff.TrustChanges[i].After = redactCLIString(diff.TrustChanges[i].After, cfg)
	}
	return diff
}

func redactCLIStrings(items []string, cfg security.Config) []string {
	for i := range items {
		items[i] = redactCLIString(items[i], cfg)
	}
	return items
}

func redactCLIString(value string, cfg security.Config) string {
	redacted, _ := security.RedactString(value, cfg)
	return redacted
}
