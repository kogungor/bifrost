package adapters

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
)

// Adapter defines the integration contract for a single AI coding tool.
type Adapter interface {
	Name() string            // "claude-code"
	DisplayName() string     // "Claude Code"
	IsInstalled() bool       // detect installation
	CommandsDir() string     // ~/.claude/commands/
	MCPConfigPath() string   // ~/.claude.json (empty string if N/A)
	InstructionFile() string // "CLAUDE.md"

	// CommandPath is where the named Bifrost command ("handoff", "handin",
	// "plan", "review") is installed for this tool.
	CommandPath(name string) string
	// AddMCPServer returns the tool's MCP config file contents (nil when the
	// file does not exist yet) with `bifrost mcp-serve` registered in that
	// tool's format. Everything Bifrost does not own is kept.
	AddMCPServer(data []byte) ([]byte, error)
	// HasMCPServer reports whether the config file contents register Bifrost.
	HasMCPServer(data []byte) bool
	// LegacyPaths lists locations older Bifrost versions wrote to.
	LegacyPaths() []string
}

// markdownCommandPath is the flat <dir>/<name>.md layout used by slash commands.
func markdownCommandPath(dir, name string) string {
	return filepath.Join(dir, name+".md")
}

// parseJSONConfig parses a JSON config file, treating empty input as {}.
// Numbers stay json.Number so values Bifrost does not own (e.g. large IDs in
// ~/.claude.json) survive a rewrite unchanged.
func parseJSONConfig(data []byte) (map[string]any, error) {
	config := make(map[string]any)
	if len(bytes.TrimSpace(data)) == 0 {
		return config, nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&config); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}
	if config == nil {
		config = make(map[string]any)
	}
	return config, nil
}

// encodeJSONConfig writes config with two-space indentation and without
// escaping <, > and & so untouched strings keep their original form.
func encodeJSONConfig(config map[string]any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(config); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// childObject returns parent[key] as a JSON object, creating it when absent.
func childObject(parent map[string]any, key string) (map[string]any, error) {
	switch v := parent[key].(type) {
	case map[string]any:
		return v, nil
	case nil:
		child := map[string]any{}
		parent[key] = child
		return child, nil
	default:
		return nil, fmt.Errorf("%q is not a JSON object", key)
	}
}

// hasChild reports whether config contains the nested object path.
func hasChild(config map[string]any, path ...string) bool {
	current := config
	for i, key := range path {
		v, ok := current[key]
		if !ok {
			return false
		}
		if i == len(path)-1 {
			return true
		}
		if current, ok = v.(map[string]any); !ok {
			return false
		}
	}
	return false
}
