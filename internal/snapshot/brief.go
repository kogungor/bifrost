package snapshot

import (
	"fmt"
	"strings"
	"time"
)

const (
	BriefModeImplement = "implement"
	BriefModeDebug     = "debug"
	BriefModeReview    = "review"
	BriefModePlan      = "plan"
	BriefModeHandoff   = "handoff"
)

type BriefOptions struct {
	Mode        string
	BudgetChars int
	Full        bool
	Now         time.Time
	Verify      *VerifyResult
	PlanSummary *PlanExecutionSummary
}

type BriefResult struct {
	Mode        string   `json:"mode"`
	BudgetChars int      `json:"budget_chars,omitempty"`
	Full        bool     `json:"full"`
	Rendered    string   `json:"rendered"`
	Omitted     []string `json:"omitted,omitempty"`
}

type briefSection struct {
	Title     string
	Lines     []string
	Priority  int
	Mandatory bool
	OmitLabel string
}

func BuildBrief(snap *SnapshotV2, opts BriefOptions) BriefResult {
	mode := normalizeBriefMode(opts.Mode)
	budget := opts.BudgetChars
	if budget <= 0 {
		budget = 5000
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if snap == nil {
		rendered := "Bifrost Briefing\n\nNo Bifrost snapshot found.\n"
		return BriefResult{Mode: mode, BudgetChars: budget, Full: opts.Full, Rendered: rendered}
	}

	sections := buildBriefSections(snap, mode, now, opts.Verify, opts.PlanSummary)
	rendered, omitted := renderBriefSections(sections, budget, opts.Full)
	return BriefResult{
		Mode:        mode,
		BudgetChars: budget,
		Full:        opts.Full,
		Rendered:    rendered,
		Omitted:     omitted,
	}
}

func normalizeBriefMode(mode string) string {
	switch mode {
	case BriefModeImplement, BriefModeDebug, BriefModeReview, BriefModePlan, BriefModeHandoff:
		return mode
	case "":
		return BriefModeImplement
	default:
		return BriefModeImplement
	}
}

func buildBriefSections(snap *SnapshotV2, mode string, now time.Time, verify *VerifyResult, plan *PlanExecutionSummary) []briefSection {
	var sections []briefSection
	sections = append(sections, briefSection{
		Title:     "Context",
		Lines:     briefHeaderLines(snap, now),
		Priority:  0,
		Mandatory: true,
	})
	if verify != nil {
		sections = append(sections, verificationBriefSections(*verify)...)
	}
	sections = append(sections, criticalRiskQuestionSections(snap)...)
	sections = append(sections, modeBriefSections(snap, mode, plan)...)
	return sections
}

func briefHeaderLines(snap *SnapshotV2, now time.Time) []string {
	lines := []string{
		"Project    " + snap.Project.Name,
		"From       " + snap.Source.Tool,
		"Captured   " + briefCapturedAge(now, snap.CapturedAt),
	}
	if snap.Observed.Git != nil && snap.Observed.Git.Commit != "" {
		commit := snap.Observed.Git.Commit
		if len(commit) > 8 {
			commit = commit[:8]
		}
		lines = append(lines, "Commit     "+commit)
	}
	if snap.Session.Intent != "" {
		lines = append(lines, "Intent     "+snap.Session.Intent)
	}
	if snap.Session.Pressure != "" {
		lines = append(lines, "Pressure   "+snap.Session.Pressure)
	}
	return lines
}

func verificationBriefSections(result VerifyResult) []briefSection {
	trust := make([]string, 0)
	verifyFirst := make([]string, 0)
	doNotAssume := make([]string, 0)
	for _, check := range result.Checks {
		switch check.Status {
		case VerifyPass:
			if len(trust) < 5 {
				trust = append(trust, check.ID+": "+check.Message)
			}
		case VerifyWarn, VerifyFail:
			line := check.ID + ": " + check.Message
			if check.SafeNextAction != "" {
				line += " Next: " + check.SafeNextAction
			}
			verifyFirst = append(verifyFirst, line)
			if assumption := doNotAssumeForCheck(check.ID); assumption != "" {
				doNotAssume = append(doNotAssume, assumption)
			}
		}
	}
	sections := []briefSection{{
		Title:     "Verification summary",
		Lines:     []string{"Status     " + result.Status, "Safe next  " + emptyFallback(result.RecommendedNextAction, "Continue from the recorded next step.")},
		Priority:  1,
		Mandatory: true,
	}}
	if len(trust) > 0 {
		sections = append(sections, briefSection{Title: "Trust this", Lines: trust, Priority: 8, OmitLabel: fmt.Sprintf("%d pass verification checks", len(trust))})
	}
	if len(verifyFirst) > 0 {
		sections = append(sections, briefSection{Title: "Verify this first", Lines: verifyFirst, Priority: 1, Mandatory: true})
	}
	if len(doNotAssume) > 0 {
		sections = append(sections, briefSection{Title: "Do not assume", Lines: uniqueStrings(doNotAssume), Priority: 1, Mandatory: true})
	}
	return sections
}

func doNotAssumeForCheck(id string) string {
	switch id {
	case "commands.test_claims":
		return "Do not assume tests pass."
	case "files.active_changed":
		return "Do not assume active files are unchanged."
	case "git.branch_match", "git.commit_match":
		return "Do not assume snapshot git context is current."
	case "questions.unresolved_high":
		return "Do not assume high-severity questions are resolved."
	case "risks.unresolved_high":
		return "Do not assume high-severity risks are resolved."
	case "claims.evidence":
		return "Do not assume model claims are evidence-backed."
	default:
		return ""
	}
}

func criticalRiskQuestionSections(snap *SnapshotV2) []briefSection {
	var sections []briefSection
	questions := make([]string, 0)
	for _, q := range snap.Interpretation.OpenQuestions {
		if q.Text != "" && q.Severity == "high" {
			questions = append(questions, formatSeverityLine(q.Severity, q.Text))
		}
	}
	if len(questions) > 0 {
		sections = append(sections, briefSection{Title: "High severity open questions", Lines: questions, Priority: 1, Mandatory: true})
	}
	risks := make([]string, 0)
	for _, r := range snap.Interpretation.Risks {
		if r.Text != "" && r.Severity == "high" {
			line := formatSeverityLine(r.Severity, r.Text)
			if r.Mitigation != "" {
				line += " Mitigation: " + r.Mitigation
			}
			risks = append(risks, line)
		}
	}
	if len(risks) > 0 {
		sections = append(sections, briefSection{Title: "High severity risks", Lines: risks, Priority: 1, Mandatory: true})
	}
	return sections
}

func modeBriefSections(snap *SnapshotV2, mode string, plan *PlanExecutionSummary) []briefSection {
	switch mode {
	case BriefModeDebug:
		return debugBriefSections(snap, plan)
	case BriefModeReview:
		return reviewBriefSections(snap, plan)
	case BriefModePlan:
		return planBriefSections(snap, plan)
	case BriefModeHandoff:
		return handoffBriefSections(snap, plan)
	default:
		return implementBriefSections(snap, plan)
	}
}

func implementBriefSections(snap *SnapshotV2, plan *PlanExecutionSummary) []briefSection {
	sections := []briefSection{
		{Title: "Task", Lines: singleLine(snap.Session.Task), Priority: 2, Mandatory: true},
		{Title: "Next step", Lines: singleLine(snap.Session.NextStep), Priority: 2, Mandatory: true},
		{Title: "Active files", Lines: activeFileLines(snap.ActiveFiles, true), Priority: 3, OmitLabel: fmt.Sprintf("%d active files", len(snap.ActiveFiles))},
		{Title: "Failing commands", Lines: commandLines(snap.Observed.Commands, false), Priority: 4, OmitLabel: "failing command history"},
		{Title: "Status", Lines: statusLines(snap.Interpretation.StatusItems), Priority: 5, OmitLabel: fmt.Sprintf("%d status items", len(snap.Interpretation.StatusItems))},
	}
	return appendPlanSection(sections, plan)
}

func debugBriefSections(snap *SnapshotV2, plan *PlanExecutionSummary) []briefSection {
	sections := []briefSection{
		{Title: "Task", Lines: singleLine(snap.Session.Task), Priority: 2, Mandatory: true},
		{Title: "Failing commands", Lines: commandLines(snap.Observed.Commands, false), Priority: 2, Mandatory: true},
		{Title: "Recent changes", Lines: gitChangeLines(snap.Observed.Git), Priority: 3, OmitLabel: "recent git changes"},
		{Title: "Active files", Lines: activeFileLines(snap.ActiveFiles, false), Priority: 4, OmitLabel: fmt.Sprintf("%d active files", len(snap.ActiveFiles))},
		{Title: "Assumptions", Lines: assumptionLines(snap.Interpretation.Assumptions), Priority: 5, OmitLabel: fmt.Sprintf("%d assumptions", len(snap.Interpretation.Assumptions))},
	}
	return appendPlanSection(sections, plan)
}

func reviewBriefSections(snap *SnapshotV2, plan *PlanExecutionSummary) []briefSection {
	sections := []briefSection{
		{Title: "Task", Lines: singleLine(snap.Session.Task), Priority: 2, Mandatory: true},
		{Title: "Risks", Lines: nonHighRiskLines(snap.Interpretation.Risks), Priority: 2, Mandatory: true},
		{Title: "Open questions", Lines: nonHighQuestionLines(snap.Interpretation.OpenQuestions), Priority: 2, Mandatory: true},
		{Title: "Key decisions", Lines: decisionLines(snap.Interpretation.Decisions), Priority: 3, OmitLabel: fmt.Sprintf("%d decisions", len(snap.Interpretation.Decisions))},
		{Title: "Low trust files", Lines: lowTrustFileLines(snap.ActiveFiles), Priority: 3, OmitLabel: "low trust active files"},
	}
	return appendPlanSection(sections, plan)
}

func planBriefSections(snap *SnapshotV2, plan *PlanExecutionSummary) []briefSection {
	sections := []briefSection{
		{Title: "Goal", Lines: singleLine(snap.Session.Task), Priority: 2, Mandatory: true},
		{Title: "Next step", Lines: singleLine(snap.Session.NextStep), Priority: 2, Mandatory: true},
		{Title: "Open questions", Lines: nonHighQuestionLines(snap.Interpretation.OpenQuestions), Priority: 2, Mandatory: true},
		{Title: "Risks", Lines: nonHighRiskLines(snap.Interpretation.Risks), Priority: 3, OmitLabel: fmt.Sprintf("%d risks", len(snap.Interpretation.Risks))},
		{Title: "Key decisions", Lines: decisionLines(snap.Interpretation.Decisions), Priority: 4, OmitLabel: fmt.Sprintf("%d decisions", len(snap.Interpretation.Decisions))},
	}
	return appendPlanSection(sections, plan)
}

func handoffBriefSections(snap *SnapshotV2, plan *PlanExecutionSummary) []briefSection {
	sections := []briefSection{
		{Title: "Task", Lines: singleLine(snap.Session.Task), Priority: 2, Mandatory: true},
		{Title: "Next step", Lines: singleLine(snap.Session.NextStep), Priority: 2, Mandatory: true},
		{Title: "Active files", Lines: activeFileLines(snap.ActiveFiles, false), Priority: 3, OmitLabel: fmt.Sprintf("%d active files", len(snap.ActiveFiles))},
	}
	return appendPlanSection(sections, plan)
}

func appendPlanSection(sections []briefSection, plan *PlanExecutionSummary) []briefSection {
	if plan == nil {
		return sections
	}
	lines := []string{
		fmt.Sprintf("Health      %d/100", plan.Health),
		fmt.Sprintf("Steps       %d total, %d verified, %d claimed, %d blocked", plan.Total, plan.VerifiedDone, plan.ClaimedDone, plan.Blocked),
		"Safe next   " + plan.NextAction,
	}
	if plan.NextStep != nil {
		lines = append(lines, "Next step   "+plan.NextStep.ID+" - "+plan.NextStep.Title)
	}
	return append(sections, briefSection{Title: "Active plan", Lines: lines, Priority: 3, OmitLabel: "active plan summary"})
}

func renderBriefSections(sections []briefSection, budget int, full bool) (string, []string) {
	included := make([]bool, len(sections))
	for i, section := range sections {
		if len(section.Lines) > 0 || section.Mandatory {
			included[i] = true
		}
	}
	if !full {
		for renderedBriefLength(sections, included, omittedLabels(sections, included)) > budget {
			drop := -1
			for i, section := range sections {
				if !included[i] || section.Mandatory {
					continue
				}
				if drop == -1 || section.Priority > sections[drop].Priority {
					drop = i
				}
			}
			if drop == -1 {
				break
			}
			included[drop] = false
		}
	}
	omitted := omittedLabels(sections, included)
	return renderIncludedBrief(sections, included, omitted), omitted
}

func renderedBriefLength(sections []briefSection, included []bool, omitted []string) int {
	return len(renderIncludedBrief(sections, included, omitted))
}

func renderIncludedBrief(sections []briefSection, included []bool, omitted []string) string {
	var b strings.Builder
	b.WriteString("Bifrost Briefing\n")
	for i, section := range sections {
		if !included[i] || len(section.Lines) == 0 {
			continue
		}
		b.WriteString("\n")
		b.WriteString(section.Title)
		b.WriteString("\n")
		for _, line := range section.Lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			b.WriteString("- ")
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	if len(omitted) > 0 {
		b.WriteString("\nOmitted due to budget\n")
		for _, item := range omitted {
			b.WriteString("- ")
			b.WriteString(item)
			b.WriteString("\n")
		}
		b.WriteString("- Run `bifrost brief --full` to inspect.\n")
	}
	return b.String()
}

func omittedLabels(sections []briefSection, included []bool) []string {
	var omitted []string
	for i, section := range sections {
		if included[i] || section.Mandatory || len(section.Lines) == 0 {
			continue
		}
		label := section.OmitLabel
		if label == "" {
			label = strings.ToLower(section.Title)
		}
		omitted = append(omitted, label)
	}
	return omitted
}

func singleLine(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return []string{value}
}

func statusLines(items []StatusItemV2) []string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		if item.Text == "" {
			continue
		}
		lines = append(lines, item.State+": "+item.Text)
	}
	return lines
}

func activeFileLines(files []ActiveFileV2, includeTrust bool) []string {
	lines := make([]string, 0, len(files))
	for _, file := range files {
		if file.Path == "" {
			continue
		}
		line := file.Path
		if file.Note != "" {
			line += " - " + file.Note
		}
		if includeTrust {
			if trust := briefTrustSummary(file.Trust); trust != "" {
				line += " [" + trust + "]"
			}
		}
		lines = append(lines, line)
	}
	return lines
}

func lowTrustFileLines(files []ActiveFileV2) []string {
	var lines []string
	for _, file := range files {
		trust := briefTrustSummary(file.Trust)
		if strings.Contains(trust, "low") || strings.Contains(trust, "stale") || strings.Contains(trust, "weak") {
			line := file.Path
			if trust != "" {
				line += " [" + trust + "]"
			}
			lines = append(lines, line)
		}
	}
	return lines
}

func briefTrustSummary(trust TrustV2) string {
	var parts []string
	appendTrust := func(name, value string) {
		if value != "" {
			parts = append(parts, name+"="+value)
		}
	}
	appendTrust("implementation", trust.Implementation)
	appendTrust("tests", trust.Tests)
	appendTrust("security", trust.Security)
	appendTrust("architecture", trust.Architecture)
	appendTrust("freshness", trust.Freshness)
	appendTrust("evidence", trust.Evidence)
	return strings.Join(parts, ", ")
}

func commandLines(commands []CommandObservedV2, includePassing bool) []string {
	var lines []string
	for _, cmd := range commands {
		if cmd.Command == "" {
			continue
		}
		if !includePassing && cmd.ExitCode == 0 {
			continue
		}
		line := fmt.Sprintf("exit %d: %s", cmd.ExitCode, cmd.Command)
		if cmd.Summary != "" {
			line += " - " + cmd.Summary
		}
		lines = append(lines, line)
	}
	return lines
}

func gitChangeLines(git *GitObservedV2) []string {
	if git == nil {
		return nil
	}
	var lines []string
	if git.Branch != "" {
		lines = append(lines, "branch: "+git.Branch)
	}
	if git.Commit != "" {
		lines = append(lines, "commit: "+git.Commit)
	}
	for _, file := range git.ChangedFiles {
		lines = append(lines, "changed: "+file)
	}
	for _, file := range git.StagedFiles {
		lines = append(lines, "staged: "+file)
	}
	for _, file := range git.UntrackedFiles {
		lines = append(lines, "untracked: "+file)
	}
	return lines
}

func decisionLines(decisions []DecisionV2) []string {
	lines := make([]string, 0, len(decisions))
	for _, decision := range decisions {
		if decision.Text != "" {
			lines = append(lines, decision.Text)
		}
	}
	return lines
}

func assumptionLines(assumptions []AssumptionV2) []string {
	lines := make([]string, 0, len(assumptions))
	for _, assumption := range assumptions {
		if assumption.Text == "" {
			continue
		}
		line := assumption.Text
		if assumption.VerificationState != "" {
			line += " [" + assumption.VerificationState + "]"
		}
		lines = append(lines, line)
	}
	return lines
}

func questionLines(questions []OpenQuestionV2, highOnly bool) []string {
	lines := make([]string, 0, len(questions))
	for _, question := range questions {
		if question.Text == "" || (highOnly && question.Severity != "high") {
			continue
		}
		lines = append(lines, formatSeverityLine(question.Severity, question.Text))
	}
	return lines
}

func nonHighQuestionLines(questions []OpenQuestionV2) []string {
	lines := make([]string, 0, len(questions))
	for _, question := range questions {
		if question.Text == "" || question.Severity == "high" {
			continue
		}
		lines = append(lines, formatSeverityLine(question.Severity, question.Text))
	}
	return lines
}

func riskLines(risks []RiskV2, highOnly bool) []string {
	lines := make([]string, 0, len(risks))
	for _, risk := range risks {
		if risk.Text == "" || (highOnly && risk.Severity != "high") {
			continue
		}
		line := formatSeverityLine(risk.Severity, risk.Text)
		if risk.Mitigation != "" {
			line += " Mitigation: " + risk.Mitigation
		}
		lines = append(lines, line)
	}
	return lines
}

func nonHighRiskLines(risks []RiskV2) []string {
	lines := make([]string, 0, len(risks))
	for _, risk := range risks {
		if risk.Text == "" || risk.Severity == "high" {
			continue
		}
		line := formatSeverityLine(risk.Severity, risk.Text)
		if risk.Mitigation != "" {
			line += " Mitigation: " + risk.Mitigation
		}
		lines = append(lines, line)
	}
	return lines
}

func formatSeverityLine(severity, text string) string {
	if severity == "" {
		return text
	}
	return severity + ": " + text
}

func briefAge(now, capturedAt time.Time) string {
	if capturedAt.IsZero() {
		return "unknown"
	}
	d := now.Sub(capturedAt)
	if d < 0 {
		return "in the future"
	}
	if d < time.Minute {
		return "less than a minute"
	}
	if d < time.Hour {
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%d hours", int(d.Hours()))
	}
	return fmt.Sprintf("%d days", int(d.Hours()/24))
}

func briefCapturedAge(now, capturedAt time.Time) string {
	age := briefAge(now, capturedAt)
	switch age {
	case "unknown", "in the future":
		return age
	default:
		return age + " ago"
	}
}

func uniqueStrings(items []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}
