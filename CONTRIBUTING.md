# Contributing to Bifrost

## Prerequisites

- Go 1.23+
- Git

## Getting Started

```bash
git clone https://github.com/kogungor/bifrost.git
cd bifrost
go build ./...
go test ./...
```

## Project Structure

```
cmd/bifrost/           Entry point, embeds slash command files
internal/
  cli/                 Cobra commands (install, init, doctor, status, history, etc.)
  snapshot/            Snapshot read/write/parse/archive logic + plan management
  project/             Project root detection, .gitignore management
  adapters/            AI tool adapters (Claude Code, OpenCode)
  mcp/                 MCP server (stdio JSON-RPC, 9 tools: 4 snapshot + 5 plan)
  ui/                  Terminal output helpers (colors, formatting)
commands/              Embedded slash command markdown files
  claude-code/         /handoff, /handin, /plan, /review for Claude Code
  opencode/            /handoff, /handin, /plan, /review for OpenCode
```

## Running Tests

```bash
# All tests
go test ./...

# Specific package
go test ./internal/mcp/ -v

# Integration tests (builds binary, runs CLI end-to-end)
go test ./internal/cli/ -v
```

## Adding a New Adapter

1. Create `internal/adapters/<name>.go` implementing the `Adapter` interface:

```go
type Adapter interface {
    Name() string           // "my-tool"
    DisplayName() string    // "My Tool"
    IsInstalled() bool      // detect installation
    CommandsDir() string    // where slash commands go
    MCPConfigPath() string  // MCP config path (empty if N/A)
    InstructionFile() string // "INSTRUCTIONS.md"
}
```

2. Register it in `internal/adapters/registry.go` → `All()`
3. Add slash commands in `commands/<name>/handoff.md`, `commands/<name>/handin.md`, `commands/<name>/plan.md`, and `commands/<name>/review.md`
4. Add tests in `internal/adapters/<name>_test.go`

## Adding a New CLI Command

1. Create `internal/cli/<command>.go` with a `cobra.Command`
2. Register via `init()` → `rootCmd.AddCommand()`
3. Use `resolveProject()` for project root, `ui.*` for output
4. Follow the pattern of existing commands (see `status.go` or `doctor.go`)

## Release Process

Releases are automated via GoReleaser on GitHub Actions:

1. Tag a version: `git tag v0.2.0`
2. Push the tag: `git push origin v0.2.0`
3. GitHub Actions builds cross-platform binaries and creates a release
4. The Homebrew cask is updated automatically

## Code Style

- `go vet ./...` must pass
- No external dependencies beyond cobra and yaml
- Keep the MCP server dependency-free (no MCP library)
- All output goes through `internal/ui` — never `fmt.Print` directly in commands
- Snapshot format is Markdown with YAML frontmatter — not JSON, not YAML-only

## Commit Messages

Use conventional-style messages focused on the "why":

```
Add MCP server for tool-call-based snapshot access
Fix snapshot archive collision when timestamps match
```
