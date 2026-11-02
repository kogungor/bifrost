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
	if a.MCPConfigPath() != "/home/user/.claude.json" {
		t.Errorf("unexpected MCP config: %s", a.MCPConfigPath())
	}
	if legacy := a.LegacyPaths(); len(legacy) != 1 || legacy[0] != "/home/user/.claude/mcp.json" {
		t.Errorf("unexpected legacy paths: %v", legacy)
	}
}

func TestClaudeCodeAddMCPServer(t *testing.T) {
	a := &ClaudeCode{homeDir: "/home/user"}
	config := map[string]any{
		"numStartups": 3,
		"mcpServers":  map[string]any{"other": map[string]any{"command": "other"}},
	}
	if a.HasMCPServer(config) {
		t.Fatal("HasMCPServer before AddMCPServer")
	}
	if err := a.AddMCPServer(config); err != nil {
		t.Fatalf("AddMCPServer: %v", err)
	}
	if !a.HasMCPServer(config) {
		t.Fatal("HasMCPServer after AddMCPServer")
	}
	servers := config["mcpServers"].(map[string]any)
	if _, ok := servers["other"]; !ok {
		t.Error("existing server was dropped")
	}
	entry := servers["bifrost"].(map[string]any)
	if entry["type"] != "stdio" || entry["command"] != "bifrost" {
		t.Errorf("unexpected entry: %v", entry)
	}
	if config["numStartups"] != 3 {
		t.Error("unrelated key was changed")
	}
}

func TestClaudeCodeAddMCPServerRejectsNonObject(t *testing.T) {
	a := &ClaudeCode{homeDir: "/home/user"}
	if err := a.AddMCPServer(map[string]any{"mcpServers": []any{}}); err == nil {
		t.Fatal("expected error when mcpServers is not an object")
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
