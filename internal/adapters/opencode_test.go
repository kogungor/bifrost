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
	if a.CommandsDir() != "/home/user/.config/opencode/commands" {
		t.Errorf("unexpected commands dir: %s", a.CommandsDir())
	}
	wantMCP := "/home/user/.config/opencode/opencode.json"
	if a.MCPConfigPath() != wantMCP {
		t.Errorf("MCPConfigPath() = %s; want %s", a.MCPConfigPath(), wantMCP)
	}
	if !filepath.IsAbs(a.MCPConfigPath()) {
		t.Errorf("MCPConfigPath() returned relative path: %s", a.MCPConfigPath())
	}
}

func TestOpenCodePathsHonorXDGConfigHome(t *testing.T) {
	a := &OpenCode{homeDir: "/home/user", configHome: "/xdg"}
	if a.CommandsDir() != "/xdg/opencode/commands" {
		t.Errorf("unexpected commands dir: %s", a.CommandsDir())
	}
	if a.MCPConfigPath() != "/xdg/opencode/opencode.json" {
		t.Errorf("unexpected MCP config: %s", a.MCPConfigPath())
	}
}

func TestOpenCodeDetection(t *testing.T) {
	for _, dir := range []string{filepath.Join(".config", "opencode"), ".opencode"} {
		tmp := t.TempDir()
		a := &OpenCode{homeDir: tmp}
		if a.IsInstalled() {
			t.Fatalf("should not be installed without %s", dir)
		}
		os.MkdirAll(filepath.Join(tmp, dir), 0755)
		if !a.IsInstalled() {
			t.Errorf("should be installed with %s", dir)
		}
	}
}

func TestOpenCodeAddMCPServer(t *testing.T) {
	a := &OpenCode{homeDir: "/home/user"}
	config := map[string]any{"theme": "dark"}
	if err := a.AddMCPServer(config); err != nil {
		t.Fatalf("AddMCPServer: %v", err)
	}
	if !a.HasMCPServer(config) {
		t.Fatal("HasMCPServer after AddMCPServer")
	}
	entry := config["mcp"].(map[string]any)["bifrost"].(map[string]any)
	if entry["type"] != "local" || entry["enabled"] != true {
		t.Errorf("unexpected entry: %v", entry)
	}
	command := entry["command"].([]string)
	if len(command) != 2 || command[0] != "bifrost" || command[1] != "mcp-serve" {
		t.Errorf("unexpected command: %v", command)
	}
	if _, ok := config["mcpServers"]; ok {
		t.Error("OpenCode config must not use the mcpServers key")
	}
}

func TestOpenCodeAddMCPServerNestedServers(t *testing.T) {
	a := &OpenCode{homeDir: "/home/user"}
	config := map[string]any{"mcp": map[string]any{"servers": map[string]any{}}}
	if err := a.AddMCPServer(config); err != nil {
		t.Fatalf("AddMCPServer: %v", err)
	}
	mcp := config["mcp"].(map[string]any)
	if _, ok := mcp["bifrost"]; ok {
		t.Error("entry written beside servers instead of inside it")
	}
	if !hasChild(config, "mcp", "servers", "bifrost") {
		t.Error("entry missing from mcp.servers")
	}
	if !a.HasMCPServer(config) {
		t.Error("HasMCPServer should find nested entry")
	}
}
