package adapters

import (
	"os"
	"path/filepath"
)

// OpenCode implements the Adapter interface for OpenCode.
type OpenCode struct {
	homeDir string
}

func newOpenCode() *OpenCode {
	home, _ := os.UserHomeDir()
	return &OpenCode{homeDir: home}
}

func (a *OpenCode) Name() string        { return "opencode" }
func (a *OpenCode) DisplayName() string  { return "OpenCode" }

func (a *OpenCode) IsInstalled() bool {
	_, err := os.Stat(filepath.Join(a.homeDir, ".opencode"))
	return err == nil
}

func (a *OpenCode) CommandsDir() string {
	return filepath.Join(a.homeDir, ".opencode", "commands")
}

func (a *OpenCode) MCPConfigPath() string { return "opencode.json" }

func (a *OpenCode) InstructionFile() string { return "AGENTS.md" }
