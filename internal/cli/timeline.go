package cli

import (
	"fmt"

	"github.com/kogungor/bifrost/internal/snapshot"
	"github.com/kogungor/bifrost/internal/ui"
	"github.com/spf13/cobra"
)

var timelineCmd = &cobra.Command{
	Use:   "timeline",
	Short: "Show local Bifrost integrity events",
	RunE:  runTimeline,
}

var timelineLimit int

func init() {
	timelineCmd.Flags().IntVar(&timelineLimit, "limit", 20, "Maximum number of events to show")
	rootCmd.AddCommand(timelineCmd)
}

func runTimeline(cmd *cobra.Command, args []string) error {
	root, err := resolveProject()
	if err != nil {
		ui.Error("Could not determine project root.", err.Error())
		return err
	}
	events, err := snapshot.ReadTimeline(root, timelineLimit)
	if err != nil {
		ui.Error("Could not read timeline.", err.Error())
		return err
	}
	if len(events) == 0 {
		ui.Warning("No timeline events found.")
		ui.Dim("Events are written by snapshot, verify, and plan operations.")
		return nil
	}
	ui.Blank()
	ui.Plain("Bifrost timeline")
	ui.Blank()
	for _, event := range events {
		ui.Plain(formatTimelineEvent(event))
	}
	return nil
}

func formatTimelineEvent(event snapshot.TimelineEvent) string {
	ts := "unknown-time"
	if !event.Timestamp.IsZero() {
		ts = event.Timestamp.UTC().Format("2006-01-02 15:04:05")
	}
	parts := []string{ts, event.Type}
	if event.Status != "" {
		parts = append(parts, "status="+event.Status)
	}
	if event.Snapshot != "" {
		parts = append(parts, "snapshot="+event.Snapshot)
	}
	if event.Plan != "" {
		parts = append(parts, "plan="+event.Plan)
	}
	if event.Step != "" {
		parts = append(parts, "step="+event.Step)
	}
	if event.Check != "" {
		parts = append(parts, "check="+event.Check)
	}
	if event.Task != "" {
		parts = append(parts, fmt.Sprintf("task=%q", truncate(event.Task, 60)))
	}
	return "  " + joinTimelineParts(parts)
}

func joinTimelineParts(parts []string) string {
	out := ""
	for i, part := range parts {
		if i > 0 {
			out += "  "
		}
		out += part
	}
	return out
}
