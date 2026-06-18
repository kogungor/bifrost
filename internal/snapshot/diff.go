package snapshot

import (
	"sort"
	"strings"
)

type SnapshotDiff struct {
	From          string        `json:"from"`
	To            string        `json:"to"`
	TaskChanged   *TextChange   `json:"task_changed,omitempty"`
	NextChanged   *TextChange   `json:"next_changed,omitempty"`
	NewRisks      []string      `json:"new_risks,omitempty"`
	ResolvedRisks []string      `json:"resolved_risks,omitempty"`
	NewQuestions  []string      `json:"new_questions,omitempty"`
	ResolvedQs    []string      `json:"resolved_questions,omitempty"`
	ActiveFiles   FileSetChange `json:"active_files"`
	TrustChanges  []TrustChange `json:"trust_changes,omitempty"`
}

type TextChange struct {
	Before string `json:"before"`
	After  string `json:"after"`
}

type FileSetChange struct {
	Added   []string `json:"added,omitempty"`
	Removed []string `json:"removed,omitempty"`
	Common  []string `json:"common,omitempty"`
}

type TrustChange struct {
	Path      string `json:"path"`
	Dimension string `json:"dimension"`
	Before    string `json:"before,omitempty"`
	After     string `json:"after,omitempty"`
}

func DiffSnapshots(from, to *SnapshotV2) SnapshotDiff {
	diff := SnapshotDiff{
		From: snapshotLabel(from),
		To:   snapshotLabel(to),
	}
	if from == nil || to == nil {
		return diff
	}
	if from.Session.Task != to.Session.Task {
		diff.TaskChanged = &TextChange{Before: from.Session.Task, After: to.Session.Task}
	}
	if from.Session.NextStep != to.Session.NextStep {
		diff.NextChanged = &TextChange{Before: from.Session.NextStep, After: to.Session.NextStep}
	}
	fromRisks := riskSet(from)
	toRisks := riskSet(to)
	diff.NewRisks = sortedDifference(toRisks, fromRisks)
	diff.ResolvedRisks = sortedDifference(fromRisks, toRisks)
	fromQuestions := questionSet(from)
	toQuestions := questionSet(to)
	diff.NewQuestions = sortedDifference(toQuestions, fromQuestions)
	diff.ResolvedQs = sortedDifference(fromQuestions, toQuestions)
	diff.ActiveFiles = diffActiveFiles(from.ActiveFiles, to.ActiveFiles)
	diff.TrustChanges = diffTrust(from.ActiveFiles, to.ActiveFiles)
	return diff
}

func snapshotLabel(s *SnapshotV2) string {
	if s == nil {
		return ""
	}
	if s.ID != "" {
		return s.ID
	}
	if !s.CapturedAt.IsZero() {
		return s.CapturedAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	return "unknown"
}

func riskSet(s *SnapshotV2) map[string]bool {
	out := map[string]bool{}
	for _, risk := range s.Interpretation.Risks {
		if text := strings.TrimSpace(risk.Text); text != "" {
			out[text] = true
		}
	}
	return out
}

func questionSet(s *SnapshotV2) map[string]bool {
	out := map[string]bool{}
	for _, question := range s.Interpretation.OpenQuestions {
		if text := strings.TrimSpace(question.Text); text != "" {
			out[text] = true
		}
	}
	return out
}

func sortedDifference(left, right map[string]bool) []string {
	var out []string
	for item := range left {
		if !right[item] {
			out = append(out, item)
		}
	}
	sort.Strings(out)
	return out
}

func diffActiveFiles(from, to []ActiveFileV2) FileSetChange {
	fromSet := activeFileSet(from)
	toSet := activeFileSet(to)
	return FileSetChange{
		Added:   sortedDifference(toSet, fromSet),
		Removed: sortedDifference(fromSet, toSet),
		Common:  sortedIntersection(fromSet, toSet),
	}
}

func activeFileSet(files []ActiveFileV2) map[string]bool {
	out := map[string]bool{}
	for _, file := range files {
		if path := strings.TrimSpace(file.Path); path != "" {
			out[path] = true
		}
	}
	return out
}

func sortedIntersection(left, right map[string]bool) []string {
	var out []string
	for item := range left {
		if right[item] {
			out = append(out, item)
		}
	}
	sort.Strings(out)
	return out
}

func diffTrust(from, to []ActiveFileV2) []TrustChange {
	fromByPath := map[string]TrustV2{}
	for _, file := range from {
		if file.Path != "" {
			fromByPath[file.Path] = file.Trust
		}
	}
	var changes []TrustChange
	for _, file := range to {
		before, ok := fromByPath[file.Path]
		if !ok {
			continue
		}
		changes = append(changes, compareTrust(file.Path, before, file.Trust)...)
	}
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Path == changes[j].Path {
			return changes[i].Dimension < changes[j].Dimension
		}
		return changes[i].Path < changes[j].Path
	})
	return changes
}

func compareTrust(path string, before, after TrustV2) []TrustChange {
	checks := []struct {
		dimension string
		before    string
		after     string
	}{
		{"implementation", before.Implementation, after.Implementation},
		{"tests", before.Tests, after.Tests},
		{"security", before.Security, after.Security},
		{"architecture", before.Architecture, after.Architecture},
		{"freshness", before.Freshness, after.Freshness},
		{"evidence", before.Evidence, after.Evidence},
	}
	var changes []TrustChange
	for _, check := range checks {
		if check.before != check.after {
			changes = append(changes, TrustChange{
				Path:      path,
				Dimension: check.dimension,
				Before:    check.before,
				After:     check.after,
			})
		}
	}
	return changes
}
