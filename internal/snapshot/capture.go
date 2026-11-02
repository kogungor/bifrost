package snapshot

import (
	"errors"
	"path/filepath"
	"time"

	"github.com/kogungor/bifrost/internal/security"
)

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
	Created  bool // the snapshot was created rather than merged into
	Commands int
	Files    int
}

// CaptureSession records what a session observably did into the project's
// snapshot. A snapshot written during this session (for example by /handoff)
// keeps its interpretation and gains the transcript's commands and files; an
// older snapshot, or none, is replaced by an automatic snapshot that records
// only observed facts. Nothing is written when the transcript shows no
// commands or edits.
func CaptureSession(projectRoot string, activity TranscriptActivity, opts CaptureOptions) (CaptureResult, error) {
	var result CaptureResult
	if len(activity.Commands) == 0 && len(activity.EditedFiles) == 0 {
		return result, nil
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	snap, err := SnapshotFromProject(projectRoot)
	if err != nil && !errors.Is(err, ErrNoSnapshot) {
		return result, err
	}
	fromThisSession := err == nil && (activity.SessionStart.IsZero() || !snap.CapturedAt.Before(activity.SessionStart))
	if !fromThisSession {
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
		snap = SnapshotToV2(projectRoot, legacy)
		result.Created = true
	}

	cfg := security.LoadConfig(projectRoot)
	commands := make([]ReportedCommand, 0, len(activity.Commands))
	for _, command := range activity.Commands {
		command.Command, _ = security.RedactString(command.Command, cfg)
		commands = append(commands, command)
	}

	known := map[string]bool{}
	for _, file := range snap.ActiveFiles {
		known[file.Path] = true
	}
	edited := activity.EditedFiles
	if len(edited) > maxCapturedEditedFiles {
		edited = edited[len(edited)-maxCapturedEditedFiles:]
	}
	for _, path := range edited {
		if known[path] || !IsSafeRelativePath(path) {
			continue
		}
		snap.ActiveFiles = append(snap.ActiveFiles, ActiveFileV2{Path: path, Note: "edited this session (from the " + opts.SourceTool + " transcript)"})
		result.Files++
	}

	if err := EnrichSnapshotV2WithOptions(projectRoot, snap, EnrichOptions{TranscriptCommands: commands}); err != nil {
		return result, err
	}
	if err := WriteSnapshotV2(projectRoot, snap); err != nil {
		return result, err
	}
	if err := WriteEvidenceRecordsV2(projectRoot, snap.Evidence); err != nil {
		return result, err
	}
	result.Written = true
	result.Commands = len(commands)
	return result, nil
}
