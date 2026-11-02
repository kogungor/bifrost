package adapters

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodexPaths(t *testing.T) {
	a := &Codex{homeDir: "/home/user"}
	if a.Name() != "codex" || a.DisplayName() != "Codex" || a.InstructionFile() != "AGENTS.md" {
		t.Errorf("unexpected identity: %s %s %s", a.Name(), a.DisplayName(), a.InstructionFile())
	}
	if a.CommandPath("handin") != "/home/user/.agents/skills/bifrost-handin/SKILL.md" {
		t.Errorf("unexpected command path: %s", a.CommandPath("handin"))
	}
	if a.MCPConfigPath() != "/home/user/.codex/config.toml" {
		t.Errorf("unexpected MCP config: %s", a.MCPConfigPath())
	}

	a = &Codex{homeDir: "/home/user", codexHome: "/opt/codex"}
	if a.MCPConfigPath() != "/opt/codex/config.toml" {
		t.Errorf("CODEX_HOME not honored: %s", a.MCPConfigPath())
	}
}

func TestCodexDetection(t *testing.T) {
	tmp := t.TempDir()
	a := &Codex{homeDir: tmp}
	if a.IsInstalled() {
		t.Fatal("should not be installed without ~/.codex")
	}
	os.MkdirAll(filepath.Join(tmp, ".codex"), 0755)
	if !a.IsInstalled() {
		t.Error("should be installed with ~/.codex")
	}
}

func TestCodexAddMCPServerKeepsExistingTOML(t *testing.T) {
	a := &Codex{homeDir: "/home/user"}
	in := []byte("# my settings\nmodel = \"gpt-5\"\n\n[mcp_servers.other]\ncommand = \"other\"")
	out, err := a.AddMCPServer(in)
	if err != nil {
		t.Fatalf("AddMCPServer: %v", err)
	}
	if !strings.HasPrefix(string(out), string(in)) {
		t.Fatalf("existing content was not kept byte for byte:\n%s", out)
	}
	if !strings.HasSuffix(string(out), codexMCPTable) {
		t.Fatalf("bifrost table not appended:\n%s", out)
	}
	if !a.HasMCPServer(out) {
		t.Fatal("HasMCPServer after AddMCPServer")
	}

	again, err := a.AddMCPServer(out)
	if err != nil {
		t.Fatalf("second AddMCPServer: %v", err)
	}
	if string(again) != string(out) {
		t.Fatalf("AddMCPServer should be idempotent:\n%s", again)
	}
}

func TestCodexAddMCPServerToMissingFile(t *testing.T) {
	a := &Codex{homeDir: "/home/user"}
	out, err := a.AddMCPServer(nil)
	if err != nil {
		t.Fatalf("AddMCPServer: %v", err)
	}
	if string(out) != codexMCPTable {
		t.Fatalf("unexpected new file:\n%s", out)
	}
}

func TestCodexAddMCPServerRefusesToBreakConfig(t *testing.T) {
	a := &Codex{homeDir: "/home/user"}
	if _, err := a.AddMCPServer([]byte("mcp_servers = { other = { command = \"x\" } }\n")); err == nil {
		t.Fatal("appending a table to an inline mcp_servers table must fail")
	}
	if _, err := a.AddMCPServer([]byte("not = [valid")); err == nil {
		t.Fatal("invalid TOML must be rejected")
	}
}

func TestCodexDetectsExistingEntryInAnyForm(t *testing.T) {
	a := &Codex{homeDir: "/home/user"}
	for _, in := range []string{
		"[mcp_servers.bifrost]\ncommand = \"bifrost\"\n",
		"[mcp_servers]\nbifrost = { command = \"bifrost\" }\n",
		"[mcp_servers.\"bifrost\"]\ncommand = \"bifrost\"\n",
	} {
		if !a.HasMCPServer([]byte(in)) {
			t.Errorf("entry not detected in:\n%s", in)
		}
	}
}
