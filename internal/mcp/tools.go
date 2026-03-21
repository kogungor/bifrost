package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"time"

	"github.com/kogungor/bifrost/internal/snapshot"
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
			Description: "Read the current Bifrost session snapshot for this project. Returns the full snapshot including task status, active files, decisions, and handoff notes.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			Name:        "bifrost_write_snapshot",
			Description: "Write a new Bifrost session snapshot. Automatically archives the previous snapshot, fills in timestamp and project name.",
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
								"path": map[string]any{"type": "string"},
								"note": map[string]any{"type": "string"},
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
			Description: "Get a quick status summary: snapshot existence, age, project name, handoff note presence, and history count.",
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
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func (ts *ToolSet) readSnapshot() (any, error) {
	snap, err := snapshot.Read(ts.projectRoot)
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
		activeFiles[i] = map[string]string{"path": f.Path, "note": f.Note}
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
			"current_task":      snap.CurrentTask,
			"status":            snap.Status,
			"active_files":      activeFiles,
			"decisions":         snap.Decisions,
			"environment_notes": snap.EnvNotes,
			"next_step":         snap.NextStep,
		},
	}

	if handoffNote != "" {
		result["handoff_note"] = handoffNote
	}

	return result, nil
}

// writeSnapshotParams maps the JSON input for bifrost_write_snapshot.
type writeSnapshotParams struct {
	SourceTool    string `json:"source_tool"`
	TokenPressure string `json:"token_pressure"`
	CurrentTask   string `json:"current_task"`
	Status        []string `json:"status"`
	ActiveFiles   []struct {
		Path string `json:"path"`
		Note string `json:"note"`
	} `json:"active_files"`
	Decisions  []string `json:"decisions"`
	EnvNotes   []string `json:"environment_notes"`
	NextStep   string   `json:"next_step"`
}

func (ts *ToolSet) writeSnapshot(args json.RawMessage) (any, error) {
	var params writeSnapshotParams
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid params: %w", err)
	}

	if params.SourceTool == "" || params.CurrentTask == "" {
		return nil, fmt.Errorf("source_tool and current_task are required")
	}

	projectName := filepath.Base(ts.projectRoot)

	var activeFiles []snapshot.ActiveFile
	for _, f := range params.ActiveFiles {
		activeFiles = append(activeFiles, snapshot.ActiveFile{Path: f.Path, Note: f.Note})
	}

	snap := &snapshot.Snapshot{
		BifrostVersion: 1,
		Timestamp:      time.Now().UTC(),
		SourceTool:     params.SourceTool,
		Project:        projectName,
		TokenPressure:  params.TokenPressure,
		CurrentTask:    params.CurrentTask,
		Status:         params.Status,
		ActiveFiles:    activeFiles,
		Decisions:      params.Decisions,
		EnvNotes:       params.EnvNotes,
		NextStep:       params.NextStep,
	}

	if err := snapshot.Write(ts.projectRoot, snap); err != nil {
		return nil, err
	}

	return map[string]any{
		"ok":        true,
		"timestamp": snap.Timestamp.Format(time.RFC3339),
		"project":   projectName,
	}, nil
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
		"project":        projectName,
		"has_snapshot":   false,
		"has_handoff":    false,
		"history_count":  0,
	}

	snap, err := snapshot.Read(ts.projectRoot)
	if err == nil {
		result["has_snapshot"] = true
		result["age_seconds"] = int(math.Round(snap.Age().Seconds()))
	}

	note, _ := snapshot.ReadNote(ts.projectRoot)
	if note != nil && note.Text != "" {
		result["has_handoff"] = true
	}

	history, _ := snapshot.History(ts.projectRoot)
	result["history_count"] = len(history)

	return result, nil
}
