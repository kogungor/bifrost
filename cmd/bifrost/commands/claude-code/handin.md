Load the Bifrost context snapshot and brief the user before continuing.

## Your task

Read the Bifrost snapshot files and present a structured briefing. Do not start
working until the user confirms.

## Steps

**Step 1 — Read the snapshot**

Attempt to read `.bifrost/session.md`.

If the file does not exist, print:

```
  No Bifrost snapshot found.

  Run /handoff in your other AI coding tool first.
  If you just installed Bifrost, run 'bifrost install' in your terminal.
```

Then stop.

**Step 2 — Read supporting files**

- Read `.bifrost/handoff.md` if it exists.
- Read `BIFROST.md` if it exists in the project root.

**Step 3 — Check snapshot age**

Calculate the difference between `timestamp` in the frontmatter and now.
- If older than 2 hours but less than 24 hours: note the age in the briefing.
- If older than 24 hours: show a prominent warning before the briefing.

**Step 4 — Print the briefing**

```
  ─────────────────────────────────────────
   Bifrost Briefing
  ─────────────────────────────────────────

  Project    <project from frontmatter>
  From       <source_tool from frontmatter>
  Captured   <human-readable age, e.g. "22 minutes ago">
  Pressure   <token_pressure — explain if high: "previous session was near context limit">

  Task
  <Current Task content>

  Status
  <Status checklist, formatted with checkboxes>

  Active files
  <Active Files list>

  Key decisions
  <Decisions Made list>

  Environment notes
  <Environment Notes list>

  Next step
  <Next Step content>
  ─────────────────────────────────────────
```

If `BIFROST.md` exists, prepend the briefing with:

```
  Project config loaded from BIFROST.md
  <Stack and Conventions sections from BIFROST.md>
  ─────────────────────────────────────────
```

If `.bifrost/handoff.md` exists, append after the briefing:

```
  Handoff note from <from field>
  "<note text>"
```

**Step 5 — Ask before proceeding**

After the briefing, ask:

```
  Ready to continue from here. Any adjustments before we start?
```

Wait for the user's response. Do not begin any work until they respond.
