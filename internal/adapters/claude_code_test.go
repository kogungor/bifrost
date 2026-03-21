package adapters

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClaudeCodeName(t *testing.T) {
	a := &ClaudeCode{homeDir: "/tmp/fake"}
	if a.Name() != "claude-code" {
		t.Errorf("expected claude-code, got %s", a.Name())
	}
	if a.DisplayName() != "Claude Code" {
		t.Errorf("expected Claude Code, got %s", a.DisplayName())
	}
	if a.InstructionFile() != "CLAUDE.md" {
		t.Errorf("expected CLAUDE.md, got %s", a.InstructionFile())
	}
}

func TestClaudeCodePaths(t *testing.T) {
	a := &ClaudeCode{homeDir: "/home/user"}
	if a.CommandsDir() != "/home/user/.claude/commands" {
		t.Errorf("unexpected commands dir: %s", a.CommandsDir())
	}
	if a.MCPConfigPath() != "/home/user/.claude/mcp.json" {
		t.Errorf("unexpected MCP config: %s", a.MCPConfigPath())
	}
}

func TestClaudeCodeDetection(t *testing.T) {
	tmp := t.TempDir()

	// Not installed
	a := &ClaudeCode{homeDir: tmp}
	if a.IsInstalled() {
		t.Error("should not be installed without .claude dir")
	}

	// Installed
	os.Mkdir(filepath.Join(tmp, ".claude"), 0755)
	if !a.IsInstalled() {
		t.Error("should be installed with .claude dir")
	}
}
