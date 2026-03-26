package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kogungor/bifrost/internal/snapshot"
	"github.com/kogungor/bifrost/internal/ui"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current state of the bridge",
	Long:  "Shows snapshot age, session intent, active plan, open question count, handoff note, snapshot history count, and project config status.",
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	root, err := resolveProject()
	if err != nil {
		ui.Error("No project root found.", "Run from inside a project directory, or run 'bifrost init' to initialize one.")
		return err
	}

	projectName := filepath.Base(root)

	ui.Blank()
	ui.Plain(fmt.Sprintf("Bifrost — %s", projectName))
	ui.Blank()

	// Snapshot
	snap, err := snapshot.Read(root)
	if err != nil {
		ui.Section("Snapshot", "none")
		ui.Blank()
		ui.Dim("No snapshot found.")
		ui.Blank()
		ui.Dim("Run /handoff in your AI coding tool to create one.")
		return nil
	}

	age := formatAge(snap.Timestamp)
	ui.Section("Snapshot", fmt.Sprintf(".bifrost/session.md  (%s)", age))
	if snap.SessionIntent != "" {
		ui.Section("Intent", snap.SessionIntent)
	}
	if snap.ActivePlanName != "" {
		ui.Section("Active plan", snap.ActivePlanName)
	}
	if len(snap.OpenQuestions) > 0 {
		ui.Section("Open questions", fmt.Sprintf("%d unresolved", len(snap.OpenQuestions)))
	}

	// Handoff note
	notePath := filepath.Join(root, ".bifrost", "handoff.md")
	if _, err := os.Stat(notePath); err == nil {
		ui.Section("Handoff note", ".bifrost/handoff.md  ✓")
	} else {
		ui.Section("Handoff note", "none")
	}

	// History count
	history, _ := snapshot.History(root)
	if len(history) > 0 {
		ui.Section("History", fmt.Sprintf("%d archived snapshots", len(history)))
	} else {
		ui.Section("History", "no archived snapshots")
	}

	// Config
	configPath := filepath.Join(root, "BIFROST.md")
	if _, err := os.Stat(configPath); err == nil {
		ui.Section("Config", "BIFROST.md  ✓")
	} else {
		ui.Section("Config", "no BIFROST.md")
	}

	ui.Blank()
	ui.Section("Last handoff", fmt.Sprintf("%s  (%s)", snap.SourceTool, snap.Timestamp.Format("2006-01-02 15:04")))
	ui.Blank()
	ui.Dim("Run /handin in your target tool to load this context.")

	return nil
}

func formatAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "yesterday"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}
