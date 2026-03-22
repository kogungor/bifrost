Create a structured implementation plan and save it via Bifrost so another AI tool
can review it.

## Your task

Build an implementation plan for what the user wants to accomplish, then write it
using the `bifrost_write_plan` MCP tool.

## Steps

**Step 1 — Gather context**

Read the current Bifrost snapshot (if one exists) using `bifrost_read_snapshot`
to understand the project state.

If `$ARGUMENTS` is provided and is not empty, treat it as the plan name (e.g.
`/plan auth-refactor` creates a plan named "auth-refactor"). If no argument is
given, use the default name "plan".

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
- `name`: the plan name from Step 1
- `title`: the plan title
- `goal`: the goal text
- `steps`: array of `{description, files}` objects
- `constraints`: array of constraint strings

**Step 5 — Confirm to the user**

Print:

```
  Plan written to .bifrost/<name>.plan.md

  Title    <plan title>
  Status   draft
  Steps    <count> steps defined (0% complete)
  Files    <total unique file count> files referenced

  Ask another AI tool to run /review to get a critical analysis.
  Use bifrost_update_plan to change status to "active" when work begins.
```

The user's argument to this command is: $ARGUMENTS
