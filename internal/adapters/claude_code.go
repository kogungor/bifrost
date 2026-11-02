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

// LegacyPaths: Bifrost <= 1.3 registered MCP in ~/.claude/mcp.json, which
// Claude Code never reads.
func (a *ClaudeCode) LegacyPaths() []string {
	return []string{filepath.Join(a.homeDir, ".claude", "mcp.json")}
}
