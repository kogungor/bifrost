package adapters

import (
	"os"
	"path/filepath"
)

// OpenCode implements the Adapter interface for OpenCode.
type OpenCode struct {
	homeDir    string
	configHome string // $XDG_CONFIG_HOME; empty means ~/.config
}

func newOpenCode() *OpenCode {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return &OpenCode{homeDir: home, configHome: os.Getenv("XDG_CONFIG_HOME")}
}

func (a *OpenCode) Name() string        { return "opencode" }
func (a *OpenCode) DisplayName() string { return "OpenCode" }

// configDir is OpenCode's global config directory, ~/.config/opencode.
func (a *OpenCode) configDir() string {
	base := a.configHome
	if base == "" {
		base = filepath.Join(a.homeDir, ".config")
	}
	return filepath.Join(base, "opencode")
}

// IsInstalled detects the global config directory, or ~/.opencode where
// OpenCode's install script puts the binary.
func (a *OpenCode) IsInstalled() bool {
	for _, dir := range []string{a.configDir(), filepath.Join(a.homeDir, ".opencode")} {
		if _, err := os.Stat(dir); err == nil {
			return true
		}
	}
	return false
}

func (a *OpenCode) CommandsDir() string {
	return filepath.Join(a.configDir(), "commands")
}

func (a *OpenCode) MCPConfigPath() string {
	return filepath.Join(a.configDir(), "opencode.json")
}

func (a *OpenCode) InstructionFile() string { return "AGENTS.md" }

func (a *OpenCode) CommandPath(name string) string {
	return markdownCommandPath(a.CommandsDir(), name)
}

// AddMCPServer writes a local server under "mcp", the documented layout.
// Configs already using the nested "mcp.servers" layout get it there instead.
func (a *OpenCode) AddMCPServer(data []byte) ([]byte, error) {
	config, err := parseJSONConfig(data)
	if err != nil {
		return nil, err
	}
	mcp, err := childObject(config, "mcp")
	if err != nil {
		return nil, err
	}
	command := []string{"bifrost", "mcp-serve"}
	if servers, ok := mcp["servers"].(map[string]any); ok {
		servers["bifrost"] = map[string]any{"type": "local", "command": command}
	} else {
		mcp["bifrost"] = map[string]any{"type": "local", "command": command, "enabled": true}
	}
	return encodeJSONConfig(config)
}

func (a *OpenCode) HasMCPServer(data []byte) bool {
	config, err := parseJSONConfig(data)
	if err != nil {
		return false
	}
	return hasChild(config, "mcp", "bifrost") || hasChild(config, "mcp", "servers", "bifrost")
}

// LegacyPaths: Bifrost <= 1.3 installed commands under ~/.opencode and wrote
// an "mcpServers" key that OpenCode ignores.
func (a *OpenCode) LegacyPaths() []string {
	return []string{
		filepath.Join(a.homeDir, ".opencode", "commands"),
		filepath.Join(a.homeDir, ".opencode", "opencode.json"),
	}
}
