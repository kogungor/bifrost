package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kogungor/bifrost/internal/project"
	"github.com/kogungor/bifrost/internal/snapshot"
	"github.com/spf13/cobra"
)

// hookEvents are the `bifrost hook` subcommands `bifrost install --hooks`
// registers with Claude Code.
var hookEvents = []string{"pre-compact", "post-compact", "session-start", "session-end"}

var hookCmd = &cobra.Command{
	Use:   "hook <event>",
	Short: "Run Bifrost from an AI tool hook (reads the hook's JSON on stdin)",
	Long: "Called by Claude Code hooks that `bifrost install --hooks` registers. Events:\n" +
		"  pre-compact, session-end  record the session's commands and edits from its transcript\n" +
		"  post-compact              save the compaction summary as the handoff note\n" +
		"  session-start             print the briefing when a session resumes or is compacted\n" +
		"It does nothing in projects without .bifrost/ or BIFROST.md, and never fails the session:\n" +
		"problems are reported on stderr and the exit code is always 0.",
	Args:      cobra.ExactArgs(1),
	ValidArgs: hookEvents,
	RunE:      runHook,
}

func init() {
	rootCmd.AddCommand(hookCmd)
}

// hookInput holds the fields Bifrost uses from Claude Code's hook input.
type hookInput struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	Cwd            string `json:"cwd"`
	Source         string `json:"source"`          // SessionStart
	Trigger        string `json:"trigger"`         // PreCompact, PostCompact
	CompactSummary string `json:"compact_summary"` // PostCompact
}

const (
	maxHookInput      = 4 << 20
	hookSourceTool    = "claude-code"
	compactNoteSource = "claude-code compaction summary"
	maxNoteChars      = 50000
)

func runHook(cmd *cobra.Command, args []string) error {
	data, err := io.ReadAll(io.LimitReader(os.Stdin, maxHookInput))
	if err != nil {
		hookLog("could not read hook input: %v", err)
		return nil
	}
	var in hookInput
	if err := json.Unmarshal(data, &in); err != nil {
		hookLog("invalid hook input: %v", err)
		return nil
	}
	root, ok := hookProjectRoot(in.Cwd)
	if !ok {
		return nil
	}

	switch args[0] {
	case "pre-compact":
		pressure := ""
		if in.Trigger == "auto" {
			pressure = "high"
		}
		hookCapture(root, in, pressure)
	case "session-end":
		hookCapture(root, in, "")
	case "post-compact":
		hookSaveSummary(root, in)
	case "session-start":
		hookBriefing(root, in)
	default:
		hookLog("unknown hook event %q (expected one of %s)", args[0], strings.Join(hookEvents, ", "))
	}
	return nil
}

// hookProjectRoot resolves the project the hook fired in and reports whether
// it uses Bifrost. Hooks are installed for every project, so they must not
// create .bifrost/ in repositories that never opted in.
func hookProjectRoot(cwd string) (string, bool) {
	if flagProject != "" {
		cwd = flagProject
	}
	if cwd == "" {
		var err error
		if cwd, err = os.Getwd(); err != nil {
			return "", false
		}
	}
	root, _, err := project.Root(cwd)
	if err != nil {
		return "", false
	}
	if fileExists(snapshot.Dir(root)) || fileExists(filepath.Join(root, "BIFROST.md")) {
		return root, true
	}
	return "", false
}

func hookCapture(root string, in hookInput, pressure string) {
	if in.TranscriptPath == "" {
		return
	}
	activity, err := snapshot.ReadClaudeTranscript(in.TranscriptPath, root)
	if err != nil {
		hookLog("could not read transcript: %v", err)
		return
	}
	if _, err := snapshot.CaptureSession(root, activity, snapshot.CaptureOptions{
		SourceTool: hookSourceTool,
		SessionID:  in.SessionID,
		Pressure:   pressure,
	}); err != nil {
		hookLog("could not capture session: %v", err)
	}
}

// hookSaveSummary stores Claude Code's compaction summary as the handoff note,
// unless the agent wrote its own note during this session.
func hookSaveSummary(root string, in hookInput) {
	summary := strings.TrimSpace(in.CompactSummary)
	if summary == "" {
		return
	}
	if note, err := snapshot.ReadNote(root); err == nil && note != nil && note.From != compactNoteSource {
		written, err := time.Parse(time.RFC3339, note.Timestamp)
		if err == nil && !written.Before(hookSessionStart(root, in)) {
			return
		}
	}
	if len(summary) > maxNoteChars {
		summary = summary[:maxNoteChars]
		for !utf8.ValidString(summary) {
			summary = summary[:len(summary)-1]
		}
	}
	if err := snapshot.WriteNote(root, &snapshot.HandoffNote{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		From:      compactNoteSource,
		Text:      summary,
	}); err != nil {
		hookLog("could not write handoff note: %v", err)
	}
}

// hookSessionStart is when the hook's session began, or the zero time if the
// transcript cannot tell.
func hookSessionStart(root string, in hookInput) time.Time {
	if in.TranscriptPath == "" {
		return time.Time{}
	}
	activity, err := snapshot.ReadClaudeTranscript(in.TranscriptPath, root)
	if err != nil {
		return time.Time{}
	}
	return activity.SessionStart
}

// hookBriefing prints context for Claude Code to add at session start: the
// full briefing when work resumes or continues after compaction, and a
// one-line pointer for a new session while a recent handoff exists.
func hookBriefing(root string, in hookInput) {
	snap, err := snapshot.SnapshotFromProject(root)
	if err != nil {
		return
	}
	switch in.Source {
	case "resume", "compact":
		result := buildProjectBrief(root, snap, snapshot.BriefModeImplement, 5000, false)
		fmt.Fprintf(os.Stdout, "Bifrost handoff for this project. Check the verification lines before relying on it.\n\n%s", result.Rendered)
	case "startup":
		if time.Since(snap.CapturedAt) > 24*time.Hour {
			return
		}
		fmt.Fprintf(os.Stdout, "Bifrost: a handoff from %s (%s) is available for this project. Run /handin --verify to load it before continuing earlier work.\n",
			snap.Source.Tool, formatAge(snap.CapturedAt))
	}
}

func hookLog(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "bifrost hook: "+format+"\n", args...)
}
