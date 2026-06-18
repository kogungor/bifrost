package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/kogungor/bifrost/internal/snapshot"
	"github.com/kogungor/bifrost/internal/ui"
	"github.com/spf13/cobra"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Print structured state as JSON to stdout",
	Long:  "Export the current snapshot and/or plans as JSON. Useful for CI pipelines, scripts, or any tooling that can't use the MCP server.",
	RunE:  runExport,
}

var exportFormat string

func init() {
	exportCmd.Flags().StringVar(&exportFormat, "format", "snapshot", "What to export: snapshot, plans, or all")
	rootCmd.AddCommand(exportCmd)
}

func runExport(cmd *cobra.Command, args []string) error {
	projectRoot, err := resolveProject()
	if err != nil {
		ui.Error("Could not determine project root", err.Error())
		return err
	}

	switch exportFormat {
	case "snapshot":
		return exportSnapshot(projectRoot)
	case "plans":
		return exportPlans(projectRoot)
	case "all":
		out := map[string]any{}
		snap, err := buildSnapshotExport(projectRoot)
		if err == nil {
			out["snapshot"] = snap
		} else if !errors.Is(err, snapshot.ErrNoSnapshot) {
			return err
		}
		plans, err := buildPlansExport(projectRoot)
		if err != nil {
			return err
		}
		out["plans"] = plans
		return printJSON(out)
	default:
		ui.Error(fmt.Sprintf("Unknown format: %s", exportFormat), "Use snapshot, plans, or all")
		return fmt.Errorf("unknown format")
	}
}

func exportSnapshot(projectRoot string) error {
	out, err := buildSnapshotExport(projectRoot)
	if err != nil {
		if errors.Is(err, snapshot.ErrNoSnapshot) {
			ui.Warning("No snapshot found")
			return nil
		}
		return err
	}
	return printJSON(out)
}

func buildSnapshotExport(projectRoot string) (map[string]any, error) {
	if fileExists(snapshot.SnapshotJSONPath(projectRoot)) {
		snapV2, err := snapshot.ReadSnapshotV2(projectRoot)
		if err != nil {
			return nil, err
		}
		return buildSnapshotExportFromSnapshot(snapshot.SnapshotFromV2(snapV2), projectRoot)
	}
	snap, err := snapshot.Read(projectRoot)
	if err != nil {
		return nil, err
	}
	return buildSnapshotExportFromSnapshot(snap, projectRoot)
}

func buildSnapshotExportFromSnapshot(snap *snapshot.Snapshot, projectRoot string) (map[string]any, error) {
	note, _ := snapshot.ReadNote(projectRoot)
	var handoffNote string
	if note != nil {
		handoffNote = note.Text
	}

	activeFiles := make([]map[string]string, len(snap.ActiveFiles))
	for i, f := range snap.ActiveFiles {
		activeFiles[i] = map[string]string{
			"path":       f.Path,
			"note":       f.Note,
			"confidence": f.Confidence,
		}
	}

	out := map[string]any{
		"found":       true,
		"age_seconds": int(math.Round(snap.Age().Seconds())),
		"snapshot": map[string]any{
			"bifrost_version":   snap.BifrostVersion,
			"timestamp":         snap.Timestamp.UTC().Format(time.RFC3339),
			"source_tool":       snap.SourceTool,
			"project":           snap.Project,
			"token_pressure":    snap.TokenPressure,
			"session_intent":    snap.SessionIntent,
			"active_plan_name":  snap.ActivePlanName,
			"git_sha":           snap.GitSHA,
			"session_start":     snap.SessionStart,
			"current_task":      snap.CurrentTask,
			"status":            snap.Status,
			"active_files":      activeFiles,
			"decisions":         snap.Decisions,
			"environment_notes": snap.EnvNotes,
			"next_step":         snap.NextStep,
			"assumptions":       snap.Assumptions,
			"open_questions":    snap.OpenQuestions,
			"risks":             snap.Risks,
		},
	}

	if handoffNote != "" {
		out["handoff_note"] = handoffNote
	}

	return out, nil
}

func exportPlans(projectRoot string) error {
	out, err := buildPlansExport(projectRoot)
	if err != nil {
		return err
	}
	return printJSON(map[string]any{"plans": out})
}

func buildPlansExport(projectRoot string) ([]map[string]any, error) {
	names, err := snapshot.ListPlans(projectRoot)
	if err != nil {
		return nil, err
	}

	result := make([]map[string]any, 0, len(names))
	seen := map[string]bool{}
	jsonPlans, _ := filepath.Glob(filepath.Join(snapshot.PlansDir(projectRoot), "*.json"))
	for _, path := range jsonPlans {
		name := planNameFromJSONPath(path)
		p, err := snapshot.ReadPlanV2(projectRoot, name)
		if err != nil {
			continue
		}
		result = append(result, buildPlanExportMap(name, snapshot.PlanFromV2(p)))
		seen[name] = true
	}

	for _, name := range names {
		if seen[name] {
			continue
		}
		p, err := snapshot.ReadPlan(projectRoot, name)
		if err != nil {
			continue
		}
		result = append(result, buildPlanExportMap(name, p))
	}

	return result, nil
}

func buildPlanExportMap(name string, p *snapshot.Plan) map[string]any {
	steps := make([]map[string]any, len(p.Steps))
	for i, s := range p.Steps {
		files := s.Files
		if files == nil {
			files = []string{}
		}
		steps[i] = map[string]any{
			"id":          s.ID,
			"description": s.Description,
			"status":      s.Status,
			"files":       files,
		}
	}

	reviewNotes := make([]map[string]any, len(p.ReviewNotes))
	for i, rn := range p.ReviewNotes {
		reviewNotes[i] = map[string]any{
			"from":         rn.From,
			"at":           rn.At.UTC().Format(time.RFC3339),
			"plan_version": rn.PlanVersion,
			"outcome":      rn.Outcome,
			"text":         rn.Text,
		}
	}

	done, pending, blocked := p.StepSummary()
	return map[string]any{
		"name":                  name,
		"bifrost_version":       p.BifrostVersion,
		"created_at":            p.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at":            p.UpdatedAt.UTC().Format(time.RFC3339),
		"source_tool":           p.SourceTool,
		"project":               p.Project,
		"status":                p.Status,
		"plan_version":          p.PlanVersion,
		"proposed_by":           p.ProposedBy,
		"revision_count":        p.RevisionCount,
		"consensus_state":       p.ConsensusState,
		"activation_reason":     p.ActivationReason,
		"deadlock_detected":     p.DeadlockDetected,
		"latest_review_outcome": p.LatestReviewOutcome(),
		"title":                 p.Title,
		"goal":                  p.Goal,
		"steps":                 steps,
		"constraints":           p.Constraints,
		"review_notes":          reviewNotes,
		"completion_pct":        p.CompletionPct(),
		"steps_done":            done,
		"steps_pending":         pending,
		"steps_blocked":         blocked,
	}
}

func planNameFromJSONPath(path string) string {
	return filepath.Base(path[:len(path)-len(filepath.Ext(path))])
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
