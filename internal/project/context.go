package project

import (
	"encoding/json"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kogungor/bifrost/internal/security"
	"github.com/kogungor/bifrost/internal/snapshot"
)

type ContextCandidate struct {
	ID                 string `json:"id"`
	Type               string `json:"type"`
	Text               string `json:"text"`
	SourceSnapshot     string `json:"source_snapshot,omitempty"`
	SourceClaim        string `json:"source_claim,omitempty"`
	RecommendedSection string `json:"recommended_section"`
	Confidence         string `json:"confidence"`
	Reason             string `json:"reason"`
}

type ContextReport struct {
	Path            string             `json:"path"`
	Exists          bool               `json:"exists"`
	MissingSections []string           `json:"missing_sections,omitempty"`
	Placeholders    []string           `json:"placeholders,omitempty"`
	Contradictions  []string           `json:"contradictions,omitempty"`
	Candidates      []ContextCandidate `json:"candidates,omitempty"`
	Ignored         []string           `json:"ignored,omitempty"`
}

type contextDoc struct {
	content  string
	sections map[string]string
}

var defaultContextSections = []string{
	"What this is",
	"Stack",
	"Conventions",
	"Always read before working",
	"Commands",
	"Do not touch",
}

func AnalyzeContext(projectRoot string, snap *snapshot.SnapshotV2) (ContextReport, error) {
	path := filepath.Join(projectRoot, "BIFROST.md")
	doc, exists, err := readContextDoc(path)
	if err != nil {
		return ContextReport{}, err
	}
	report := ContextReport{Path: path, Exists: exists}
	report.MissingSections = missingContextSections(doc)
	report.Placeholders = placeholderSections(doc)
	report.Contradictions = contextContradictions(projectRoot, doc)
	ignored, err := readIgnoredPromotionIDs(projectRoot)
	if err != nil {
		return ContextReport{}, err
	}
	report.Ignored = sortedStringSet(ignored)
	report.Candidates = filterIgnoredCandidates(contextCandidates(projectRoot, snap, doc), ignored)
	return report, nil
}

func contextContradictions(projectRoot string, doc contextDoc) []string {
	if strings.TrimSpace(doc.content) == "" {
		return nil
	}
	detectedPM := map[string]bool{}
	for _, pm := range detectPackageManagers(projectRoot) {
		detectedPM[pm] = true
	}
	knownPMs := []string{"npm", "pnpm", "yarn", "go", "cargo", "python"}
	lower := strings.ToLower(doc.content)
	var contradictions []string
	for _, pm := range knownPMs {
		if strings.Contains(lower, "package manager: "+pm) && !detectedPM[pm] {
			contradictions = append(contradictions, "BIFROST.md mentions package manager "+pm+" but project files do not support it")
		}
	}
	return contradictions
}

func ContextPatch(report ContextReport, ids []string) string {
	selected := selectCandidates(report.Candidates, ids)
	var b strings.Builder
	b.WriteString("BIFROST.md patch preview\n\n")
	if !report.Exists {
		b.WriteString("- create BIFROST.md\n")
	}
	if len(report.MissingSections) > 0 {
		b.WriteString("- missing sections: ")
		b.WriteString(strings.Join(report.MissingSections, ", "))
		b.WriteString("\n")
	}
	if len(report.Placeholders) > 0 {
		b.WriteString("- placeholder sections: ")
		b.WriteString(strings.Join(report.Placeholders, ", "))
		b.WriteString("\n")
	}
	if len(selected) == 0 {
		b.WriteString("- no promotion candidates\n")
		return b.String()
	}
	current := ""
	for _, candidate := range selected {
		if candidate.RecommendedSection != current {
			current = candidate.RecommendedSection
			b.WriteString("\n## ")
			b.WriteString(current)
			b.WriteString("\n")
		}
		b.WriteString("- [")
		b.WriteString(candidate.ID)
		b.WriteString("] ")
		b.WriteString(candidate.Text)
		b.WriteString(" (")
		b.WriteString(candidate.Reason)
		b.WriteString(sourceSuffix(candidate))
		b.WriteString(")\n")
	}
	return b.String()
}

func ApplyContextCandidates(projectRoot string, report ContextReport, ids []string) error {
	selected := selectCandidates(report.Candidates, ids)
	if missing := MissingAcceptedCandidateIDs(report.Candidates, ids); len(missing) > 0 {
		return fmt.Errorf("no accepted promotion candidates matched: %s", strings.Join(missing, ", "))
	}
	if len(selected) == 0 {
		return nil
	}
	path := report.Path
	content := ""
	if data, err := os.ReadFile(path); err == nil {
		content = string(data)
	} else if !os.IsNotExist(err) {
		return err
	}
	if strings.TrimSpace(content) == "" {
		content = defaultBifrostContent(filepath.Base(projectRoot))
	}
	for _, candidate := range selected {
		content = appendCandidate(content, candidate)
	}
	redacted, err := redactContextBeforeWrite(projectRoot, content)
	if err != nil {
		return err
	}
	content = redacted
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func MissingAcceptedCandidateIDs(candidates []ContextCandidate, ids []string) []string {
	return missingAcceptedIDs(candidates, ids)
}

func IgnorePromotionIDs(projectRoot string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	ignored, err := readIgnoredPromotionIDs(projectRoot)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if id = strings.TrimSpace(id); id != "" {
			ignored[id] = true
		}
	}
	return writeIgnoredPromotionIDs(projectRoot, ignored)
}

func readContextDoc(path string) (contextDoc, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return contextDoc{sections: map[string]string{}}, false, nil
		}
		return contextDoc{}, false, err
	}
	content := string(data)
	return contextDoc{content: content, sections: parseMarkdownSections(content)}, true, nil
}

func parseMarkdownSections(content string) map[string]string {
	sections := map[string]string{}
	current := ""
	var b strings.Builder
	flush := func() {
		if current != "" {
			sections[current] = strings.TrimSpace(b.String())
		}
		b.Reset()
	}
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "## ") {
			flush()
			current = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			continue
		}
		if current != "" {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	flush()
	return sections
}

func missingContextSections(doc contextDoc) []string {
	var missing []string
	for _, section := range defaultContextSections {
		if _, ok := doc.sections[section]; !ok {
			missing = append(missing, section)
		}
	}
	return missing
}

func placeholderSections(doc contextDoc) []string {
	var placeholders []string
	for _, section := range defaultContextSections {
		body := doc.sections[section]
		if strings.Contains(body, "[") && strings.Contains(body, "]") {
			placeholders = append(placeholders, section)
		}
	}
	return placeholders
}

func contextCandidates(projectRoot string, snap *snapshot.SnapshotV2, doc contextDoc) []ContextCandidate {
	var candidates []ContextCandidate
	add := func(kind, text, section, confidence, reason, sourceClaim string) {
		text = strings.TrimSpace(text)
		if text == "" || contextContains(doc, text) {
			return
		}
		sourceSnapshot := ""
		if snap != nil {
			sourceSnapshot = snap.ID
		}
		candidates = append(candidates, ContextCandidate{
			ID:                 promotionID(kind, section, text),
			Type:               kind,
			Text:               text,
			SourceSnapshot:     sourceSnapshot,
			SourceClaim:        sourceClaim,
			RecommendedSection: section,
			Confidence:         confidence,
			Reason:             reason,
		})
	}
	for _, pm := range detectPackageManagers(projectRoot) {
		add("package_manager", "Package manager: "+pm, "Stack", "high", "detected from project files", "")
	}
	for _, cmd := range detectCommandCandidates(projectRoot) {
		add("command", cmd, "Commands", "high", "detected from project files", "")
	}
	for _, framework := range detectFrameworks(projectRoot) {
		add("framework", framework, "Stack", "medium", "detected from dependency metadata", "")
	}
	for _, path := range detectExistingPaths(projectRoot, []string{"migrations", "db/migrations", "prisma/migrations"}) {
		add("convention", "Migration directory: "+path, "Conventions", "medium", "detected directory", "")
	}
	for _, path := range detectExistingPaths(projectRoot, []string{"dist", "build", "coverage", "node_modules", "vendor"}) {
		add("convention", "Do not edit generated/vendor directory: "+path, "Do not touch", "medium", "detected generated/vendor directory", "")
	}
	for _, path := range detectExistingPaths(projectRoot, []string{".env.example", ".env.sample", "example.env"}) {
		add("env_example", "Environment example file: "+path, "Always read before working", "medium", "detected env example", "")
	}
	for _, path := range detectExistingPaths(projectRoot, []string{"go.mod", "package.json", "tsconfig.json", "vite.config.ts", "next.config.js", ".goreleaser.yml", "Dockerfile", "docker-compose.yml"}) {
		add("important_config", "Important config: "+path, "Always read before working", "medium", "detected config file", "")
	}
	if snap != nil {
		for _, decision := range snap.Interpretation.Decisions {
			add("decision", decision.Text, "Decisions", "medium", "promoted from snapshot decision", decision.ID)
		}
		for _, risk := range snap.Interpretation.Risks {
			if risk.Severity == "high" {
				add("risk", risk.Text, "Risks", "medium", "high severity snapshot risk", risk.ID)
			}
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].RecommendedSection == candidates[j].RecommendedSection {
			return candidates[i].ID < candidates[j].ID
		}
		return candidates[i].RecommendedSection < candidates[j].RecommendedSection
	})
	return candidates
}

func contextContains(doc contextDoc, text string) bool {
	return strings.Contains(strings.ToLower(doc.content), strings.ToLower(text))
}

func detectPackageManagers(projectRoot string) []string {
	found := map[string]bool{}
	for file, name := range map[string]string{
		"package-lock.json": "npm",
		"pnpm-lock.yaml":    "pnpm",
		"yarn.lock":         "yarn",
		"go.mod":            "go",
		"Cargo.toml":        "cargo",
		"pyproject.toml":    "python",
	} {
		if exists(filepath.Join(projectRoot, file)) {
			found[name] = true
		}
	}
	return sortedStringSet(found)
}

func detectCommandCandidates(projectRoot string) []string {
	found := map[string]bool{}
	if exists(filepath.Join(projectRoot, "go.mod")) {
		found["go test ./..."] = true
	}
	if exists(filepath.Join(projectRoot, "Makefile")) {
		found["make test"] = true
	}
	for _, cmd := range packageJSONScriptCommands(projectRoot) {
		found[cmd] = true
	}
	return sortedStringSet(found)
}

func packageJSONScriptCommands(projectRoot string) []string {
	data, err := os.ReadFile(filepath.Join(projectRoot, "package.json"))
	if err != nil {
		return nil
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
		Deps    map[string]string `json:"dependencies"`
		DevDeps map[string]string `json:"devDependencies"`
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

func detectFrameworks(projectRoot string) []string {
	data, err := os.ReadFile(filepath.Join(projectRoot, "package.json"))
	if err != nil {
		return nil
	}
	var pkg struct {
		Deps    map[string]string `json:"dependencies"`
		DevDeps map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}
	deps := map[string]bool{}
	for name := range pkg.Deps {
		deps[name] = true
	}
	for name := range pkg.DevDeps {
		deps[name] = true
	}
	var out []string
	for dep, label := range map[string]string{
		"next":       "Framework: Next.js",
		"react":      "Library: React",
		"vue":        "Framework: Vue",
		"svelte":     "Framework: Svelte",
		"vite":       "Build tool: Vite",
		"express":    "Server framework: Express",
		"typescript": "Language: TypeScript",
	} {
		if deps[dep] {
			out = append(out, label)
		}
	}
	sort.Strings(out)
	return out
}

func detectExistingPaths(projectRoot string, paths []string) []string {
	var found []string
	for _, path := range paths {
		if exists(filepath.Join(projectRoot, path)) {
			found = append(found, path)
		}
	}
	return found
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func promotionID(kind, section, text string) string {
	sum := crc32.ChecksumIEEE([]byte(kind + "|" + section + "|" + text))
	return fmt.Sprintf("prom_%08x", sum)
}

func filterIgnoredCandidates(candidates []ContextCandidate, ignored map[string]bool) []ContextCandidate {
	out := candidates[:0]
	for _, candidate := range candidates {
		if !ignored[candidate.ID] {
			out = append(out, candidate)
		}
	}
	return out
}

func selectCandidates(candidates []ContextCandidate, ids []string) []ContextCandidate {
	wantedAll := len(ids) == 0
	wanted := map[string]bool{}
	for _, id := range ids {
		if id == "all" {
			wantedAll = true
			break
		}
		wanted[id] = true
	}
	var selected []ContextCandidate
	for _, candidate := range candidates {
		if wantedAll || wanted[candidate.ID] {
			selected = append(selected, candidate)
		}
	}
	return selected
}

func acceptsAll(ids []string) bool {
	for _, id := range ids {
		if id == "all" {
			return true
		}
	}
	return false
}

func missingAcceptedIDs(candidates []ContextCandidate, ids []string) []string {
	if len(ids) == 0 || acceptsAll(ids) {
		return nil
	}
	available := map[string]bool{}
	for _, candidate := range candidates {
		available[candidate.ID] = true
	}
	var missing []string
	for _, id := range ids {
		if !available[id] {
			missing = append(missing, id)
		}
	}
	return missing
}

func redactContextBeforeWrite(projectRoot, content string) (string, error) {
	cfg := security.LoadConfig(projectRoot)
	redacted, findings := security.RedactString(content, cfg)
	active := security.CountActive(findings)
	if active == 0 {
		return content, nil
	}
	if cfg.Strict {
		return "", fmt.Errorf("BIFROST.md promotion contains secret-like values: %s", security.Summary(findings))
	}
	if !cfg.RedactBeforeWrite {
		return content, nil
	}
	return redacted, nil
}

func appendCandidate(content string, candidate ContextCandidate) string {
	line := "- " + candidate.Text + " (" + candidate.Reason + sourceSuffix(candidate) + ")"
	if strings.Contains(content, line) {
		return content
	}
	sectionHeader := "## " + candidate.RecommendedSection
	if !hasExactSection(content, sectionHeader) {
		return strings.TrimRight(content, "\n") + "\n\n" + sectionHeader + "\n" + line + "\n"
	}
	lines := strings.Split(content, "\n")
	insert := len(lines)
	for i, item := range lines {
		if strings.TrimSpace(item) != sectionHeader {
			continue
		}
		insert = i + 1
		for insert < len(lines) && !strings.HasPrefix(lines[insert], "## ") {
			insert++
		}
		break
	}
	lines = append(lines[:insert], append([]string{line}, lines[insert:]...)...)
	return strings.Join(lines, "\n")
}

func hasExactSection(content, sectionHeader string) bool {
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == sectionHeader {
			return true
		}
	}
	return false
}

func sourceSuffix(candidate ContextCandidate) string {
	parts := []string{}
	if candidate.SourceSnapshot != "" {
		parts = append(parts, "snapshot: "+candidate.SourceSnapshot)
	}
	if candidate.SourceClaim != "" {
		parts = append(parts, "claim: "+candidate.SourceClaim)
	}
	if len(parts) == 0 {
		return ""
	}
	return "; " + strings.Join(parts, "; ")
}

func defaultBifrostContent(name string) string {
	return fmt.Sprintf("---\nproject: %s\n---\n\n# Project: %s\n", name, name)
}

func ignorePath(projectRoot string) string {
	return filepath.Join(projectRoot, ".bifrost", "promotions_ignore.json")
}

func readIgnoredPromotionIDs(projectRoot string) (map[string]bool, error) {
	ignored := map[string]bool{}
	data, err := os.ReadFile(ignorePath(projectRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return ignored, nil
		}
		return nil, err
	}
	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		return nil, err
	}
	for _, id := range ids {
		ignored[id] = true
	}
	return ignored, nil
}

func writeIgnoredPromotionIDs(projectRoot string, ignored map[string]bool) error {
	if err := os.MkdirAll(filepath.Join(projectRoot, ".bifrost"), 0700); err != nil {
		return err
	}
	ids := sortedStringSet(ignored)
	data, err := json.MarshalIndent(ids, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(ignorePath(projectRoot), data, 0600)
}

func sortedStringSet(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for item := range set {
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}
