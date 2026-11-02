package adapters

import (
	"fmt"
	"os"
	"path/filepath"
)

// ClaudeCode implements the Adapter interface for Claude Code.
type ClaudeCode struct {
	homeDir string
}

func newClaudeCode() *ClaudeCode {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return &ClaudeCode{homeDir: home}
}

func (a *ClaudeCode) Name() string        { return "claude-code" }
func (a *ClaudeCode) DisplayName() string { return "Claude Code" }

func (a *ClaudeCode) IsInstalled() bool {
	_, err := os.Stat(filepath.Join(a.homeDir, ".claude"))
	return err == nil
}

func (a *ClaudeCode) CommandsDir() string {
	return filepath.Join(a.homeDir, ".claude", "commands")
}

// MCPConfigPath is where Claude Code keeps user-scope MCP servers.
func (a *ClaudeCode) MCPConfigPath() string {
	return filepath.Join(a.homeDir, ".claude.json")
}

func (a *ClaudeCode) InstructionFile() string { return "CLAUDE.md" }

func (a *ClaudeCode) CommandPath(name string) string {
	return markdownCommandPath(a.CommandsDir(), name)
}

// AddMCPServer writes the same entry as `claude mcp add --scope user`.
func (a *ClaudeCode) AddMCPServer(data []byte) ([]byte, error) {
	config, err := parseJSONConfig(data)
	if err != nil {
		return nil, err
	}
	servers, err := childObject(config, "mcpServers")
	if err != nil {
		return nil, err
	}
	servers["bifrost"] = map[string]any{
		"type":    "stdio",
		"command": "bifrost",
		"args":    []string{"mcp-serve"},
		"env":     map[string]any{},
	}
	return encodeJSONConfig(config)
}

func (a *ClaudeCode) HasMCPServer(data []byte) bool {
	config, err := parseJSONConfig(data)
	return err == nil && hasChild(config, "mcpServers", "bifrost")
}

// claudeCodeHooks maps Claude Code hook events to `bifrost hook` subcommands,
// with timeouts in seconds. SessionEnd hooks otherwise share a 1.5 s budget.
var claudeCodeHooks = []struct {
	event   string
	command string
	timeout int
}{
	{"PreCompact", "bifrost hook pre-compact", 30},
	{"PostCompact", "bifrost hook post-compact", 30},
	{"SessionStart", "bifrost hook session-start", 30},
	{"SessionEnd", "bifrost hook session-end", 10},
}

// HooksConfigPath is Claude Code's user settings file.
func (a *ClaudeCode) HooksConfigPath() string {
	return filepath.Join(a.homeDir, ".claude", "settings.json")
}

// AddHooks adds one matcher group per event that runs the bifrost hook
// command. Events that already run it are left alone.
func (a *ClaudeCode) AddHooks(data []byte) ([]byte, error) {
	config, err := parseJSONConfig(data)
	if err != nil {
		return nil, err
	}
	hooks, err := childObject(config, "hooks")
	if err != nil {
		return nil, err
	}
	for _, h := range claudeCodeHooks {
		groups, ok := hooks[h.event].([]any)
		if hooks[h.event] != nil && !ok {
			return nil, fmt.Errorf("hooks.%s is not a JSON array", h.event)
		}
		if hasHookCommand(groups, h.command) {
			continue
		}
		hooks[h.event] = append(groups, map[string]any{
			"hooks": []any{map[string]any{"type": "command", "command": h.command, "timeout": h.timeout}},
		})
	}
	return encodeJSONConfig(config)
}

func (a *ClaudeCode) HasHooks(data []byte) bool {
	config, err := parseJSONConfig(data)
	if err != nil {
		return false
	}
	hooks, _ := config["hooks"].(map[string]any)
	for _, h := range claudeCodeHooks {
		groups, _ := hooks[h.event].([]any)
		if !hasHookCommand(groups, h.command) {
			return false
		}
	}
	return true
}

// hasHookCommand reports whether any matcher group already runs command.
func hasHookCommand(groups []any, command string) bool {
	for _, g := range groups {
		group, _ := g.(map[string]any)
		handlers, _ := group["hooks"].([]any)
		for _, h := range handlers {
			if handler, _ := h.(map[string]any); handler["command"] == command {
				return true
			}
		}
	}
	return false
}

// LegacyPaths: Bifrost <= 1.3 registered MCP in ~/.claude/mcp.json, which
// Claude Code never reads.
func (a *ClaudeCode) LegacyPaths() []string {
	return []string{filepath.Join(a.homeDir, ".claude", "mcp.json")}
}
