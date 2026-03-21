package adapters

// Adapter defines the integration contract for a single AI coding tool.
type Adapter interface {
	Name() string           // "claude-code"
	DisplayName() string    // "Claude Code"
	IsInstalled() bool      // detect installation
	CommandsDir() string    // ~/.claude/commands/
	MCPConfigPath() string  // ~/.claude/mcp.json (empty string if N/A)
	InstructionFile() string // "CLAUDE.md"
}
