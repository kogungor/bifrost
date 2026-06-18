Load the Bifrost context snapshot and brief the user before continuing.

## Your task

Read the Bifrost snapshot and present a structured briefing. Do not start working
until the user confirms.

If the command was invoked as `/handin --verify`, run verification first and let
that result shape the briefing. Normal `/handin` behavior must remain unchanged
when `--verify` is absent.

## Steps

**Step 1 — Read the snapshot**

Try `bifrost_read_snapshot` MCP tool first (no arguments). If the tool is available
and returns `found: true`, use its response for all subsequent steps.

If the MCP tool is unavailable, read local snapshot files directly. Prefer
`.bifrost/session.json` when it exists because it has machine-readable fields.
If `session.json` cannot be parsed or validated, mention that problem in the
briefing and fall back to `.bifrost/session.md` so legacy handins still work.
When reading Markdown, parse the YAML frontmatter and sections manually.

If neither source yields a snapshot, print:

```
  No Bifrost snapshot found.

  Run /handoff in your other AI coding tool first.
  If you just installed Bifrost, run 'bifrost install' in your terminal.
```

Then stop.

**Step 2 — Optional verification (`--verify`)**

If the user invoked `/handin --verify`, run:

```bash
bifrost verify --json
```

If the command exits non-zero but prints JSON, still parse and use the JSON. If
the command is unavailable or no parseable JSON is produced, continue with the
normal briefing and add this at the start:

```
  Verification summary
  Could not run `bifrost verify --json`; treat this briefing as unverified.
```

Do not run destructive commands and do not use `--fix`.

When verification JSON is available, read:
- `status`
- `checks`
- `recommended_next_action`

At the beginning of the briefing, before task/status details, include:

```
  Verification summary
  <overall status: pass, warn, or fail>

  Trust this
  <important pass checks, summarized briefly>

  Verify this first
  <warn/fail checks with check id, message, and safe_next_action when present>

  Do not assume
  <critical stale or unverified claims implied by warn/fail checks>

  Safe next action
  <recommended_next_action>
```

Use these mappings for "Do not assume":
- `commands.test_claims` warn/fail: do not assume tests pass.
- `files.active_changed` warn/fail: do not assume active files are unchanged.
- `git.branch_match` or `git.commit_match` warn/fail: do not assume snapshot git context is current.
- `questions.unresolved_high` warn/fail: do not assume high-severity questions are resolved.
- `risks.unresolved_high` warn/fail: do not assume high-severity risks are resolved.
- `claims.evidence` warn/fail: do not assume model claims are evidence-backed.

If verification status is `fail`, finish the briefing and ask before starting
any implementation.

**Step 3 — Read BIFROST.md**

Read `BIFROST.md` if it exists in the project root. You will use its Stack and
Conventions sections in the briefing.

**Step 4 — Check snapshot age**

If using MCP: use `age_seconds` from the response.
If reading the file directly: calculate age from the `timestamp` frontmatter field.

- If older than 2 hours but less than 24 hours: note the age in the briefing.
- If older than 24 hours: show a prominent warning before the briefing.

**Step 5 — Print the briefing**

```
  ─────────────────────────────────────────
   Bifrost Briefing
  ─────────────────────────────────────────

  Project    <project>
  From       <source_tool>
  Captured   <human-readable age, e.g. "22 minutes ago">
  Commit     <git_sha, first 8 chars — omit if empty>
  Intent     <session_intent — omit if empty>
  Pressure   <token_pressure — explain if high: "previous session was near context limit">

  Task
  <current_task>

  Status
  <status checklist>

  Active files
  <active_files — show path, note, and confidence if present, e.g.:
    - src/auth.ts — stub written (confidence: medium)>

  Key decisions
  <decisions>

  Environment notes
  <environment_notes>

  Next step
  <next_step>
```

If assumptions non-empty, append:

```
  Assumptions (not verified)
  <assumptions>
```

If risks non-empty, append:

```
  Risks
  <risks>
```

Close the briefing block:

```
  ─────────────────────────────────────────
```

If `/handin --verify` was used, prepend the verification summary from Step 2 to
this briefing.

If `BIFROST.md` exists, prepend the briefing with:

```
  Project config loaded from BIFROST.md
  <Stack and Conventions sections from BIFROST.md>
  ─────────────────────────────────────────
```

If a handoff note exists (from MCP response or `.bifrost/handoff.md`), append:

```
  Handoff note
  "<handoff note text>"
```

**Step 6 — Load active plan (if set)**

If `active_plan_name` is non-empty:
- If MCP available: call `bifrost_read_plan` with that name.
- If MCP unavailable: read `.bifrost/<active_plan_name>.plan.md` directly.

If the plan is found, append to the briefing:

```
  ─────────────────────────────────────────
  Active plan   <plan title>
  Status        <plan status>
  Progress      <completion %>% (<steps done>/<total> steps done)
  Next step     <first pending step, or "all steps complete">
  Blocked       <blocked count> step(s)  ← omit line if 0
  ─────────────────────────────────────────
```

**Step 7 — Surface open questions**

If `open_questions` is non-empty, print after the briefing:

```
  Open questions — address these before starting:
  <open_questions, one per line>
```

**Step 8 — Ask before proceeding**

Print:

```
  Ready to continue from here. Any adjustments before we start?
```

Wait for the user's response. Do not begin any work until they respond.
