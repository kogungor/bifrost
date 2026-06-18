package snapshot

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kogungor/bifrost/internal/security"
)

const (
	VerifyPass = "pass"
	VerifyWarn = "warn"
	VerifyFail = "fail"
	VerifyInfo = "info"
)

// VerifyOptions controls non-destructive snapshot verification.
type VerifyOptions struct {
	Now            time.Time
	WarnAfter      time.Duration
	FailAfter      time.Duration
	Strict         bool
	LoadActivePlan func(name string) (string, error)
}

// VerifyResult is the machine-readable output of `bifrost verify`.
type VerifyResult struct {
	Status                string        `json:"status"`
	Strict                bool          `json:"strict"`
	SnapshotID            string        `json:"snapshot_id,omitempty"`
	GeneratedAt           time.Time     `json:"generated_at"`
	Checks                []VerifyCheck `json:"checks"`
	RecommendedNextAction string        `json:"recommended_next_action"`
}

type VerifyCheck struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	Message        string `json:"message"`
	SafeNextAction string `json:"safe_next_action,omitempty"`
}

// VerifySnapshotV2 checks whether a snapshot still matches current local facts.
func VerifySnapshotV2(projectRoot string, snap *SnapshotV2, opts VerifyOptions) VerifyResult {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if opts.WarnAfter == 0 {
		opts.WarnAfter = 2 * time.Hour
	}
	if opts.FailAfter == 0 {
		opts.FailAfter = 24 * time.Hour
	}
	result := VerifyResult{
		Status:      VerifyPass,
		Strict:      opts.Strict,
		GeneratedAt: now,
	}
	if snap == nil {
		result.add(VerifyFail, "schema.valid", "No snapshot available.", "Run /handoff before verifying.")
		result.finalize()
		return result
	}
	result.SnapshotID = snap.ID

	result.addSnapshotAge(now, snap.CapturedAt, opts.WarnAfter, opts.FailAfter)
	result.addGitChecks(projectRoot, snap)
	result.addFileChecks(projectRoot, snap)
	result.addEvidenceChecks(snap)
	result.addQuestionRiskChecks(snap)
	result.addPlanCheck(snap, opts.LoadActivePlan)
	result.addSecurityCheck(projectRoot, snap)
	result.finalize()
	return result
}

func (r *VerifyResult) add(status, id, message, action string) {
	r.Checks = append(r.Checks, VerifyCheck{ID: id, Status: status, Message: message, SafeNextAction: action})
}

func (r *VerifyResult) finalize() {
	status := VerifyPass
	for _, check := range r.Checks {
		if check.Status == VerifyFail {
			status = VerifyFail
			break
		}
		if check.Status == VerifyWarn && status == VerifyPass {
			status = VerifyWarn
		}
	}
	r.Status = status
	r.RecommendedNextAction = recommendedAction(r.Checks)
}

func (r *VerifyResult) addSnapshotAge(now, capturedAt time.Time, warnAfter, failAfter time.Duration) {
	if capturedAt.IsZero() {
		r.add(VerifyFail, "snapshot.age", "Snapshot timestamp is missing.", "Recreate the snapshot with /handoff.")
		return
	}
	age := now.Sub(capturedAt)
	switch {
	case age < 0:
		r.add(VerifyWarn, "snapshot.age", "Snapshot timestamp is in the future.", "Check system clock and snapshot timestamp.")
	case age >= failAfter:
		r.add(VerifyFail, "snapshot.age", fmt.Sprintf("Snapshot is %s old.", formatDuration(age)), "Refresh context with /handoff before continuing.")
	case age >= warnAfter:
		r.add(VerifyWarn, "snapshot.age", fmt.Sprintf("Snapshot is %s old.", formatDuration(age)), "Inspect changed files before trusting the handoff.")
	default:
		r.add(VerifyPass, "snapshot.age", fmt.Sprintf("Snapshot is %s old.", formatDuration(age)), "")
	}
}

func (r *VerifyResult) addGitChecks(projectRoot string, snap *SnapshotV2) {
	if snap.Observed.Git == nil {
		r.add(VerifyWarn, "git.status", "Snapshot has no observed git status.", "Run `bifrost snapshot --enrich`.")
		return
	}
	current, _, err := collectGitEvidence(projectRoot, time.Now().UTC())
	if err != nil {
		if errors.Is(err, errNotGitRepo) {
			r.add(VerifyWarn, "git.status", "Current directory is not a git repository.", "Verify active files manually.")
			return
		}
		r.add(VerifyWarn, "git.status", "Could not collect current git status: "+err.Error(), "Inspect git status manually.")
		return
	}
	if snap.Observed.Git.Branch != "" && current.Branch != snap.Observed.Git.Branch {
		r.add(VerifyFail, "git.branch_match", fmt.Sprintf("Branch changed: snapshot %s, current %s.", snap.Observed.Git.Branch, current.Branch), "Switch to the snapshot branch or refresh the handoff.")
	} else {
		r.add(VerifyPass, "git.branch_match", fmt.Sprintf("Branch: %s.", emptyFallback(current.Branch, "unknown")), "")
	}
	if snap.Observed.Git.Commit != "" && current.Commit != snap.Observed.Git.Commit {
		r.add(VerifyWarn, "git.commit_match", fmt.Sprintf("Commit changed: snapshot %s, current %s.", snap.Observed.Git.Commit, current.Commit), "Review changes since the snapshot before implementing.")
	} else {
		r.add(VerifyPass, "git.commit_match", fmt.Sprintf("Commit: %s.", emptyFallback(current.Commit, "unknown")), "")
	}
	if !sameStringSet(snapshotDirtyFiles(snap.Observed.Git), snapshotDirtyFiles(current)) {
		r.add(VerifyWarn, "git.dirty_changed", "Dirty file set changed after snapshot.", "Run `git status --short` and inspect changed active files.")
	} else {
		r.add(VerifyPass, "git.dirty_changed", "Dirty file set matches snapshot.", "")
	}
}

func (r *VerifyResult) addFileChecks(projectRoot string, snap *SnapshotV2) {
	byPath := map[string]FileObservedV2{}
	for _, file := range snap.Observed.Files {
		byPath[file.Path] = file
	}
	if len(snap.ActiveFiles) == 0 {
		r.add(VerifyInfo, "files.active_exist", "No active files recorded.", "")
		return
	}
	missing := []string{}
	changed := []string{}
	unknownMetadata := []string{}
	for _, active := range snap.ActiveFiles {
		current, err := observeFile(projectRoot, active.Path)
		if err != nil {
			r.add(VerifyWarn, "files.active_exist", "Could not inspect "+active.Path+": "+err.Error(), "Inspect the file manually.")
			continue
		}
		if !current.Exists {
			missing = append(missing, active.Path)
			continue
		}
		previous, ok := byPath[active.Path]
		if !ok {
			unknownMetadata = append(unknownMetadata, active.Path)
			continue
		}
		if (!previous.Exists && current.Exists) || (previous.Exists && fileChanged(previous, current)) {
			changed = append(changed, active.Path)
		}
	}
	if len(missing) > 0 {
		r.add(VerifyFail, "files.active_exist", fmt.Sprintf("Missing active files: %s.", strings.Join(missing, ", ")), "Restore or remove missing active file references.")
	} else {
		r.add(VerifyPass, "files.active_exist", fmt.Sprintf("Active files exist: %d/%d.", len(snap.ActiveFiles), len(snap.ActiveFiles)), "")
	}
	if len(changed) > 0 {
		r.add(VerifyWarn, "files.active_changed", fmt.Sprintf("Active files changed after snapshot: %s.", strings.Join(changed, ", ")), "Inspect changed files before continuing.")
	} else if len(unknownMetadata) > 0 {
		r.add(VerifyWarn, "files.active_changed", fmt.Sprintf("No snapshot metadata for active files: %s.", strings.Join(unknownMetadata, ", ")), "Run `bifrost snapshot --enrich` or inspect active files manually.")
	} else {
		r.add(VerifyPass, "files.active_changed", "No active file metadata changes detected.", "")
	}
}

func (r *VerifyResult) addEvidenceChecks(snap *SnapshotV2) {
	evidence := evidenceIDSet(snap.Evidence)
	unevidenced := []string{}
	unverifiedTests := []string{}
	for _, item := range snap.Interpretation.StatusItems {
		if len(item.EvidenceRefs) == 0 || !allRefsExist(item.EvidenceRefs, evidence) {
			unevidenced = append(unevidenced, item.Text)
		}
		if looksLikeTestClaim(item.Text) && !hasTestEvidence(snap.Evidence) {
			unverifiedTests = append(unverifiedTests, item.Text)
		}
	}
	if len(unevidenced) > 0 {
		r.add(VerifyWarn, "claims.evidence", fmt.Sprintf("%d status claim(s) lack evidence.", len(unevidenced)), "Treat unevidenced status as model interpretation, not repo fact.")
	} else {
		r.add(VerifyPass, "claims.evidence", "Status claims have evidence references.", "")
	}
	if len(unverifiedTests) > 0 {
		r.add(VerifyFail, "commands.test_claims", "Test-pass claim has no test_result or passing command_result evidence.", "Run or inspect tests before assuming they pass.")
	} else {
		r.add(VerifyPass, "commands.test_claims", "No unevidenced test-pass claims detected.", "")
	}
}

func (r *VerifyResult) addQuestionRiskChecks(snap *SnapshotV2) {
	highQuestions := []string{}
	for _, q := range snap.Interpretation.OpenQuestions {
		if strings.EqualFold(q.Severity, "high") {
			highQuestions = append(highQuestions, q.Text)
		}
	}
	if len(highQuestions) > 0 {
		r.add(VerifyFail, "questions.unresolved_high", fmt.Sprintf("High severity open questions: %s.", strings.Join(highQuestions, "; ")), "Resolve high severity questions before coding.")
	} else {
		r.add(VerifyPass, "questions.unresolved_high", "No high severity open questions.", "")
	}
	highRisks := []string{}
	for _, risk := range snap.Interpretation.Risks {
		if strings.EqualFold(risk.Severity, "high") {
			highRisks = append(highRisks, risk.Text)
		}
	}
	if len(highRisks) > 0 {
		r.add(VerifyFail, "risks.unresolved_high", fmt.Sprintf("High severity risks: %s.", strings.Join(highRisks, "; ")), "Mitigate high severity risks before trusting the handoff.")
	} else {
		r.add(VerifyPass, "risks.unresolved_high", "No high severity risks.", "")
	}
}

func (r *VerifyResult) addPlanCheck(snap *SnapshotV2, loadPlan func(name string) (string, error)) {
	if snap.ActivePlan == nil || snap.ActivePlan.Name == "" {
		r.add(VerifyInfo, "plans.status", "No active plan recorded.", "")
		return
	}
	if loadPlan == nil {
		r.add(VerifyWarn, "plans.status", "Active plan recorded but no plan loader configured.", "Inspect the active plan manually.")
		return
	}
	status, err := loadPlan(snap.ActivePlan.Name)
	if err != nil {
		r.add(VerifyFail, "plans.status", "Active plan not readable: "+err.Error(), "Restore or recreate the active plan.")
		return
	}
	switch status {
	case PlanStatusActive:
		r.add(VerifyPass, "plans.status", "Active plan is active.", "")
	case PlanStatusDraft:
		r.add(VerifyWarn, "plans.status", "Active plan is still draft.", "Activate or review the plan before implementation.")
	case PlanStatusCompleted, PlanStatusArchived:
		r.add(VerifyWarn, "plans.status", "Active plan is "+status+".", "Confirm this is still the intended plan.")
	default:
		r.add(VerifyWarn, "plans.status", "Active plan has unknown status "+status+".", "Inspect the active plan manually.")
	}
}

func (r *VerifyResult) addSecurityCheck(projectRoot string, snap *SnapshotV2) {
	data, err := json.Marshal(snap)
	if err != nil {
		r.add(VerifyWarn, "security.secrets", "Could not scan snapshot for secret-like values.", "Inspect snapshot JSON manually.")
		return
	}
	findings := security.ScanString(string(data), security.LoadConfig(projectRoot))
	if security.CountActive(findings) > 0 {
		r.add(VerifyFail, "security.secrets", "Snapshot contains secret-like values: "+security.Summary(findings)+".", "Run `bifrost scrub --write` before sharing or continuing.")
		return
	}
	if allowlisted := security.CountAllowlisted(findings); allowlisted > 0 {
		r.add(VerifyPass, "security.secrets", fmt.Sprintf("No active secret-like values detected (%d allowlisted).", allowlisted), "")
		return
	}
	r.add(VerifyPass, "security.secrets", "No secret-like values detected in snapshot JSON.", "")
}

func fileChanged(previous, current FileObservedV2) bool {
	if previous.SHA256 != "" && current.SHA256 != "" {
		return previous.SHA256 != current.SHA256
	}
	if !previous.MTime.IsZero() && !current.MTime.IsZero() && !previous.MTime.Equal(current.MTime) {
		return true
	}
	return previous.Size != current.Size
}

func snapshotDirtyFiles(git *GitObservedV2) []string {
	if git == nil {
		return nil
	}
	set := map[string]bool{}
	for _, item := range git.ChangedFiles {
		set[item] = true
	}
	for _, item := range git.StagedFiles {
		set[item] = true
	}
	for _, item := range git.UntrackedFiles {
		set[item] = true
	}
	out := make([]string, 0, len(set))
	for item := range set {
		out = append(out, item)
	}
	return out
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	set := map[string]int{}
	for _, item := range a {
		set[item]++
	}
	for _, item := range b {
		set[item]--
	}
	for _, count := range set {
		if count != 0 {
			return false
		}
	}
	return true
}

func evidenceIDSet(evidence []EvidenceV2) map[string]bool {
	set := map[string]bool{}
	for _, ev := range evidence {
		set[ev.ID] = true
	}
	return set
}

func allRefsExist(refs []string, evidence map[string]bool) bool {
	for _, ref := range refs {
		if !evidence[ref] {
			return false
		}
	}
	return true
}

func looksLikeTestClaim(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "test") && (strings.Contains(lower, "pass") || strings.Contains(lower, "green") || strings.Contains(lower, "geç"))
}

func hasTestEvidence(evidence []EvidenceV2) bool {
	for _, ev := range evidence {
		if ev.Type == EvidenceTypeTestResult && commandEvidencePassed(ev) {
			return true
		}
		if ev.Type == EvidenceTypeCommandResult && commandEvidenceLooksLikeTest(ev) && commandEvidencePassed(ev) {
			return true
		}
	}
	return false
}

func commandEvidenceLooksLikeTest(ev EvidenceV2) bool {
	if command, ok := ev.Data["command"].(string); ok && strings.Contains(strings.ToLower(command), "test") {
		return true
	}
	return strings.Contains(strings.ToLower(ev.Summary), "test")
}

func commandEvidencePassed(ev EvidenceV2) bool {
	if exit, ok := ev.Data["exit_code"].(float64); ok {
		return int(exit) == 0
	}
	if exit, ok := ev.Data["exit_code"].(int); ok {
		return exit == 0
	}
	return strings.Contains(strings.ToLower(ev.Summary), "pass")
}

func recommendedAction(checks []VerifyCheck) string {
	for _, check := range checks {
		if check.Status == VerifyFail && check.SafeNextAction != "" {
			return check.SafeNextAction
		}
	}
	for _, check := range checks {
		if check.Status == VerifyWarn && check.SafeNextAction != "" {
			return check.SafeNextAction
		}
	}
	return "Context looks usable. Continue from the recorded next step."
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "less than a minute"
	}
	if d < time.Hour {
		return fmt.Sprintf("%d minute(s)", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%d hour(s)", int(d.Hours()))
	}
	return fmt.Sprintf("%d day(s)", int(d.Hours()/24))
}

func emptyFallback(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func LoadPlanStatus(projectRoot, name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("plan name is empty")
	}
	if fileExists(PlanJSONPath(projectRoot, name)) {
		plan, err := ReadPlanV2(projectRoot, name)
		if err != nil {
			return "", err
		}
		return plan.Status, nil
	}
	plan, err := ReadPlan(projectRoot, name)
	if err != nil {
		return "", err
	}
	return plan.Status, nil
}

func SnapshotFromProject(projectRoot string) (*SnapshotV2, error) {
	if fileExists(SnapshotJSONPath(projectRoot)) {
		return ReadSnapshotV2(projectRoot)
	}
	legacy, err := Read(projectRoot)
	if err != nil {
		return nil, err
	}
	return SnapshotToV2(projectRoot, legacy), nil
}
