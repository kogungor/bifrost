package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	EvidenceTypeGitStatus       = "git_status"
	EvidenceTypeFileMetadata    = "file_metadata"
	EvidenceTypeDiffSummary     = "diff_summary"
	EvidenceTypeProjectMetadata = "project_metadata"
	EvidenceTypeCommandResult   = "command_result"
	EvidenceTypeTestResult      = "test_result"
	EvidenceTypeManualNote      = "manual_note"
	EvidenceTypeModelClaim      = "model_claim"
)

// ReportedCommand captures a caller-supplied command result. It is recorded as
// evidence; Bifrost does not execute the command.
type ReportedCommand struct {
	ID         string    `json:"id,omitempty"`
	Command    string    `json:"command"`
	ExitCode   int       `json:"exit_code"`
	CapturedAt time.Time `json:"captured_at,omitempty"`
	Summary    string    `json:"summary,omitempty"`
	TestResult bool      `json:"test_result,omitempty"`
}

// ManualEvidence captures an explicit human/user note as evidence.
type ManualEvidence struct {
	ID         string    `json:"id,omitempty"`
	Text       string    `json:"text"`
	Source     string    `json:"source,omitempty"`
	ObservedAt time.Time `json:"observed_at,omitempty"`
}

// NewCommandEvidence creates a command_result or test_result evidence record
// from caller-reported data.
func NewCommandEvidence(command ReportedCommand, observedAt time.Time) (CommandObservedV2, EvidenceV2) {
	if command.CapturedAt.IsZero() {
		command.CapturedAt = observedAt
	}
	cmdID := command.ID
	if cmdID == "" {
		cmdID = evidenceID("cmd", map[string]any{
			"command":     command.Command,
			"exit_code":   command.ExitCode,
			"captured_at": command.CapturedAt.UTC().Format(time.RFC3339),
		})
	}
	typ := EvidenceTypeCommandResult
	if command.TestResult {
		typ = EvidenceTypeTestResult
	}
	observed := CommandObservedV2{
		ID:         cmdID,
		Command:    command.Command,
		ExitCode:   command.ExitCode,
		CapturedAt: command.CapturedAt.UTC(),
		Summary:    command.Summary,
	}
	data := map[string]any{
		"id":          observed.ID,
		"command":     observed.Command,
		"exit_code":   observed.ExitCode,
		"captured_at": observed.CapturedAt,
		"summary":     observed.Summary,
	}
	return observed, EvidenceV2{
		ID:         evidenceID("ev_cmd", data),
		Type:       typ,
		Source:     "reported.command",
		ObservedAt: observedAt,
		Summary:    commandEvidenceSummary(command),
		Data:       data,
	}
}

// NewManualEvidence creates a manual_note evidence record.
func NewManualEvidence(note ManualEvidence, observedAt time.Time) EvidenceV2 {
	if note.ObservedAt.IsZero() {
		note.ObservedAt = observedAt
	}
	source := note.Source
	if source == "" {
		source = "manual"
	}
	data := map[string]any{
		"text":        note.Text,
		"source":      source,
		"observed_at": note.ObservedAt.UTC(),
	}
	id := note.ID
	if id == "" {
		id = evidenceID("ev_manual", data)
	}
	return EvidenceV2{
		ID:         id,
		Type:       EvidenceTypeManualNote,
		Source:     source,
		ObservedAt: note.ObservedAt.UTC(),
		Summary:    note.Text,
		Data:       data,
	}
}

func commandEvidenceSummary(command ReportedCommand) string {
	state := "passed"
	if command.ExitCode != 0 {
		state = "failed"
	}
	if command.Summary != "" {
		return command.Command + " " + state + ": " + command.Summary
	}
	return command.Command + " " + state
}

// ValidateEvidenceV2 validates a standalone evidence record.
func ValidateEvidenceV2(ev *EvidenceV2) error {
	if ev == nil {
		return &ValidationError{Subject: "evidence.v2", Issues: []ValidationIssue{{Field: "$", Value: "nil", Expected: "evidence object"}}}
	}
	errs := &ValidationError{Subject: "evidence.v2"}
	validateEvidenceRecord(errs, "evidence", *ev)
	return errs.errOrNil()
}

// EvidencePath returns the path for a structured evidence record.
func EvidencePath(projectRoot, id string) string {
	return filepath.Join(EvidenceDir(projectRoot), id+".json")
}

// WriteEvidenceV2 writes one evidence record under `.bifrost/evidence/`.
func WriteEvidenceV2(projectRoot string, ev EvidenceV2) error {
	if err := ValidateEvidenceV2(&ev); err != nil {
		return err
	}
	if err := os.MkdirAll(EvidenceDir(projectRoot), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(ev, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := EvidencePath(projectRoot, ev.ID) + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, EvidencePath(projectRoot, ev.ID))
}

// ReadEvidenceV2 reads and validates one structured evidence record.
func ReadEvidenceV2(projectRoot, id string) (*EvidenceV2, error) {
	if !isSafeEvidenceID(id) {
		return nil, fmt.Errorf("invalid evidence ID: %s", id)
	}
	data, err := os.ReadFile(EvidencePath(projectRoot, id))
	if err != nil {
		return nil, err
	}
	var ev EvidenceV2
	if err := json.Unmarshal(data, &ev); err != nil {
		return nil, err
	}
	if err := ValidateEvidenceV2(&ev); err != nil {
		return nil, err
	}
	return &ev, nil
}

// ListEvidenceV2 reads evidence records from `.bifrost/evidence/`.
func ListEvidenceV2(projectRoot string) ([]EvidenceV2, error) {
	entries, err := os.ReadDir(EvidenceDir(projectRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []EvidenceV2
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		ev, err := ReadEvidenceV2(projectRoot, id)
		if err != nil {
			return nil, err
		}
		out = append(out, *ev)
	}
	sortEvidence(out)
	return out, nil
}

// WriteEvidenceRecordsV2 writes each evidence record to `.bifrost/evidence/`.
func WriteEvidenceRecordsV2(projectRoot string, evidence []EvidenceV2) error {
	seen := map[string]bool{}
	for _, ev := range evidence {
		if seen[ev.ID] {
			continue
		}
		seen[ev.ID] = true
		if err := WriteEvidenceV2(projectRoot, ev); err != nil {
			return err
		}
	}
	return nil
}

func mergeEvidence(existing, incoming []EvidenceV2) []EvidenceV2 {
	byID := make(map[string]EvidenceV2, len(existing)+len(incoming))
	for _, ev := range existing {
		if ev.ID != "" {
			byID[ev.ID] = ev
		}
	}
	for _, ev := range incoming {
		if ev.ID != "" {
			byID[ev.ID] = ev
		}
	}
	out := make([]EvidenceV2, 0, len(byID))
	for _, ev := range byID {
		out = append(out, ev)
	}
	sortEvidence(out)
	return out
}

func sortEvidence(evidence []EvidenceV2) {
	sort.Slice(evidence, func(i, j int) bool {
		if !evidence[i].ObservedAt.Equal(evidence[j].ObservedAt) {
			return evidence[i].ObservedAt.After(evidence[j].ObservedAt)
		}
		return evidence[i].ID < evidence[j].ID
	})
}

func evidenceID(prefix string, data any) string {
	encoded, err := json.Marshal(data)
	if err != nil {
		encoded = []byte(fmt.Sprintf("%v", data))
	}
	return stableID(prefix, string(encoded))
}

func isSafeEvidenceID(id string) bool {
	if id == "" || len(id) > 120 {
		return false
	}
	for _, r := range id {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}
