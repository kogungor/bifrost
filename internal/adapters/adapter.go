package adapters

import "fmt"

// Adapter defines the integration contract for a single AI coding tool.
type Adapter interface {
	Name() string            // "claude-code"
	DisplayName() string     // "Claude Code"
	IsInstalled() bool       // detect installation
	CommandsDir() string     // ~/.claude/commands/
	MCPConfigPath() string   // ~/.claude.json (empty string if N/A)
	InstructionFile() string // "CLAUDE.md"

	// AddMCPServer registers `bifrost mcp-serve` in the tool's parsed config
	// using that tool's schema. Keys Bifrost does not own are left untouched.
	AddMCPServer(config map[string]any) error
	// HasMCPServer reports whether the parsed config registers Bifrost.
	HasMCPServer(config map[string]any) bool
	// LegacyPaths lists locations older Bifrost versions wrote to.
	LegacyPaths() []string
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
