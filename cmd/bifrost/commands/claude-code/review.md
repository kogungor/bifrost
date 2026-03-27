Review an implementation plan or any file as a critical analyst, identifying risks,
gaps, and improvements.

## Your task

Act as a critical technical reviewer. Analyze the target document and provide
structured feedback with an explicit outcome: approved or needs_revision.

## Steps

**Step 1 — Determine review target**

Check `$ARGUMENTS`:
- If empty: review the default plan (name "plan")
- If the argument starts with `/` or contains path separators and a file exists at
  that path: review that file directly (file review mode)
- If a file exists at the argument path (relative to project root): review that file
- Otherwise: treat it as a plan name (e.g. `/review auth-refactor`)

The key rule: **check if the argument is an existing file path first**. If the file
exists on disk, treat it as a file review. If it does not exist, treat it as a plan name.

**Step 2a — Plan review mode** (when reviewing a Bifrost plan)

Read the plan using `bifrost_read_plan` with the appropriate name.

If no plan is found, print:

```
  No plan found.

  Run /plan in an AI coding tool first to create one.
```

Then stop.

**Check consensus state before reviewing:**
- If `consensus_state` is `reached` or `overridden`: the plan is already active. Inform the user and stop unless they explicitly want a re-review.
- If `deadlock_detected` is true: warn the user before proceeding.

Analyze the plan critically. Consider:
- **Completeness**: Are there missing steps? Gaps in the workflow?
- **Ordering**: Are steps in the right sequence? Are dependencies respected?
- **Edge cases**: What could go wrong? What's not handled?
- **Security**: Are there security implications not addressed?
- **Architecture**: Does the approach fit the project's patterns?
- **Over-engineering**: Is anything unnecessarily complex?
- **Testing**: Is the test strategy adequate?
- **File coverage**: Are all affected files listed?

**Decide your outcome:** approved or needs_revision.

Call `bifrost_update_plan` with:
- `name`: the plan name
- `source_tool`: "claude-code"
- `review_outcome`: `"approved"` or `"needs_revision"`
- `review_feedback`: your complete review findings as a single string (specific, actionable)

Do NOT set `plan_status` manually — the consensus mechanism handles activation automatically.

If `review_outcome` is `"approved"`:
- The plan will automatically become active (consensus_state: reached).

If `review_outcome` is `"needs_revision"`:
- The planner must revise and re-submit. If `deadlock_detected` is returned as true in the
  response, inform the user that max revisions has been reached and `/plan --force-accept`
  is available to override.

**Step 2b — File review mode** (when reviewing an arbitrary file)

Read the file at the given path. Analyze it as a critical reviewer:
- **Structure**: Is it well-organized and clear?
- **Completeness**: Are there gaps or missing sections?
- **Edge cases**: What scenarios are not addressed?
- **Risks**: Are there technical or process risks?
- **Clarity**: Is the writing precise and unambiguous?

Present your findings directly to the user. Do not attempt to write to a plan file.

**Step 3 — Present summary**

For plan reviews, print:

```
  Review complete for .bifrost/<name>.plan.md

  Outcome       <approved | needs_revision>
  Plan version  v<plan_version>
  Revision      <revision_count> of <max_revisions>
  Consensus     <consensus_state>

  Key findings:
  - <finding 1>
  - <finding 2>
  - <finding 3>

  <If approved:>  Plan is now active. Work can begin.
  <If needs_revision:>  Planner should run /plan --revise to address feedback.
  <If deadlock:>  Max revisions reached. Planner can run /plan --force-accept to override.
```

For file reviews, print:

```
  Review of <file path>

  Key findings:
  - <finding 1>
  - <finding 2>
  - <finding 3>
```

The user's argument to this command is: $ARGUMENTS
