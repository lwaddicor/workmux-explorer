## Context

This is a greenfield Go tool with no existing code. It is a thin, read-mostly layer over tooling that already exists on the machine: the `workmux` CLI (v0.1.234 verified), `tmux`, and `git`. See `proposal.md` for motivation.

Three constraints shape the approach:

- workmux has **no central state file** and **no importable API** — its state lives in the tmux server, the git worktrees, and agent-status hooks. Its stable, documented surface is the CLI.
- `workmux list` and `workmux status` are **CWD-scoped** (they report the project in the current directory), so a cross-project view must discover projects first.
- workmux windows are named with a configurable prefix (default `wm-`) and are the source of truth for "what is running"; workmux's own `dashboard` TUI already spans all sessions for this reason.

Verified runtime data shapes (workmux 0.1.234):
- `workmux list --json` → per worktree: `handle, branch, path, is_main, mode, has_uncommitted_changes, is_open, created_at`.
- `workmux status --json --git` → per *active* agent: `worktree, branch, status (working|done|waiting), elapsed_secs, title, agent_kind, session, window_name, updated_ts, git{has_staged, has_unstaged, has_unmerged_commits}`.
- `workmux capture <name>` → recent agent terminal output (plain text).
- Actions: `workmux open <name>`, `workmux close <name>`, `workmux remove <name> [-f]`, `workmux send <name> <text>`.

## Goals / Non-Goals

**Goals:**
- A single, cross-project view of every workmux worktree and active agent on the machine (the core differentiator vs. per-project `workmux list`).
- Stay a thin wrapper over the workmux CLI so we never drift from workmux's own lifecycle behavior.
- One self-contained binary: embedded web UI + JSON HTTP API, bound to loopback, with live-ish refresh.
- Safe actions: destructive operations (remove) gated behind explicit confirmation and an uncommitted-changes warning.

**Non-Goals:**
- Branch merging (deferred; not selected for this change).
- Creating new worktrees.
- Multi-user access, authentication, or network exposure.
- Remote/SSH targets.
- Reimplementing workmux's agent-status tracking (we rely on workmux's own hooks).

## Decisions

### 1. Integrate by shelling out to the workmux CLI
We invoke the `workmux` binary as a subprocess for every read and write, rather than parsing internal state or re-implementing workmux logic.

- **Why:** workmux is a Rust binary with no Go-importable API and no stable state file. Its CLI is the documented, version-robust contract. Subprocess isolation keeps us decoupled from workmux internals.
- **Mechanics:** `list`/`status` are CWD-scoped, so we run them with the working directory set to each discovered project root. `capture`/`open`/`close`/`remove`/`send` are scoped to a worktree name + the project CWD.
- **Alternatives considered:** reading workmux state files (none exist / unstable); re-implementing git-worktree + tmux logic in Go (duplicates workmux, drifts); Rust FFI bindings (heavy, brittle). The CLI subprocess wins on simplicity and coupling.

### 2. Discover projects via the tmux server, then enrich per project
Enumerate the tmux server (`tmux list-panes -s`) to collect every window's name and `pane_current_path`. Workmux windows (matching the configured prefix, default `wm-`) yield worktree paths; resolve each to its project root and dedupe. For each project root, run `workmux list --json` and `workmux status --json --git` to build the record set.

- **Why:** the tmux server is the only global, cross-session, cross-project source of "what is running," and it is exactly what workmux's own dashboard relies on. It also surfaces projects that the server process was not started inside.
- **Fallbacks:** also merge in `workmux list` from the CWD the server started in, and match any window whose path resolves to a git worktree (to tolerate a non-default window prefix). If tmux is absent, report "no running worktrees" with a readable reason.
- **Alternatives considered:** a single `workmux list` from one CWD (sees only one project); a hardcoded project list (brittle); a full filesystem git-repo scan (slow, noisy). Tmux enumeration + per-project enrichment wins.

### 3. Data model = join `list` (inventory) with `status` (active agents) per project
`list` is the full inventory (including closed/stale worktrees); `status --json --git` is only the active agents with status and git state. We join them by worktree handle per project so active worktrees carry agent status and inactive ones do not. `capture` is fetched **lazily** for the selected worktree only (it is comparatively heavy), never for the whole inventory.

- **Why:** matches how workmux itself splits the two concerns, and keeps the always-on polling cheap (only `list` + `status`, not `capture`).

### 4. Delivery: single Go binary, `embed` UI, stdlib `net/http`, loopback bind
Use Go's `embed` package to bake a static HTML/JS dashboard into the binary, serve it and the JSON API with the standard library `net/http` + `encoding/json`, and bind to `127.0.0.1` by default (with an explicit host/port override flag for later use).

- **Why:** satisfies "a web tool in golang" + the single-binary goal with zero build-step or frontend-install dependencies for the user. The endpoint surface is small, so a web framework adds little.
- **Alternatives considered:** a React/Next frontend + Go API (extra build step, contradicts single-binary); a Go web framework such as chi/fiber (optional nicety — adoptable later if routing grows). stdlib + embed wins for v1.

### 5. Live refresh = client-side polling
The UI polls the JSON inventory endpoint at a short, configurable interval (default ~3–5s); the per-worktree output view polls `capture` on its own cadence while open.

- **Why:** workmux/tmux expose no push, webhook, or stream to an external process, and agent output is a terminal buffer. Polling is simple, stateless, and adequate for a local single-user dashboard.
- **Alternatives considered:** WebSocket/SSE fan-out (needs a server-side poll loop + broadcast; more complexity for marginal benefit here); `workmux wait` (blocking — wrong model). Polling wins for v1; SSE can be layered on if polling feels too coarse.

### 6. Actions reuse workmux commands; destructive ops are confirmed
Each UI action maps to the corresponding `workmux` command run with the target project's CWD. `remove` is gated behind an explicit confirmation and, when the worktree has uncommitted changes, a specific "uncommitted changes will be lost" warning.

- **Why:** reuses workmux's correct lifecycle behavior (hooks, file ops, branch cleanup) instead of re-implementing it, and keeps the only mutating, irreversible operation behind a guardrail.

## Risks / Trade-offs

- **workmux JSON output changes across versions** → Parse tolerantly (ignore unknown fields), validate the few fields we depend on, and surface a clear error naming the missing field. Detect the workmux version at startup (`workmux --version`) and report incompatibility.
- **Agent status depends on workmux hooks being installed** (`workmux setup`) → When `status` returns nothing for a worktree that is clearly running, present the worktree as "running, status unknown" and hint at `workmux setup` rather than dropping it.
- **Per-project `list`/`status` can be slow with many projects** → Read projects concurrently with a bounded worker pool; briefly cache per-project results; keep `capture` lazy.
- **Non-default `window_prefix` or tmux absent** → Read the prefix from workmux config when available; fall back to matching windows by worktree path; report "no running worktrees" clearly.
- **Command-injection surface when passing user input to argv** → Always pass user-controlled values as discrete argv elements (never through a shell string) and validate worktree names against a strict pattern before use.
- **Destructive `remove` from a browser** → Explicit confirmation + uncommitted-changes warning + no auto-run + an action log.
- **Polling cost on a busy machine** → Short interval, bounded concurrency, lazy `capture`, configurable interval.
- **Loopback-only may limit later cross-device use** → Keep an explicit host/port override flag, but default stays `127.0.0.1`.

## Migration Plan

Nothing to migrate — this is a standalone tool with no data store or schema. Deploy: `go build` then `gittreemux serve`. Rollback: stop the process / remove the binary. It has no effect on workmux, tmux, or the user's repos beyond the read operations; the only mutation is `remove`, which runs solely on explicit user action.

## Open Questions

- Default poll interval and the cap on concurrent project reads — pick sensible defaults and expose them as flags; deferrable without changing the specs.
- Whether to surface non-workmux worktrees that `workmux list` also reports (e.g. `.claude/worktrees/*`) — current assumption is to include them in the inventory (workmux reports them) but highlight only workmux-managed/active worktrees; can be refined after first use.
- Frontend styling/JS approach (vanilla JS vs. a small library) — a design detail deferrable to implementation; default is vanilla JS under `embed`.
