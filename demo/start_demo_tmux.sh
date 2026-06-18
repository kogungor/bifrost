#!/usr/bin/env bash
set -euo pipefail

SESSION="${BIFROST_DEMO_SESSION:-demo}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK="${BIFROST_DEMO_WORKDIR:-/tmp/bifrost-demo}"
PROJECT="$WORK/project"
BIN_DIR="$WORK/bin"
BIN="$BIN_DIR/bifrost"

rm -rf "$WORK"
mkdir -p "$BIN_DIR" "$PROJECT/src" "$PROJECT/tests" "$PROJECT/.bifrost/history" "$PROJECT/.bifrost/plans"

(cd "$ROOT" && go build -o "$BIN" ./cmd/bifrost)

cat >"$PROJECT/BIFROST.md" <<'EOF'
# BIFROST.md

## Stack
- TypeScript service

## Commands
- Test: npm test
- Typecheck: npm run typecheck

## Conventions
- Keep auth changes evidence-backed before handoff.
EOF

cat >"$PROJECT/src/tokens.ts" <<'EOF'
export function rotateRefreshToken(token: string): string {
  return `${token}:rotated`
}
EOF

cat >"$PROJECT/tests/tokens.test.ts" <<'EOF'
import { rotateRefreshToken } from "../src/tokens"

it("rotates refresh tokens", () => {
  expect(rotateRefreshToken("refresh")).toContain("rotated")
})
EOF

cat >"$PROJECT/package.json" <<'EOF'
{
  "scripts": {
    "test": "echo \"2 failing tests in tokens.test.ts\" && exit 1",
    "typecheck": "echo \"typecheck passed\""
  }
}
EOF

git -C "$PROJECT" init -q
git -C "$PROJECT" config user.name "Bifrost Demo"
git -C "$PROJECT" config user.email "demo@example.com"
git -C "$PROJECT" add .
git -C "$PROJECT" commit -q -m "Initial demo project"

write_previous_snapshot() {
  local ts
  ts="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
  cat >"$PROJECT/.bifrost/session.md" <<EOF
---
bifrost_version: 2
timestamp: $ts
source_tool: opencode
project: bifrost-demo
token_pressure: high
session_intent: implementing
active_plan_name: auth-refactor
git_sha: $(git -C "$PROJECT" rev-parse HEAD)
---

# Session Snapshot

## Current Task
Design refresh token rotation tests.

## Status
- [x] Auth flow inspected
- [-] Test plan drafted
- [ ] Verification not run

## Active Files
- \`src/tokens.ts\` — rotation helper exists but behavior is not evidence-backed [confidence: medium]

## Decisions Made
- **Use local-only handoff state**: Keep demo evidence in .bifrost, not committed project files.

## Environment Notes
- Test command candidate: \`npm test\`

## Next Step
Write a stronger handoff with evidence and run verification before continuing.

## Open Questions
- Should refresh tokens be single-use?

## Risks
- Test status has not been verified.
EOF
  rm -f "$PROJECT/.bifrost/session.json"
  "$BIN" migrate --project "$PROJECT" >/dev/null
  "$BIN" snapshot --enrich --project "$PROJECT" >/dev/null
}

write_previous_snapshot

plan_ts="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
cat >"$PROJECT/.bifrost/plans/auth-refactor.json" <<EOF
{
  "schema_version": "plan.v2",
  "name": "auth-refactor",
  "title": "Auth refresh token integrity demo",
  "goal": "Finish refresh token rotation with evidence-backed verification.",
  "status": "active",
  "version": "v2",
  "created_at": "$plan_ts",
  "updated_at": "$plan_ts",
  "steps": [
    {
      "id": "step_collect",
      "title": "Collect observed git and file evidence",
      "status": "verified_done",
      "expected_files": ["src/tokens.ts"],
      "verification": {
        "required": true,
        "commands": ["bifrost snapshot --enrich"],
        "last_result": {
          "state": "pass",
          "command": "bifrost snapshot --enrich",
          "exit_code": 0,
          "captured_at": "$plan_ts"
        }
      }
    },
    {
      "id": "step_verify",
      "title": "Resolve verify warnings before implementation",
      "status": "claimed_done",
      "expected_files": ["src/tokens.ts", "tests/tokens.test.ts"],
      "verification": {
        "required": true,
        "commands": ["npm test", "npm run typecheck"],
        "last_result": {
          "state": "fail",
          "command": "npm test",
          "exit_code": 1,
          "captured_at": "$plan_ts"
        }
      }
    },
    {
      "id": "step_finish",
      "title": "Finish safe refresh token behavior",
      "status": "not_started",
      "expected_files": ["src/tokens.ts", "tests/tokens.test.ts"],
      "verification": {
        "required": true,
        "commands": ["npm test", "npm run typecheck"]
      }
    }
  ]
}
EOF

cat >"$WORK/run_handoff.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

PROJECT="${BIFROST_DEMO_PROJECT:?missing demo project}"
BIN="${BIFROST_DEMO_BIN:?missing demo binary}"
ts="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
hist_ts="$(date -u +"%Y-%m-%dT%H%M%SZ")"

mkdir -p "$PROJECT/.bifrost/history"
if [[ -f "$PROJECT/.bifrost/session.md" ]]; then
  cp "$PROJECT/.bifrost/session.md" "$PROJECT/.bifrost/history/$hist_ts.session.md"
fi
if [[ -f "$PROJECT/.bifrost/session.json" ]]; then
  cp "$PROJECT/.bifrost/session.json" "$PROJECT/.bifrost/history/$hist_ts.session.json"
fi

cat >"$PROJECT/.bifrost/session.md" <<SNAPSHOT
---
bifrost_version: 2
timestamp: $ts
source_tool: opencode
project: bifrost-demo
token_pressure: critical
session_intent: implementing
active_plan_name: auth-refactor
git_sha: $(git -C "$PROJECT" rev-parse HEAD)
---

# Session Snapshot

## Current Task
Implement trust-aware handin verification for refresh token rotation.

## Status
- [x] Snapshot v2 JSON state written
- [x] Evidence collector wired into handoff
- [-] Verify warnings need review
- [ ] Tests still need a passing command result

## Active Files
- \`src/tokens.ts\` — refresh token rotation changed after handoff and must be inspected [confidence: medium]
- \`tests/tokens.test.ts\` — failing test command recorded as unverified evidence [confidence: low]

## Decisions Made
- **Separate observed facts from model interpretation**: Trust the git/file facts before model claims.

## Environment Notes
- \`npm test\` currently reports 2 failing tests.
- \`npm run typecheck\` is the next low-risk command.

## Next Step
Inspect \`src/tokens.ts\`, then resolve the single-use token question before continuing implementation.

## Assumptions
- Redis is available locally for token revocation.

## Open Questions
- Should refresh tokens be single-use?

## Risks
- Token reuse behavior is unresolved.
- Do not assume tests pass.
SNAPSHOT

cat >"$PROJECT/.bifrost/handoff.md" <<NOTE
---
timestamp: $ts
from: opencode
---

Previous session hit the context limit. Continue from verification, not from claims.
NOTE

rm -f "$PROJECT/.bifrost/session.json"
"$BIN" migrate --project "$PROJECT" >/dev/null
"$BIN" snapshot --enrich --project "$PROJECT" >/dev/null

cat >>"$PROJECT/src/tokens.ts" <<'CHANGE'

// Demo drift after handoff: verify should warn before trusting this file.
CHANGE

printf "  Snapshot written to .bifrost/session.md\n"
printf "  JSON state written to .bifrost/session.json\n\n"
printf "  Task    Implement trust-aware handin verification for refresh token rotation.\n"
printf "  Files   2 active files captured\n"
printf "  Note    Previous session hit the context limit. Continue from verification...\n\n"
printf "  Switch to your target tool and run /handin --verify\n"
EOF
chmod +x "$WORK/run_handoff.sh"

cat >"$WORK/demo_env.zsh" <<EOF
export PATH="$BIN_DIR:\$PATH"
export BIFROST_DEMO_PROJECT="$PROJECT"
export BIFROST_DEMO_BIN="$BIN"
export BIFROST_DEMO_RUN_HANDOFF="$WORK/run_handoff.sh"
export NO_COLOR=1
alias cls='clear'
handoff() { "\$BIFROST_DEMO_RUN_HANDOFF"; }
handin() { bifrost brief --mode implement --budget 5000; }
doctor() { bifrost doctor --security --project "\$BIFROST_DEMO_PROJECT" || true; tmux select-pane -t +1; }
PROMPT='%F{cyan}bifrost-demo%f %1~ %# '
EOF

tmux kill-session -t "$SESSION" 2>/dev/null || true
tmux new-session -d -s "$SESSION" -c "$PROJECT" "zsh -f -i"
tmux send-keys -t "$SESSION" "source '$WORK/demo_env.zsh'; clear" Enter
tmux split-window -h -t "$SESSION" -c "$PROJECT" "zsh -f -i"
tmux send-keys -t "$SESSION" "source '$WORK/demo_env.zsh'; clear" Enter
tmux select-pane -t "$SESSION" -L
