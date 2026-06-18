---
bifrost_version: 2
created_at: 2026-06-18T09:00:00Z
updated_at: 2026-06-18T10:22:33Z
source_tool: claude-code
project: bifrost
status: active
plan_version: 2
proposed_by: claude-code
max_revisions: 3
revision_count: 1
consensus_state: reached
activation_reason: consensus
---

# Integrity Foundation

## Goal
Add baseline tests before JSON migration.

## Steps
- [x] Audit current Markdown boundaries
  - id: step_audit
  - `internal/snapshot/snapshot.go`
- [ ] Add golden render tests
  - id: step_golden
  - `internal/snapshot/snapshot_test.go`
  - `internal/snapshot/plan_test.go`
- [!] Decide JSON migration write policy
  - id: step_policy
  - `INTEGRITY_PACK_TODO.md`

## Constraints
- Keep existing session.md and plan.md readable
- Do not change adapter behavior

## Review Notes
> [opencode | 2026-06-18T10:00:00Z | v2 | approved] Baseline looks safe
