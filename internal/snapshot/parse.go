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
	}

	// Parse markdown sections
	sections := parseSections(body)

	s.CurrentTask = strings.TrimSpace(sections["Current Task"])
	s.Status = parseList(sections["Status"])
	s.ActiveFiles = parseActiveFiles(sections["Active Files"])
	s.Decisions = parseList(sections["Decisions Made"])
	s.EnvNotes = parseList(sections["Environment Notes"])
	s.NextStep = strings.TrimSpace(sections["Next Step"])

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
			b.WriteString(fmt.Sprintf("- `%s` — %s\n", f.Path, f.Note))
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
		files = append(files, ActiveFile{Path: path, Note: note})
	}
	return files
}
