# Bifrost Integrity Pack Implementation Notes

These notes record current behavior before the Integrity Pack changes begin.
Use this as the baseline for backward compatibility checks.

## Phase 0.1 - Baseline Audit

Date: 2026-06-18
Branch: `dev`

### Current Format Boundary

- Runtime snapshot canonical file today: `.bifrost/session.md`
- Runtime plan canonical files today: `.bifrost/*.plan.md`
- JSON exists as an export/MCP response shape, not as persisted canonical state.
- `snapshot.CurrentVersion` is `2`, but this means the current Markdown snapshot
  format version. It is not the same as the proposed `schema_version:
  "snapshot.v2"` JSON schema.

### Snapshot Model

Source files:

- `internal/snapshot/snapshot.go`
- `internal/snapshot/parse.go`
- `internal/snapshot/archive.go`

Current snapshot fields:

- frontmatter: `bifrost_version`, `timestamp`, `source_tool`, `project`,
  `token_pressure`, optional `session_intent`, `active_plan_name`, `git_sha`,
  `session_start`
- body sections: `Current Task`, `Status`, `Active Files`, `Decisions Made`,
  `Environment Notes`, `Next Step`, optional `Assumptions`, `Open Questions`,
  `Risks`
- active file trust signal: optional `confidence` string with values
  `high|medium|low`

Read path:

- `snapshot.Read(projectRoot)` reads `.bifrost/session.md`
- `snapshot.Read` calls `snapshot.Parse`
- `snapshot.Parse` requires YAML frontmatter and parses Markdown sections

Write path:

- `snapshot.Write(projectRoot, snap)` ensures `.bifrost/` and
  `.bifrost/history/`
- if a current snapshot exists, it archives it first
- archive filename is based on the parsed snapshot timestamp
- history pruning keeps `snapshot.DefaultMaxHistory`, currently `50`
- `snapshot.Render` renders Markdown
- write is atomic via `.tmp` then rename

History and restore:

- `snapshot.History` reads `.bifrost/history/*.md`, parses Markdown, and sorts
  newest first
- `snapshot.Restore` archives the current snapshot, renders the selected history
  snapshot, and writes it back to `.bifrost/session.md`

Important compatibility constraint:

- Any JSON-backed implementation must keep `snapshot.Read` working when only
  `.bifrost/session.md` exists.
- Any future write path must preserve `.bifrost/session.md` as a readable
  artifact unless intentionally gated by a versioned migration.

### Plan Model

Source file:

- `internal/snapshot/plan.go`

Current plan fields:

- frontmatter: `bifrost_version`, `created_at`, `updated_at`, `source_tool`,
  `project`, `status`, optional consensus fields
- body sections: title heading, `Goal`, `Steps`, `Constraints`,
  `Review Notes`
- steps have stable IDs, description, status, and file list
- current step statuses are `pending`, `done`, and `blocked`
- review consensus fields include `plan_version`, `proposed_by`,
  `max_revisions`, `revision_count`, `consensus_state`,
  `activation_reason`, `deadlock_detected`, and `deadlock_reason`

Read path:

- `snapshot.ReadPlan(projectRoot, name)` reads `.bifrost/<name>.plan.md`
- `snapshot.ReadPlan` calls `snapshot.ParsePlan`
- legacy `timestamp` frontmatter is still supported for old plans

Write path:

- `snapshot.WritePlan(projectRoot, name, plan)` writes
  `.bifrost/<name>.plan.md`
- writes are guarded by `.bifrost/<name>.plan.lock`
- stale locks older than 30 seconds are removed
- active locks return a clear error
- missing step IDs are generated before write
- write is atomic via `.tmp` then rename

Important compatibility constraint:

- Plan JSON must not remove or invalidate existing `.plan.md` files.
- Existing step IDs must be preserved across migration.
- Existing `pending|done|blocked` statuses need an explicit mapping before
  introducing `claimed_done` and `verified_done`.

### MCP Baseline

Source files:

- `internal/mcp/server.go`
- `internal/mcp/tools.go`
- `internal/cli/mcp.go`

Current tools:

- `bifrost_read_snapshot`
- `bifrost_write_snapshot`
- `bifrost_write_note`
- `bifrost_status`
- `bifrost_read_plan`
- `bifrost_write_plan`
- `bifrost_update_plan`
- `bifrost_delete_plan`
- `bifrost_list_plans`

MCP snapshot behavior:

- `bifrost_read_snapshot` reads the Markdown snapshot through
  `snapshot.Read`
- response includes `age_seconds`, the structured snapshot fields, and optional
  handoff note
- `bifrost_write_snapshot` validates required fields, array sizes,
  `session_intent`, active file paths, and active file `confidence`
- `bifrost_write_snapshot` auto-fills timestamp, project name, and git SHA
- `bifrost_write_snapshot` writes through `snapshot.Write`

MCP plan behavior:

- plan tools read and write Markdown plans through `snapshot.ReadPlan` and
  `snapshot.WritePlan`
- `bifrost_update_plan` supports review consensus, force accept, revise,
  lifecycle status updates, legacy review notes, and step updates by index
- step updates currently accept `pending|done|blocked`

Important compatibility constraint:

- Existing MCP tool names and accepted input fields are public API.
- New evidence, JSON, verify, or trust fields should be additive unless a new
  versioned tool is introduced.

### CLI Baseline

Registered commands:

- `install`
- `init`
- `restore`
- `doctor`
- `status`
- `history`
- hidden `mcp-serve`
- `export`
- `update`
- `version`
- `completion`

Current missing Integrity Pack commands:

- `validate`
- `migrate`
- `render`
- `verify`
- `brief`
- `scrub`
- `evidence`
- `snapshot --enrich`
- `context`
- `promote`
- `diff`
- `timeline`
- `plan status|next|verify|step`

Baseline command outputs from this repo:

- `bifrost status` finds `.bifrost/session.md`, reports it as 82 days old,
  shows size, intent, handoff note, no history, and no `BIFROST.md`
- `bifrost export --format snapshot` emits JSON derived from the Markdown
  snapshot and handoff note
- `bifrost export --format plans` emits an empty `plans` array in the current
  repo state
- `bifrost doctor` reports registered Claude Code and OpenCode commands, missing
  optional MCP registrations, no `BIFROST.md`, stale snapshot warning, and
  `.bifrost/` covered by `.gitignore`; exit is successful
- MCP `tools/list` returns 9 tools

Commands used for baseline:

```bash
go run ./cmd/bifrost --no-color --project . status
go run ./cmd/bifrost --no-color --project . export --format snapshot
go run ./cmd/bifrost --no-color --project . export --format plans
go run ./cmd/bifrost --no-color --project . doctor
printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' \
  | go run ./cmd/bifrost --project . mcp-serve
```

### Slash Command Baseline

Source files:

- `cmd/bifrost/commands/claude-code/handoff.md`
- `cmd/bifrost/commands/claude-code/handin.md`
- `cmd/bifrost/commands/claude-code/plan.md`
- `cmd/bifrost/commands/claude-code/review.md`
- matching OpenCode files under `cmd/bifrost/commands/opencode/`

Current behavior:

- `/handoff` prefers MCP `bifrost_write_snapshot`; direct file write is fallback
- `/handoff` writes `.bifrost/session.md` and `.bifrost/handoff.md`
- `/handin` prefers MCP `bifrost_read_snapshot`; direct file read is fallback
- `/handin` reads `BIFROST.md` if present
- `/handin` warns on old snapshots by age only
- `/handin` can load active plan by name
- `/handin` surfaces open questions and waits for user confirmation
- `/plan` creates, revises, or force-accepts plans through MCP
- `/review` participates in the existing consensus flow

Important compatibility constraint:

- `/handoff`, `/handin`, `/plan`, and `/review` should remain the primary simple
  UX. Integrity Pack details should be surfaced through CLI/MCP internals and
  optional flags such as `/handin --verify`.
