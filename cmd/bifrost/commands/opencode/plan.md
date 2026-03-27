Create a structured implementation plan and save it via Bifrost so another AI tool
can review it.

## Your task

Build an implementation plan for what the user wants to accomplish, then write it
using the `bifrost_write_plan` MCP tool.

Check `$ARGUMENTS` for mode flags before proceeding:
- If `$ARGUMENTS` contains `--revise`: run in **revise mode** (Step 6)
- If `$ARGUMENTS` contains `--force-accept`: run in **force-accept mode** (Step 7)
- Otherwise: run in **create mode** (Steps 1–5)

---

## Create mode (default)

**Step 1 — Gather context**

Read the current Bifrost snapshot (if one exists) using `bifrost_read_snapshot`
to understand the project state.

The plan name is the first word of `$ARGUMENTS` (excluding flags). If no name
given, use "plan".

**Step 2 — Understand the goal**

If the user has not already described what they want to build, ask them. Be
specific: what feature, what problem, what outcome.

**Step 3 — Break it down**

Create a structured plan with:
- **Title**: A clear, concise name for the work
- **Goal**: One to three sentences explaining what success looks like
- **Steps**: Ordered list of implementation steps, each with:
  - A clear description of the work
  - List of files that will be created or modified
- **Constraints**: Any requirements, limitations, or rules to follow

**Step 4 — Write the plan**

Use the `bifrost_write_plan` MCP tool with these fields:
- `source_tool`: "opencode"
- `name`: the plan name
- `title`: the plan title
- `goal`: the goal text
- `steps`: array of `{description, files}` objects
- `constraints`: array of constraint strings

The tool initializes `plan_version: 1`, `proposed_by`, `max_revisions: 3`, and
`consensus_state: none` automatically.

**Step 5 — Confirm to the user**

Print:

```
  Plan written to .bifrost/<name>.plan.md

  Title         <plan title>
  Status        draft
  Plan version  v1
  Steps         <count> steps defined (0% complete)
  Files         <total unique file count> files referenced

  Ask another AI tool to run /review to get a critical analysis.
  Consensus required: reviewer must approve before work begins.
```

---

## Revise mode (`/plan <name> --revise`)

The plan was reviewed and needs changes. Read the current review notes, address
the feedback, update the plan, then signal the revision.

**Step 6 — Revise**

1. Read the plan using `bifrost_read_plan` with the plan name.
2. Show the user the latest review notes (outcome + feedback).
3. Ask the user what changes to make, or proceed with your own assessment of the
   feedback if the intent is clear.
4. Apply content changes using `bifrost_update_plan` with `step_updates`,
   updated `title`, or `goal` as needed.
5. Signal the revision by calling `bifrost_update_plan` with `revise: true`.
   This increments `plan_version` and `revision_count`, and resets `consensus_state`.
6. Print:

```
  Plan revised — .bifrost/<name>.plan.md

  Plan version  v<new plan_version>
  Revisions     <revision_count> of <max_revisions>
  Consensus     pending re-review

  Ask another AI tool to run /review again.
  <If revision_count >= max_revisions - 1:> Warning: approaching max revisions. Next
  rejection will trigger deadlock. Consider /plan --force-accept if blocked.
```

---

## Force-accept mode (`/plan <name> --force-accept`)

The review process is deadlocked or blocked. Override consensus and activate the plan.

**Step 7 — Force accept**

1. Read the plan using `bifrost_read_plan` with the plan name.
2. Show the user the current state: revision count, deadlock status, latest review notes.
3. Confirm the user wants to proceed despite unresolved review feedback.
4. Call `bifrost_update_plan` with `force_accept: true`.
5. Print:

```
  Plan force-accepted — .bifrost/<name>.plan.md

  Status        active
  Consensus     overridden
  Activation    force_accepted

  Review feedback was not fully resolved. Proceed with caution.
```

The user's argument to this command is: $ARGUMENTS
