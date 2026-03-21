package snapshot

import (
	"errors"
	"os"
	"path/filepath"
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
	if err := os.MkdirAll(Dir(projectRoot), 0755); err != nil {
		return err
	}
	return os.MkdirAll(HistoryDir(projectRoot), 0755)
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
	if err := os.WriteFile(tmp, []byte(data), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, SessionPath(projectRoot))
}

// Age returns the time elapsed since the snapshot was taken.
func (s *Snapshot) Age() time.Duration {
	return time.Since(s.Timestamp)
}
