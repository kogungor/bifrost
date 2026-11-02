package adapters

import (
	"os"
	"path/filepath"
	"strings"
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
	if a.CommandPath("handin") != "/home/user/.claude/commands/handin.md" {
		t.Errorf("unexpected command path: %s", a.CommandPath("handin"))
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
	in := []byte(`{"numStartups": 12345678901234567, "note": "<b>&</b>", "mcpServers": {"other": {"command": "other"}}}`)
	if a.HasMCPServer(in) {
		t.Fatal("HasMCPServer before AddMCPServer")
	}
	out, err := a.AddMCPServer(in)
	if err != nil {
		t.Fatalf("AddMCPServer: %v", err)
	}
	if !a.HasMCPServer(out) {
		t.Fatalf("HasMCPServer after AddMCPServer:\n%s", out)
	}
	for _, want := range []string{`"other"`, `"numStartups": 12345678901234567`, `"note": "<b>&</b>"`, `"type": "stdio"`, `"mcp-serve"`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("output missing %s:\n%s", want, out)
		}
	}
}

func TestClaudeCodeAddMCPServerToMissingFile(t *testing.T) {
	a := &ClaudeCode{homeDir: "/home/user"}
	out, err := a.AddMCPServer(nil)
	if err != nil {
		t.Fatalf("AddMCPServer: %v", err)
	}
	if !a.HasMCPServer(out) {
		t.Fatalf("entry missing:\n%s", out)
	}
}

func TestClaudeCodeAddMCPServerRejectsNonObject(t *testing.T) {
	a := &ClaudeCode{homeDir: "/home/user"}
	if _, err := a.AddMCPServer([]byte(`{"mcpServers": []}`)); err == nil {
		t.Fatal("expected error when mcpServers is not an object")
	}
	if _, err := a.AddMCPServer([]byte(`{not json`)); err == nil {
		t.Fatal("expected error for invalid JSON")
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
