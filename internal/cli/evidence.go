package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/kogungor/bifrost/internal/snapshot"
	"github.com/kogungor/bifrost/internal/ui"
	"github.com/spf13/cobra"
)

var evidenceCmd = &cobra.Command{
	Use:   "evidence",
	Short: "Inspect snapshot evidence records",
}

var evidenceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List evidence records",
	RunE:  runEvidenceList,
}

var evidenceShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show one evidence record as JSON",
	Args:  cobra.ExactArgs(1),
	RunE:  runEvidenceShow,
}

func init() {
	evidenceCmd.AddCommand(evidenceListCmd)
	evidenceCmd.AddCommand(evidenceShowCmd)
	rootCmd.AddCommand(evidenceCmd)
}

func runEvidenceList(cmd *cobra.Command, args []string) error {
	root, err := resolveProject()
	if err != nil {
		ui.Error("Could not determine project root.", err.Error())
		return err
	}
	evidence, err := loadEvidence(root)
	if err != nil {
		ui.Error("Could not read evidence.", err.Error())
		return err
	}
	if len(evidence) == 0 {
		ui.Warning("No evidence records found.")
		ui.Dim("Run `bifrost snapshot --enrich` first.")
		return nil
	}
	ui.Section("Evidence", fmt.Sprintf("%d record(s)", len(evidence)))
	for _, ev := range evidence {
		ui.Plain(fmt.Sprintf("  %-14s %-16s %-18s %s", ev.ID, ev.Type, ev.Source, shortEvidenceSummary(ev)))
	}
	return nil
}

func runEvidenceShow(cmd *cobra.Command, args []string) error {
	root, err := resolveProject()
	if err != nil {
		ui.Error("Could not determine project root.", err.Error())
		return err
	}
	evidence, err := loadEvidence(root)
	if err != nil {
		ui.Error("Could not read evidence.", err.Error())
		return err
	}
	for _, ev := range evidence {
		if ev.ID == args[0] {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(ev)
		}
	}
	err = fmt.Errorf("evidence not found: %s", args[0])
	ui.Error("Evidence not found.", args[0])
	return err
}

func loadEvidence(root string) ([]snapshot.EvidenceV2, error) {
	byID := map[string]snapshot.EvidenceV2{}
	if fileExists(snapshot.SnapshotJSONPath(root)) {
		snap, err := snapshot.ReadSnapshotV2(root)
		if err != nil {
			return nil, err
		}
		for _, ev := range snap.Evidence {
			byID[ev.ID] = ev
		}
	}
	external, err := snapshot.ListEvidenceV2(root)
	if err != nil {
		return nil, err
	}
	for _, ev := range external {
		byID[ev.ID] = ev
	}
	evidence := make([]snapshot.EvidenceV2, 0, len(byID))
	for _, ev := range byID {
		evidence = append(evidence, ev)
	}
	sort.Slice(evidence, func(i, j int) bool {
		if !evidence[i].ObservedAt.Equal(evidence[j].ObservedAt) {
			return evidence[i].ObservedAt.After(evidence[j].ObservedAt)
		}
		return evidence[i].ID < evidence[j].ID
	})
	return evidence, nil
}

func shortEvidenceSummary(ev snapshot.EvidenceV2) string {
	when := "unknown time"
	if !ev.ObservedAt.IsZero() {
		when = ev.ObservedAt.UTC().Format(time.RFC3339)
	}
	if ev.Summary == "" {
		return when
	}
	return when + " - " + ev.Summary
}
