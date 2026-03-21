package adapters

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCodeName(t *testing.T) {
	a := &OpenCode{homeDir: "/tmp/fake"}
	if a.Name() != "opencode" {
		t.Errorf("expected opencode, got %s", a.Name())
	}
	if a.DisplayName() != "OpenCode" {
		t.Errorf("expected OpenCode, got %s", a.DisplayName())
	}
	if a.InstructionFile() != "AGENTS.md" {
		t.Errorf("expected AGENTS.md, got %s", a.InstructionFile())
	}
}

func TestOpenCodePaths(t *testing.T) {
	a := &OpenCode{homeDir: "/home/user"}
	if a.CommandsDir() != "/home/user/.opencode/commands" {
		t.Errorf("unexpected commands dir: %s", a.CommandsDir())
	}
	if a.MCPConfigPath() != "opencode.json" {
		t.Errorf("unexpected MCP config: %s", a.MCPConfigPath())
	}
}

func TestOpenCodeDetection(t *testing.T) {
	tmp := t.TempDir()

	// Not installed
	a := &OpenCode{homeDir: tmp}
	if a.IsInstalled() {
		t.Error("should not be installed without .opencode dir")
	}

	// Installed
	os.Mkdir(filepath.Join(tmp, ".opencode"), 0755)
	if !a.IsInstalled() {
		t.Error("should be installed with .opencode dir")
	}
}
