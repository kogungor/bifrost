# Changelog

## [0.8.0] — 2026-03-28

### Added

- **`bifrost doctor --fix`** — automatically fixes detected issues: registers missing slash commands, adds `.bifrost/` to `.gitignore`, and creates a `BIFROST.md` template. Issues that cannot be fixed automatically are still reported with manual instructions. MCP registration is also attempted when not configured.
- **Snapshot history retention limit** — `.bifrost/history/` is automatically pruned to the 50 most recent entries after each `/handoff`. Oldest snapshots are removed first. The limit is exposed as `DefaultMaxHistory = 50` in the snapshot package.
- **Snapshot size indicator in `bifrost status`** — shows snapshot size in KB. Warns at >5 KB ("consider trimming") and >10 KB ("large snapshot, consider trimming decisions or environment notes").
- **Plan consensus mechanism** — two-party approval flow before a plan becomes active. Reviewer submits `approved` or `needs_revision` via `/review`; planner revises with `/plan --revise`; deadlock auto-detected after `max_revisions` (default 3); force-accept escape hatch via `/plan --force-accept`.
- **`plan_version` tracking** — increments on every content edit; each review note is tied to the plan version it reviewed. Editing an approved plan automatically resets consensus and returns to draft.
- **Deadlock detection** — automatic when `revision_count >= max_revisions` with unresolved `needs_revision`. Returns `deadlock_detected: true` and `deadlock_reason` in response.
- **Reviewer identity** — `source_tool` param on `bifrost_update_plan`; review notes now carry `from`, `at`, `plan_version`, and `outcome`.
- **`bifrost_list_plans`** — now returns `consensus_state`, `revision_count`, `deadlock_detected`, `latest_review_outcome` per plan.
- **`bifrost export`** — plan export now includes all consensus fields and enriched review notes.

### Fixed

- **Plan lock conflict now reports clearly** — previously, `WritePlan` would silently force-remove an active lock after retries and proceed. Now returns an explicit error: `"plan is locked by another process"` with the lock path for manual removal. Stale locks (>30s) are still cleared automatically.

### Changed

- **`/review` command** — uses `review_outcome: approved | needs_revision` instead of adding raw notes and auto-setting status. Plan activation is driven by consensus, not the reviewer directly.
- **`/plan` command** — new `--revise` mode (address feedback, increment version) and `--force-accept` mode (override deadlock).
- **`bifrost_update_plan` MCP tool** — new params: `source_tool`, `review_outcome`, `review_feedback`, `force_accept`, `revise`. JSON schema updated.

---

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
