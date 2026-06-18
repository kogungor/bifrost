package snapshot

import (
	"encoding/json"
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
	return archiveRawWithExt(projectRoot, data, name, ".md")
}

func archiveRawWithExt(projectRoot string, data []byte, name, ext string) error {
	if err := os.MkdirAll(HistoryDir(projectRoot), 0700); err != nil {
		return err
	}

	dest := filepath.Join(HistoryDir(projectRoot), name+ext)

	// Use O_EXCL to atomically check-and-create, avoiding TOCTOU races
	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		if !os.IsExist(err) {
			return err
		}
		// File exists — append a suffix to avoid collision
		for i := 1; ; i++ {
			candidate := filepath.Join(HistoryDir(projectRoot), fmt.Sprintf("%s-%d%s", name, i, ext))
			f, err = os.OpenFile(candidate, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if err == nil {
				dest = candidate
				break
			}
			if !os.IsExist(err) {
				return err
			}
		}
	}
	defer f.Close()

	_, err = f.Write(data)
	return err
}

func ArchiveSnapshotJSON(projectRoot string) error {
	path := SnapshotJSONPath(projectRoot)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	name := "unknown-json"
	var snap SnapshotV2
	if err := json.Unmarshal(data, &snap); err == nil && !snap.CapturedAt.IsZero() {
		name = snap.CapturedAt.UTC().Format("2006-01-02T15-04-05Z")
	}
	return archiveRawWithExt(projectRoot, data, name, ".json")
}

// DefaultMaxHistory is the default maximum number of snapshots to retain in history.
const DefaultMaxHistory = 50

// Prune removes the oldest archived snapshots, keeping at most maxKeep entries.
// If maxKeep <= 0, no pruning is done.
// Errors from individual deletions are ignored — prune is best-effort.
func Prune(projectRoot string, maxKeep int) error {
	return pruneHistoryExt(projectRoot, maxKeep, ".md")
}

func PruneSnapshotJSON(projectRoot string, maxKeep int) error {
	return pruneHistoryExt(projectRoot, maxKeep, ".json")
}

func pruneHistoryExt(projectRoot string, maxKeep int, ext string) error {
	if maxKeep <= 0 {
		return nil
	}

	dir := HistoryDir(projectRoot)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var files []os.DirEntry
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ext) {
			files = append(files, e)
		}
	}

	// ReadDir returns entries sorted by name; filenames are timestamps so this
	// is chronological order — oldest entries are at the front.
	if len(files) <= maxKeep {
		return nil
	}

	toDelete := files[:len(files)-maxKeep]
	for _, e := range toDelete {
		os.Remove(filepath.Join(dir, e.Name())) //nolint:errcheck — best-effort
	}
	return nil
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

func HistoryV2(projectRoot string) ([]*SnapshotV2, error) {
	dir := HistoryDir(projectRoot)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var snapshots []*SnapshotV2
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var snap SnapshotV2
		if err := json.Unmarshal(data, &snap); err != nil {
			continue
		}
		if err := ValidateSnapshotV2(&snap); err != nil {
			continue
		}
		snapshots = append(snapshots, &snap)
	}
	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].CapturedAt.Equal(snapshots[j].CapturedAt) {
			return snapshots[i].ID > snapshots[j].ID
		}
		return snapshots[i].CapturedAt.After(snapshots[j].CapturedAt)
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
	if err := os.WriteFile(tmp, []byte(data), 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, SessionPath(projectRoot)); err != nil {
		return err
	}
	if fileExists(SnapshotJSONPath(projectRoot)) {
		if err := WriteSnapshotV2(projectRoot, SnapshotToV2(projectRoot, history[index])); err != nil {
			return err
		}
	}
	_ = AppendTimelineEvent(projectRoot, TimelineEvent{
		Type:     "snapshot.restore",
		Snapshot: snapshotID(history[index]),
		Task:     history[index].CurrentTask,
	})
	return nil
}

func RestoreSnapshotV2(projectRoot string, selected *SnapshotV2) error {
	if err := ValidateSnapshotV2(selected); err != nil {
		return err
	}
	if err := WriteSnapshotV2(projectRoot, selected); err != nil {
		return err
	}
	if _, statErr := os.Stat(SessionPath(projectRoot)); statErr == nil {
		if err := Archive(projectRoot); err != nil {
			return err
		}
	}
	data := Render(SnapshotFromV2(selected))
	tmp := SessionPath(projectRoot) + ".tmp"
	if err := os.WriteFile(tmp, []byte(data), 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, SessionPath(projectRoot)); err != nil {
		return err
	}
	_ = AppendTimelineEvent(projectRoot, TimelineEvent{
		Type:     "snapshot.restore",
		Snapshot: selected.ID,
		Task:     selected.Session.Task,
	})
	return nil
}
