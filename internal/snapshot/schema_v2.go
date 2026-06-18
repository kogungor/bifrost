package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// SnapshotSchemaV2 is the canonical JSON schema identifier for snapshots.
	SnapshotSchemaV2 = "snapshot.v2"
	// PlanSchemaV2 is the canonical JSON schema identifier for plans.
	PlanSchemaV2 = "plan.v2"

	sessionJSONFile = "session.json"
	plansDir        = "plans"
	evidenceDir     = "evidence"
)

// SnapshotJSONPath returns the canonical JSON snapshot path for a project.
func SnapshotJSONPath(projectRoot string) string {
	return filepath.Join(projectRoot, bifrostDir, sessionJSONFile)
}

// PlansDir returns the directory for canonical JSON plans.
func PlansDir(projectRoot string) string {
	return filepath.Join(projectRoot, bifrostDir, plansDir)
}

// PlanJSONPath returns the canonical JSON path for a named plan.
func PlanJSONPath(projectRoot, name string) string {
	return filepath.Join(PlansDir(projectRoot), name+".json")
}

// EvidenceDir returns the directory for optional structured evidence records.
func EvidenceDir(projectRoot string) string {
	return filepath.Join(projectRoot, bifrostDir, evidenceDir)
}

// SnapshotV2 is the canonical JSON model for an integrity-aware snapshot.
type SnapshotV2 struct {
	SchemaVersion  string              `json:"schema_version"`
	ID             string              `json:"id"`
	Project        ProjectRefV2        `json:"project"`
	CapturedAt     time.Time           `json:"captured_at"`
	Source         SourceV2            `json:"source"`
	Session        SessionStateV2      `json:"session"`
	Observed       ObservedV2          `json:"observed,omitempty"`
	Interpretation InterpretationV2    `json:"interpretation,omitempty"`
	ActiveFiles    []ActiveFileV2      `json:"active_files,omitempty"`
	ActivePlan     *ActivePlanRefV2    `json:"active_plan,omitempty"`
	Integrity      SnapshotIntegrityV2 `json:"integrity,omitempty"`
	Evidence       []EvidenceV2        `json:"evidence,omitempty"`

	Extra map[string]json.RawMessage `json:"-"`
}

type ProjectRefV2 struct {
	Name string `json:"name"`
	Root string `json:"root,omitempty"`
}

type SourceV2 struct {
	Tool           string `json:"tool"`
	Agent          string `json:"agent,omitempty"`
	BifrostVersion string `json:"bifrost_version,omitempty"`
}

type SessionStateV2 struct {
	Intent       string `json:"intent,omitempty"`
	Pressure     string `json:"pressure,omitempty"`
	Task         string `json:"task"`
	NextStep     string `json:"next_step,omitempty"`
	SessionStart string `json:"session_start,omitempty"`
}

type ObservedV2 struct {
	Git      *GitObservedV2      `json:"git,omitempty"`
	Files    []FileObservedV2    `json:"files,omitempty"`
	Commands []CommandObservedV2 `json:"commands,omitempty"`
	Project  *ProjectObservedV2  `json:"project,omitempty"`
}

type GitObservedV2 struct {
	Branch         string   `json:"branch,omitempty"`
	Commit         string   `json:"commit,omitempty"`
	Dirty          bool     `json:"dirty,omitempty"`
	ChangedFiles   []string `json:"changed_files,omitempty"`
	StagedFiles    []string `json:"staged_files,omitempty"`
	UntrackedFiles []string `json:"untracked_files,omitempty"`
}

type FileObservedV2 struct {
	Path   string    `json:"path"`
	Exists bool      `json:"exists"`
	SHA256 string    `json:"sha256,omitempty"`
	MTime  time.Time `json:"mtime,omitempty"`
	Size   int64     `json:"size,omitempty"`
}

type CommandObservedV2 struct {
	ID         string    `json:"id"`
	Command    string    `json:"command"`
	ExitCode   int       `json:"exit_code"`
	CapturedAt time.Time `json:"captured_at"`
	Summary    string    `json:"summary,omitempty"`
}

type ProjectObservedV2 struct {
	Root                     string   `json:"root,omitempty"`
	BifrostMDExists          bool     `json:"bifrost_md_exists,omitempty"`
	PackageManagerCandidates []string `json:"package_manager_candidates,omitempty"`
	CommandCandidates        []string `json:"command_candidates,omitempty"`
}

type InterpretationV2 struct {
	StatusItems      []StatusItemV2   `json:"status_items,omitempty"`
	Decisions        []DecisionV2     `json:"decisions,omitempty"`
	EnvironmentNotes []string         `json:"environment_notes,omitempty"`
	Assumptions      []AssumptionV2   `json:"assumptions,omitempty"`
	OpenQuestions    []OpenQuestionV2 `json:"open_questions,omitempty"`
	Risks            []RiskV2         `json:"risks,omitempty"`
}

type StatusItemV2 struct {
	ID           string          `json:"id"`
	Text         string          `json:"text"`
	State        string          `json:"state"`
	EvidenceRefs []string        `json:"evidence_refs,omitempty"`
	Verification *VerificationV2 `json:"verification,omitempty"`
}

type VerificationV2 struct {
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
}

type DecisionV2 struct {
	ID           string   `json:"id"`
	Text         string   `json:"text"`
	Scope        string   `json:"scope,omitempty"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
}

type AssumptionV2 struct {
	ID                string `json:"id"`
	Text              string `json:"text"`
	VerificationState string `json:"verification_state,omitempty"`
}

type OpenQuestionV2 struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	Severity string `json:"severity,omitempty"`
}

type RiskV2 struct {
	ID         string `json:"id"`
	Text       string `json:"text"`
	Severity   string `json:"severity,omitempty"`
	Mitigation string `json:"mitigation,omitempty"`
}

type ActiveFileV2 struct {
	Path         string   `json:"path"`
	Note         string   `json:"note,omitempty"`
	Trust        TrustV2  `json:"trust,omitempty"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
}

type TrustV2 struct {
	Implementation string `json:"implementation,omitempty"`
	Tests          string `json:"tests,omitempty"`
	Security       string `json:"security,omitempty"`
	Architecture   string `json:"architecture,omitempty"`
	Freshness      string `json:"freshness,omitempty"`
	Evidence       string `json:"evidence,omitempty"`
}

type ActivePlanRefV2 struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type SnapshotIntegrityV2 struct {
	RedactionApplied bool   `json:"redaction_applied,omitempty"`
	VerifyStatus     string `json:"verify_status,omitempty"`
}

type EvidenceV2 struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Source     string         `json:"source"`
	ObservedAt time.Time      `json:"observed_at"`
	Summary    string         `json:"summary,omitempty"`
	Data       map[string]any `json:"data,omitempty"`
}

// PlanV2 is the canonical JSON model for an integrity-aware plan.
type PlanV2 struct {
	SchemaVersion string       `json:"schema_version"`
	Name          string       `json:"name"`
	Title         string       `json:"title"`
	Goal          string       `json:"goal"`
	Status        string       `json:"status"`
	Version       string       `json:"version"`
	Project       ProjectRefV2 `json:"project,omitempty"`
	Source        SourceV2     `json:"source,omitempty"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
	Steps         []PlanStepV2 `json:"steps,omitempty"`
	Consensus     ConsensusV2  `json:"consensus,omitempty"`
	Review        PlanReviewV2 `json:"review,omitempty"`

	Extra map[string]json.RawMessage `json:"-"`
}

type PlanStepV2 struct {
	ID            string                  `json:"id"`
	Title         string                  `json:"title"`
	Status        string                  `json:"status"`
	ExpectedFiles []string                `json:"expected_files,omitempty"`
	Verification  *PlanStepVerificationV2 `json:"verification,omitempty"`
}

type PlanStepVerificationV2 struct {
	Required   bool                `json:"required,omitempty"`
	Commands   []string            `json:"commands,omitempty"`
	LastResult *CommandResultRefV2 `json:"last_result,omitempty"`
}

type CommandResultRefV2 struct {
	State       string    `json:"state"`
	Command     string    `json:"command,omitempty"`
	ExitCode    int       `json:"exit_code,omitempty"`
	CapturedAt  time.Time `json:"captured_at,omitempty"`
	EvidenceRef string    `json:"evidence_ref,omitempty"`
}

type PlanReviewV2 struct {
	Outcome string         `json:"outcome,omitempty"`
	Notes   []string       `json:"notes,omitempty"`
	Details []ReviewNoteV2 `json:"details,omitempty"`
}

type ReviewNoteV2 struct {
	From        string    `json:"from,omitempty"`
	At          time.Time `json:"at,omitempty"`
	PlanVersion int       `json:"plan_version,omitempty"`
	Outcome     string    `json:"outcome,omitempty"`
	Text        string    `json:"text"`
}

type ConsensusV2 struct {
	PlanVersion      int    `json:"plan_version,omitempty"`
	ProposedBy       string `json:"proposed_by,omitempty"`
	MaxRevisions     int    `json:"max_revisions,omitempty"`
	RevisionCount    int    `json:"revision_count,omitempty"`
	ConsensusState   string `json:"consensus_state,omitempty"`
	ActivationReason string `json:"activation_reason,omitempty"`
	DeadlockDetected bool   `json:"deadlock_detected,omitempty"`
	DeadlockReason   string `json:"deadlock_reason,omitempty"`
}

// ValidationIssue describes one actionable schema validation issue.
type ValidationIssue struct {
	Field    string
	Value    string
	Expected string
}

// ValidationError groups schema validation issues for one object.
type ValidationError struct {
	Subject string
	Issues  []ValidationIssue
}

func (e *ValidationError) Error() string {
	if e == nil || len(e.Issues) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("invalid " + e.Subject + " schema")
	for _, issue := range e.Issues {
		b.WriteString(fmt.Sprintf("\n- field: %s\n  value: %s\n  expected: %s", issue.Field, issue.Value, issue.Expected))
	}
	return b.String()
}

func (e *ValidationError) add(field string, value any, expected string) {
	e.Issues = append(e.Issues, ValidationIssue{
		Field:    field,
		Value:    validationValue(value),
		Expected: expected,
	})
}

func validationValue(v any) string {
	switch t := v.(type) {
	case string:
		if t == "" {
			return "(empty)"
		}
		return t
	case time.Time:
		if t.IsZero() {
			return "(zero time)"
		}
		return t.UTC().Format(time.RFC3339)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (e *ValidationError) errOrNil() error {
	if len(e.Issues) == 0 {
		return nil
	}
	return e
}

func ValidateSnapshotV2(s *SnapshotV2) error {
	if s == nil {
		return &ValidationError{Subject: "snapshot.v2", Issues: []ValidationIssue{{Field: "$", Value: "nil", Expected: "snapshot object"}}}
	}
	errs := &ValidationError{Subject: "snapshot.v2"}
	if s.SchemaVersion != SnapshotSchemaV2 {
		errs.add("schema_version", s.SchemaVersion, SnapshotSchemaV2)
	}
	if s.ID == "" {
		errs.add("id", s.ID, "non-empty snapshot ID")
	}
	if s.Project.Name == "" {
		errs.add("project.name", s.Project.Name, "non-empty project name")
	}
	if s.CapturedAt.IsZero() {
		errs.add("captured_at", s.CapturedAt, "RFC3339 timestamp")
	}
	if s.Source.Tool == "" {
		errs.add("source.tool", s.Source.Tool, "non-empty source tool")
	}
	if s.Session.Task == "" {
		errs.add("session.task", s.Session.Task, "non-empty task")
	}
	validateOptionalEnum(errs, "session.intent", s.Session.Intent, validSnapshotIntents)
	validateOptionalEnum(errs, "session.pressure", s.Session.Pressure, validTokenPressures)
	validateOptionalEnum(errs, "integrity.verify_status", s.Integrity.VerifyStatus, validVerifyStatuses)
	for i, item := range s.Interpretation.StatusItems {
		prefix := fmt.Sprintf("interpretation.status_items[%d]", i)
		if item.ID == "" {
			errs.add(prefix+".id", item.ID, "non-empty status item ID")
		}
		if item.Text == "" {
			errs.add(prefix+".text", item.Text, "non-empty status text")
		}
		validateRequiredEnum(errs, prefix+".state", item.State, validWorkStates)
		if item.Verification != nil {
			validateRequiredEnum(errs, prefix+".verification.state", item.Verification.State, validVerificationStates)
		}
	}
	for i, f := range s.ActiveFiles {
		prefix := fmt.Sprintf("active_files[%d]", i)
		if !isSafeRelativePath(f.Path) {
			errs.add(prefix+".path", f.Path, "non-empty relative path without traversal")
		}
		validateTrust(errs, prefix+".trust", f.Trust)
	}
	for i, f := range s.Observed.Files {
		if !isSafeRelativePath(f.Path) {
			errs.add(fmt.Sprintf("observed.files[%d].path", i), f.Path, "non-empty relative path without traversal")
		}
	}
	for i, cmd := range s.Observed.Commands {
		prefix := fmt.Sprintf("observed.commands[%d]", i)
		if cmd.ID == "" {
			errs.add(prefix+".id", cmd.ID, "non-empty command ID")
		}
		if cmd.Command == "" {
			errs.add(prefix+".command", cmd.Command, "non-empty command")
		}
		if cmd.CapturedAt.IsZero() {
			errs.add(prefix+".captured_at", cmd.CapturedAt, "RFC3339 timestamp")
		}
	}
	for i, ev := range s.Evidence {
		validateEvidenceRecord(errs, fmt.Sprintf("evidence[%d]", i), ev)
	}
	return errs.errOrNil()
}

func ValidatePlanV2(p *PlanV2) error {
	if p == nil {
		return &ValidationError{Subject: "plan.v2", Issues: []ValidationIssue{{Field: "$", Value: "nil", Expected: "plan object"}}}
	}
	errs := &ValidationError{Subject: "plan.v2"}
	if p.SchemaVersion != PlanSchemaV2 {
		errs.add("schema_version", p.SchemaVersion, PlanSchemaV2)
	}
	if p.Name == "" {
		errs.add("name", p.Name, "non-empty plan name")
	} else if err := ValidatePlanName(p.Name); err != nil {
		errs.add("name", p.Name, err.Error())
	}
	if p.Title == "" {
		errs.add("title", p.Title, "non-empty title")
	}
	if p.Goal == "" {
		errs.add("goal", p.Goal, "non-empty goal")
	}
	validateRequiredEnum(errs, "status", p.Status, validPlanStatuses)
	if p.Version == "" {
		errs.add("version", p.Version, "non-empty version")
	}
	if p.CreatedAt.IsZero() {
		errs.add("created_at", p.CreatedAt, "RFC3339 timestamp")
	}
	if p.UpdatedAt.IsZero() {
		errs.add("updated_at", p.UpdatedAt, "RFC3339 timestamp")
	}
	for i, step := range p.Steps {
		prefix := fmt.Sprintf("steps[%d]", i)
		if step.ID == "" {
			errs.add(prefix+".id", step.ID, "non-empty step ID")
		}
		if step.Title == "" {
			errs.add(prefix+".title", step.Title, "non-empty step title")
		}
		validateRequiredEnum(errs, prefix+".status", step.Status, validWorkStates)
		for j, path := range step.ExpectedFiles {
			if !isSafeRelativePath(path) {
				errs.add(fmt.Sprintf("%s.expected_files[%d]", prefix, j), path, "non-empty relative path without traversal")
			}
		}
		if step.Verification != nil && step.Verification.LastResult != nil {
			validateRequiredEnum(errs, prefix+".verification.last_result.state", step.Verification.LastResult.State, validVerificationStates)
		}
	}
	return errs.errOrNil()
}

func validateTrust(errs *ValidationError, field string, trust TrustV2) {
	validateOptionalEnum(errs, field+".implementation", trust.Implementation, validTrustLevels)
	validateOptionalEnum(errs, field+".tests", trust.Tests, validTrustLevels)
	validateOptionalEnum(errs, field+".security", trust.Security, validTrustLevels)
	validateOptionalEnum(errs, field+".architecture", trust.Architecture, validTrustLevels)
	validateOptionalEnum(errs, field+".freshness", trust.Freshness, validFreshnessLevels)
	validateOptionalEnum(errs, field+".evidence", trust.Evidence, validEvidenceTrustLevels)
}

func validateEvidenceRecord(errs *ValidationError, prefix string, ev EvidenceV2) {
	if ev.ID == "" {
		errs.add(prefix+".id", ev.ID, "non-empty evidence ID")
	} else if !isSafeEvidenceID(ev.ID) {
		errs.add(prefix+".id", ev.ID, "safe evidence ID")
	}
	if ev.Type == "" {
		errs.add(prefix+".type", ev.Type, "non-empty evidence type")
	}
	if ev.Source == "" {
		errs.add(prefix+".source", ev.Source, "non-empty evidence source")
	}
	if ev.ObservedAt.IsZero() {
		errs.add(prefix+".observed_at", ev.ObservedAt, "RFC3339 timestamp")
	}
}

func validateRequiredEnum(errs *ValidationError, field, value string, allowed map[string]bool) {
	if !allowed[value] {
		errs.add(field, value, strings.Join(mapKeys(allowed), " | "))
	}
}

func validateOptionalEnum(errs *ValidationError, field, value string, allowed map[string]bool) {
	if value == "" {
		return
	}
	validateRequiredEnum(errs, field, value, allowed)
}

var validSnapshotIntents = map[string]bool{
	"planning": true, "implementing": true, "debugging": true, "reviewing": true,
}

var validTokenPressures = map[string]bool{
	"low": true, "medium": true, "high": true, "critical": true,
}

var validWorkStates = map[string]bool{
	"not_started": true, "in_progress": true, "blocked": true, "claimed_done": true, "verified_done": true, "invalidated": true,
}

var validPlanStatuses = map[string]bool{
	PlanStatusDraft: true, PlanStatusActive: true, PlanStatusCompleted: true, PlanStatusArchived: true,
}

var validVerificationStates = map[string]bool{
	"not_run": true, "pass": true, "warn": true, "fail": true, "failed": true, "unverified": true,
}

var validVerifyStatuses = map[string]bool{
	"not_run": true, "pass": true, "warn": true, "fail": true,
}

var validTrustLevels = map[string]bool{
	"high": true, "medium": true, "low": true, "unknown": true,
}

var validEvidenceTrustLevels = map[string]bool{
	"strong": true, "medium": true, "weak": true, "high": true, "low": true, "unknown": true,
}

var validFreshnessLevels = map[string]bool{
	"current": true, "stale": true, "unknown": true,
}

func mapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func isSafeRelativePath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" || strings.Contains(path, "\x00") {
		return false
	}
	normalized := strings.ReplaceAll(path, "\\", "/")
	if filepath.IsAbs(path) || strings.HasPrefix(normalized, "/") {
		return false
	}
	if len(normalized) >= 2 && normalized[1] == ':' {
		return false
	}
	for _, part := range strings.Split(normalized, "/") {
		if part == ".." {
			return false
		}
	}
	cleaned := pathpkg.Clean(normalized)
	return cleaned != "." && cleaned != ".." && !strings.HasPrefix(cleaned, "../")
}

// IsSafeRelativePath reports whether path is a non-empty relative path that
// cannot escape the project root.
func IsSafeRelativePath(path string) bool {
	return isSafeRelativePath(path)
}

// ReadSnapshotV2 reads and validates `.bifrost/session.json`.
func ReadSnapshotV2(projectRoot string) (*SnapshotV2, error) {
	data, err := os.ReadFile(SnapshotJSONPath(projectRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoSnapshot
		}
		return nil, err
	}
	var snap SnapshotV2
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, err
	}
	if err := ValidateSnapshotV2(&snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

// WriteSnapshotV2 writes `.bifrost/session.json` atomically.
func WriteSnapshotV2(projectRoot string, s *SnapshotV2) error {
	if err := ValidateSnapshotV2(s); err != nil {
		return err
	}
	if err := EnsureDir(projectRoot); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if redacted, err := redactBeforeWrite(projectRoot, string(data)); err != nil {
		return err
	} else if redacted != string(data) {
		s.Integrity.RedactionApplied = true
		data, err = json.MarshalIndent(s, "", "  ")
		if err != nil {
			return err
		}
		redacted, err = redactBeforeWrite(projectRoot, string(data))
		if err != nil {
			return err
		}
		data = []byte(redacted)
	}
	data = append(data, '\n')
	if _, err := os.Stat(SnapshotJSONPath(projectRoot)); err == nil {
		if err := ArchiveSnapshotJSON(projectRoot); err != nil {
			return err
		}
		_ = PruneSnapshotJSON(projectRoot, DefaultMaxHistory)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	tmp := SnapshotJSONPath(projectRoot) + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, SnapshotJSONPath(projectRoot)); err != nil {
		return err
	}
	_ = AppendTimelineEvent(projectRoot, TimelineEvent{
		Type:     "snapshot.write",
		Snapshot: s.ID,
		Task:     s.Session.Task,
	})
	return nil
}

// ReadPlanV2 reads and validates `.bifrost/plans/<name>.json`.
func ReadPlanV2(projectRoot, name string) (*PlanV2, error) {
	if err := ValidatePlanName(name); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(PlanJSONPath(projectRoot, name))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoPlan
		}
		return nil, err
	}
	var plan PlanV2
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, err
	}
	if err := ValidatePlanV2(&plan); err != nil {
		return nil, err
	}
	return &plan, nil
}

// WritePlanV2 writes `.bifrost/plans/<name>.json` atomically.
func WritePlanV2(projectRoot string, p *PlanV2) error {
	if err := ValidatePlanV2(p); err != nil {
		return err
	}
	if err := EnsureDir(projectRoot); err != nil {
		return err
	}
	if err := os.MkdirAll(PlansDir(projectRoot), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := PlanJSONPath(projectRoot, p.Name) + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, PlanJSONPath(projectRoot, p.Name)); err != nil {
		return err
	}
	_ = AppendTimelineEvent(projectRoot, TimelineEvent{
		Type:   "plan.write",
		Plan:   p.Name,
		Status: p.Status,
	})
	return nil
}

func (s *SnapshotV2) UnmarshalJSON(data []byte) error {
	type snapshotAlias SnapshotV2
	var alias snapshotAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for _, key := range snapshotV2KnownFields {
		delete(raw, key)
	}
	*s = SnapshotV2(alias)
	if len(raw) > 0 {
		s.Extra = raw
	}
	return nil
}

func (s SnapshotV2) MarshalJSON() ([]byte, error) {
	type snapshotAlias SnapshotV2
	base, err := json.Marshal(snapshotAlias(s))
	if err != nil {
		return nil, err
	}
	return mergeExtraFields(base, s.Extra)
}

func (p *PlanV2) UnmarshalJSON(data []byte) error {
	type planAlias PlanV2
	var alias planAlias
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for _, key := range planV2KnownFields {
		delete(raw, key)
	}
	*p = PlanV2(alias)
	if len(raw) > 0 {
		p.Extra = raw
	}
	return nil
}

func (p PlanV2) MarshalJSON() ([]byte, error) {
	type planAlias PlanV2
	base, err := json.Marshal(planAlias(p))
	if err != nil {
		return nil, err
	}
	return mergeExtraFields(base, p.Extra)
}

var snapshotV2KnownFields = []string{
	"schema_version", "id", "project", "captured_at", "source", "session",
	"observed", "interpretation", "active_files", "active_plan", "integrity", "evidence",
}

var planV2KnownFields = []string{
	"schema_version", "name", "title", "goal", "status", "version", "project", "source", "created_at", "updated_at", "steps", "consensus", "review",
}

func mergeExtraFields(base []byte, extra map[string]json.RawMessage) ([]byte, error) {
	if len(extra) == 0 {
		return base, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(base, &fields); err != nil {
		return nil, err
	}
	for key, value := range extra {
		if _, exists := fields[key]; !exists {
			fields[key] = value
		}
	}
	return json.Marshal(fields)
}
