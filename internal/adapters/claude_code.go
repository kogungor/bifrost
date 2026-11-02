package adapters

import (
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

// AddMCPServer writes the same entry as `claude mcp add --scope user`.
func (a *ClaudeCode) AddMCPServer(config map[string]any) error {
	servers, err := childObject(config, "mcpServers")
	if err != nil {
		return err
	}
	servers["bifrost"] = map[string]any{
		"type":    "stdio",
		"command": "bifrost",
		"args":    []string{"mcp-serve"},
		"env":     map[string]any{},
	}
	return nil
}

func (a *ClaudeCode) HasMCPServer(config map[string]any) bool {
	return hasChild(config, "mcpServers", "bifrost")
}

// LegacyPaths: Bifrost <= 1.3 registered MCP in ~/.claude/mcp.json, which
// Claude Code never reads.
func (a *ClaudeCode) LegacyPaths() []string {
	return []string{filepath.Join(a.homeDir, ".claude", "mcp.json")}
}
