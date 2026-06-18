package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const maxHashFileSize = 10 << 20

// EnrichOptions controls optional evidence supplied by callers. Bifrost records
// these results but does not execute the reported commands.
type EnrichOptions struct {
	ReportedCommands []ReportedCommand
	ManualEvidence   []ManualEvidence
}

// EnrichSnapshotV2 adds deterministic observed facts and evidence records to a
// snapshot without running tests, builds, or other project commands.
func EnrichSnapshotV2(projectRoot string, snap *SnapshotV2) error {
	return EnrichSnapshotV2WithOptions(projectRoot, snap, EnrichOptions{})
}

// EnrichSnapshotV2WithOptions enriches a snapshot with deterministic collectors
// and optional caller-reported evidence.
func EnrichSnapshotV2WithOptions(projectRoot string, snap *SnapshotV2, opts EnrichOptions) error {
	if snap == nil {
		return ValidateSnapshotV2(nil)
	}
	now := time.Now().UTC()
	if snap.Project.Root == "" {
		snap.Project.Root = projectRoot
	}
	if snap.Project.Name == "" && projectRoot != "" {
		snap.Project.Name = filepath.Base(projectRoot)
	}
	if snap.Integrity.VerifyStatus == "" {
		snap.Integrity.VerifyStatus = "not_run"
	}

	var evidence []EvidenceV2
	if git, ev, err := collectGitEvidence(projectRoot, now); err == nil && git != nil {
		snap.Observed.Git = git
		evidence = append(evidence, ev)
		if diffEvidence, ok := collectDiffSummaryEvidence(projectRoot, now); ok {
			evidence = append(evidence, diffEvidence)
		}
	} else if err != nil && !errors.Is(err, errNotGitRepo) {
		return err
	}

	files, fileEvidence, refs, err := collectFileEvidence(projectRoot, snap.ActiveFiles, now)
	if err != nil {
		return err
	}
	snap.Observed.Files = files
	evidence = append(evidence, fileEvidence...)
	attachActiveFileEvidence(snap.ActiveFiles, refs)

	project, projectEvidence := collectProjectEvidence(projectRoot, now)
	snap.Observed.Project = &project
	evidence = append(evidence, projectEvidence)

	commands, commandEvidence := reportedCommandEvidence(opts.ReportedCommands, now)
	snap.Observed.Commands = mergeCommands(snap.Observed.Commands, commands)
	evidence = append(evidence, commandEvidence...)
	for _, note := range opts.ManualEvidence {
		if strings.TrimSpace(note.Text) != "" {
			evidence = append(evidence, NewManualEvidence(note, now))
		}
	}

	modelClaimEvidence := annotateModelClaims(snap, now)
	evidence = append(evidence, modelClaimEvidence...)
	snap.Evidence = mergeEvidence(snap.Evidence, evidence)
	return ValidateSnapshotV2(snap)
}

var errNotGitRepo = errors.New("not a git repository")

func collectGitEvidence(projectRoot string, observedAt time.Time) (*GitObservedV2, EvidenceV2, error) {
	if _, err := runGit(projectRoot, "rev-parse", "--is-inside-work-tree"); err != nil {
		return nil, EvidenceV2{}, errNotGitRepo
	}
	branch, _ := runGit(projectRoot, "rev-parse", "--abbrev-ref", "HEAD")
	commit, _ := runGit(projectRoot, "rev-parse", "--short=12", "HEAD")
	status, _ := runGit(projectRoot, "status", "--porcelain=v1", "--untracked-files=all")
	git := &GitObservedV2{
		Branch: strings.TrimSpace(branch),
		Commit: strings.TrimSpace(commit),
	}
	git.ChangedFiles, git.StagedFiles, git.UntrackedFiles = parseGitPorcelain(status)
	git.Dirty = len(git.ChangedFiles) > 0 || len(git.StagedFiles) > 0 || len(git.UntrackedFiles) > 0
	data := map[string]any{
		"branch":          git.Branch,
		"commit":          git.Commit,
		"dirty":           git.Dirty,
		"changed_files":   git.ChangedFiles,
		"staged_files":    git.StagedFiles,
		"untracked_files": git.UntrackedFiles,
	}
	return git, EvidenceV2{
		ID:         evidenceID("ev_git", data),
		Type:       EvidenceTypeGitStatus,
		Source:     "collector.git",
		ObservedAt: observedAt,
		Summary:    gitEvidenceSummary(git),
		Data:       data,
	}, nil
}

func collectDiffSummaryEvidence(projectRoot string, observedAt time.Time) (EvidenceV2, bool) {
	var lines []string
	if stat, err := runGit(projectRoot, "diff", "--stat", "--no-ext-diff"); err == nil {
		lines = append(lines, limitedNonEmptyLines(stat, 10)...)
	}
	if stat, err := runGit(projectRoot, "diff", "--cached", "--stat", "--no-ext-diff"); err == nil {
		lines = append(lines, limitedNonEmptyLines(stat, 10)...)
	}
	if len(lines) == 0 {
		return EvidenceV2{}, false
	}
	data := map[string]any{
		"stat":  lines,
		"limit": 20,
	}
	return EvidenceV2{
		ID:         evidenceID("ev_diff", data),
		Type:       EvidenceTypeDiffSummary,
		Source:     "collector.git.diff",
		ObservedAt: observedAt,
		Summary:    "Git diff summary with " + pluralCount(len(lines), "line"),
		Data:       data,
	}, true
}

func runGit(projectRoot string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = projectRoot
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func parseGitPorcelain(status string) (changed, staged, untracked []string) {
	changedSet := map[string]bool{}
	stagedSet := map[string]bool{}
	untrackedSet := map[string]bool{}
	for _, line := range strings.Split(status, "\n") {
		if len(line) < 4 {
			continue
		}
		x, y := line[0], line[1]
		path := strings.TrimSpace(line[3:])
		if idx := strings.LastIndex(path, " -> "); idx >= 0 {
			path = strings.TrimSpace(path[idx+4:])
		}
		if path == "" {
			continue
		}
		if x == '?' && y == '?' {
			untrackedSet[path] = true
			continue
		}
		if x != ' ' {
			stagedSet[path] = true
		}
		if y != ' ' || x != ' ' {
			changedSet[path] = true
		}
	}
	return sortedKeys(changedSet), sortedKeys(stagedSet), sortedKeys(untrackedSet)
}

func gitEvidenceSummary(git *GitObservedV2) string {
	state := "clean tree"
	if git.Dirty {
		state = "dirty tree"
	}
	parts := []string{}
	if git.Branch != "" {
		parts = append(parts, "branch "+git.Branch)
	}
	if git.Commit != "" {
		parts = append(parts, "commit "+git.Commit)
	}
	parts = append(parts, state)
	if len(git.ChangedFiles) > 0 {
		parts = append(parts, pluralCount(len(git.ChangedFiles), "changed file"))
	}
	if len(git.UntrackedFiles) > 0 {
		parts = append(parts, pluralCount(len(git.UntrackedFiles), "untracked file"))
	}
	return strings.Join(parts, ", ")
}

func collectFileEvidence(projectRoot string, activeFiles []ActiveFileV2, observedAt time.Time) ([]FileObservedV2, []EvidenceV2, map[string]string, error) {
	files := make([]FileObservedV2, 0, len(activeFiles))
	evidence := make([]EvidenceV2, 0, len(activeFiles))
	refs := map[string]string{}
	for _, active := range activeFiles {
		if active.Path == "" {
			continue
		}
		observed, err := observeFile(projectRoot, active.Path)
		if err != nil {
			return nil, nil, nil, err
		}
		files = append(files, observed)
		data := map[string]any{
			"path":   observed.Path,
			"exists": observed.Exists,
			"sha256": observed.SHA256,
			"mtime":  observed.MTime,
			"size":   observed.Size,
		}
		ev := EvidenceV2{
			ID:         evidenceID("ev_file", data),
			Type:       EvidenceTypeFileMetadata,
			Source:     "collector.file",
			ObservedAt: observedAt,
			Summary:    fileEvidenceSummary(observed),
			Data:       data,
		}
		evidence = append(evidence, ev)
		refs[active.Path] = ev.ID
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, evidence, refs, nil
}

func observeFile(projectRoot, relPath string) (FileObservedV2, error) {
	observed := FileObservedV2{Path: relPath}
	if !isSafeRelativePath(relPath) {
		return observed, nil
	}
	info, err := os.Stat(filepath.Join(projectRoot, filepath.FromSlash(relPath)))
	if err != nil {
		if os.IsNotExist(err) {
			return observed, nil
		}
		return observed, err
	}
	observed.Exists = true
	observed.MTime = info.ModTime().UTC()
	observed.Size = info.Size()
	if info.Mode().IsRegular() && info.Size() <= maxHashFileSize {
		hash, err := fileSHA256(filepath.Join(projectRoot, filepath.FromSlash(relPath)))
		if err != nil {
			return observed, err
		}
		observed.SHA256 = hash
	}
	return observed, nil
}

func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func fileEvidenceSummary(file FileObservedV2) string {
	if !file.Exists {
		return file.Path + " does not exist"
	}
	return file.Path + " exists, " + pluralCount(int(file.Size), "byte")
}

func attachActiveFileEvidence(files []ActiveFileV2, refs map[string]string) {
	for i := range files {
		ref := refs[files[i].Path]
		if ref != "" && !containsString(files[i].EvidenceRefs, ref) {
			files[i].EvidenceRefs = append(files[i].EvidenceRefs, ref)
		}
	}
}

func collectProjectEvidence(projectRoot string, observedAt time.Time) (ProjectObservedV2, EvidenceV2) {
	project := ProjectObservedV2{
		Root:                     projectRoot,
		BifrostMDExists:          fileExists(filepath.Join(projectRoot, "BIFROST.md")),
		PackageManagerCandidates: packageManagerCandidates(projectRoot),
		CommandCandidates:        commandCandidates(projectRoot),
	}
	data := map[string]any{
		"root":                       project.Root,
		"bifrost_md_exists":          project.BifrostMDExists,
		"package_manager_candidates": project.PackageManagerCandidates,
		"command_candidates":         project.CommandCandidates,
	}
	return project, EvidenceV2{
		ID:         evidenceID("ev_project", data),
		Type:       EvidenceTypeProjectMetadata,
		Source:     "collector.project",
		ObservedAt: observedAt,
		Summary:    projectEvidenceSummary(project),
		Data:       data,
	}
}

func packageManagerCandidates(projectRoot string) []string {
	candidates := map[string]bool{}
	for file, name := range map[string]string{
		"package-lock.json": "npm",
		"pnpm-lock.yaml":    "pnpm",
		"yarn.lock":         "yarn",
		"go.mod":            "go",
		"Cargo.toml":        "cargo",
		"pyproject.toml":    "python",
	} {
		if fileExists(filepath.Join(projectRoot, file)) {
			candidates[name] = true
		}
	}
	return sortedKeys(candidates)
}

func commandCandidates(projectRoot string) []string {
	candidates := map[string]bool{}
	if fileExists(filepath.Join(projectRoot, "go.mod")) {
		candidates["go test ./..."] = true
	}
	if fileExists(filepath.Join(projectRoot, "Makefile")) {
		candidates["make test"] = true
	}
	for _, cmd := range packageJSONCommands(projectRoot) {
		candidates[cmd] = true
	}
	return sortedKeys(candidates)
}

func packageJSONCommands(projectRoot string) []string {
	data, err := os.ReadFile(filepath.Join(projectRoot, "package.json"))
	if err != nil {
		return nil
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}
	var out []string
	for _, script := range []string{"test", "lint", "typecheck", "build"} {
		if _, ok := pkg.Scripts[script]; ok {
			out = append(out, "npm run "+script)
		}
	}
	sort.Strings(out)
	return out
}

func projectEvidenceSummary(project ProjectObservedV2) string {
	parts := []string{"project metadata collected"}
	if len(project.PackageManagerCandidates) > 0 {
		parts = append(parts, "package managers: "+strings.Join(project.PackageManagerCandidates, ", "))
	}
	if project.BifrostMDExists {
		parts = append(parts, "BIFROST.md exists")
	}
	return strings.Join(parts, ", ")
}

func annotateModelClaims(snap *SnapshotV2, observedAt time.Time) []EvidenceV2 {
	var evidence []EvidenceV2
	for i := range snap.Interpretation.StatusItems {
		item := &snap.Interpretation.StatusItems[i]
		if item.Verification == nil {
			item.Verification = &VerificationV2{State: "unverified", Reason: "No evidence-backed verification recorded"}
		}
		if len(item.EvidenceRefs) > 0 {
			continue
		}
		data := map[string]any{"claim_id": item.ID, "text": item.Text, "state": item.State}
		ev := EvidenceV2{
			ID:         evidenceID("ev_claim", data),
			Type:       EvidenceTypeModelClaim,
			Source:     "model.interpretation",
			ObservedAt: observedAt,
			Summary:    "Model claim without observed evidence: " + item.Text,
			Data:       data,
		}
		item.EvidenceRefs = append(item.EvidenceRefs, ev.ID)
		evidence = append(evidence, ev)
	}
	return evidence
}

func reportedCommandEvidence(commands []ReportedCommand, observedAt time.Time) ([]CommandObservedV2, []EvidenceV2) {
	observed := make([]CommandObservedV2, 0, len(commands))
	evidence := make([]EvidenceV2, 0, len(commands))
	for _, command := range commands {
		if strings.TrimSpace(command.Command) == "" {
			continue
		}
		cmd, ev := NewCommandEvidence(command, observedAt)
		observed = append(observed, cmd)
		evidence = append(evidence, ev)
	}
	return observed, evidence
}

func mergeCommands(existing, incoming []CommandObservedV2) []CommandObservedV2 {
	byID := map[string]CommandObservedV2{}
	for _, cmd := range existing {
		if cmd.ID != "" {
			byID[cmd.ID] = cmd
		}
	}
	for _, cmd := range incoming {
		if cmd.ID != "" {
			byID[cmd.ID] = cmd
		}
	}
	out := make([]CommandObservedV2, 0, len(byID))
	for _, cmd := range byID {
		out = append(out, cmd)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CapturedAt.Equal(out[j].CapturedAt) {
			return out[i].CapturedAt.Before(out[j].CapturedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func limitedNonEmptyLines(text string, maxLines int) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
		if len(out) >= maxLines {
			break
		}
	}
	return out
}

func sortedKeys(items map[string]bool) []string {
	out := make([]string, 0, len(items))
	for item := range items {
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

func pluralCount(n int, singular string) string {
	if n == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %ss", n, singular)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
