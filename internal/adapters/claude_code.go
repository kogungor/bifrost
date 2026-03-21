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
	home, _ := os.UserHomeDir()
	return &ClaudeCode{homeDir: home}
}

func (a *ClaudeCode) Name() string        { return "claude-code" }
func (a *ClaudeCode) DisplayName() string  { return "Claude Code" }

func (a *ClaudeCode) IsInstalled() bool {
	_, err := os.Stat(filepath.Join(a.homeDir, ".claude"))
	return err == nil
}

func (a *ClaudeCode) CommandsDir() string {
	return filepath.Join(a.homeDir, ".claude", "commands")
}

func (a *ClaudeCode) MCPConfigPath() string {
	return filepath.Join(a.homeDir, ".claude", "mcp.json")
}

func (a *ClaudeCode) InstructionFile() string { return "CLAUDE.md" }
