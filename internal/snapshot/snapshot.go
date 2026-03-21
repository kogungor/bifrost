package snapshot

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ErrNoSnapshot is returned when no snapshot file exists.
var ErrNoSnapshot = errors.New("no snapshot found")

// Snapshot represents a session context snapshot.
type Snapshot struct {
	BifrostVersion int       `yaml:"bifrost_version"`
	Timestamp      time.Time `yaml:"timestamp"`
	SourceTool     string    `yaml:"source_tool"`
	Project        string    `yaml:"project"`
	TokenPressure  string    `yaml:"token_pressure"`

	CurrentTask string
	Status      []string
	ActiveFiles []ActiveFile
	Decisions   []string
	EnvNotes    []string
	NextStep    string
}

// ActiveFile represents a file touched during the session.
type ActiveFile struct {
	Path string
	Note string
}

const bifrostDir = ".bifrost"
const sessionFile = "session.md"
const historyDir = "history"

// Dir returns the .bifrost directory path for a project.
func Dir(projectRoot string) string {
	return filepath.Join(projectRoot, bifrostDir)
}

// SessionPath returns the path to session.md for a project.
func SessionPath(projectRoot string) string {
	return filepath.Join(projectRoot, bifrostDir, sessionFile)
}

// HistoryDir returns the path to the history directory.
func HistoryDir(projectRoot string) string {
	return filepath.Join(projectRoot, bifrostDir, historyDir)
}

// EnsureDir creates .bifrost/ and .bifrost/history/ if they don't exist.
func EnsureDir(projectRoot string) error {
	if err := os.MkdirAll(Dir(projectRoot), 0700); err != nil {
		return err
	}
	return os.MkdirAll(HistoryDir(projectRoot), 0700)
}

// Read parses .bifrost/session.md in the given project root.
func Read(projectRoot string) (*Snapshot, error) {
	path := SessionPath(projectRoot)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoSnapshot
		}
		return nil, err
	}
	return Parse(data)
}

// Write serializes the snapshot and writes it to .bifrost/session.md.
// Automatically archives the previous snapshot if one exists.
func Write(projectRoot string, s *Snapshot) error {
	if err := EnsureDir(projectRoot); err != nil {
		return err
	}

	// Archive previous snapshot if it exists
	if _, err := os.Stat(SessionPath(projectRoot)); err == nil {
		if err := Archive(projectRoot); err != nil {
			return err
		}
	}

	data := Render(s)

	// Atomic write: temp file then rename
	tmp := SessionPath(projectRoot) + ".tmp"
	if err := os.WriteFile(tmp, []byte(data), 0600); err != nil {
		return err
	}
	return os.Rename(tmp, SessionPath(projectRoot))
}

// Age returns the time elapsed since the snapshot was taken.
func (s *Snapshot) Age() time.Duration {
	return time.Since(s.Timestamp)
}

// HandoffNote represents the freeform handoff note.
type HandoffNote struct {
	Timestamp string
	From      string
	Text      string
}

// NotePath returns the path to handoff.md for a project.
func NotePath(projectRoot string) string {
	return filepath.Join(projectRoot, bifrostDir, "handoff.md")
}

// ReadNote reads and parses .bifrost/handoff.md.
func ReadNote(projectRoot string) (*HandoffNote, error) {
	data, err := os.ReadFile(NotePath(projectRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return ParseNote(data), nil
}

// WriteNote writes .bifrost/handoff.md atomically.
func WriteNote(projectRoot string, note *HandoffNote) error {
	if err := EnsureDir(projectRoot); err != nil {
		return err
	}

	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("timestamp: %s\n", note.Timestamp))
	b.WriteString(fmt.Sprintf("from: %s\n", note.From))
	b.WriteString("---\n\n")
	b.WriteString(note.Text + "\n")

	tmp := NotePath(projectRoot) + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0600); err != nil {
		return err
	}
	return os.Rename(tmp, NotePath(projectRoot))
}

// ParseNote extracts the handoff note from raw bytes.
func ParseNote(data []byte) *HandoffNote {
	content := string(data)
	note := &HandoffNote{}

	if strings.HasPrefix(content, "---\n") {
		rest := content[4:]
		idx := strings.Index(rest, "\n---\n")
		if idx >= 0 {
			// Parse frontmatter fields manually
			for _, line := range strings.Split(rest[:idx], "\n") {
				if strings.HasPrefix(line, "timestamp:") {
					note.Timestamp = strings.TrimSpace(strings.TrimPrefix(line, "timestamp:"))
				} else if strings.HasPrefix(line, "from:") {
					note.From = strings.TrimSpace(strings.TrimPrefix(line, "from:"))
				}
			}
			content = rest[idx+5:]
		}
	}

	note.Text = strings.TrimSpace(content)
	return note
}
