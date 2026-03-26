# Bifrost

> When tokens run dry, the bridge holds.

A session context bridge between AI coding tools. Write `/handoff` in one tool, `/handin` in another. Continue exactly where you left off. Create implementation plans with `/plan` in one tool, get critical analysis with `/review` in another.

---

## The Problem

You're deep in a session with Claude Code. The context window or credit is nearly full. You have:

- A dozen architectural decisions that aren't in any file
- Three subtle bugs identified but not yet fixed
- A clear plan for the next few hours of work
- Environmental knowledge — which commands to run, which gotchas to avoid

The session ends. You open OpenCode — or start a fresh Claude Code session. You explain everything again from scratch.

Bifrost solves this. Session context transfers between AI coding tools the same way a terminal session transfers between machines — seamlessly, without manual work.

## How It Works

```
Claude Code                              OpenCode
┌──────────────┐                        ┌──────────────┐
│              │                        │              │
│  /handoff ───┼──── .bifrost/ ────────▶┼── /handin    │
│              │    session.md          │              │
│  /handin  ◀──┼──── .bifrost/ ────────┼── /handoff   │
│              │    session.md          │              │
│  /plan   ────┼──── .bifrost/ ────────▶┼── /review    │
│              │    *.plan.md           │              │
└──────────────┘                        └──────────────┘
```

1. Type `/handoff` — the AI captures your current task, status, active files (with confidence), decisions, environment notes, next step, session intent, assumptions, open questions, and risks into `.bifrost/session.md`
2. Switch tools
3. Type `/handin` — the AI reads the snapshot and presents a structured briefing
4. Continue working with zero context loss

## What Gets Captured

| Category          | Example                                                                  |
| ----------------- | ------------------------------------------------------------------------ |
| Current task      | "Implement JWT refresh token rotation"                                   |
| Status            | `[x] validateToken()`, `[ ] refreshToken()`                              |
| Active files      | `src/auth.ts` — middleware stubbed, not wired (confidence: medium)       |
| Decisions         | "Using jsonwebtoken not jose — already installed"                        |
| Environment notes | `AUTH_SECRET` must be in `.env`, not `.env.local`                        |
| Next step         | "Write unit tests for validateToken() using pattern from crypto.test.ts" |
| Session intent    | `implementing` — tells the next tool what mode to operate in             |
| Assumptions       | "Redis is available on localhost:6379" — unverified assumptions          |
| Open questions    | "Should refresh tokens be single-use or reusable across devices?"        |
| Risks             | "Token revocation list not yet implemented"                              |

What does **not** get captured: file contents, conversation history, secrets, or generated artifacts. The snapshot is a structured summary of working memory — things in the AI's context that aren't yet in any file.

## Installation

### Homebrew (macOS / Linux)

```bash
brew tap kogungor/bifrost
brew install --cask bifrost
```

### Shell One-Liner

```bash
curl -fsSL https://raw.githubusercontent.com/kogungor/bifrost/dev/install.sh | sh
```

### Manual (all platforms)

Download the binary for your platform from [GitHub Releases](https://github.com/kogungor/bifrost/releases), then:

```bash
chmod +x bifrost
sudo mv bifrost /usr/local/bin/
```

### Verify Installation

```bash
bifrost version
```

## Setup

### 1. Register Slash Commands

This installs `/handoff`, `/handin`, `/plan`, and `/review` for all detected AI tools:

```bash
bifrost install
```

To also register Bifrost as an MCP server (recommended — enables richer `/handin` briefings via structured tool calls):

```bash
bifrost install --mcp
```

Without `--mcp`, `/handin` falls back to reading `.bifrost/session.md` directly — it still works, but MCP gives it structured access to all fields.

To install for a specific tool only:

```bash
bifrost install --adapter claude-code
bifrost install --adapter opencode
```

To see what would happen without making changes:

```bash
bifrost install --dry-run
```

To overwrite existing command files (e.g., after an update):

```bash
bifrost install --force
```

### 2. Initialize a Project (optional)

```bash
cd ~/my-project
bifrost init
```

This creates:

- `.bifrost/` directory (gitignored automatically)
- `.bifrost/history/` for archived snapshots
- `BIFROST.md` — a project config file you can commit to git

`BIFROST.md` gives the incoming AI tool persistent project context (stack, conventions, important files, commands). It's optional — everything works without it.

### 3. Verify

```bash
bifrost doctor
```

A healthy output looks like:

```
  Bifrost Doctor

  ✓  Binary               0.1.0
  ✓  Claude Code          commands registered  (/path/to/commands)
  ✓  Claude Code          MCP server registered
  ✓  OpenCode             commands registered  (/path/to/commands)
  ✓  OpenCode             MCP server registered
  ✓  Project              BIFROST.md found
  ✓  Snapshot             22 minutes old
  ✓  Gitignore            .bifrost/ excluded

  All checks passed.
```

## Usage

### Handing Off

In your AI coding tool (Claude Code or OpenCode), type:

```
/handoff
```

The AI will capture the session state and write it to `.bifrost/session.md`. You'll see:

```
  Snapshot written to .bifrost/session.md

  Task    Implement JWT refresh token rotation
  Files   3 active files captured
  Note    auth module half done, JWT middleware needs tests

  Switch to your target tool and run /handin
```

You can add a note:

```
/handoff auth module half done, pick up with the unit tests
```

### Handing In

In your target tool, type:

```
/handin
```

The AI reads the snapshot and presents a briefing:

```
  ─────────────────────────────────────────
   Bifrost Briefing
  ─────────────────────────────────────────

  Project    my-api
  From       claude-code
  Captured   22 minutes ago
  Commit     18270bac
  Intent     implementing
  Pressure   high (previous session was near context limit)

  Task
  Implement JWT refresh token rotation

  Status
  - [x] validateToken()
  - [-] refreshToken() — stub written, logic incomplete
  - [ ] revokeToken()

  Active files
  - src/auth.ts — middleware stubbed, not wired (confidence: medium)
  - src/tokens.ts — new file, refresh logic (confidence: low)

  Key decisions
  - Using jsonwebtoken not jose: already installed
  - Refresh tokens in Redis: faster lookup

  Environment notes
  - AUTH_SECRET must be in .env, not .env.local
  - Run with: npm run dev -- --port 3001

  Next step
  Write unit tests for validateToken() using pattern
  from crypto.test.ts. Focus on the expiry edge case.

  Assumptions (not verified)
  - Redis is available on localhost:6379
  - AUTH_SECRET is already set in .env

  Risks
  - Token revocation list not yet implemented
  ─────────────────────────────────────────

  Active plan   auth-refactor
  Status        active
  Progress      40% (2/5 steps done)
  Next step     Write unit tests for validateToken()
  ─────────────────────────────────────────

  Open questions — address these before starting:
  - Should refresh tokens be single-use or reusable across devices?

  Ready to continue from here. Any adjustments before we start?
```

The AI waits for your confirmation before starting any work.

### Creating Plans

In any AI coding tool, type:

```
/plan auth-refactor
```

The AI creates a structured implementation plan and saves it to `.bifrost/auth-refactor.plan.md`:

```
  Plan written to .bifrost/auth-refactor.plan.md

  Title    Auth token refresh refactor
  Status   draft
  Steps    5 steps defined (0% complete)
  Files    8 files referenced

  Ask another AI tool to run /review to get a critical analysis.
  Use bifrost_update_plan to change status to "active" when work begins.
```

Plans support a full lifecycle: `draft` → `active` → `completed` → `archived`.

### Reviewing Plans

Switch to another AI tool and type:

```
/review auth-refactor
```

The AI reads the plan, analyzes it critically (edge cases, security, architecture, missing steps), and adds review notes directly to the plan file. You can also review arbitrary files:

```
/review docs/rfc-auth.md
```

### Snapshot Freshness

`/handin` always shows the snapshot age. If it's older than 2 hours, a warning is shown. If older than 24 hours, a prominent warning appears. You're never blocked — just informed.

### Working with BIFROST.md

If `BIFROST.md` exists in your project root, `/handin` loads it and includes project context (stack, conventions, commands) in the briefing. This gives every new session a baseline understanding of the project.

Example `BIFROST.md`:

```markdown
---
project: my-api
---

# Project: my-api

## What this is

REST API for the mobile app. Currently in beta.

## Stack

- Node.js 20, Express, PostgreSQL, Redis
- TypeScript, Jest for testing

## Conventions

- All new endpoints need integration tests
- Use zod for request validation
- Error responses follow RFC 7807

## Commands

- `npm run dev` — development server (port 3001)
- `npm test` — full test suite
- `npm run db:migrate` — run pending migrations

## Do not touch

- `src/generated/` — auto-generated from OpenAPI spec
```

## CLI Reference

| Command                      | Description                                   |
| ---------------------------- | --------------------------------------------- |
| `bifrost install`            | Register slash commands for detected AI tools  |
| `bifrost install --mcp`      | Also register Bifrost as an MCP server        |
| `bifrost init`               | Initialize Bifrost in the current project     |
| `bifrost status`             | Show the current bridge state                 |
| `bifrost export`             | Export snapshot and/or plans as JSON to stdout |
| `bifrost doctor`             | Diagnose installation and configuration       |
| `bifrost history`            | List archived snapshots                       |
| `bifrost restore <n>`        | Restore a historical snapshot                 |
| `bifrost update`             | Check for updates                             |
| `bifrost version`            | Print version                                 |
| `bifrost completion <shell>` | Generate shell completions (bash, zsh, fish)  |

### Global Flags

| Flag               | Description                          |
| ------------------ | ------------------------------------ |
| `--no-color`       | Disable color output                 |
| `--quiet`          | Print only errors                    |
| `--project <path>` | Override project root auto-detection |

### Snapshot History

Every `/handoff` automatically archives the previous snapshot. View and restore them:

```bash
bifrost history

  Snapshot history — my-api

  #  Timestamp              Age          Source         Task
  ─  ─────────────────────  ───────────  ─────────────  ──────────────────────
  1  2025-03-21 14:32:17    22 min ago   claude-code    auth module half done
  2  2025-03-21 11:18:44    3 hr ago     opencode       database schema finalized
  3  2025-03-20 19:05:31    yesterday    claude-code    initial project setup

bifrost restore 2
```

## Supported Tools

| Tool        | Status    |
| ----------- | --------- |
| Claude Code | Supported |
| OpenCode    | Supported |
| Cursor      | Planned   |
| Gemini CLI  | Planned   |
| Codex       | Planned   |

Adding a new tool requires only a new adapter file — no changes to core logic.

## Files

| Path                        | Purpose                             | In Git? |
| --------------------------- | ----------------------------------- | ------- |
| `BIFROST.md`                | Project config (stack, conventions) | Yes     |
| `.bifrost/session.md`       | Active snapshot                     | No      |
| `.bifrost/handoff.md`       | Freeform handoff note               | No      |
| `.bifrost/history/`         | Archived snapshots                  | No      |
| `.bifrost/<name>.plan.md`   | Implementation plans                | No      |

`.bifrost/` is automatically added to `.gitignore`.

## MCP Server

Bifrost can run as an [MCP](https://modelcontextprotocol.io/) server, exposing snapshot and plan operations as formal tool calls over stdio JSON-RPC. AI tools that support MCP can call these tools directly — no slash commands needed.

Register it:

```bash
bifrost install --mcp
```

This writes config to each adapter's MCP config path (e.g. `~/.claude/mcp.json`). The server runs as a subprocess — no network sockets, no background daemon.

### Snapshot Tools

| Tool                     | Description                                    |
| ------------------------ | ---------------------------------------------- |
| `bifrost_read_snapshot`  | Read the current session snapshot (returns all fields including semantic enrichments) |
| `bifrost_write_snapshot` | Write a new snapshot — accepts session intent, assumptions, open questions, risks, confidence on files, and active plan name; auto-collects git SHA |
| `bifrost_write_note`     | Write a freeform handoff note                  |
| `bifrost_status`         | Quick status: snapshot age, session intent, active plan, open question count, history count |

### Plan Tools

| Tool                     | Description                                                   |
| ------------------------ | ------------------------------------------------------------- |
| `bifrost_read_plan`      | Read a named plan (default: "plan")                          |
| `bifrost_write_plan`     | Create a new plan with title, goal, steps, and constraints   |
| `bifrost_update_plan`    | Add review notes, update step statuses/content, change plan status |
| `bifrost_delete_plan`    | Delete a named plan                                          |
| `bifrost_list_plans`     | List all plans with name, status, title, and completion %    |

## What Bifrost Is Not

- A decision memory system
- A build output bridge
- A model router or proxy
- A replacement for CLAUDE.md or AGENTS.md

Bifrost is a point-in-time session snapshot protocol and cross-tool planning workflow with a simple slash command UX.

## Security

- All data stays local. No network calls after installation.
- No telemetry or analytics.
- No secrets are stored (the AI is instructed not to include them).
- `.bifrost/` is gitignored by default.
- The snapshot is plain Markdown — open it in any editor to inspect.

## License

MIT
