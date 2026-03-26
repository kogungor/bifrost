# Changelog

## [0.7.0] — 2026-03-26

### Added

- **Semantic snapshot fields** — `session_intent`, `assumptions`, `open_questions`, `risks`, `active_plan_name`, `git_sha`, `session_start` on the Snapshot struct. All optional, all backward-compatible with v1 snapshots.
- **Confidence on active files** — `[confidence: high|medium|low]` suffix on each active file entry signals how certain the previous AI was about the file's state.
- **Stable plan step IDs** — each `PlanStep` now has a stable `id` (8-char hex) assigned on creation and preserved through status updates and reorders.
- **`bifrost export` command** — exports the current snapshot and/or plans as JSON to stdout for CI scripts and external tooling. Flags: `--format snapshot|plans|all`.
- **Enriched `bifrost status`** — shows `Intent`, `Active plan`, and `Open questions` count when set.
- **Enriched `bifrost_status` MCP tool** — returns `session_intent`, `active_plan`, and `open_question_count`.
- **MCP check in `bifrost doctor`** — warns when the MCP server is not registered, with the fix command.

### Changed

- **`/handoff` prefers MCP when available** — calls `bifrost_write_snapshot` and `bifrost_write_note` for reliable atomic writes, auto-archiving, and git SHA collection. Falls back to direct file write when MCP is not configured.
- **`/handin` prefers MCP when available** — calls `bifrost_read_snapshot` for structured access to all fields. Falls back to reading `.bifrost/session.md` directly when MCP is not configured. MCP is no longer required.
- **`/handin` enriched briefing** — shows `Intent`, `Commit` (git SHA), file confidence, `Assumptions (not verified)`, `Risks`, active plan progress, and `Open questions` before the confirmation prompt.
- **`bifrost_write_snapshot` MCP tool** — accepts all new semantic fields; validates `session_intent` and `confidence` enums; auto-collects git SHA via `git rev-parse HEAD` with 2s timeout.
- **`bifrost_read_snapshot` MCP tool** — exposes all new fields including `confidence` on active files.
- **`bifrost_read_plan` MCP tool** — exposes step `id` on each step.
- **Snapshot `bifrost_version` bumped to 2.** v1 snapshots continue to parse cleanly.
- **Plan `bifrost_version` now uses `CurrentVersion`** — previously hardcoded to 1.

### Fixed

- `confidence` enum validation in `bifrost_write_snapshot` — previously accepted any string; now rejects values outside `high|medium|low`.
- `bifrost status` CLI now shows semantic fields (`Intent`, `Active plan`, `Open questions`) — previously only shown in MCP `bifrost_status`.

---

## [0.5.0] — 2025-03-18

- Brew cask distribution

## [0.4.1] — 2025-03-15

- Brew cask update

## [0.4.0] — 2025-03-14

- Plan review workflow (`/review`)
- `bifrost_update_plan` MCP tool

## [0.3.0] — 2025-03-10

- Implementation plans (`/plan`, `bifrost_write_plan`, `bifrost_read_plan`, `bifrost_list_plans`, `bifrost_delete_plan`)

## [0.2.0] — 2025-03-05

- MCP server mode (`bifrost mcp-serve`, `bifrost install --mcp`)
- `bifrost_read_snapshot`, `bifrost_write_snapshot`, `bifrost_write_note`, `bifrost_status`

## [0.1.0] — 2025-03-01

- Initial release
- `/handoff` and `/handin` for Claude Code and OpenCode
- `bifrost install`, `init`, `status`, `doctor`, `history`, `restore`
- Homebrew + curl install
