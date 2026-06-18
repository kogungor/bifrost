# Contributing to Bifrost

Thanks for contributing. Bifrost is a local, trust-aware handoff layer for AI
coding agents. The core product goal is to move context across tools without
making the next agent blindly trust model-written summaries.

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

```text
cmd/bifrost/           Entry point; embeds slash command files
internal/
  adapters/            AI tool adapters (Claude Code, OpenCode)
  cli/                 Cobra commands and CLI integration surface
  mcp/                 Dependency-free stdio JSON-RPC MCP server
  project/             Project detection, gitignore, BIFROST.md analyzer/promotion
  security/            Deterministic secret scanner, redaction, allowlist handling
  snapshot/            Snapshot/plan models, render/parse, JSON v2 schema, collectors,
                       evidence, trust, verify, brief, diff, timeline, history
  ui/                  Terminal output helpers (colors, formatting)
cmd/bifrost/commands/
  claude-code/         /handoff, /handin, /plan, /review for Claude Code
  opencode/            /handoff, /handin, /plan, /review for OpenCode
```

## State Model

Bifrost keeps backward-compatible Markdown files for humans and existing
workflows, while JSON v2 files carry canonical integrity state.

Important paths:

```text
BIFROST.md                         Durable project context, committed to git
.bifrost/session.md                Human-readable active snapshot
.bifrost/session.json              Canonical snapshot.v2 state
.bifrost/history/                  Archived Markdown and JSON snapshots
.bifrost/evidence/                 Structured evidence records
.bifrost/plans/<name>.json         Canonical plan.v2 state
.bifrost/<name>.plan.md            Human-readable plan compatibility file
.bifrost/timeline.jsonl            Local snapshot/verify/plan event history
.bifrost/config.json               Optional local runtime config
```

Rules:

- Preserve existing `.bifrost/session.md` and `.bifrost/*.plan.md` behavior.
- Prefer JSON v2 for machine-readable state and verification.
- Keep Markdown rendering human-readable and backward-compatible.
- Do not store full file contents, full command logs, conversation history, or
  generated artifacts.
- Do not write secret-like values; use `internal/security` before adding new
  write paths for Bifrost state.

## Running Tests

Use this before opening a PR:

```bash
go test ./... -count=1
go vet ./...
git diff --check
```

For changes touching file writes, history, plan execution, CLI state, or shared
snapshot logic, also run:

```bash
go test -race ./...
```

Useful targeted commands:

```bash
go test ./internal/snapshot -v
go test ./internal/cli -v
go test ./internal/security -v
go test ./internal/mcp -v
```

The CLI integration tests build the `bifrost` binary and run end-to-end commands
with an isolated temp `HOME`.

## Common Development Areas

### Snapshot And Integrity State

Core files:

- `internal/snapshot/schema_v2.go` — `snapshot.v2` and `plan.v2` models,
  validation, JSON read/write
- `internal/snapshot/convert_v2.go` — legacy Markdown compatibility conversion
- `internal/snapshot/collectors.go` — observed git/file/project facts
- `internal/snapshot/evidence.go` — evidence records
- `internal/snapshot/trust.go` — trust downgrade rules and dimensions
- `internal/snapshot/verify.go` — freshness/evidence/security checks
- `internal/snapshot/brief.go` — mode-aware compact briefing
- `internal/snapshot/diff.go` and `timeline.go` — local change tracking

When adding fields:

- Version behavior explicitly.
- Validate enums, IDs, timestamps, and required fields.
- Preserve unknown JSON fields where practical.
- Add render/migration coverage if Markdown output changes.

### CLI Commands

CLI commands live in `internal/cli`. Current important surfaces include:

```text
validate, migrate, render
snapshot --enrich
evidence list/show
verify
brief
scrub
plan status/next/verify/step
context check/update
promote
diff
timeline
history/restore
doctor
```

Guidelines:

- Use `resolveProject()` for project root detection.
- Use `ui.*` for user-facing output.
- Keep slash-command UX simple; put advanced functionality in CLI/MCP internals.
- Errors should be actionable: include what failed and a safe next command when
  possible.

### Security

Secret safety is deterministic, not left to model instructions.

- Use `internal/security.RedactString` or `ScanString` before writing new local
  state that may include model/user-provided text.
- Strict mode should fail writes before modifying existing state.
- Never print raw secret values in CLI output.
- Add corpus tests for new detector formats.

### Plans

Plans have both compatibility Markdown and canonical JSON state.

- Step status and verification state are separate.
- `claimed_done` means the model/user says it is done.
- `verified_done` means verification evidence exists or the user explicitly
  marked it verified.
- Plan execution changes should preserve legacy `.plan.md` readability.

### BIFROST.md Context

`BIFROST.md` is durable project context and may be committed to git.

- `bifrost context check` detects missing/stale durable context.
- `bifrost promote` requires explicit acceptance before editing `BIFROST.md`.
- Do not auto-overwrite durable project context without user intent.

## Adding a New Adapter

Adapters are intentionally separate from core integrity logic.

1. Create `internal/adapters/<name>.go` implementing the `Adapter` interface:

```go
type Adapter interface {
	Name() string            // "my-tool"
	DisplayName() string     // "My Tool"
	IsInstalled() bool       // detect installation
	CommandsDir() string     // where slash commands go
	MCPConfigPath() string   // MCP config path (empty if N/A)
	InstructionFile() string // "INSTRUCTIONS.md"
}
```

2. Register it in `internal/adapters/registry.go` via `All()`.
3. Add slash commands in:

```text
cmd/bifrost/commands/<name>/handoff.md
cmd/bifrost/commands/<name>/handin.md
cmd/bifrost/commands/<name>/plan.md
cmd/bifrost/commands/<name>/review.md
```

4. Add tests in `internal/adapters/<name>_test.go`.
5. Keep adapter work scoped; do not change snapshot integrity semantics unless
   the adapter truly requires it.

## MCP Server

The MCP server is intentionally dependency-free. Do not add an MCP library
unless there is a clear maintenance reason.

When changing MCP tools:

- Preserve existing client inputs when possible.
- Keep optional new fields optional.
- Add tests in `internal/mcp`.
- Keep local-first behavior; no network calls or telemetry.

## Documentation

Update README and command help when adding user-facing behavior.

README should answer:

- What problem does this solve?
- What files are written?
- What should the next AI trust, verify first, and not assume?
- What commands should users run?
- What is intentionally out of scope?

## Release Process

Releases are automated via GoReleaser on GitHub Actions:

1. Tag a version: `git tag v0.2.0`
2. Push the tag: `git push origin v0.2.0`
3. GitHub Actions builds cross-platform binaries and creates a release
4. The Homebrew cask is updated automatically

## Code Style

- `go test ./... -count=1` must pass.
- `go vet ./...` must pass.
- `git diff --check` must pass.
- Keep dependencies minimal. Current external dependencies are intentionally
  small.
- Use structured parsers/APIs over ad hoc string manipulation where practical.
- Prefer small, focused changes with tests over broad refactors.
- Preserve local-first behavior: no telemetry, cloud sync, or network dependency
  in core workflows.

## Commit Messages

Use concise conventional-style messages focused on the "why":

```text
Add evidence-backed snapshot verification
Fix JSON-backed restore to update active session state
```
