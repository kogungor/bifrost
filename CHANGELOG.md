# Changelog

## [1.4.0] — 2026-11-02

### Added

- **Codex support** — `bifrost install` sets up Codex: the four commands install as Agent Skills in `~/.agents/skills/bifrost-<name>/SKILL.md` (invoked as `$bifrost-handoff`, `$bifrost-handin`, `$bifrost-plan`, `$bifrost-review`), and `--mcp` appends `[mcp_servers.bifrost]` to `~/.codex/config.toml` without touching the rest of the file. `$CODEX_HOME` is honored.
- **`bifrost plan verify` confirmation** — the command lists every step command before running it and asks first. `--dry-run` only lists them, `--yes` skips the prompt, and a non-interactive stdin is refused without `--yes`.

### Changed

- **Evidence trust follows its source** — command and test results Bifrost ran itself (`bifrost plan verify`) are recorded as `collector.command` and count as strong evidence. Results the previous agent reported stay `reported.command` and now count as medium. `bifrost verify` warns when a test-pass claim rests only on reported results. MCP callers can no longer supply `collector.*` sources.
- **README** — rewritten around the handoff workflow with real `bifrost brief` output; the full command, file, and MCP reference moved to `docs/reference.md`.

### Fixed

- **Claude Code MCP registration** — `bifrost install --mcp` wrote to `~/.claude/mcp.json`, which Claude Code does not read. It now writes a user-scope entry to `~/.claude.json`, preserving the rest of the file. `bifrost doctor` checks the same file, so it no longer reports a server that Claude Code never loads.
- **OpenCode paths and MCP schema** — slash commands now install to `~/.config/opencode/commands/`, and MCP registration writes `mcp.bifrost` (`type: local`, command array) to `~/.config/opencode/opencode.json`. The old `mcpServers` key was ignored by OpenCode. Both paths honor `$XDG_CONFIG_HOME`.
- **Legacy install files** — `bifrost doctor` lists Bifrost files left in the old locations so they can be removed. Run `bifrost install --mcp --force` after upgrading.
- **License** — added the MIT `LICENSE` file the README already referred to.
- **`claims.evidence` could never warn** — every status claim got an automatic `model_claim` record, and the check counted it as evidence, so briefings listed "Status claims have evidence references" under *Trust this*. Model claims no longer count; not-started items are not counted as unbacked claims.
- **Briefing freshness contradicted verify** — files that `bifrost verify` found changed or missing are now shown as `freshness=stale` in `bifrost brief`.
- **`bifrost verify` output** — the "Next action" label no longer runs into its value.
- **Dependencies** — `go.mod` is tidied; cobra and yaml.v3 were marked indirect.

### Removed

- **`Casks/bifrost.rb`** — a stale 0.5.0 copy; the published cask lives in `kogungor/homebrew-bifrost` and is updated by GoReleaser.

## [1.3.1] — 2026-06-19

### Added

- **Canonical snapshot v2 JSON** — `.bifrost/session.json` now carries machine-readable session state while `.bifrost/session.md` remains the human-readable and backward-compatible handoff artifact.
- **Schema validation, migration, and rendering** — `bifrost validate`, `bifrost migrate`, and `bifrost render` support v2 snapshots and plans without breaking legacy Markdown files.
- **Observed facts and evidence anchors** — snapshots can separate collected repo facts from model interpretation, with git/file/project evidence attached to active files and status claims.
- **Verification workflow** — `bifrost verify` and `/handin --verify` check snapshot age, branch and commit drift, active file freshness, claim evidence, unresolved risks/questions, plan health, and secret-like values.
- **Trust model v2** — active files and plan/status items can distinguish `claimed_done` from `verified_done` and report implementation, tests, security, architecture, freshness, and evidence trust dimensions.
- **Plan execution tracking** — plan JSON, step verification, plan health, `plan status`, `plan next`, `plan verify`, and `plan step` commands track claimed vs verified progress.
- **Secret scanner and scrubber** — deterministic redaction runs before snapshot writes; `bifrost scrub` and `bifrost doctor --security` inspect and clean local Bifrost state.
- **Mode-aware briefings** — `bifrost brief` generates compact implement/debug/review/plan/handoff summaries with omitted-context reporting.
- **Project context workflow** — `bifrost context` and `bifrost promote` analyze `BIFROST.md` and propose user-approved durable context updates.
- **Snapshot diff and timeline** — `bifrost diff`, `bifrost timeline`, and `restore --preview` expose local context changes without storing full file contents.

### Changed

- **`/handin` briefing model** — incoming agents now get explicit "Trust this", "Verify this first", and "Do not assume this" guidance when v2 state is available.
- **MCP snapshot tools** — snapshot read/write tools remain backward-compatible with legacy confidence fields while supporting v2 JSON, evidence, command results, and manual evidence.
- **Documentation and help text** — README, design docs, CLI help, and release metadata now describe Bifrost as a local context integrity layer, not only a session bridge.

---

## [0.9.0] — 2026-03-28

### Added

- **`bifrost doctor --fix`** — automatically fixes detected issues: registers missing slash commands, adds `.bifrost/` to `.gitignore`, and creates a `BIFROST.md` template. Issues that cannot be fixed automatically are still reported with manual instructions. MCP registration is also attempted when not configured.
- **Snapshot history retention limit** — `.bifrost/history/` is automatically pruned to the 50 most recent entries after each `/handoff`. Oldest snapshots are removed first. The limit is exposed as `DefaultMaxHistory = 50` in the snapshot package.
- **Snapshot size indicator in `bifrost status`** — shows snapshot size in KB. Warns at >5 KB ("consider trimming") and >10 KB ("large snapshot, consider trimming decisions or environment notes").

### Fixed

- **OpenCode MCPConfigPath** — now returns an absolute path; previously returned a relative path that broke MCP registration outside the home directory.
- **Plan lock conflict now reports clearly** — previously, `WritePlan` would silently force-remove an active lock after retries and proceed. Now returns an explicit error: `"plan is locked by another process"` with the lock path for manual removal. Stale locks (>30s) are still cleared automatically.

---

## [0.8.0] — 2026-03-27

### Added

- **Plan consensus mechanism** — two-party approval flow before a plan becomes active. Reviewer submits `approved` or `needs_revision` via `/review`; planner revises with `/plan --revise`; deadlock auto-detected after `max_revisions` (default 3); force-accept escape hatch via `/plan --force-accept`.
- **`plan_version` tracking** — increments on every content edit; each review note is tied to the plan version it reviewed. Editing an approved plan automatically resets consensus and returns to draft.
- **Deadlock detection** — automatic when `revision_count >= max_revisions` with unresolved `needs_revision`. Returns `deadlock_detected: true` and `deadlock_reason` in response.
- **Reviewer identity** — `source_tool` param on `bifrost_update_plan`; review notes now carry `from`, `at`, `plan_version`, and `outcome`.
- **`bifrost_list_plans`** — now returns `consensus_state`, `revision_count`, `deadlock_detected`, `latest_review_outcome` per plan.
- **`bifrost export`** — plan export now includes all consensus fields and enriched review notes.

### Changed

- **`/review` command** — uses `review_outcome: approved | needs_revision` instead of adding raw notes and auto-setting status. Plan activation is driven by consensus, not the reviewer directly.
- **`/plan` command** — new `--revise` mode (address feedback, increment version) and `--force-accept` mode (override deadlock).
- **`bifrost_update_plan` MCP tool** — new params: `source_tool`, `review_outcome`, `review_feedback`, `force_accept`, `revise`. JSON schema updated.

---

## [0.7.0] — 2026-03-27

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

## [0.5.0] — 2026-03-22

- Brew cask distribution

## [0.4.1] — 2026-03-21

- Brew cask update

## [0.4.0] — 2026-03-21

- Plan review workflow (`/review`)
- `bifrost_update_plan` MCP tool

## [0.3.0] — 2026-03-21

- Implementation plans (`/plan`, `bifrost_write_plan`, `bifrost_read_plan`, `bifrost_list_plans`, `bifrost_delete_plan`)

## [0.2.0] — 2026-03-21

- MCP server mode (`bifrost mcp-serve`, `bifrost install --mcp`)
- `bifrost_read_snapshot`, `bifrost_write_snapshot`, `bifrost_write_note`, `bifrost_status`

## [0.1.0] — 2026-03-21

- Initial release
- `/handoff` and `/handin` for Claude Code and OpenCode
- `bifrost install`, `init`, `status`, `doctor`, `history`, `restore`
- Homebrew + curl install
