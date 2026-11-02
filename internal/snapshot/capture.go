package snapshot

import (
	"errors"
	"path/filepath"
	"time"

	"github.com/kogungor/bifrost/internal/security"
)

// AutoCaptureAgent marks snapshots that hooks wrote from a transcript, as
// opposed to a /handoff an agent wrote. Only these are ever replaced.
const AutoCaptureAgent = "bifrost hook"

// maxCapturedEditedFiles caps how many transcript-edited files a capture adds
// as active files.
const maxCapturedEditedFiles = 10

// CaptureOptions describes the session a capture comes from.
type CaptureOptions struct {
	SourceTool string // e.g. "claude-code"
	SessionID  string
	// Pressure is recorded on snapshots the capture creates, e.g. "high"
	// before an automatic compaction.
	Pressure string
	Now      time.Time
}

// CaptureResult reports what CaptureSession wrote.
type CaptureResult struct {
	Written  bool // a snapshot was written
	Created  bool // an automatic snapshot was written rather than merged into a handoff
	Commands int
	Files    int
}

// CaptureSession records what a session observably did, without ever losing
// a handoff an agent wrote:
//   - a /handoff written during this session keeps everything it recorded
//     (including its file baseline, so later drift still shows) and gains the
//     transcript's command results;
//   - a /handoff from an earlier session, e.g. from another tool, is left
//     untouched;
//   - otherwise (no snapshot, or one a hook wrote) an automatic snapshot of
//     observed facts replaces it.
//
// Nothing is written when the transcript shows no commands or edits.
func CaptureSession(projectRoot string, activity TranscriptActivity, opts CaptureOptions) (CaptureResult, error) {
	var result CaptureResult
	if len(activity.Commands) == 0 && len(activity.EditedFiles) == 0 {
		return result, nil
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	existing, err := SnapshotFromProject(projectRoot)
	if err != nil && !errors.Is(err, ErrNoSnapshot) {
		return result, err
	}
	commands := redactCommands(projectRoot, activity.Commands)

	if err == nil && existing.Source.Agent != AutoCaptureAgent {
		if !activity.SessionStart.IsZero() && existing.CapturedAt.Before(activity.SessionStart) {
			return result, nil
		}
		AddTranscriptCommands(existing, commands, now)
		if err := ValidateSnapshotV2(existing); err != nil {
			return result, err
		}
		if err := writeCapture(projectRoot, existing); err != nil {
			return result, err
		}
		result.Written = true
		result.Commands = len(commands)
		return result, nil
	}

	legacy := &Snapshot{
		BifrostVersion: CurrentVersion,
		Timestamp:      now,
		SourceTool:     opts.SourceTool,
		Project:        filepath.Base(projectRoot),
		TokenPressure:  opts.Pressure,
		CurrentTask:    "Session captured automatically; no /handoff was written",
		NextStep:       "Review the observed commands and changed files, then ask the user how to continue.",
	}
	if !activity.SessionStart.IsZero() {
		legacy.SessionStart = activity.SessionStart.UTC().Format(time.RFC3339)
	}
	// Keep session.md in step so /handin without MCP sees the same snapshot.
	if err := Write(projectRoot, legacy); err != nil {
		return result, err
	}
	snap := SnapshotToV2(projectRoot, legacy)
	snap.Source.Agent = AutoCaptureAgent

	edited := activity.EditedFiles
	if len(edited) > maxCapturedEditedFiles {
		edited = edited[len(edited)-maxCapturedEditedFiles:]
	}
	for _, path := range edited {
		if IsSafeRelativePath(path) {
			snap.ActiveFiles = append(snap.ActiveFiles, ActiveFileV2{Path: path, Note: "edited this session (from the " + opts.SourceTool + " transcript)"})
			result.Files++
		}
	}
	if err := EnrichSnapshotV2WithOptions(projectRoot, snap, EnrichOptions{TranscriptCommands: commands}); err != nil {
		return result, err
	}
	if err := writeCapture(projectRoot, snap); err != nil {
		return result, err
	}
	result.Written = true
	result.Created = true
	result.Commands = len(commands)
	return result, nil
}

func writeCapture(projectRoot string, snap *SnapshotV2) error {
	if err := WriteSnapshotV2(projectRoot, snap); err != nil {
		return err
	}
	return WriteEvidenceRecordsV2(projectRoot, snap.Evidence)
}

func redactCommands(projectRoot string, commands []ReportedCommand) []ReportedCommand {
	cfg := security.LoadConfig(projectRoot)
	out := make([]ReportedCommand, 0, len(commands))
	for _, command := range commands {
		command.Command, _ = security.RedactString(command.Command, cfg)
		out = append(out, command)
	}
	return out
}
