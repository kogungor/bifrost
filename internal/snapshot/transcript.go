package snapshot

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// maxTranscriptCommands caps how many recent commands one capture records.
const maxTranscriptCommands = 30

// TranscriptActivity is what a Claude Code session did, read from its
// transcript rather than from the model's own summary.
type TranscriptActivity struct {
	// SessionStart is the timestamp of the transcript's first entry.
	SessionStart time.Time
	// Commands are Bash commands that ran to completion inside the project,
	// with the exit codes the harness recorded, oldest first.
	Commands []ReportedCommand
	// EditedFiles are project-relative paths changed with Edit/Write tools,
	// most recent last.
	EditedFiles []string
}

type transcriptEntry struct {
	Timestamp     time.Time       `json:"timestamp"`
	Cwd           string          `json:"cwd"`
	Message       json.RawMessage `json:"message"`
	ToolUseResult json.RawMessage `json:"toolUseResult"`
}

type transcriptBlock struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	IsError   bool            `json:"is_error"`
	Content   json.RawMessage `json:"content"`
}

type pendingCommand struct {
	command string
	at      time.Time
}

// transcriptState follows tool calls across lines until their results arrive.
type transcriptState struct {
	root     string
	commands map[string]pendingCommand // Bash tool_use ID → command
	edits    map[string]string         // edit tool_use ID → project-relative path
	edited   map[string]int            // path → order of its latest successful edit
	order    int
}

var transcriptExitCodeRE = regexp.MustCompile(`^(?:Error: )?Exit code (\d+)`)

// ReadClaudeTranscript extracts finished Bash commands and edited files from a
// Claude Code transcript (JSON Lines). Commands run outside projectRoot,
// interrupted or backgrounded commands, results whose exit code Claude Code
// did not keep, and tool calls that were denied or failed are skipped. Unreadable lines are ignored, since the session may
// still be appending to the file.
func ReadClaudeTranscript(path, projectRoot string) (TranscriptActivity, error) {
	var activity TranscriptActivity
	f, err := os.Open(path)
	if err != nil {
		return activity, err
	}
	defer f.Close()

	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return activity, err
	}
	state := &transcriptState{
		root:     root,
		commands: map[string]pendingCommand{},
		edits:    map[string]string{},
		edited:   map[string]int{},
	}
	reader := bufio.NewReaderSize(f, 1<<20)
	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 {
			state.readLine(line, &activity)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return activity, readErr
		}
	}

	activity.Commands = latestPerCommand(activity.Commands)
	if len(activity.Commands) > maxTranscriptCommands {
		activity.Commands = activity.Commands[len(activity.Commands)-maxTranscriptCommands:]
	}
	for path := range state.edited {
		activity.EditedFiles = append(activity.EditedFiles, path)
	}
	sort.Slice(activity.EditedFiles, func(i, j int) bool {
		return state.edited[activity.EditedFiles[i]] < state.edited[activity.EditedFiles[j]]
	})
	return activity, nil
}

func (state *transcriptState) readLine(line []byte, activity *TranscriptActivity) {
	var entry transcriptEntry
	if activity.SessionStart.IsZero() {
		if json.Unmarshal(line, &entry) == nil && !entry.Timestamp.IsZero() {
			activity.SessionStart = entry.Timestamp
		}
	}
	// Most lines (prompts, API payloads) carry no tool calls; skip parsing them.
	if !bytes.Contains(line, []byte(`"tool_use`)) && !bytes.Contains(line, []byte(`"tool_result"`)) {
		return
	}
	if json.Unmarshal(line, &entry) != nil || !insideRoot(entry.Cwd, state.root) {
		return
	}
	var message struct {
		Content json.RawMessage `json:"content"`
	}
	if json.Unmarshal(entry.Message, &message) != nil || !bytes.HasPrefix(bytes.TrimSpace(message.Content), []byte("[")) {
		return
	}
	var blocks []transcriptBlock
	if json.Unmarshal(message.Content, &blocks) != nil {
		return
	}
	for _, block := range blocks {
		switch block.Type {
		case "tool_use":
			var input struct {
				Command         string `json:"command"`
				RunInBackground bool   `json:"run_in_background"`
				FilePath        string `json:"file_path"`
				NotebookPath    string `json:"notebook_path"`
			}
			_ = json.Unmarshal(block.Input, &input)
			switch block.Name {
			case "Bash":
				if input.Command != "" && !input.RunInBackground {
					state.commands[block.ID] = pendingCommand{command: input.Command, at: entry.Timestamp}
				}
			case "Edit", "MultiEdit", "Write", "NotebookEdit":
				target := input.FilePath
				if target == "" {
					target = input.NotebookPath
				}
				if rel, ok := relativeToRoot(target, state.root); ok {
					state.edits[block.ID] = rel
				}
			}
		case "tool_result":
			if path, ok := state.edits[block.ToolUseID]; ok {
				delete(state.edits, block.ToolUseID)
				if !block.IsError {
					state.order++
					state.edited[path] = state.order
				}
				continue
			}
			cmd, ok := state.commands[block.ToolUseID]
			if !ok {
				continue
			}
			delete(state.commands, block.ToolUseID)
			exitCode, finished := transcriptExitCode(block, entry.ToolUseResult)
			if !finished {
				continue
			}
			capturedAt := entry.Timestamp
			if capturedAt.IsZero() {
				capturedAt = cmd.at
			}
			activity.Commands = append(activity.Commands, ReportedCommand{
				Command:    cmd.command,
				ExitCode:   exitCode,
				CapturedAt: capturedAt.UTC(),
				TestResult: LooksLikeTestCommand(cmd.command),
			})
		}
	}
}

// latestPerCommand keeps only the most recent run of each command, so a test
// that failed and then passed is recorded as passing. Order stays oldest first.
func latestPerCommand(commands []ReportedCommand) []ReportedCommand {
	seen := map[string]bool{}
	kept := make([]ReportedCommand, 0, len(commands))
	for i := len(commands) - 1; i >= 0; i-- {
		if seen[commands[i].Command] {
			continue
		}
		seen[commands[i].Command] = true
		kept = append(kept, commands[i])
	}
	for i, j := 0, len(kept)-1; i < j; i, j = i+1, j-1 {
		kept[i], kept[j] = kept[j], kept[i]
	}
	return kept
}

// transcriptExitCode returns the exit code of a Bash tool result, and false
// when the command did not run to completion (interrupted, denied, or failed
// before starting).
func transcriptExitCode(block transcriptBlock, toolUseResult json.RawMessage) (int, bool) {
	if !block.IsError {
		var result struct {
			Interrupted bool    `json:"interrupted"`
			Stdout      *string `json:"stdout"`
			// Set when Claude Code accepted a non-zero exit (e.g. grep with no
			// match) or moved the command to the background; the real exit
			// code is then unknown.
			ReturnCodeInterpretation string `json:"returnCodeInterpretation"`
			BackgroundTaskID         string `json:"backgroundTaskId"`
			TimedOutAfterMs          int64  `json:"timedOutAfterMs"`
		}
		if json.Unmarshal(toolUseResult, &result) != nil || result.Stdout == nil || result.Interrupted ||
			result.ReturnCodeInterpretation != "" || result.BackgroundTaskID != "" || result.TimedOutAfterMs > 0 {
			return 0, false
		}
		return 0, true
	}
	text := ""
	if json.Unmarshal(toolUseResult, &text) != nil || text == "" {
		var content string
		if json.Unmarshal(block.Content, &content) == nil {
			text = content
		} else {
			var parts []struct {
				Text string `json:"text"`
			}
			if json.Unmarshal(block.Content, &parts) == nil && len(parts) > 0 {
				text = parts[0].Text
			}
		}
	}
	match := transcriptExitCodeRE.FindStringSubmatch(strings.TrimSpace(text))
	if match == nil {
		return 0, false
	}
	code, err := strconv.Atoi(match[1])
	return code, err == nil
}

func insideRoot(dir, root string) bool {
	if dir == "" {
		return false
	}
	_, ok := relativeToRoot(dir, root)
	return ok || filepath.Clean(dir) == root
}

// relativeToRoot returns path relative to root when it lies inside root.
func relativeToRoot(path, root string) (string, bool) {
	if path == "" || !filepath.IsAbs(path) {
		return "", false
	}
	rel, err := filepath.Rel(root, filepath.Clean(path))
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}
