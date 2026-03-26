package snapshot

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// frontmatter holds the YAML frontmatter fields.
type frontmatter struct {
	BifrostVersion int    `yaml:"bifrost_version"`
	Timestamp      string `yaml:"timestamp"`
	SourceTool     string `yaml:"source_tool"`
	Project        string `yaml:"project"`
	TokenPressure  string `yaml:"token_pressure"`
	SessionIntent  string `yaml:"session_intent,omitempty"`
	ActivePlanName string `yaml:"active_plan_name,omitempty"`
	GitSHA         string `yaml:"git_sha,omitempty"`
	SessionStart   string `yaml:"session_start,omitempty"`
}

// Parse parses a snapshot from raw markdown bytes.
// Missing sections produce empty values. Missing frontmatter fields produce errors.
func Parse(data []byte) (*Snapshot, error) {
	content := string(data)

	// Extract YAML frontmatter
	fm, body, err := extractFrontmatter(content)
	if err != nil {
		return nil, err
	}

	ts, err := time.Parse(time.RFC3339, fm.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp %q: %w", fm.Timestamp, err)
	}

	s := &Snapshot{
		BifrostVersion: fm.BifrostVersion,
		Timestamp:      ts,
		SourceTool:     fm.SourceTool,
		Project:        fm.Project,
		TokenPressure:  fm.TokenPressure,
		SessionIntent:  fm.SessionIntent,
		ActivePlanName: fm.ActivePlanName,
		GitSHA:         fm.GitSHA,
		SessionStart:   fm.SessionStart,
	}

	// Parse markdown sections
	sections := parseSections(body)

	s.CurrentTask = strings.TrimSpace(sections["Current Task"])
	s.Status = parseList(sections["Status"])
	s.ActiveFiles = parseActiveFiles(sections["Active Files"])
	s.Decisions = parseList(sections["Decisions Made"])
	s.EnvNotes = parseList(sections["Environment Notes"])
	s.NextStep = strings.TrimSpace(sections["Next Step"])
	s.Assumptions = parseList(sections["Assumptions"])
	s.OpenQuestions = parseList(sections["Open Questions"])
	s.Risks = parseList(sections["Risks"])

	return s, nil
}

// Render serializes a snapshot to markdown with YAML frontmatter.
func Render(s *Snapshot) string {
	var b strings.Builder

	// Frontmatter
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("bifrost_version: %d\n", s.BifrostVersion))
	b.WriteString(fmt.Sprintf("timestamp: %s\n", s.Timestamp.UTC().Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("source_tool: %s\n", s.SourceTool))
	b.WriteString(fmt.Sprintf("project: %s\n", s.Project))
	b.WriteString(fmt.Sprintf("token_pressure: %s\n", s.TokenPressure))
	if s.SessionIntent != "" {
		b.WriteString(fmt.Sprintf("session_intent: %s\n", s.SessionIntent))
	}
	if s.ActivePlanName != "" {
		b.WriteString(fmt.Sprintf("active_plan_name: %s\n", s.ActivePlanName))
	}
	if s.GitSHA != "" {
		b.WriteString(fmt.Sprintf("git_sha: %s\n", s.GitSHA))
	}
	if s.SessionStart != "" {
		b.WriteString(fmt.Sprintf("session_start: %s\n", s.SessionStart))
	}
	b.WriteString("---\n\n")

	// Body
	b.WriteString("# Session Snapshot\n\n")

	b.WriteString("## Current Task\n")
	b.WriteString(s.CurrentTask + "\n\n")

	b.WriteString("## Status\n")
	if len(s.Status) == 0 {
		b.WriteString("Nothing to note.\n")
	} else {
		for _, item := range s.Status {
			b.WriteString(item + "\n")
		}
	}
	b.WriteString("\n")

	b.WriteString("## Active Files\n")
	if len(s.ActiveFiles) == 0 {
		b.WriteString("Nothing to note.\n")
	} else {
		for _, f := range s.ActiveFiles {
			if f.Confidence != "" {
				b.WriteString(fmt.Sprintf("- `%s` — %s [confidence: %s]\n", f.Path, f.Note, f.Confidence))
			} else {
				b.WriteString(fmt.Sprintf("- `%s` — %s\n", f.Path, f.Note))
			}
		}
	}
	b.WriteString("\n")

	b.WriteString("## Decisions Made\n")
	if len(s.Decisions) == 0 {
		b.WriteString("Nothing to note.\n")
	} else {
		for _, d := range s.Decisions {
			b.WriteString(d + "\n")
		}
	}
	b.WriteString("\n")

	b.WriteString("## Environment Notes\n")
	if len(s.EnvNotes) == 0 {
		b.WriteString("Nothing to note.\n")
	} else {
		for _, n := range s.EnvNotes {
			b.WriteString(n + "\n")
		}
	}
	b.WriteString("\n")

	b.WriteString("## Next Step\n")
	b.WriteString(s.NextStep + "\n")

	if len(s.Assumptions) > 0 {
		b.WriteString("\n## Assumptions\n")
		for _, a := range s.Assumptions {
			b.WriteString(a + "\n")
		}
	}

	if len(s.OpenQuestions) > 0 {
		b.WriteString("\n## Open Questions\n")
		for _, q := range s.OpenQuestions {
			b.WriteString(q + "\n")
		}
	}

	if len(s.Risks) > 0 {
		b.WriteString("\n## Risks\n")
		for _, r := range s.Risks {
			b.WriteString(r + "\n")
		}
	}

	return b.String()
}

func extractFrontmatter(content string) (*frontmatter, string, error) {
	// Must start with ---
	if !strings.HasPrefix(content, "---\n") {
		return nil, "", fmt.Errorf("snapshot missing YAML frontmatter")
	}

	// Find closing ---
	rest := content[4:]
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		// Try ending with --- at EOF
		if strings.HasSuffix(rest, "\n---") {
			idx = len(rest) - 4
		} else {
			return nil, "", fmt.Errorf("snapshot frontmatter not closed")
		}
	}

	yamlStr := rest[:idx]
	body := rest[idx+4:] // skip \n---\n

	var fm frontmatter
	if err := yaml.Unmarshal([]byte(yamlStr), &fm); err != nil {
		return nil, "", fmt.Errorf("invalid frontmatter YAML: %w", err)
	}

	if fm.Timestamp == "" {
		return nil, "", fmt.Errorf("frontmatter missing required field: timestamp")
	}
	if fm.SourceTool == "" {
		return nil, "", fmt.Errorf("frontmatter missing required field: source_tool")
	}

	return &fm, body, nil
}

// parseSections splits markdown body into map of heading→content.
func parseSections(body string) map[string]string {
	sections := make(map[string]string)
	var currentHeading string
	var currentContent strings.Builder

	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "## ") {
			if currentHeading != "" {
				sections[currentHeading] = currentContent.String()
			}
			currentHeading = strings.TrimPrefix(line, "## ")
			currentContent.Reset()
		} else if currentHeading != "" {
			currentContent.WriteString(line + "\n")
		}
	}
	if currentHeading != "" {
		sections[currentHeading] = currentContent.String()
	}

	return sections
}

// parseList extracts list items (lines starting with "- ").
func parseList(section string) []string {
	var items []string
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") {
			items = append(items, line)
		}
	}
	return items
}

// parseActiveFiles extracts file entries in the format: - `path` — note
func parseActiveFiles(section string) []ActiveFile {
	var files []ActiveFile
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- `") {
			continue
		}
		// Extract path between backticks
		rest := line[3:] // after "- `"
		idx := strings.Index(rest, "`")
		if idx < 0 {
			continue
		}
		path := rest[:idx]
		note := ""
		after := rest[idx+1:]
		if strings.HasPrefix(after, " — ") {
			note = strings.TrimPrefix(after, " — ")
		} else if strings.HasPrefix(after, " - ") {
			note = strings.TrimPrefix(after, " - ")
		}

		// Extract optional [confidence: X] suffix
		confidence := ""
		const confPrefix = " [confidence: "
		if ci := strings.Index(note, confPrefix); ci >= 0 {
			suffix := note[ci+len(confPrefix):]
			if ei := strings.Index(suffix, "]"); ei >= 0 {
				confidence = suffix[:ei]
				note = note[:ci]
			}
		}

		files = append(files, ActiveFile{Path: path, Note: note, Confidence: confidence})
	}
	return files
}
