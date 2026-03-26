Capture the current session state as a Bifrost snapshot so the work can continue
in another AI coding tool.

## Your task

Write a structured snapshot capturing the current session state, then confirm to
the user.

## Steps

**Step 1 — Write the snapshot**

If the `bifrost_write_snapshot` MCP tool is available, call it with these fields:

```
source_tool:      opencode
token_pressure:   <low|medium|high|critical — your honest assessment>
session_intent:   <planning|implementing|debugging|reviewing>
current_task:     <one sentence: what is being built or fixed right now>
status:           <array of checklist items: "[x] done", "[-] in progress", "[ ] todo">
active_files:     <array of {path, note, confidence} objects>
decisions:        <array of key decisions made>
environment_notes: <array of commands, env vars, gotchas>
next_step:        <one to three sentences: exactly where to resume>
active_plan_name: <name of plan being executed, omit if none>
session_start:    <ISO 8601 timestamp of when this session began, omit if unknown>
assumptions:      <array of genuine uncertainties — omit if nothing real>
open_questions:   <array of unresolved questions — omit if none>
risks:            <array of known risks or blockers — omit if none>
```

The tool automatically handles archiving the previous snapshot, atomic file write,
and git SHA collection.

If MCP is unavailable, fall back to writing `.bifrost/session.md` directly:

First, prepare the directory:
- Check if `.bifrost/` exists. If not, create it.
- Check if `.bifrost/history/` exists. If not, create it.

Archive the previous snapshot if it exists:
- Copy `.bifrost/session.md` to `.bifrost/history/<current-ISO8601-timestamp>.md`
  (replace colons with hyphens in the filename)

Then write `.bifrost/session.md` with this exact structure:

```
---
bifrost_version: 2
timestamp: <current UTC time in ISO 8601 format>
source_tool: opencode
project: <name from BIFROST.md frontmatter, or current directory name>
token_pressure: <low|medium|high|critical>
session_intent: <planning|implementing|debugging|reviewing>
active_plan_name: <name of the .bifrost/*.plan.md being executed, omit if none>
session_start: <ISO 8601 timestamp of when this session began, omit if unknown>
git_sha: <current HEAD SHA from git rev-parse HEAD, omit if unavailable>
---

# Session Snapshot

## Current Task
<One sentence. What is being built or fixed right now. Be specific.>

## Status
- [x] <Completed item>
- [-] <In-progress item — describe partial state>
- [ ] <Not started>

## Active Files
- `<relative/path/to/file>` — <one-line note about its current state> [confidence: <high|medium|low>]

## Decisions Made
- **<Decision>**: <rationale. Why this over alternatives.>

## Environment Notes
- <Discovered command, env var, gotcha, or constraint worth preserving>

## Next Step
<One to three sentences. Specific enough that someone starting fresh can begin
immediately. Name files and functions.>
```

If you have genuine uncertainties, unresolved decisions, or known risks, append
one or more of these optional sections (omit entirely if nothing meaningful to capture):

```
## Assumptions
- <Something you assumed but haven't verified>

## Open Questions
- <An unresolved question the incoming tool should address before proceeding>

## Risks
- <A known risk or blocker the incoming tool should be aware of>
```

**Rules:**
- Core sections (Current Task, Status, Active Files, Decisions Made, Environment Notes, Next Step) must always be present, even if content is "Nothing to note."
- Optional sections (Assumptions, Open Questions, Risks) must be omitted entirely if there is nothing genuine to capture — never write placeholder text.
- File paths must be relative to project root.
- Do not include file contents, diffs, or code blocks.
- Next Step must be actionable, not vague.
- `session_intent` must be one of: planning, implementing, debugging, reviewing.
- `confidence` on active files must be one of: high, medium, low.

**Step 2 — Write the handoff note**

If the user provided text after /handoff:
- If MCP available: call `bifrost_write_note` with `text` set to that text and `from: opencode`.
- If MCP unavailable: write it verbatim to `.bifrost/handoff.md`.

If no text was provided, write a one-sentence AI-generated summary of the session state.

Format for direct write to `.bifrost/handoff.md`:

```
---
timestamp: <same timestamp as session.md>
from: opencode
---

<note text>
```

**Step 3 — Check .gitignore**

If `.gitignore` exists and does not contain `.bifrost/`, append:

```
# Bifrost runtime data
.bifrost/
```

**Step 4 — Confirm to the user**

Print exactly this (substituting actual values):

```
  Snapshot written to .bifrost/session.md

  Task    <first sentence of Current Task>
  Files   <count> active files captured
  Note    <handoff note text, truncated to 80 chars if needed>

  Switch to your target tool and run /handin
```

The user's argument to this command is: $ARGUMENTS
