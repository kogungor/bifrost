package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kogungor/bifrost/internal/snapshot"
)

// Input size limits to prevent resource exhaustion.
const (
	maxFieldLen   = 10000 // max chars per text field
	maxArrayItems = 100   // max items per array field
	maxNoteLen    = 50000 // max chars for handoff note
)

// ToolDefinition describes a tool for tools/list.
type ToolDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"inputSchema"`
}

// ToolSet holds the tool handlers for a project.
type ToolSet struct {
	projectRoot string
}

// NewToolSet creates a ToolSet bound to a project root.
func NewToolSet(projectRoot string) *ToolSet {
	return &ToolSet{projectRoot: projectRoot}
}

// Definitions returns the 4 tool definitions with JSON schemas.
func (ts *ToolSet) Definitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "bifrost_read_snapshot",
			Description: "Read the current Bifrost session snapshot. Returns all fields: task, status, active files (with confidence), decisions, environment notes, next step, session intent, active plan name, git SHA, assumptions, open questions, risks, and handoff note.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "bifrost_write_snapshot",
			Description: "Write a new Bifrost session snapshot. Archives the previous snapshot automatically, fills in timestamp, project name, and git SHA. Accepts semantic enrichments: session_intent (planning|implementing|debugging|reviewing), assumptions, open_questions, risks, active_plan_name, and confidence on each active file.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"source_tool": map[string]any{
						"type":        "string",
						"description": "The AI tool writing the snapshot (e.g. claude-code, opencode)",
					},
					"token_pressure": map[string]any{
						"type":        "string",
						"enum":        []string{"low", "medium", "high", "critical"},
						"description": "Current token/context pressure level",
					},
					"current_task": map[string]any{
						"type":        "string",
						"description": "What is being worked on right now",
					},
					"status": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Checklist items, e.g. [\"[x] done\", \"[ ] todo\"]",
					},
					"active_files": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"path":       map[string]any{"type": "string"},
								"note":       map[string]any{"type": "string"},
								"confidence": map[string]any{"type": "string", "enum": []string{"high", "medium", "low"}, "description": "How certain the AI is about the file's state"},
							},
							"required": []string{"path"},
						},
						"description": "Files currently being worked on",
					},
					"decisions": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Key decisions made during this session",
					},
					"environment_notes": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Environment-specific notes (versions, configs, etc.)",
					},
					"next_step": map[string]any{
						"type":        "string",
						"description": "What should be done next",
					},
					"session_intent": map[string]any{
						"type":        "string",
						"enum":        []string{"planning", "implementing", "debugging", "reviewing"},
						"description": "What mode the session was in — helps the incoming tool resume correctly",
					},
					"active_plan_name": map[string]any{
						"type":        "string",
						"description": "Name of the plan being executed (maps to .bifrost/<name>.plan.md)",
					},
					"session_start": map[string]any{
						"type":        "string",
						"description": "RFC3339 timestamp of when this session began",
					},
					"assumptions": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Things the AI assumed but isn't certain about — surfaces trust signals for the incoming tool",
					},
					"open_questions": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Unresolved questions the incoming tool should address before proceeding",
					},
					"risks": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Known risks or blockers the incoming tool should be aware of",
					},
					"commands": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"id":          map[string]any{"type": "string"},
								"command":     map[string]any{"type": "string"},
								"exit_code":   map[string]any{"type": "number"},
								"captured_at": map[string]any{"type": "string"},
								"summary":     map[string]any{"type": "string"},
								"test_result": map[string]any{"type": "boolean"},
							},
							"required": []string{"command", "exit_code"},
						},
						"description": "Caller-reported command results to record as evidence. Bifrost does not execute these commands.",
					},
					"evidence": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"id":          map[string]any{"type": "string"},
								"type":        map[string]any{"type": "string"},
								"source":      map[string]any{"type": "string"},
								"observed_at": map[string]any{"type": "string"},
								"summary":     map[string]any{"type": "string"},
								"data":        map[string]any{"type": "object"},
							},
							"required": []string{"id", "type", "source", "observed_at"},
						},
						"description": "Optional prebuilt evidence records to attach to session.json.",
					},
					"manual_evidence": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"id":          map[string]any{"type": "string"},
								"text":        map[string]any{"type": "string"},
								"source":      map[string]any{"type": "string"},
								"observed_at": map[string]any{"type": "string"},
							},
							"required": []string{"text"},
						},
						"description": "Explicit user or human notes to record as manual_note evidence.",
					},
				},
				"required": []string{"source_tool", "current_task"},
			},
		},
		{
			Name:        "bifrost_write_note",
			Description: "Write a freeform handoff note to .bifrost/handoff.md for the next session.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"text": map[string]any{
						"type":        "string",
						"description": "The handoff note text",
					},
					"from": map[string]any{
						"type":        "string",
						"description": "Who is writing the note (e.g. claude-code)",
					},
				},
				"required": []string{"text", "from"},
			},
		},
		{
			Name:        "bifrost_status",
			Description: "Get a quick status summary: snapshot existence, age, project name, session intent, active plan name, open question count, handoff note presence, history count, and plan count.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "bifrost_read_plan",
			Description: "Read a named Bifrost implementation plan. Returns the plan with title, goal, steps, constraints, review notes, and completion percentage.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Plan name (defaults to \"plan\"). Maps to .bifrost/<name>.plan.md",
					},
				},
			},
		},
		{
			Name:        "bifrost_write_plan",
			Description: "Create or overwrite a named Bifrost implementation plan. Automatically fills in timestamps, project name, and bifrost_version.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"source_tool": map[string]any{
						"type":        "string",
						"description": "The AI tool writing the plan (e.g. claude-code, opencode)",
					},
					"title": map[string]any{
						"type":        "string",
						"description": "Plan title",
					},
					"name": map[string]any{
						"type":        "string",
						"description": "Plan name (defaults to \"plan\"). Maps to .bifrost/<name>.plan.md",
					},
					"goal": map[string]any{
						"type":        "string",
						"description": "What the plan aims to achieve",
					},
					"steps": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"description": map[string]any{"type": "string"},
								"files":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
							},
							"required": []string{"description"},
						},
						"description": "Implementation steps",
					},
					"constraints": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Constraints or rules to follow",
					},
				},
				"required": []string{"source_tool", "title"},
			},
		},
		{
			Name:        "bifrost_update_plan",
			Description: "Update an existing Bifrost plan: submit a review outcome (approved/needs_revision), revise after feedback, force-accept to override deadlock, update step statuses, or change plan status.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Plan name (defaults to \"plan\")",
					},
					"source_tool": map[string]any{
						"type":        "string",
						"description": "The AI tool making this update — used as reviewer identity in review notes",
					},
					"review_outcome": map[string]any{
						"type":        "string",
						"enum":        []string{"approved", "needs_revision"},
						"description": "Review decision. 'approved' activates the plan (consensus reached). 'needs_revision' records feedback and returns the plan to the planner.",
					},
					"review_feedback": map[string]any{
						"type":        "string",
						"description": "Detailed review findings. Required when review_outcome is needs_revision. Saved as a review note.",
					},
					"force_accept": map[string]any{
						"type":        "boolean",
						"description": "Override consensus and activate the plan immediately. Sets consensus_state to overridden. Use when deadlocked.",
					},
					"revise": map[string]any{
						"type":        "boolean",
						"description": "Signal a deliberate revision in response to review feedback. Increments plan_version and revision_count, resets consensus_state to none.",
					},
					"plan_status": map[string]any{
						"type":        "string",
						"enum":        []string{"draft", "active", "completed", "archived"},
						"description": "Manually update the plan lifecycle status. Ignored when review_outcome or force_accept is set.",
					},
					"review_notes": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"from": map[string]any{"type": "string"},
								"text": map[string]any{"type": "string"},
							},
							"required": []string{"from", "text"},
						},
						"description": "Legacy: append freeform review notes without consensus logic.",
					},
					"step_updates": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"index":       map[string]any{"type": "integer", "description": "0-based step index"},
								"status":      map[string]any{"type": "string", "enum": []string{"pending", "done", "blocked"}},
								"description": map[string]any{"type": "string", "description": "New step description — resets approval if plan was already approved"},
								"files":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "New file list (replaces existing)"},
							},
							"required": []string{"index"},
						},
						"description": "Step updates (status, description, and/or files)",
					},
				},
			},
		},
		{
			Name:        "bifrost_delete_plan",
			Description: "Delete a named Bifrost plan.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Plan name (defaults to \"plan\")",
					},
				},
			},
		},
		{
			Name:        "bifrost_list_plans",
			Description: "List all Bifrost plans in the project. Returns plan names with status and completion percentage.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}
}

// Call dispatches a tool call by name.
func (ts *ToolSet) Call(name string, args json.RawMessage) (any, error) {
	switch name {
	case "bifrost_read_snapshot":
		return ts.readSnapshot()
	case "bifrost_write_snapshot":
		return ts.writeSnapshot(args)
	case "bifrost_write_note":
		return ts.writeNote(args)
	case "bifrost_status":
		return ts.status()
	case "bifrost_read_plan":
		return ts.readPlan(args)
	case "bifrost_write_plan":
		return ts.writePlan(args)
	case "bifrost_update_plan":
		return ts.updatePlan(args)
	case "bifrost_delete_plan":
		return ts.deletePlan(args)
	case "bifrost_list_plans":
		return ts.listPlans()
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// collectGitSHA returns the current HEAD SHA of the repository at dir.
// Returns an empty string if git is unavailable or the directory is not a repo.
func collectGitSHA(dir string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (ts *ToolSet) readSnapshot() (any, error) {
	snap, snapV2, err := ts.readSnapshotState()
	if err != nil {
		if errors.Is(err, snapshot.ErrNoSnapshot) {
			return map[string]any{"found": false}, nil
		}
		return nil, err
	}

	note, _ := snapshot.ReadNote(ts.projectRoot)
	var handoffNote string
	if note != nil {
		handoffNote = note.Text
	}

	activeFiles := make([]map[string]string, len(snap.ActiveFiles))
	for i, f := range snap.ActiveFiles {
		activeFiles[i] = map[string]string{"path": f.Path, "note": f.Note, "confidence": f.Confidence}
	}

	result := map[string]any{
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
		result["handoff_note"] = handoffNote
	}
	if snapV2 != nil {
		snapshotData := result["snapshot"].(map[string]any)
		snapshotData["observed"] = snapV2.Observed
		snapshotData["evidence"] = snapV2.Evidence
		snapshotData["integrity"] = snapV2.Integrity
	}

	return result, nil
}

func (ts *ToolSet) readSnapshotState() (*snapshot.Snapshot, *snapshot.SnapshotV2, error) {
	snapV2, err := snapshot.ReadSnapshotV2(ts.projectRoot)
	if err == nil {
		return snapshot.SnapshotFromV2(snapV2), snapV2, nil
	}
	if !errors.Is(err, snapshot.ErrNoSnapshot) {
		return nil, nil, err
	}
	snap, err := snapshot.Read(ts.projectRoot)
	if err != nil {
		return nil, nil, err
	}
	return snap, nil, nil
}

// validSessionIntents is the set of allowed session_intent values.
var validSessionIntents = map[string]bool{
	"planning": true, "implementing": true, "debugging": true, "reviewing": true,
}

// validConfidences is the set of allowed confidence values on active files.
var validConfidences = map[string]bool{
	"high": true, "medium": true, "low": true,
}

// writeSnapshotParams maps the JSON input for bifrost_write_snapshot.
type writeSnapshotParams struct {
	SourceTool     string   `json:"source_tool"`
	TokenPressure  string   `json:"token_pressure"`
	SessionIntent  string   `json:"session_intent"`
	ActivePlanName string   `json:"active_plan_name"`
	SessionStart   string   `json:"session_start"`
	CurrentTask    string   `json:"current_task"`
	Status         []string `json:"status"`
	ActiveFiles    []struct {
		Path       string `json:"path"`
		Note       string `json:"note"`
		Confidence string `json:"confidence"`
	} `json:"active_files"`
	Decisions     []string `json:"decisions"`
	EnvNotes      []string `json:"environment_notes"`
	NextStep      string   `json:"next_step"`
	Assumptions   []string `json:"assumptions"`
	OpenQuestions []string `json:"open_questions"`
	Risks         []string `json:"risks"`
	Commands      []struct {
		ID         string `json:"id"`
		Command    string `json:"command"`
		ExitCode   int    `json:"exit_code"`
		CapturedAt string `json:"captured_at"`
		Summary    string `json:"summary"`
		TestResult bool   `json:"test_result"`
	} `json:"commands"`
	Evidence []struct {
		ID         string         `json:"id"`
		Type       string         `json:"type"`
		Source     string         `json:"source"`
		ObservedAt string         `json:"observed_at"`
		Summary    string         `json:"summary"`
		Data       map[string]any `json:"data"`
	} `json:"evidence"`
	ManualEvidence []struct {
		ID         string `json:"id"`
		Text       string `json:"text"`
		Source     string `json:"source"`
		ObservedAt string `json:"observed_at"`
	} `json:"manual_evidence"`
}

func (ts *ToolSet) writeSnapshot(args json.RawMessage) (any, error) {
	var params writeSnapshotParams
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	if params.SourceTool == "" || params.CurrentTask == "" {
		return nil, fmt.Errorf("source_tool and current_task are required")
	}

	// Validate field sizes
	if len(params.CurrentTask) > maxFieldLen {
		return nil, fmt.Errorf("current_task exceeds %d characters", maxFieldLen)
	}
	if len(params.NextStep) > maxFieldLen {
		return nil, fmt.Errorf("next_step exceeds %d characters", maxFieldLen)
	}
	if len(params.SourceTool) > 100 {
		return nil, fmt.Errorf("source_tool exceeds 100 characters")
	}

	// Validate enum fields
	if params.SessionIntent != "" && !validSessionIntents[params.SessionIntent] {
		return nil, fmt.Errorf("invalid session_intent %q: must be planning, implementing, debugging, or reviewing", params.SessionIntent)
	}

	// Validate array sizes
	if len(params.Status) > maxArrayItems {
		return nil, fmt.Errorf("status exceeds %d items", maxArrayItems)
	}
	if len(params.ActiveFiles) > maxArrayItems {
		return nil, fmt.Errorf("active_files exceeds %d items", maxArrayItems)
	}
	if len(params.Decisions) > maxArrayItems {
		return nil, fmt.Errorf("decisions exceeds %d items", maxArrayItems)
	}
	if len(params.EnvNotes) > maxArrayItems {
		return nil, fmt.Errorf("environment_notes exceeds %d items", maxArrayItems)
	}
	if len(params.Assumptions) > maxArrayItems {
		return nil, fmt.Errorf("assumptions exceeds %d items", maxArrayItems)
	}
	if len(params.OpenQuestions) > maxArrayItems {
		return nil, fmt.Errorf("open_questions exceeds %d items", maxArrayItems)
	}
	if len(params.Risks) > maxArrayItems {
		return nil, fmt.Errorf("risks exceeds %d items", maxArrayItems)
	}
	if len(params.Commands) > maxArrayItems {
		return nil, fmt.Errorf("commands exceeds %d items", maxArrayItems)
	}
	if len(params.Evidence) > maxArrayItems {
		return nil, fmt.Errorf("evidence exceeds %d items", maxArrayItems)
	}
	if len(params.ManualEvidence) > maxArrayItems {
		return nil, fmt.Errorf("manual_evidence exceeds %d items", maxArrayItems)
	}
	if err := validateEvidenceInputSizes(params); err != nil {
		return nil, err
	}

	projectName := filepath.Base(ts.projectRoot)

	var activeFiles []snapshot.ActiveFile
	for _, f := range params.ActiveFiles {
		if !snapshot.IsSafeRelativePath(f.Path) {
			return nil, fmt.Errorf("invalid file path: %s", f.Path)
		}
		if f.Confidence != "" && !validConfidences[f.Confidence] {
			return nil, fmt.Errorf("invalid confidence %q for file %q: must be high, medium, or low", f.Confidence, f.Path)
		}
		activeFiles = append(activeFiles, snapshot.ActiveFile{Path: f.Path, Note: f.Note, Confidence: f.Confidence})
	}

	snap := &snapshot.Snapshot{
		BifrostVersion: snapshot.CurrentVersion,
		Timestamp:      time.Now().UTC(),
		SourceTool:     params.SourceTool,
		Project:        projectName,
		TokenPressure:  params.TokenPressure,
		SessionIntent:  params.SessionIntent,
		ActivePlanName: params.ActivePlanName,
		SessionStart:   params.SessionStart,
		CurrentTask:    params.CurrentTask,
		Status:         params.Status,
		ActiveFiles:    activeFiles,
		Decisions:      params.Decisions,
		EnvNotes:       params.EnvNotes,
		NextStep:       params.NextStep,
		Assumptions:    params.Assumptions,
		OpenQuestions:  params.OpenQuestions,
		Risks:          params.Risks,
		GitSHA:         collectGitSHA(ts.projectRoot),
	}

	if err := snapshot.Write(ts.projectRoot, snap); err != nil {
		return nil, err
	}
	v2 := snapshot.SnapshotToV2(ts.projectRoot, snap)
	reportedCommands, err := reportedCommandsFromParams(params)
	if err != nil {
		return nil, err
	}
	manualEvidence, err := manualEvidenceFromParams(params)
	if err != nil {
		return nil, err
	}
	if err := snapshot.EnrichSnapshotV2WithOptions(ts.projectRoot, v2, snapshot.EnrichOptions{
		ReportedCommands: reportedCommands,
		ManualEvidence:   manualEvidence,
	}); err != nil {
		return nil, err
	}
	extraEvidence, err := evidenceFromParams(params)
	if err != nil {
		return nil, err
	}
	v2.Evidence = mergeSnapshotEvidenceForMCP(v2.Evidence, extraEvidence)
	snapshot.ApplyTrustModelV2(v2)
	if err := snapshot.WriteSnapshotV2(ts.projectRoot, v2); err != nil {
		return nil, err
	}
	if err := snapshot.WriteEvidenceRecordsV2(ts.projectRoot, v2.Evidence); err != nil {
		return nil, err
	}

	return map[string]any{
		"ok":             true,
		"timestamp":      snap.Timestamp.Format(time.RFC3339),
		"project":        projectName,
		"evidence_count": len(v2.Evidence),
	}, nil
}

func validateEvidenceInputSizes(params writeSnapshotParams) error {
	for _, cmd := range params.Commands {
		if len(cmd.ID) > 120 {
			return fmt.Errorf("command id exceeds 120 characters")
		}
		if len(cmd.Command) > maxFieldLen {
			return fmt.Errorf("command exceeds %d characters", maxFieldLen)
		}
		if len(cmd.Summary) > maxFieldLen {
			return fmt.Errorf("command summary exceeds %d characters", maxFieldLen)
		}
	}
	for _, ev := range params.Evidence {
		if len(ev.ID) > 120 {
			return fmt.Errorf("evidence id exceeds 120 characters")
		}
		if len(ev.Type) > 100 {
			return fmt.Errorf("evidence type exceeds 100 characters")
		}
		if len(ev.Source) > 100 {
			return fmt.Errorf("evidence source exceeds 100 characters")
		}
		if len(ev.Summary) > maxFieldLen {
			return fmt.Errorf("evidence summary exceeds %d characters", maxFieldLen)
		}
	}
	for _, note := range params.ManualEvidence {
		if len(note.ID) > 120 {
			return fmt.Errorf("manual_evidence id exceeds 120 characters")
		}
		if len(note.Source) > 100 {
			return fmt.Errorf("manual_evidence source exceeds 100 characters")
		}
		if len(note.Text) > maxFieldLen {
			return fmt.Errorf("manual_evidence text exceeds %d characters", maxFieldLen)
		}
	}
	return nil
}

func reportedCommandsFromParams(params writeSnapshotParams) ([]snapshot.ReportedCommand, error) {
	out := make([]snapshot.ReportedCommand, 0, len(params.Commands))
	for _, cmd := range params.Commands {
		capturedAt, err := parseOptionalTime(cmd.CapturedAt)
		if err != nil {
			return nil, fmt.Errorf("invalid command captured_at: %w", err)
		}
		out = append(out, snapshot.ReportedCommand{
			ID:         cmd.ID,
			Command:    cmd.Command,
			ExitCode:   cmd.ExitCode,
			CapturedAt: capturedAt,
			Summary:    cmd.Summary,
			TestResult: cmd.TestResult,
		})
	}
	return out, nil
}

func manualEvidenceFromParams(params writeSnapshotParams) ([]snapshot.ManualEvidence, error) {
	out := make([]snapshot.ManualEvidence, 0, len(params.ManualEvidence))
	for _, note := range params.ManualEvidence {
		observedAt, err := parseOptionalTime(note.ObservedAt)
		if err != nil {
			return nil, fmt.Errorf("invalid manual_evidence observed_at: %w", err)
		}
		out = append(out, snapshot.ManualEvidence{
			ID:         note.ID,
			Text:       note.Text,
			Source:     note.Source,
			ObservedAt: observedAt,
		})
	}
	return out, nil
}

func evidenceFromParams(params writeSnapshotParams) ([]snapshot.EvidenceV2, error) {
	out := make([]snapshot.EvidenceV2, 0, len(params.Evidence))
	for _, item := range params.Evidence {
		observedAt, err := parseOptionalTime(item.ObservedAt)
		if err != nil {
			return nil, fmt.Errorf("invalid evidence observed_at: %w", err)
		}
		ev := snapshot.EvidenceV2{
			ID:         item.ID,
			Type:       item.Type,
			Source:     item.Source,
			ObservedAt: observedAt,
			Summary:    item.Summary,
			Data:       item.Data,
		}
		if err := snapshot.ValidateEvidenceV2(&ev); err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, nil
}

func parseOptionalTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, value)
}

func mergeSnapshotEvidenceForMCP(existing, incoming []snapshot.EvidenceV2) []snapshot.EvidenceV2 {
	byID := map[string]snapshot.EvidenceV2{}
	for _, ev := range existing {
		byID[ev.ID] = ev
	}
	for _, ev := range incoming {
		byID[ev.ID] = ev
	}
	out := make([]snapshot.EvidenceV2, 0, len(byID))
	for _, ev := range byID {
		out = append(out, ev)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].ObservedAt.Equal(out[j].ObservedAt) {
			return out[i].ObservedAt.After(out[j].ObservedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// writeNoteParams maps the JSON input for bifrost_write_note.
type writeNoteParams struct {
	Text string `json:"text"`
	From string `json:"from"`
}

func (ts *ToolSet) writeNote(args json.RawMessage) (any, error) {
	var params writeNoteParams
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	if params.Text == "" || params.From == "" {
		return nil, fmt.Errorf("text and from are required")
	}
	if len(params.Text) > maxNoteLen {
		return nil, fmt.Errorf("text exceeds %d characters", maxNoteLen)
	}
	if len(params.From) > 100 {
		return nil, fmt.Errorf("from exceeds 100 characters")
	}

	note := &snapshot.HandoffNote{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		From:      params.From,
		Text:      params.Text,
	}

	if err := snapshot.WriteNote(ts.projectRoot, note); err != nil {
		return nil, err
	}

	return map[string]any{"ok": true}, nil
}

func (ts *ToolSet) status() (any, error) {
	projectName := filepath.Base(ts.projectRoot)

	result := map[string]any{
		"project":       projectName,
		"has_snapshot":  false,
		"has_handoff":   false,
		"history_count": 0,
	}

	snap, err := snapshot.Read(ts.projectRoot)
	if err == nil {
		result["has_snapshot"] = true
		result["age_seconds"] = int(math.Round(snap.Age().Seconds()))
		if snap.SessionIntent != "" {
			result["session_intent"] = snap.SessionIntent
		}
		if snap.ActivePlanName != "" {
			result["active_plan"] = snap.ActivePlanName
		}
		if len(snap.OpenQuestions) > 0 {
			result["open_question_count"] = len(snap.OpenQuestions)
		}
	}

	note, _ := snapshot.ReadNote(ts.projectRoot)
	if note != nil && note.Text != "" {
		result["has_handoff"] = true
	}

	history, _ := snapshot.History(ts.projectRoot)
	result["history_count"] = len(history)

	plans, _ := snapshot.ListPlans(ts.projectRoot)
	result["plan_count"] = len(plans)

	return result, nil
}

// readPlanParams maps the JSON input for bifrost_read_plan.
type readPlanParams struct {
	Name string `json:"name"`
}

func (ts *ToolSet) readPlan(args json.RawMessage) (any, error) {
	var params readPlanParams
	if args != nil {
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
	}
	if params.Name == "" {
		params.Name = "plan"
	}

	if err := snapshot.ValidatePlanName(params.Name); err != nil {
		return nil, err
	}

	plan, err := snapshot.ReadPlan(ts.projectRoot, params.Name)
	if err != nil {
		if errors.Is(err, snapshot.ErrNoPlan) {
			return map[string]any{"found": false}, nil
		}
		return nil, err
	}

	steps := make([]map[string]any, len(plan.Steps))
	for i, s := range plan.Steps {
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

	reviewNotes := make([]map[string]any, len(plan.ReviewNotes))
	for i, rn := range plan.ReviewNotes {
		reviewNotes[i] = map[string]any{
			"from":         rn.From,
			"at":           rn.At.UTC().Format(time.RFC3339),
			"plan_version": rn.PlanVersion,
			"outcome":      rn.Outcome,
			"text":         rn.Text,
		}
	}

	done, pending, blocked := plan.StepSummary()

	return map[string]any{
		"found": true,
		"plan": map[string]any{
			"bifrost_version":   plan.BifrostVersion,
			"created_at":        plan.CreatedAt.UTC().Format(time.RFC3339),
			"updated_at":        plan.UpdatedAt.UTC().Format(time.RFC3339),
			"source_tool":       plan.SourceTool,
			"project":           plan.Project,
			"status":            plan.Status,
			"plan_version":      plan.PlanVersion,
			"proposed_by":       plan.ProposedBy,
			"max_revisions":     plan.MaxRevisions,
			"revision_count":    plan.RevisionCount,
			"consensus_state":   plan.ConsensusState,
			"activation_reason": plan.ActivationReason,
			"deadlock_detected": plan.DeadlockDetected,
			"deadlock_reason":   plan.DeadlockReason,
			"title":             plan.Title,
			"goal":              plan.Goal,
			"steps":             steps,
			"constraints":       plan.Constraints,
			"review_notes":      reviewNotes,
			"completion_pct":    plan.CompletionPct(),
			"steps_done":        done,
			"steps_pending":     pending,
			"steps_blocked":     blocked,
		},
	}, nil
}

// writePlanParams maps the JSON input for bifrost_write_plan.
type writePlanParams struct {
	SourceTool string `json:"source_tool"`
	Title      string `json:"title"`
	Name       string `json:"name"`
	Goal       string `json:"goal"`
	Steps      []struct {
		Description string   `json:"description"`
		Files       []string `json:"files"`
	} `json:"steps"`
	Constraints []string `json:"constraints"`
}

func (ts *ToolSet) writePlan(args json.RawMessage) (any, error) {
	var params writePlanParams
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	if params.SourceTool == "" || params.Title == "" {
		return nil, fmt.Errorf("source_tool and title are required")
	}
	if params.Name == "" {
		params.Name = "plan"
	}

	// Validate sizes
	if len(params.SourceTool) > 100 {
		return nil, fmt.Errorf("source_tool exceeds 100 characters")
	}
	if len(params.Title) > maxFieldLen {
		return nil, fmt.Errorf("title exceeds %d characters", maxFieldLen)
	}
	if len(params.Goal) > maxFieldLen {
		return nil, fmt.Errorf("goal exceeds %d characters", maxFieldLen)
	}
	if len(params.Steps) > maxArrayItems {
		return nil, fmt.Errorf("steps exceeds %d items", maxArrayItems)
	}
	if len(params.Constraints) > maxArrayItems {
		return nil, fmt.Errorf("constraints exceeds %d items", maxArrayItems)
	}
	for _, c := range params.Constraints {
		if len(c) > maxFieldLen {
			return nil, fmt.Errorf("constraint exceeds %d characters", maxFieldLen)
		}
	}

	if err := snapshot.ValidatePlanName(params.Name); err != nil {
		return nil, err
	}

	projectName := filepath.Base(ts.projectRoot)

	var steps []snapshot.PlanStep
	for _, s := range params.Steps {
		if len(s.Description) > maxFieldLen {
			return nil, fmt.Errorf("step description exceeds %d characters", maxFieldLen)
		}
		if len(s.Files) > maxArrayItems {
			return nil, fmt.Errorf("step files exceeds %d items", maxArrayItems)
		}
		for _, f := range s.Files {
			if strings.Contains(f, "..") || filepath.IsAbs(f) {
				return nil, fmt.Errorf("invalid file path: %s", f)
			}
		}
		steps = append(steps, snapshot.PlanStep{
			Description: s.Description,
			Status:      "pending",
			Files:       s.Files,
		})
	}

	now := time.Now().UTC()
	plan := &snapshot.Plan{
		BifrostVersion: snapshot.CurrentVersion,
		CreatedAt:      now,
		UpdatedAt:      now,
		SourceTool:     params.SourceTool,
		Project:        projectName,
		Status:         snapshot.PlanStatusDraft,
		Title:          params.Title,
		Goal:           params.Goal,
		Steps:          steps,
		Constraints:    params.Constraints,
		PlanVersion:    1,
		ProposedBy:     params.SourceTool,
		MaxRevisions:   3,
		ConsensusState: snapshot.ConsensusNone,
	}

	if err := snapshot.WritePlan(ts.projectRoot, params.Name, plan); err != nil {
		return nil, err
	}

	return map[string]any{
		"ok":         true,
		"name":       params.Name,
		"created_at": plan.CreatedAt.Format(time.RFC3339),
		"project":    projectName,
	}, nil
}

// updatePlanParams maps the JSON input for bifrost_update_plan.
type updatePlanParams struct {
	Name        string `json:"name"`
	SourceTool  string `json:"source_tool"` // who is making this update (reviewer identity)
	PlanStatus  string `json:"plan_status"`
	ReviewNotes []struct {
		From string `json:"from"`
		Text string `json:"text"`
	} `json:"review_notes"`
	StepUpdates []struct {
		Index       int      `json:"index"`
		Status      string   `json:"status"`
		Description string   `json:"description"`
		Files       []string `json:"files"`
	} `json:"step_updates"`
	// Consensus params
	ReviewOutcome  string `json:"review_outcome"`  // approved | needs_revision
	ReviewFeedback string `json:"review_feedback"` // text for the review note
	ForceAccept    bool   `json:"force_accept"`    // bypass consensus
	Revise         bool   `json:"revise"`          // signal a deliberate revision
}

func (ts *ToolSet) updatePlan(args json.RawMessage) (any, error) {
	var params updatePlanParams
	if args != nil {
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
	}
	if params.Name == "" {
		params.Name = "plan"
	}

	if err := snapshot.ValidatePlanName(params.Name); err != nil {
		return nil, err
	}

	// Validate sizes
	if len(params.ReviewNotes) > maxArrayItems {
		return nil, fmt.Errorf("review_notes exceeds %d items", maxArrayItems)
	}
	if len(params.StepUpdates) > maxArrayItems {
		return nil, fmt.Errorf("step_updates exceeds %d items", maxArrayItems)
	}

	plan, err := snapshot.ReadPlan(ts.projectRoot, params.Name)
	if err != nil {
		if errors.Is(err, snapshot.ErrNoPlan) {
			return nil, fmt.Errorf("plan %q not found", params.Name)
		}
		return nil, err
	}

	// Validate consensus params
	if params.ReviewOutcome != "" {
		switch params.ReviewOutcome {
		case snapshot.ReviewOutcomeApproved, snapshot.ReviewOutcomeNeedsRevision:
		default:
			return nil, fmt.Errorf("invalid review_outcome %q: must be approved or needs_revision", params.ReviewOutcome)
		}
	}

	// Handle consensus operations first

	if params.ForceAccept {
		plan.ConsensusState = snapshot.ConsensusOverridden
		plan.ActivationReason = snapshot.ActivationForceAccepted
		plan.Status = snapshot.PlanStatusActive
	}

	if params.ReviewOutcome != "" {
		reviewer := params.SourceTool
		if reviewer == "" {
			reviewer = "unknown"
		}
		noteText := params.ReviewFeedback
		if noteText == "" {
			noteText = params.ReviewOutcome
		}
		note := snapshot.ReviewNote{
			From:        reviewer,
			At:          time.Now().UTC(),
			PlanVersion: plan.PlanVersion,
			Outcome:     params.ReviewOutcome,
			Text:        noteText,
		}
		plan.ReviewNotes = append(plan.ReviewNotes, note)

		switch params.ReviewOutcome {
		case snapshot.ReviewOutcomeApproved:
			plan.ConsensusState = snapshot.ConsensusReached
			plan.ActivationReason = snapshot.ActivationConsensus
			plan.Status = snapshot.PlanStatusActive
		case snapshot.ReviewOutcomeNeedsRevision:
			plan.ConsensusState = snapshot.ConsensusNone
			if plan.IsDeadlocked() {
				plan.DeadlockDetected = true
				plan.DeadlockReason = fmt.Sprintf("max revisions (%d) reached with unresolved needs_revision", plan.MaxRevisions)
			}
		}
	}

	if params.Revise {
		plan.PlanVersion++
		plan.RevisionCount++
		plan.ConsensusState = snapshot.ConsensusNone
		plan.DeadlockDetected = false
		plan.DeadlockReason = ""
	}

	// Update plan status (manual override, only if no consensus operation ran)
	if params.PlanStatus != "" && params.ReviewOutcome == "" && !params.ForceAccept {
		switch params.PlanStatus {
		case snapshot.PlanStatusDraft, snapshot.PlanStatusActive, snapshot.PlanStatusCompleted, snapshot.PlanStatusArchived:
			plan.Status = params.PlanStatus
		default:
			return nil, fmt.Errorf("invalid plan status: %s", params.PlanStatus)
		}
	}

	// Apply step updates
	wasApproved := plan.ConsensusState == snapshot.ConsensusReached && plan.Status == snapshot.PlanStatusActive
	structuralEdit := false
	for _, su := range params.StepUpdates {
		if su.Index < 0 || su.Index >= len(plan.Steps) {
			return nil, fmt.Errorf("step index %d out of range (0-%d)", su.Index, len(plan.Steps)-1)
		}
		if su.Status != "" {
			switch su.Status {
			case "pending", "done", "blocked":
				plan.Steps[su.Index].Status = su.Status
			default:
				return nil, fmt.Errorf("invalid step status: %s", su.Status)
			}
		}
		if su.Description != "" {
			if len(su.Description) > maxFieldLen {
				return nil, fmt.Errorf("step description exceeds %d characters", maxFieldLen)
			}
			if su.Description != plan.Steps[su.Index].Description {
				structuralEdit = true
			}
			plan.Steps[su.Index].Description = su.Description
		}
		if su.Files != nil {
			if len(su.Files) > maxArrayItems {
				return nil, fmt.Errorf("step files exceeds %d items", maxArrayItems)
			}
			for _, f := range su.Files {
				if strings.Contains(f, "..") || filepath.IsAbs(f) {
					return nil, fmt.Errorf("invalid file path: %s", f)
				}
			}
			plan.Steps[su.Index].Files = su.Files
		}
	}
	// A structural edit after approval invalidates the review — reset to draft
	if wasApproved && structuralEdit && params.ReviewOutcome == "" && !params.ForceAccept {
		plan.PlanVersion++
		plan.ConsensusState = snapshot.ConsensusNone
		plan.Status = snapshot.PlanStatusDraft
		plan.ActivationReason = ""
	}

	// Append review notes
	for _, rn := range params.ReviewNotes {
		if rn.From == "" || rn.Text == "" {
			return nil, fmt.Errorf("review note from and text are required")
		}
		if len(rn.From) > 100 {
			return nil, fmt.Errorf("review note from exceeds 100 characters")
		}
		if len(rn.Text) > maxFieldLen {
			return nil, fmt.Errorf("review note text exceeds %d characters", maxFieldLen)
		}
		plan.ReviewNotes = append(plan.ReviewNotes, snapshot.ReviewNote{
			From: rn.From,
			Text: rn.Text,
		})
	}

	plan.UpdatedAt = time.Now().UTC()

	if err := snapshot.WritePlan(ts.projectRoot, params.Name, plan); err != nil {
		return nil, err
	}

	result := map[string]any{
		"ok":              true,
		"name":            params.Name,
		"status":          plan.Status,
		"plan_version":    plan.PlanVersion,
		"revision_count":  plan.RevisionCount,
		"consensus_state": plan.ConsensusState,
		"completion_pct":  plan.CompletionPct(),
	}
	if plan.DeadlockDetected {
		result["deadlock_detected"] = true
		result["deadlock_reason"] = plan.DeadlockReason
	}
	return result, nil
}

// deletePlanParams maps the JSON input for bifrost_delete_plan.
type deletePlanParams struct {
	Name string `json:"name"`
}

func (ts *ToolSet) deletePlan(args json.RawMessage) (any, error) {
	var params deletePlanParams
	if args != nil {
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, fmt.Errorf("invalid params: %w", err)
		}
	}
	if params.Name == "" {
		params.Name = "plan"
	}

	if err := snapshot.ValidatePlanName(params.Name); err != nil {
		return nil, err
	}

	if err := snapshot.DeletePlan(ts.projectRoot, params.Name); err != nil {
		if errors.Is(err, snapshot.ErrNoPlan) {
			return nil, fmt.Errorf("plan %q not found", params.Name)
		}
		return nil, err
	}

	return map[string]any{
		"ok":   true,
		"name": params.Name,
	}, nil
}

func (ts *ToolSet) listPlans() (any, error) {
	names, err := snapshot.ListPlans(ts.projectRoot)
	if err != nil {
		return nil, err
	}

	planInfos := make([]map[string]any, 0, len(names))
	for _, name := range names {
		info := map[string]any{"name": name}
		plan, err := snapshot.ReadPlan(ts.projectRoot, name)
		if err == nil {
			info["status"] = plan.Status
			info["title"] = plan.Title
			info["completion_pct"] = plan.CompletionPct()
			info["plan_version"] = plan.PlanVersion
			info["consensus_state"] = plan.ConsensusState
			info["revision_count"] = plan.RevisionCount
			info["deadlock_detected"] = plan.DeadlockDetected
			info["latest_review_outcome"] = plan.LatestReviewOutcome()
		}
		planInfos = append(planInfos, info)
	}

	return map[string]any{
		"plans": planInfos,
	}, nil
}
