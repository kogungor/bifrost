package snapshot

import (
	"fmt"
	"hash/crc32"
	"path/filepath"
	"strings"
)

// SnapshotToV2 converts the legacy Markdown compatibility model into
// SnapshotV2. It does not collect new observed evidence; collectors are added
// in later Integrity Pack phases.
func SnapshotToV2(projectRoot string, s *Snapshot) *SnapshotV2 {
	if s == nil {
		return nil
	}
	projectName := s.Project
	if projectName == "" && projectRoot != "" {
		projectName = filepath.Base(projectRoot)
	}
	v2 := &SnapshotV2{
		SchemaVersion: SnapshotSchemaV2,
		ID:            snapshotID(s),
		Project:       ProjectRefV2{Name: projectName, Root: projectRoot},
		CapturedAt:    s.Timestamp,
		Source: SourceV2{
			Tool:           s.SourceTool,
			BifrostVersion: fmt.Sprintf("%d", s.BifrostVersion),
		},
		Session: SessionStateV2{
			Intent:       s.SessionIntent,
			Pressure:     s.TokenPressure,
			Task:         s.CurrentTask,
			NextStep:     s.NextStep,
			SessionStart: s.SessionStart,
		},
		Interpretation: InterpretationV2{
			StatusItems:      statusItemsToV2(s.Status),
			Decisions:        decisionsToV2(s.Decisions),
			EnvironmentNotes: legacyListTexts(s.EnvNotes),
			Assumptions:      assumptionsToV2(s.Assumptions),
			OpenQuestions:    openQuestionsToV2(s.OpenQuestions),
			Risks:            risksToV2(s.Risks),
		},
		ActiveFiles: activeFilesToV2(s.ActiveFiles),
		Integrity:   SnapshotIntegrityV2{VerifyStatus: "not_run"},
	}
	if s.GitSHA != "" {
		v2.Observed.Git = &GitObservedV2{Commit: s.GitSHA}
	}
	if s.ActivePlanName != "" {
		v2.ActivePlan = &ActivePlanRefV2{Name: s.ActivePlanName, Version: "v2"}
	}
	return v2
}

// SnapshotFromV2 converts SnapshotV2 into the existing Markdown compatibility
// model so the current renderer can preserve session.md behavior.
func SnapshotFromV2(s *SnapshotV2) *Snapshot {
	if s == nil {
		return nil
	}
	gitSHA := ""
	if s.Observed.Git != nil {
		gitSHA = s.Observed.Git.Commit
	}
	activePlanName := ""
	if s.ActivePlan != nil {
		activePlanName = s.ActivePlan.Name
	}
	return &Snapshot{
		BifrostVersion: CurrentVersion,
		Timestamp:      s.CapturedAt,
		SourceTool:     s.Source.Tool,
		Project:        s.Project.Name,
		TokenPressure:  s.Session.Pressure,
		SessionIntent:  s.Session.Intent,
		ActivePlanName: activePlanName,
		GitSHA:         gitSHA,
		SessionStart:   s.Session.SessionStart,
		CurrentTask:    s.Session.Task,
		Status:         statusItemsFromV2(s.Interpretation.StatusItems),
		ActiveFiles:    activeFilesFromV2(s.ActiveFiles),
		Decisions:      decisionsFromV2(s.Interpretation.Decisions),
		EnvNotes:       legacyListItems(s.Interpretation.EnvironmentNotes),
		NextStep:       s.Session.NextStep,
		Assumptions:    assumptionsFromV2(s.Interpretation.Assumptions),
		OpenQuestions:  openQuestionsFromV2(s.Interpretation.OpenQuestions),
		Risks:          risksFromV2(s.Interpretation.Risks),
	}
}

// SnapshotFromV2WithTrustSummary converts SnapshotV2 for human-readable
// rendering where the Markdown should expose multidimensional trust. Use
// SnapshotFromV2 for compatibility APIs that still expose legacy confidence.
func SnapshotFromV2WithTrustSummary(s *SnapshotV2) *Snapshot {
	out := SnapshotFromV2(s)
	if out == nil {
		return nil
	}
	out.ActiveFiles = activeFilesFromV2TrustSummary(s.ActiveFiles)
	return out
}

// PlanToV2 converts a legacy Markdown plan into PlanV2.
func PlanToV2(p *Plan, name string) *PlanV2 {
	if p == nil {
		return nil
	}
	version := p.PlanVersion
	if version <= 0 {
		version = 1
	}
	return &PlanV2{
		SchemaVersion: PlanSchemaV2,
		Name:          name,
		Title:         p.Title,
		Goal:          p.Goal,
		Status:        p.Status,
		Version:       fmt.Sprintf("v%d", version),
		Project:       ProjectRefV2{Name: p.Project},
		Source:        SourceV2{Tool: p.SourceTool, BifrostVersion: fmt.Sprintf("%d", p.BifrostVersion)},
		CreatedAt:     p.CreatedAt,
		UpdatedAt:     p.UpdatedAt,
		Steps:         planStepsToV2(p.Steps),
		Consensus:     planConsensusToV2(p),
		Review:        planReviewToV2(p.ReviewNotes),
	}
}

// PlanFromV2 converts PlanV2 into the existing Markdown compatibility model.
func PlanFromV2(p *PlanV2) *Plan {
	if p == nil {
		return nil
	}
	return &Plan{
		BifrostVersion:   CurrentVersion,
		CreatedAt:        p.CreatedAt,
		UpdatedAt:        p.UpdatedAt,
		SourceTool:       planSourceToolFromV2(p),
		Project:          planProjectFromV2(p),
		Status:           p.Status,
		PlanVersion:      planVersionFromV2(p),
		ProposedBy:       p.Consensus.ProposedBy,
		MaxRevisions:     maxRevisionsFromV2(p),
		RevisionCount:    p.Consensus.RevisionCount,
		ConsensusState:   p.Consensus.ConsensusState,
		ActivationReason: p.Consensus.ActivationReason,
		DeadlockDetected: p.Consensus.DeadlockDetected,
		DeadlockReason:   p.Consensus.DeadlockReason,
		Title:            p.Title,
		Goal:             p.Goal,
		Steps:            planStepsFromV2(p.Steps),
		ReviewNotes:      planReviewFromV2(p.Review),
	}
}

func planSourceToolFromV2(p *PlanV2) string {
	if p.Source.Tool != "" {
		return p.Source.Tool
	}
	return "bifrost"
}

func planProjectFromV2(p *PlanV2) string {
	if p.Project.Name != "" {
		return p.Project.Name
	}
	return p.Name
}

func planVersionFromV2(p *PlanV2) int {
	if p.Consensus.PlanVersion > 0 {
		return p.Consensus.PlanVersion
	}
	var version int
	if _, err := fmt.Sscanf(strings.TrimPrefix(p.Version, "v"), "%d", &version); err == nil && version > 0 {
		return version
	}
	return 1
}

func maxRevisionsFromV2(p *PlanV2) int {
	if p.Consensus.MaxRevisions > 0 {
		return p.Consensus.MaxRevisions
	}
	return 3
}

func snapshotID(s *Snapshot) string {
	if s.Timestamp.IsZero() {
		return stableID("snap", s.Project+"|"+s.CurrentTask)
	}
	return stableID("snap", s.Timestamp.UTC().Format("20060102_150405")+"|"+s.Project+"|"+s.CurrentTask)
}

func stableID(prefix, text string) string {
	return fmt.Sprintf("%s_%08x", prefix, crc32.ChecksumIEEE([]byte(text)))
}

func statusItemsToV2(items []string) []StatusItemV2 {
	out := make([]StatusItemV2, 0, len(items))
	for _, item := range items {
		text, state := parseChecklistState(item)
		if text == "" {
			continue
		}
		out = append(out, StatusItemV2{
			ID:    stableID("status", text),
			Text:  text,
			State: state,
			Verification: &VerificationV2{
				State:  "unverified",
				Reason: "Migrated from legacy Markdown status",
			},
		})
	}
	return out
}

func statusItemsFromV2(items []StatusItemV2) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item.Text == "" {
			continue
		}
		out = append(out, renderStatusItemV2(item))
	}
	return out
}

func parseChecklistState(item string) (string, string) {
	text := strings.TrimSpace(strings.TrimPrefix(item, "- "))
	switch {
	case strings.HasPrefix(text, "[x?] "):
		return stripStatusAnnotation(strings.TrimSpace(strings.TrimPrefix(text, "[x?] "))), "claimed_done"
	case strings.HasPrefix(text, "[x] "):
		doneText := strings.TrimSpace(strings.TrimPrefix(text, "[x] "))
		if before, _, ok := strings.Cut(doneText, " — verified by "); ok {
			return strings.TrimSpace(before), "verified_done"
		}
		return stripStatusAnnotation(doneText), "claimed_done"
	case strings.HasPrefix(text, "[-] "):
		return strings.TrimSpace(strings.TrimPrefix(text, "[-] ")), "in_progress"
	case strings.HasPrefix(text, "[!] "):
		return stripStatusAnnotation(strings.TrimSpace(strings.TrimPrefix(text, "[!] "))), "blocked"
	case strings.HasPrefix(text, "[ ] "):
		return strings.TrimSpace(strings.TrimPrefix(text, "[ ] ")), "not_started"
	default:
		return text, "in_progress"
	}
}

func stripStatusAnnotation(text string) string {
	for _, marker := range []string{
		" — claimed done, not verified",
		" — verification failed: ",
		" — verification failed",
		" — verification warning: ",
		" — verification warning",
		" — blocked by ",
		" — invalidated by ",
	} {
		if before, _, ok := strings.Cut(text, marker); ok {
			return strings.TrimSpace(before)
		}
	}
	return strings.TrimSpace(text)
}

func renderStatusItemV2(item StatusItemV2) string {
	text := item.Text
	switch item.State {
	case "verified_done":
		if item.Verification != nil && item.Verification.Reason != "" {
			text += " — verified by " + item.Verification.Reason
		}
		return "- [x] " + text
	case "claimed_done":
		switch {
		case item.Verification == nil || item.Verification.State == "unverified" || item.Verification.State == "not_run":
			text += " — claimed done, not verified"
		case item.Verification.State == "fail" || item.Verification.State == "failed":
			if item.Verification.Reason != "" {
				text += " — verification failed: " + item.Verification.Reason
			} else {
				text += " — verification failed"
			}
		case item.Verification.State == "warn":
			if item.Verification.Reason != "" {
				text += " — verification warning: " + item.Verification.Reason
			} else {
				text += " — verification warning"
			}
		}
		return "- [x?] " + text
	case "blocked":
		if item.Verification != nil && item.Verification.Reason != "" {
			text += " — blocked by " + item.Verification.Reason
		}
		return "- [!] " + text
	case "invalidated":
		if item.Verification != nil && item.Verification.Reason != "" {
			text += " — invalidated by " + item.Verification.Reason
		}
		return "- [!] " + text
	case "not_started":
		return "- [ ] " + text
	default:
		return "- [-] " + text
	}
}

func activeFilesToV2(files []ActiveFile) []ActiveFileV2 {
	out := make([]ActiveFileV2, 0, len(files))
	for _, file := range files {
		trust := TrustV2{}
		if file.Confidence != "" {
			if strings.Contains(file.Confidence, "=") {
				trust = TrustFromSummary(file.Confidence)
			} else {
				trust = TrustFromLegacyConfidence(file.Confidence)
			}
		}
		out = append(out, ActiveFileV2{Path: file.Path, Note: file.Note, Trust: trust})
	}
	return out
}

func activeFilesFromV2(files []ActiveFileV2) []ActiveFile {
	out := make([]ActiveFile, 0, len(files))
	for _, file := range files {
		out = append(out, ActiveFile{Path: file.Path, Note: file.Note, Confidence: legacyConfidence(file.Trust)})
	}
	return out
}

func legacyConfidence(trust TrustV2) string {
	trust = normalizeTrust(trust)
	if trust.Implementation != "" && trust.Implementation != TrustUnknown {
		return trust.Implementation
	}
	return ""
}

func activeFilesFromV2TrustSummary(files []ActiveFileV2) []ActiveFile {
	out := make([]ActiveFile, 0, len(files))
	for _, file := range files {
		out = append(out, ActiveFile{Path: file.Path, Note: file.Note, Confidence: trustSummaryConfidence(file.Trust)})
	}
	return out
}

func trustSummaryConfidence(trust TrustV2) string {
	trust = normalizeTrust(trust)
	if trust.Tests != TrustUnknown || trust.Security != TrustUnknown || trust.Architecture != TrustUnknown || trust.Freshness != TrustUnknown || trust.Evidence != EvidenceWeak {
		return TrustSummary(trust)
	}
	if trust.Implementation != "" && trust.Implementation != TrustUnknown {
		return trust.Implementation
	}
	return ""
}

func decisionsToV2(items []string) []DecisionV2 {
	out := make([]DecisionV2, 0, len(items))
	for _, item := range items {
		text := legacyListText(item)
		if text != "" {
			out = append(out, DecisionV2{ID: stableID("dec", text), Text: text, Scope: "session"})
		}
	}
	return out
}

func decisionsFromV2(items []DecisionV2) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item.Text != "" {
			out = append(out, "- "+item.Text)
		}
	}
	return out
}

func assumptionsToV2(items []string) []AssumptionV2 {
	out := make([]AssumptionV2, 0, len(items))
	for _, item := range items {
		text := legacyListText(item)
		if text != "" {
			out = append(out, AssumptionV2{ID: stableID("asm", text), Text: text, VerificationState: "unverified"})
		}
	}
	return out
}

func assumptionsFromV2(items []AssumptionV2) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item.Text != "" {
			out = append(out, "- "+item.Text)
		}
	}
	return out
}

func openQuestionsToV2(items []string) []OpenQuestionV2 {
	out := make([]OpenQuestionV2, 0, len(items))
	for _, item := range items {
		text := legacyListText(item)
		if text != "" {
			out = append(out, OpenQuestionV2{ID: stableID("q", text), Text: text})
		}
	}
	return out
}

func openQuestionsFromV2(items []OpenQuestionV2) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item.Text != "" {
			out = append(out, "- "+item.Text)
		}
	}
	return out
}

func risksToV2(items []string) []RiskV2 {
	out := make([]RiskV2, 0, len(items))
	for _, item := range items {
		text := legacyListText(item)
		if text != "" {
			out = append(out, RiskV2{ID: stableID("risk", text), Text: text})
		}
	}
	return out
}

func risksFromV2(items []RiskV2) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item.Text != "" {
			out = append(out, "- "+item.Text)
		}
	}
	return out
}

func legacyListText(item string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(item), "- "))
}

func legacyListTexts(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		text := legacyListText(item)
		if text != "" {
			out = append(out, text)
		}
	}
	return out
}

func legacyListItems(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		text := legacyListText(item)
		if text != "" {
			out = append(out, "- "+text)
		}
	}
	return out
}

func planStepsToV2(steps []PlanStep) []PlanStepV2 {
	out := make([]PlanStepV2, 0, len(steps))
	for _, step := range steps {
		id := step.ID
		if id == "" {
			id = stableID("step", step.Description)
		}
		out = append(out, PlanStepV2{
			ID:            id,
			Title:         step.Description,
			Status:        planStepStatusToV2(step.Status),
			ExpectedFiles: step.Files,
		})
	}
	return out
}

func planStepsFromV2(steps []PlanStepV2) []PlanStep {
	out := make([]PlanStep, 0, len(steps))
	for _, step := range steps {
		out = append(out, PlanStep{
			ID:          step.ID,
			Description: step.Title,
			Status:      planStepStatusFromV2(step.Status),
			Files:       step.ExpectedFiles,
		})
	}
	return out
}

func planStepStatusToV2(status string) string {
	switch status {
	case "done":
		return "claimed_done"
	case "blocked":
		return "blocked"
	default:
		return "not_started"
	}
}

func planStepStatusFromV2(status string) string {
	switch status {
	case "claimed_done", "verified_done":
		return "done"
	case "blocked", "invalidated":
		return "blocked"
	default:
		return "pending"
	}
}

func planReviewToV2(notes []ReviewNote) PlanReviewV2 {
	if len(notes) == 0 {
		return PlanReviewV2{}
	}
	latest := notes[len(notes)-1]
	out := PlanReviewV2{Outcome: latest.Outcome}
	for _, note := range notes {
		if note.Text != "" {
			out.Notes = append(out.Notes, note.Text)
			out.Details = append(out.Details, ReviewNoteV2{
				From:        note.From,
				At:          note.At,
				PlanVersion: note.PlanVersion,
				Outcome:     note.Outcome,
				Text:        note.Text,
			})
		}
	}
	return out
}

func planReviewFromV2(review PlanReviewV2) []ReviewNote {
	if len(review.Details) > 0 {
		out := make([]ReviewNote, 0, len(review.Details))
		for _, note := range review.Details {
			out = append(out, ReviewNote{
				From:        fallbackString(note.From, "bifrost"),
				At:          note.At,
				PlanVersion: note.PlanVersion,
				Outcome:     note.Outcome,
				Text:        note.Text,
			})
		}
		return out
	}
	out := make([]ReviewNote, 0, len(review.Notes))
	for _, note := range review.Notes {
		out = append(out, ReviewNote{From: "bifrost", Text: note})
	}
	return out
}

func planConsensusToV2(p *Plan) ConsensusV2 {
	return ConsensusV2{
		PlanVersion:      p.PlanVersion,
		ProposedBy:       p.ProposedBy,
		MaxRevisions:     p.MaxRevisions,
		RevisionCount:    p.RevisionCount,
		ConsensusState:   p.ConsensusState,
		ActivationReason: p.ActivationReason,
		DeadlockDetected: p.DeadlockDetected,
		DeadlockReason:   p.DeadlockReason,
	}
}

func fallbackString(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
