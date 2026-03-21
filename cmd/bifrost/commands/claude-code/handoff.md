Capture the current session state as a Bifrost snapshot so the work can continue
in another AI coding tool.

## Your task

Write a structured snapshot to `.bifrost/session.md` using **exactly** the format
specified below. Then archive any previous snapshot, write the handoff note, and
confirm to the user.

## Steps

**Step 1 — Prepare the .bifrost directory**

Check if `.bifrost/` exists in the project root. If not, create it.
Check if `.bifrost/history/` exists. If not, create it.

**Step 2 — Archive the previous snapshot**

If `.bifrost/session.md` already exists, copy it to
`.bifrost/history/<current-ISO8601-timestamp>.md` before overwriting it.
Replace colons in the timestamp with hyphens for the filename.

**Step 3 — Write the snapshot**

Write `.bifrost/session.md` with this exact structure:

```
---
bifrost_version: 1
timestamp: <current UTC time in ISO 8601 format>
source_tool: claude-code
project: <name from BIFROST.md frontmatter, or current directory name>
token_pressure: <low|medium|high — your honest assessment of context window usage>
---

# Session Snapshot

## Current Task
<One sentence. What is being built or fixed right now. Be specific.>

## Status
- [x] <Completed item>
- [-] <In-progress item — describe partial state>
- [ ] <Not started>

## Active Files
- `<relative/path/to/file>` — <one-line note about its current state and what's notable>

## Decisions Made
- **<Decision>**: <rationale. Why this over alternatives.>

## Environment Notes
- <Discovered command, env var, gotcha, or constraint worth preserving>

## Next Step
<One to three sentences. Specific enough that someone starting fresh can begin
immediately. Name files and functions.>
```

**Rules:**
- Every section must be present even if content is "Nothing to note."
- File paths must be relative to project root.
- Do not include file contents, diffs, or code blocks.
- Next Step must be actionable, not vague.

**Step 4 — Write the handoff note**

If the user provided text after /handoff, write it verbatim to `.bifrost/handoff.md`.
If no text was provided, write a one-sentence AI-generated summary of the session state.

Format for `.bifrost/handoff.md`:

```
---
timestamp: <same timestamp as session.md>
from: claude-code
---

<note text>
```

**Step 5 — Check .gitignore**

If `.gitignore` exists and does not contain `.bifrost/`, append:

```
# Bifrost runtime data
.bifrost/
```

**Step 6 — Confirm to the user**

Print exactly this (substituting actual values):

```
  Snapshot written to .bifrost/session.md

  Task    <first sentence of Current Task>
  Files   <count> active files captured
  Note    <handoff note text, truncated to 80 chars if needed>

  Switch to your target tool and run /handin
```

The user's argument to this command is: $ARGUMENTS
