Load the Bifrost context snapshot and brief the user before continuing.

## Your task

Read the Bifrost snapshot and present a structured briefing. Do not start working
until the user confirms.

## Steps

**Step 1 — Read the snapshot**

Try `bifrost_read_snapshot` MCP tool first (no arguments). If the tool is available
and returns `found: true`, use its response for all subsequent steps.

If the MCP tool is unavailable, read `.bifrost/session.md` directly instead.
Parse the YAML frontmatter and markdown sections manually.

If neither source yields a snapshot, print:

```
  No Bifrost snapshot found.

  Run /handoff in your other AI coding tool first.
  If you just installed Bifrost, run 'bifrost install' in your terminal.
```

Then stop.

**Step 2 — Read BIFROST.md**

Read `BIFROST.md` if it exists in the project root. You will use its Stack and
Conventions sections in the briefing.

**Step 3 — Check snapshot age**

If using MCP: use `age_seconds` from the response.
If reading the file directly: calculate age from the `timestamp` frontmatter field.

- If older than 2 hours but less than 24 hours: note the age in the briefing.
- If older than 24 hours: show a prominent warning before the briefing.

**Step 4 — Print the briefing**

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

**Step 5 — Load active plan (if set)**

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

**Step 6 — Surface open questions**

If `open_questions` is non-empty, print after the briefing:

```
  Open questions — address these before starting:
  <open_questions, one per line>
```

**Step 7 — Ask before proceeding**

Print:

```
  Ready to continue from here. Any adjustments before we start?
```

Wait for the user's response. Do not begin any work until they respond.
