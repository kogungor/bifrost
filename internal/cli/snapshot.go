package cli

import (
	"errors"
	"fmt"

	"github.com/kogungor/bifrost/internal/snapshot"
	"github.com/kogungor/bifrost/internal/ui"
	"github.com/spf13/cobra"
)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Manage canonical snapshot state",
	RunE:  runSnapshot,
}

var snapshotEnrich bool

func init() {
	snapshotCmd.Flags().BoolVar(&snapshotEnrich, "enrich", false, "Collect observed git/file/project facts and evidence into session.json")
	rootCmd.AddCommand(snapshotCmd)
}

func runSnapshot(cmd *cobra.Command, args []string) error {
	if !snapshotEnrich {
		err := fmt.Errorf("no snapshot action selected")
		ui.Error("Invalid snapshot arguments.", "Use --enrich.")
		return err
	}
	root, err := resolveProject()
	if err != nil {
		ui.Error("Could not determine project root.", err.Error())
		return err
	}
	snap, err := readSnapshotForEnrich(root)
	if err != nil {
		if errors.Is(err, snapshot.ErrNoSnapshot) {
			ui.Warning("No snapshot found.")
			ui.Dim("Run /handoff first, then `bifrost snapshot --enrich`.")
			return nil
		}
		ui.Error("Could not read snapshot.", err.Error())
		return err
	}
	if err := snapshot.EnrichSnapshotV2(root, snap); err != nil {
		ui.Error("Could not enrich snapshot.", err.Error())
		return err
	}
	if err := snapshot.WriteSnapshotV2(root, snap); err != nil {
		ui.Error("Could not write enriched snapshot JSON.", err.Error())
		return err
	}
	if err := snapshot.WriteEvidenceRecordsV2(root, snap.Evidence); err != nil {
		ui.Error("Could not write evidence records.", err.Error())
		return err
	}
	ui.Success(fmt.Sprintf("Enriched snapshot with %d evidence record(s).", len(snap.Evidence)))
	ui.Dim(fmt.Sprintf("Wrote %s", snapshot.SnapshotJSONPath(root)))
	return nil
}

func readSnapshotForEnrich(root string) (*snapshot.SnapshotV2, error) {
	if fileExists(snapshot.SnapshotJSONPath(root)) {
		return snapshot.ReadSnapshotV2(root)
	}
	legacy, err := snapshot.Read(root)
	if err != nil {
		return nil, err
	}
	return snapshot.SnapshotToV2(root, legacy), nil
}
