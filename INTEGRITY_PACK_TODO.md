# Bifrost Integrity Pack TODO

This file tracks the implementation order for the Bifrost Integrity Pack. It is
based on the current repository state: Markdown snapshots and plans are the
canonical runtime format today, while JSON export exists only as a derived
interface.

## Principles

- [ ] Preserve existing `.bifrost/session.md` and `.bifrost/*.plan.md` behavior.
- [ ] Preserve existing MCP client inputs and outputs unless a change is
      explicitly versioned.
- [ ] Keep `/handoff`, `/handin`, `/plan`, and `/review` simple.
- [ ] Add power to CLI, MCP internals, and machine-readable files without
      breaking existing users.
- [ ] Treat observed repository facts separately from model-written summaries.
- [ ] Never store full file contents, full command logs, secrets, or conversation
      history in snapshots.
- [ ] Run `go test ./...` after every completed phase.

## Phase 0 - Baseline And Boundaries

- [x] Record current baseline behavior for:
  - [x] `bifrost status`
  - [x] `bifrost export --format snapshot`
  - [x] `bifrost export --format plans`
  - [x] `bifrost doctor`
  - [x] MCP `bifrost_read_snapshot`
  - [x] MCP `bifrost_write_snapshot`
  - [x] MCP plan tools
- [x] Add or update golden fixtures for current Markdown snapshot rendering.
- [x] Add or update golden fixtures for current Markdown plan rendering.
- [x] Identify all code paths that write `.bifrost/session.md`.
- [x] Identify all code paths that write `.bifrost/*.plan.md`.

## Phase 1 - JSON Schema Foundation

- [ ] Add `SnapshotV2` structs for canonical JSON state.
- [ ] Add `PlanV2` structs for canonical JSON plan state.
- [ ] Use explicit schema strings:
  - [ ] `snapshot.v2`
  - [ ] `plan.v2`
- [ ] Keep existing `Snapshot` and `Plan` structs as Markdown compatibility
      models until migration is complete.
- [ ] Add `session.json` path helpers.
- [ ] Add `.bifrost/plans/` path helpers for future plan JSON files.
- [ ] Add optional `.bifrost/evidence/` path helpers for structured evidence
      records.
- [ ] Add JSON read/write helpers with atomic writes.
- [ ] Preserve unknown JSON fields where practical.
- [ ] Add validation functions for required fields, enums, timestamps, and IDs.
- [ ] Add tests for valid and invalid snapshot JSON.
- [ ] Add tests for valid and invalid plan JSON.
- [ ] Add optional `.bifrost/config.json` model only if runtime configuration is
      needed.

## Phase 2 - Validate, Render, And Migrate CLI

- [ ] Add `bifrost validate`.
- [ ] Add `bifrost validate --snapshot <path>`.
- [ ] Add `bifrost validate --plan <path>`.
- [ ] Add actionable validation errors with field path, bad value, and expected
      value.
- [ ] Add `bifrost render` to render JSON state into backward-compatible
      Markdown.
- [ ] Add `bifrost migrate --dry-run`.
- [ ] Add `bifrost migrate` with non-destructive backup/history behavior.
- [ ] Ensure old `session.md` still works when `session.json` is absent.
- [ ] Ensure old `.plan.md` still works when plan JSON is absent.
- [ ] Ensure `bifrost status` keeps showing current fields.
- [ ] Ensure `bifrost export` supports both old Markdown-derived state and new
      JSON-derived state.
- [ ] Add CLI integration tests for validate/render/migrate.

## Phase 3 - Observed Facts And Evidence Anchors

- [ ] Add `Evidence` model with stable IDs.
- [ ] Add evidence types:
  - [ ] `git_status`
  - [ ] `file_metadata`
  - [ ] `diff_summary`
  - [ ] `project_metadata`
  - [ ] `command_result`
  - [ ] `test_result`
  - [ ] `manual_note`
  - [ ] `model_claim`
- [ ] Add `observed.git` collector:
  - [ ] branch
  - [ ] commit
  - [ ] dirty state
  - [ ] changed files
  - [ ] staged files
  - [ ] untracked files
- [ ] Add `observed.files` collector for active files:
  - [ ] exists
  - [ ] size
  - [ ] mtime
  - [ ] optional hash
- [ ] Add project metadata collector:
  - [ ] project root
  - [ ] `BIFROST.md` existence
  - [ ] package manager candidates
  - [ ] common static command candidates
- [ ] Add command evidence storage for reported command results without
      automatically running commands.
- [ ] Add `bifrost snapshot --enrich`.
- [ ] Add `bifrost evidence list`.
- [ ] Add `bifrost evidence show <id>`.
- [ ] Extend MCP snapshot read/write tools to accept optional evidence fields
      while preserving old inputs.
- [ ] Attach evidence refs to active files where possible.
- [ ] Mark claims without evidence as unverified or `model_claim`.
- [ ] Add tests using temporary git repositories.

## Phase 4 - Verify Engine

- [ ] Add `bifrost verify`.
- [ ] Add `bifrost verify --json`.
- [ ] Add `bifrost verify --strict`.
- [ ] Add `bifrost verify --fix` for safe, non-destructive fixes only.
- [ ] Add verification checks:
  - [ ] snapshot age
  - [ ] branch match
  - [ ] commit match
  - [ ] dirty tree changed
  - [ ] active files exist
  - [ ] active files changed
  - [ ] claims have evidence
  - [ ] test claims have command evidence
  - [ ] high severity open questions
  - [ ] high severity risks
  - [ ] active plan status
  - [ ] security secrets
- [ ] Define exit codes:
  - [ ] `0` pass or informational
  - [ ] `1` warnings in strict mode
  - [ ] `2` failure, security issue, or invalid schema
- [ ] Add human-readable verify report.
- [ ] Add machine-readable verify result.
- [ ] Ensure verify never runs destructive commands.
- [ ] Ensure verify recommended next action is specific and actionable.
- [ ] Add tests for stale commit, changed file, missing active file, and
      unevidenced test claims.

## Phase 5 - `/handin --verify`

- [ ] Update Claude Code `/handin` command to recognize `--verify`.
- [ ] Update OpenCode `/handin` command to recognize `--verify`.
- [ ] Prefer `bifrost verify --json` when CLI is available.
- [ ] Include verification summary at the start of the briefing.
- [ ] Include safe next action when context is stale or risky.
- [ ] Make stale or unverified claims explicit:
  - [ ] "Trust this"
  - [ ] "Verify this first"
  - [ ] "Do not assume this"
- [ ] Keep normal `/handin` behavior backward-compatible.
- [ ] Update slash command docs for:
  - [ ] `/plan <name> --next`
  - [ ] `/plan <name> --verify`

## Phase 6 - Trust Model V2

- [ ] Add trust dimensions:
  - [ ] implementation
  - [ ] tests
  - [ ] security
  - [ ] architecture
  - [ ] freshness
  - [ ] evidence
- [ ] Map legacy `confidence` into the new trust model.
- [ ] Add status states:
  - [ ] `not_started`
  - [ ] `in_progress`
  - [ ] `blocked`
  - [ ] `claimed_done`
  - [ ] `verified_done`
  - [ ] `invalidated`
- [ ] Add trust downgrade rules for missing evidence, stale files, missing test
      evidence, security-sensitive paths, and high severity risks.
- [ ] Render `claimed_done` distinctly from `verified_done`.
- [ ] Add tests for downgrade matrix and Markdown rendering.

## Phase 7 - Plan Execution Tracking

- [ ] Extend plan JSON with step verification fields.
- [ ] Preserve existing `.plan.md` step IDs.
- [ ] Add plan step status migration from `pending|done|blocked`.
- [ ] Add `bifrost plan status <name>`.
- [ ] Add `bifrost plan next <name>`.
- [ ] Add `bifrost plan verify <name>`.
- [ ] Add `bifrost plan step <name> <step-id> --claimed-done`.
- [ ] Add `bifrost plan step <name> <step-id> --verified-done`.
- [ ] Add `bifrost plan step <name> <step-id> --blocked "reason"`.
- [ ] Store verification command requirements per step.
- [ ] Store latest verification result with command, exit code, timestamp, and
      evidence ref.
- [ ] Add plan health score.
- [ ] Show active plan health in `/handin`.
- [ ] Add tests for plan migration, plan health, and next safest action.

## Phase 8 - Secret Scanner And Scrubber

- [ ] Add deterministic scanner package.
- [ ] Detect:
  - [ ] bearer tokens
  - [ ] JWT-like strings
  - [ ] private key blocks
  - [ ] database URLs and connection strings
  - [ ] `.env` assignment values
  - [ ] GitHub token formats
  - [ ] OpenAI-style API keys
  - [ ] Anthropic-style API keys
  - [ ] high-entropy API-key-like strings
- [ ] Add redaction format such as `[REDACTED:bearer_token]`.
- [ ] Run scanner before snapshot write.
- [ ] Make `security.strict` capable of failing snapshot writes when configured.
- [ ] Add `bifrost scrub --check`.
- [ ] Add `bifrost scrub --write`.
- [ ] Add `bifrost scrub --history`.
- [ ] Add `bifrost doctor --security`.
- [ ] Add minimal allowlist config.
- [ ] Report allowlisted findings explicitly without leaking secret values.
- [ ] Add secret corpus tests and false-positive allowlist tests.

## Phase 9 - Mode-Aware Briefing

- [ ] Add `bifrost brief --mode implement`.
- [ ] Add `bifrost brief --mode debug`.
- [ ] Add `bifrost brief --mode review`.
- [ ] Add `bifrost brief --mode plan`.
- [ ] Add `bifrost brief --budget <chars>`.
- [ ] Add `bifrost brief --full`.
- [ ] Add `bifrost brief --json`.
- [ ] Implement deterministic compaction.
- [ ] Never omit high severity risks or open questions.
- [ ] List omitted context when budget removes lower-priority details.
- [ ] Make `/handin` use compact safe briefing by default.
- [ ] Add golden tests for each briefing mode.

## Phase 10 - BIFROST.md Analyzer And Promotion

- [ ] Add `bifrost context check`.
- [ ] Add `bifrost context update --dry-run`.
- [ ] Add `bifrost context update`.
- [ ] Add `bifrost promote`.
- [ ] Detect durable project context candidates:
  - [ ] package manager
  - [ ] test command
  - [ ] typecheck command
  - [ ] lint command
  - [ ] framework/library candidates
  - [ ] migration directory
  - [ ] generated directories
  - [ ] env example files
  - [ ] important config files
- [ ] Detect stale or contradictory `BIFROST.md` entries.
- [ ] Generate dry-run patches without overwriting `BIFROST.md`.
- [ ] Require explicit acceptance for permanent promotion.
- [ ] Support accepting a specific promotion candidate by ID.
- [ ] Support ignore and ignore-forever handling for promotion candidates.
- [ ] Link promoted content back to source snapshot or claim.

## Phase 11 - Snapshot Diff And Timeline MVP

- [ ] Add timeline event model.
- [ ] Write local `timeline.jsonl` events for snapshot, verify, and plan updates.
- [ ] Ensure timeline events never contain secrets.
- [ ] Add `bifrost diff`.
- [ ] Add `bifrost diff latest~1..latest`.
- [ ] Add `bifrost diff --json`.
- [ ] Add `bifrost timeline`.
- [ ] Add `bifrost restore <n> --preview`.
- [ ] Add tests for human-readable and JSON diff output.

## Phase 12 - Docs, UX, And Metrics

- [ ] Update README examples for JSON-backed snapshots where needed.
- [ ] Update command help text for new CLI commands.
- [ ] Document actionable error format:
  - [ ] field path
  - [ ] bad value
  - [ ] expected value
  - [ ] suggested fix command
- [ ] Document default briefing language:
  - [ ] "Trust this"
  - [ ] "Verify this first"
  - [ ] "Do not assume this"
- [ ] Track product success metrics manually or in docs:
  - [ ] resume time
  - [ ] critical unverified claim rate
  - [ ] stale active file detection
  - [ ] secret leakage corpus result
  - [ ] plan claimed vs verified coverage
  - [ ] default briefing size

## MVP Cut

If scope must be reduced, implement only this first:

- [ ] `session.json` with `snapshot.v2`.
- [ ] Backward-compatible `session.md` rendering.
- [ ] Observed facts vs interpretation separation.
- [ ] Git and active file collectors.
- [ ] Evidence refs on active files and status items.
- [ ] `bifrost validate`.
- [ ] `bifrost verify`.
- [ ] `/handin --verify`.
- [ ] Secret scanner before snapshot write.
- [ ] `claimed_done` vs `verified_done`.

## Completion Criteria

- [ ] `go test ./...` passes.
- [ ] Existing README examples still work or are intentionally updated.
- [ ] Existing slash command workflows still work.
- [ ] Existing MCP clients still work.
- [ ] Existing Markdown snapshots remain readable.
- [ ] Existing Markdown plans remain readable.
- [ ] New JSON schema is validated by tests.
- [ ] Golden tests cover Markdown rendering.
- [ ] Secret scanner blocks or redacts known test corpus secrets.
- [ ] Verify output is actionable and feeds into handin briefing.
