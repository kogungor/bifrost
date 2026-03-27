package snapshot

import (
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ErrNoPlan is returned when no plan file exists.
var ErrNoPlan = fmt.Errorf("no plan found")

// Plan lifecycle statuses.
const (
	PlanStatusDraft     = "draft"
	PlanStatusActive    = "active"
	PlanStatusCompleted = "completed"
	PlanStatusArchived  = "archived"
)

// Consensus states.
const (
	ConsensusNone      = "none"
	ConsensusReached   = "reached"
	ConsensusOverridden = "overridden"
)

// Activation reasons.
const (
	ActivationConsensus     = "consensus"
	ActivationForceAccepted = "force_accepted"
)

// Review outcomes.
const (
	ReviewOutcomeApproved      = "approved"
	ReviewOutcomeNeedsRevision = "needs_revision"
)

// Plan represents a Bifrost implementation plan.
type Plan struct {
	BifrostVersion int       `yaml:"bifrost_version"`
	CreatedAt      time.Time `yaml:"created_at"`
	UpdatedAt      time.Time `yaml:"updated_at"`
	SourceTool     string    `yaml:"source_tool"`
	Project        string    `yaml:"project"`
	Status         string    `yaml:"status"` // draft, active, completed, archived

	// Consensus fields
	PlanVersion      int    `yaml:"plan_version"`                  // increments on every content edit
	ProposedBy       string `yaml:"proposed_by,omitempty"`          // source_tool that created the plan
	MaxRevisions     int    `yaml:"max_revisions,omitempty"`        // deadlock threshold, default 3
	RevisionCount    int    `yaml:"revision_count,omitempty"`       // how many revise cycles
	ConsensusState   string `yaml:"consensus_state,omitempty"`      // none | reached | overridden
	ActivationReason string `yaml:"activation_reason,omitempty"`    // consensus | force_accepted
	DeadlockDetected bool   `yaml:"deadlock_detected,omitempty"`
	DeadlockReason   string `yaml:"deadlock_reason,omitempty"`

	Title       string
	Goal        string
	Steps       []PlanStep
	Constraints []string
	ReviewNotes []ReviewNote
}

// PlanStep represents a single step in a plan.
type PlanStep struct {
	ID          string // stable short hash, assigned on creation, never changes on reorder
	Description string
	Status      string // "pending", "done", "blocked"
	Files       []string
}

// GenerateStepID creates a stable 8-hex-character ID from the step description and plan creation time.
func GenerateStepID(description string, createdAt time.Time) string {
	return fmt.Sprintf("%08x", crc32.ChecksumIEEE([]byte(description+createdAt.UTC().Format(time.RFC3339))))
}

// ReviewNote represents an inline review observation.
type ReviewNote struct {
	From        string
	At          time.Time // when the review was written
	PlanVersion int       // which plan_version was reviewed
	Outcome     string    // approved | needs_revision (empty for legacy notes)
	Text        string
}

// planFrontmatter holds the YAML frontmatter fields for a plan.
type planFrontmatter struct {
	BifrostVersion int    `yaml:"bifrost_version"`
	CreatedAt      string `yaml:"created_at"`
	UpdatedAt      string `yaml:"updated_at"`
	SourceTool     string `yaml:"source_tool"`
	Project        string `yaml:"project"`
	Status         string `yaml:"status"`
	// Legacy field for backward compatibility
	Timestamp string `yaml:"timestamp"`
	// Consensus fields
	PlanVersion      int    `yaml:"plan_version"`
	ProposedBy       string `yaml:"proposed_by,omitempty"`
	MaxRevisions     int    `yaml:"max_revisions,omitempty"`
	RevisionCount    int    `yaml:"revision_count,omitempty"`
	ConsensusState   string `yaml:"consensus_state,omitempty"`
	ActivationReason string `yaml:"activation_reason,omitempty"`
	DeadlockDetected bool   `yaml:"deadlock_detected,omitempty"`
	DeadlockReason   string `yaml:"deadlock_reason,omitempty"`
}

// CompletionPct returns the percentage of steps marked as done.
// Returns 0 if there are no steps.
func (p *Plan) CompletionPct() int {
	if len(p.Steps) == 0 {
		return 0
	}
	done := 0
	for _, s := range p.Steps {
		if s.Status == "done" {
			done++
		}
	}
	return (done * 100) / len(p.Steps)
}

// StepSummary returns counts of steps by status: done, pending, blocked.
func (p *Plan) StepSummary() (done, pending, blocked int) {
	for _, s := range p.Steps {
		switch s.Status {
		case "done":
			done++
		case "blocked":
			blocked++
		default:
			pending++
		}
	}
	return
}

// LatestReviewOutcome returns the outcome of the most recent ReviewNote, or "" if none.
func (p *Plan) LatestReviewOutcome() string {
	if len(p.ReviewNotes) == 0 {
		return ""
	}
	return p.ReviewNotes[len(p.ReviewNotes)-1].Outcome
}

// IsDeadlocked returns true when the revision count has reached the max threshold
// and the latest review outcome is still needs_revision.
func (p *Plan) IsDeadlocked() bool {
	if p.DeadlockDetected {
		return true
	}
	max := p.MaxRevisions
	if max <= 0 {
		max = 3
	}
	return p.RevisionCount >= max && p.LatestReviewOutcome() == ReviewOutcomeNeedsRevision
}

// maxPlanNameLen limits plan name length.
const maxPlanNameLen = 100

// ValidatePlanName checks that a plan name is safe and well-formed.
func ValidatePlanName(name string) error {
	if name == "" {
		return fmt.Errorf("plan name cannot be empty")
	}
	if len(name) > maxPlanNameLen {
		return fmt.Errorf("plan name exceeds %d characters", maxPlanNameLen)
	}
	if strings.Contains(name, "..") || strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return fmt.Errorf("invalid plan name: %s", name)
	}
	if name == "." {
		return fmt.Errorf("invalid plan name: %s", name)
	}
	return nil
}

// PlanPath returns the path to a named plan file.
func PlanPath(projectRoot, name string) string {
	return filepath.Join(projectRoot, bifrostDir, name+".plan.md")
}

// planLockPath returns the path to the lock file for a named plan.
func planLockPath(projectRoot, name string) string {
	return filepath.Join(projectRoot, bifrostDir, name+".plan.lock")
}

// ReadPlan reads and parses a named plan from the project.
func ReadPlan(projectRoot, name string) (*Plan, error) {
	path := PlanPath(projectRoot, name)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoPlan
		}
		return nil, err
	}
	return ParsePlan(data)
}

// WritePlan writes a named plan atomically with file locking.
func WritePlan(projectRoot, name string, p *Plan) error {
	if err := EnsureDir(projectRoot); err != nil {
		return err
	}

	// Acquire file lock
	lockPath := planLockPath(projectRoot, name)
	unlock, err := acquireLock(lockPath)
	if err != nil {
		return fmt.Errorf("acquire plan lock: %w", err)
	}
	defer unlock()

	// Ensure every step has a stable ID before writing
	for i := range p.Steps {
		if p.Steps[i].ID == "" {
			p.Steps[i].ID = GenerateStepID(p.Steps[i].Description, p.CreatedAt)
		}
	}

	data := RenderPlan(p)

	path := PlanPath(projectRoot, name)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(data), 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// DeletePlan removes a named plan file.
func DeletePlan(projectRoot, name string) error {
	path := PlanPath(projectRoot, name)
	err := os.Remove(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNoPlan
		}
		return err
	}
	// Clean up lock file if it exists
	os.Remove(planLockPath(projectRoot, name))
	return nil
}

// ListPlans returns the names of all plan files in .bifrost/.
func ListPlans(projectRoot string) ([]string, error) {
	pattern := filepath.Join(projectRoot, bifrostDir, "*.plan.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, m := range matches {
		base := filepath.Base(m)
		name := strings.TrimSuffix(base, ".plan.md")
		names = append(names, name)
	}
	return names, nil
}

// ParsePlan parses a plan from raw markdown bytes.
func ParsePlan(data []byte) (*Plan, error) {
	content := string(data)

	fm, body, err := extractPlanFrontmatter(content)
	if err != nil {
		return nil, err
	}

	// Handle created_at/updated_at or legacy timestamp field
	createdAtStr := fm.CreatedAt
	updatedAtStr := fm.UpdatedAt
	if createdAtStr == "" && fm.Timestamp != "" {
		createdAtStr = fm.Timestamp
		updatedAtStr = fm.Timestamp
	}

	createdAt, err := time.Parse(time.RFC3339, createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("invalid created_at %q: %w", createdAtStr, err)
	}

	updatedAt := createdAt
	if updatedAtStr != "" {
		updatedAt, err = time.Parse(time.RFC3339, updatedAtStr)
		if err != nil {
			return nil, fmt.Errorf("invalid updated_at %q: %w", updatedAtStr, err)
		}
	}

	status := fm.Status
	if status == "" {
		status = PlanStatusDraft
	}

	planVersion := fm.PlanVersion
	if planVersion <= 0 {
		planVersion = 1
	}
	maxRevisions := fm.MaxRevisions
	if maxRevisions <= 0 {
		maxRevisions = 3
	}

	p := &Plan{
		BifrostVersion:   fm.BifrostVersion,
		CreatedAt:        createdAt,
		UpdatedAt:        updatedAt,
		SourceTool:       fm.SourceTool,
		Project:          fm.Project,
		Status:           status,
		PlanVersion:      planVersion,
		ProposedBy:       fm.ProposedBy,
		MaxRevisions:     maxRevisions,
		RevisionCount:    fm.RevisionCount,
		ConsensusState:   fm.ConsensusState,
		ActivationReason: fm.ActivationReason,
		DeadlockDetected: fm.DeadlockDetected,
		DeadlockReason:   fm.DeadlockReason,
	}

	sections := parseSections(body)

	// Title comes from the first # heading
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "# ") {
			p.Title = strings.TrimPrefix(line, "# ")
			break
		}
	}

	p.Goal = strings.TrimSpace(sections["Goal"])
	p.Steps = parsePlanSteps(sections["Steps"])
	p.Constraints = parsePlanList(sections["Constraints"])
	p.ReviewNotes = parsePlanReviewNotes(sections["Review Notes"])

	return p, nil
}

// RenderPlan serializes a plan to markdown with YAML frontmatter.
func RenderPlan(p *Plan) string {
	var b strings.Builder

	// Frontmatter
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("bifrost_version: %d\n", p.BifrostVersion))
	b.WriteString(fmt.Sprintf("created_at: %s\n", p.CreatedAt.UTC().Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("updated_at: %s\n", p.UpdatedAt.UTC().Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("source_tool: %s\n", p.SourceTool))
	b.WriteString(fmt.Sprintf("project: %s\n", p.Project))
	b.WriteString(fmt.Sprintf("status: %s\n", p.Status))
	b.WriteString(fmt.Sprintf("plan_version: %d\n", p.PlanVersion))
	if p.ProposedBy != "" {
		b.WriteString(fmt.Sprintf("proposed_by: %s\n", p.ProposedBy))
	}
	maxRev := p.MaxRevisions
	if maxRev <= 0 {
		maxRev = 3
	}
	b.WriteString(fmt.Sprintf("max_revisions: %d\n", maxRev))
	if p.RevisionCount > 0 {
		b.WriteString(fmt.Sprintf("revision_count: %d\n", p.RevisionCount))
	}
	if p.ConsensusState != "" {
		b.WriteString(fmt.Sprintf("consensus_state: %s\n", p.ConsensusState))
	}
	if p.ActivationReason != "" {
		b.WriteString(fmt.Sprintf("activation_reason: %s\n", p.ActivationReason))
	}
	if p.DeadlockDetected {
		b.WriteString("deadlock_detected: true\n")
		if p.DeadlockReason != "" {
			b.WriteString(fmt.Sprintf("deadlock_reason: %s\n", p.DeadlockReason))
		}
	}
	b.WriteString("---\n\n")

	// Title
	b.WriteString(fmt.Sprintf("# %s\n\n", p.Title))

	// Goal
	b.WriteString("## Goal\n")
	b.WriteString(p.Goal + "\n\n")

	// Steps
	b.WriteString("## Steps\n")
	if len(p.Steps) == 0 {
		b.WriteString("No steps defined.\n")
	} else {
		for _, step := range p.Steps {
			switch step.Status {
			case "done":
				b.WriteString(fmt.Sprintf("- [x] %s\n", step.Description))
			case "blocked":
				b.WriteString(fmt.Sprintf("- [!] %s\n", step.Description))
			default:
				b.WriteString(fmt.Sprintf("- [ ] %s\n", step.Description))
			}
			if step.ID != "" {
				b.WriteString(fmt.Sprintf("  - id: %s\n", step.ID))
			}
			for _, f := range step.Files {
				b.WriteString(fmt.Sprintf("  - `%s`\n", f))
			}
		}
	}
	b.WriteString("\n")

	// Constraints
	b.WriteString("## Constraints\n")
	if len(p.Constraints) == 0 {
		b.WriteString("No constraints.\n")
	} else {
		for _, c := range p.Constraints {
			b.WriteString(fmt.Sprintf("- %s\n", c))
		}
	}
	b.WriteString("\n")

	// Review Notes
	b.WriteString("## Review Notes\n")
	if len(p.ReviewNotes) == 0 {
		b.WriteString("No review notes yet.\n")
	} else {
		for _, rn := range p.ReviewNotes {
			if rn.Outcome != "" {
				at := rn.At.UTC().Format(time.RFC3339)
				b.WriteString(fmt.Sprintf("> [%s | %s | v%d | %s] %s\n", rn.From, at, rn.PlanVersion, rn.Outcome, rn.Text))
			} else {
				// Legacy format for notes without outcome
				b.WriteString(fmt.Sprintf("> [%s] %s\n", rn.From, rn.Text))
			}
		}
	}

	return b.String()
}

func extractPlanFrontmatter(content string) (*planFrontmatter, string, error) {
	if !strings.HasPrefix(content, "---\n") {
		return nil, "", fmt.Errorf("plan missing YAML frontmatter")
	}

	rest := content[4:]
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		if strings.HasSuffix(rest, "\n---") {
			idx = len(rest) - 4
		} else {
			return nil, "", fmt.Errorf("plan frontmatter not closed")
		}
	}

	yamlStr := rest[:idx]
	body := rest[idx+4:]

	var fm planFrontmatter
	if err := yaml.Unmarshal([]byte(yamlStr), &fm); err != nil {
		return nil, "", fmt.Errorf("invalid frontmatter YAML: %w", err)
	}

	// Require either created_at or legacy timestamp
	if fm.CreatedAt == "" && fm.Timestamp == "" {
		return nil, "", fmt.Errorf("frontmatter missing required field: created_at")
	}
	if fm.SourceTool == "" {
		return nil, "", fmt.Errorf("frontmatter missing required field: source_tool")
	}

	return &fm, body, nil
}

func parsePlanSteps(section string) []PlanStep {
	var steps []PlanStep
	lines := strings.Split(section, "\n")

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		var status string
		var desc string

		if strings.HasPrefix(line, "- [x] ") {
			status = "done"
			desc = strings.TrimPrefix(line, "- [x] ")
		} else if strings.HasPrefix(line, "- [ ] ") {
			status = "pending"
			desc = strings.TrimPrefix(line, "- [ ] ")
		} else if strings.HasPrefix(line, "- [!] ") {
			status = "blocked"
			desc = strings.TrimPrefix(line, "- [!] ")
		} else {
			continue
		}

		step := PlanStep{
			Description: desc,
			Status:      status,
		}

		// Collect sub-items: id line and file paths
		for i+1 < len(lines) {
			nextLine := strings.TrimSpace(lines[i+1])
			if strings.HasPrefix(nextLine, "- id: ") {
				step.ID = strings.TrimPrefix(nextLine, "- id: ")
				i++
			} else if strings.HasPrefix(nextLine, "- `") && strings.HasSuffix(nextLine, "`") {
				filePath := nextLine[3 : len(nextLine)-1] // strip "- `" and trailing "`"
				step.Files = append(step.Files, filePath)
				i++
			} else if strings.HasPrefix(nextLine, "files: ") {
				// Legacy comma-separated format
				filesStr := strings.TrimPrefix(nextLine, "files: ")
				for _, f := range strings.Split(filesStr, ", ") {
					f = strings.TrimSpace(f)
					if f != "" {
						step.Files = append(step.Files, f)
					}
				}
				i++
			} else {
				break
			}
		}

		steps = append(steps, step)
	}

	return steps
}

func parsePlanList(section string) []string {
	var items []string
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") {
			items = append(items, strings.TrimPrefix(line, "- "))
		}
	}
	return items
}

func parsePlanReviewNotes(section string) []ReviewNote {
	var notes []ReviewNote
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "> [") {
			continue
		}
		inner := strings.TrimPrefix(line, "> [")
		idx := strings.Index(inner, "] ")
		if idx < 0 {
			continue
		}
		header := inner[:idx]
		text := inner[idx+2:]

		// New format: "from | at | vN | outcome"
		parts := strings.Split(header, " | ")
		if len(parts) == 4 {
			from := strings.TrimSpace(parts[0])
			atStr := strings.TrimSpace(parts[1])
			versionStr := strings.TrimPrefix(strings.TrimSpace(parts[2]), "v")
			outcome := strings.TrimSpace(parts[3])

			at, _ := time.Parse(time.RFC3339, atStr)
			version := 0
			fmt.Sscanf(versionStr, "%d", &version)

			notes = append(notes, ReviewNote{
				From:        from,
				At:          at,
				PlanVersion: version,
				Outcome:     outcome,
				Text:        text,
			})
		} else {
			// Legacy format: "from"
			notes = append(notes, ReviewNote{From: header, Text: text})
		}
	}
	return notes
}

// acquireLock creates a lock file and returns an unlock function.
// Uses O_CREATE|O_EXCL for atomic creation. Retries briefly on contention.
func acquireLock(lockPath string) (func(), error) {
	// Try to create lock file atomically
	for attempts := 0; attempts < 10; attempts++ {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err == nil {
			f.Close()
			return func() { os.Remove(lockPath) }, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
		// Check if lock is stale (older than 30 seconds)
		info, statErr := os.Stat(lockPath)
		if statErr != nil || time.Since(info.ModTime()) > 30*time.Second {
			os.Remove(lockPath)
			continue
		}
		time.Sleep(50 * time.Millisecond)
	}
	// Force-remove stale lock after retries exhausted
	os.Remove(lockPath)
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock after retries: %w", err)
	}
	f.Close()
	return func() { os.Remove(lockPath) }, nil
}
