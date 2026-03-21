package snapshot

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Archive copies the current session.md to .bifrost/history/<timestamp>.md.
func Archive(projectRoot string) error {
	path := SessionPath(projectRoot)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // nothing to archive
		}
		return err
	}

	// Parse to get the timestamp for the filename
	snap, err := Parse(data)
	if err != nil {
		// If we can't parse, still archive with a fallback name
		return archiveRaw(projectRoot, data, "unknown")
	}

	// Format: 2025-03-21T14-32-17Z.md (colons replaced with hyphens)
	ts := snap.Timestamp.UTC().Format("2006-01-02T15-04-05Z")

	return archiveRaw(projectRoot, data, ts)
}

func archiveRaw(projectRoot string, data []byte, name string) error {
	if err := os.MkdirAll(HistoryDir(projectRoot), 0755); err != nil {
		return err
	}

	dest := filepath.Join(HistoryDir(projectRoot), name+".md")

	// Don't overwrite existing archives
	if _, err := os.Stat(dest); err == nil {
		// Append a suffix to avoid collision
		for i := 1; ; i++ {
			candidate := filepath.Join(HistoryDir(projectRoot), fmt.Sprintf("%s-%d.md", name, i))
			if _, err := os.Stat(candidate); os.IsNotExist(err) {
				dest = candidate
				break
			}
		}
	}

	return os.WriteFile(dest, data, 0644)
}

// History returns all archived snapshots sorted newest-first.
func History(projectRoot string) ([]*Snapshot, error) {
	dir := HistoryDir(projectRoot)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var snapshots []*Snapshot
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		snap, err := Parse(data)
		if err != nil {
			continue
		}
		snapshots = append(snapshots, snap)
	}

	// Sort newest-first
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Timestamp.After(snapshots[j].Timestamp)
	})

	return snapshots, nil
}

// Restore copies a historical snapshot to session.md (archiving the current one first).
func Restore(projectRoot string, index int) error {
	history, err := History(projectRoot)
	if err != nil {
		return err
	}
	if index < 0 || index >= len(history) {
		return fmt.Errorf("snapshot %d not found", index+1)
	}

	// Archive current snapshot first
	if _, statErr := os.Stat(SessionPath(projectRoot)); statErr == nil {
		if err := Archive(projectRoot); err != nil {
			return err
		}
	}

	// Write the selected historical snapshot as current
	data := Render(history[index])

	tmp := SessionPath(projectRoot) + ".tmp"
	if err := os.WriteFile(tmp, []byte(data), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, SessionPath(projectRoot))
}
