package adapters

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/BurntSushi/toml"
)

// codexMCPTable is what `codex mcp add bifrost -- bifrost mcp-serve` writes.
const codexMCPTable = "[mcp_servers.bifrost]\ncommand = \"bifrost\"\nargs = [\"mcp-serve\"]\n"

// inlineMCPServers matches `mcp_servers = { ... }`. TOML forbids extending an
// inline table with a later [mcp_servers.bifrost] header, and the TOML parser
// used here does not enforce that, so it is checked separately.
var inlineMCPServers = regexp.MustCompile(`(?m)^[ \t]*mcp_servers[ \t]*=`)

// Codex implements the Adapter interface for the OpenAI Codex CLI. Codex
// replaced custom prompts with skills, so Bifrost's commands install as
// skills invoked with $bifrost-handoff, $bifrost-handin, and so on.
type Codex struct {
	homeDir   string
	codexHome string // $CODEX_HOME; empty means ~/.codex
}

func newCodex() *Codex {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return &Codex{homeDir: home, codexHome: os.Getenv("CODEX_HOME")}
}

func (a *Codex) Name() string        { return "codex" }
func (a *Codex) DisplayName() string { return "Codex" }

// configDir is Codex's home directory, ~/.codex unless $CODEX_HOME is set.
func (a *Codex) configDir() string {
	if a.codexHome != "" {
		return a.codexHome
	}
	return filepath.Join(a.homeDir, ".codex")
}

func (a *Codex) IsInstalled() bool {
	_, err := os.Stat(a.configDir())
	return err == nil
}

// CommandsDir is the user-level Agent Skills directory Codex reads.
func (a *Codex) CommandsDir() string {
	return filepath.Join(a.homeDir, ".agents", "skills")
}

// CommandPath gives each command its own skill directory, named to match the
// skill's frontmatter name as the Agent Skills format requires.
func (a *Codex) CommandPath(name string) string {
	return filepath.Join(a.CommandsDir(), "bifrost-"+name, "SKILL.md")
}

func (a *Codex) MCPConfigPath() string {
	return filepath.Join(a.configDir(), "config.toml")
}

func (a *Codex) InstructionFile() string { return "AGENTS.md" }

// AddMCPServer appends a [mcp_servers.bifrost] table. The existing file,
// comments and formatting included, is kept byte for byte.
func (a *Codex) AddMCPServer(data []byte) ([]byte, error) {
	config, err := parseTOMLConfig(data)
	if err != nil {
		return nil, err
	}
	if hasChild(config, "mcp_servers", "bifrost") {
		return data, nil
	}
	if inlineMCPServers.Match(data) {
		return nil, fmt.Errorf("mcp_servers is an inline table; run `codex mcp add bifrost -- bifrost mcp-serve` instead")
	}
	out := append([]byte{}, data...)
	if len(bytes.TrimSpace(out)) > 0 {
		if !bytes.HasSuffix(out, []byte("\n")) {
			out = append(out, '\n')
		}
		out = append(out, '\n')
	}
	out = append(out, codexMCPTable...)
	// Appending a table fails if mcp_servers is an inline table; check the result.
	if _, err := parseTOMLConfig(out); err != nil {
		return nil, fmt.Errorf("cannot append [mcp_servers.bifrost] (%v); run `codex mcp add bifrost -- bifrost mcp-serve` instead", err)
	}
	return out, nil
}

func (a *Codex) HasMCPServer(data []byte) bool {
	config, err := parseTOMLConfig(data)
	return err == nil && hasChild(config, "mcp_servers", "bifrost")
}

func (a *Codex) LegacyPaths() []string { return nil }

func parseTOMLConfig(data []byte) (map[string]any, error) {
	config := make(map[string]any)
	if _, err := toml.Decode(string(data), &config); err != nil {
		return nil, fmt.Errorf("parse TOML: %w", err)
	}
	return config, nil
}
