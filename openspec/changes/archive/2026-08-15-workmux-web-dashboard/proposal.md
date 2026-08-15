## Why

workmux pairs git worktrees with tmux windows to run many AI agents in parallel, but its built-in views are per-project and terminal-bound (`workmux list`, the `workmux dashboard` TUI). When several projects each have multiple worktree agents running, there is no single, glanceable, cross-project place to see *everything that is currently running on the machine* and act on it. This change adds a local web dashboard that surfaces every workmux worktree and agent across all projects and lets you manage them from the browser.

## What Changes

- A new Go binary `gittreemux` that discovers every workmux project/worktree/agent running locally and presents one unified, cross-project view.
- A global read model per worktree: branch, agent status (working / done / waiting), agent kind, elapsed time, task title, git state (staged / unstaged / unmerged commits), and a live terminal-output preview.
- Browser-driven actions on a worktree: open its tmux window, close its tmux window, send a prompt to its running agent, and remove the worktree (worktree + window + branch) behind a confirmation.
- Delivered as a single self-contained binary: embedded web UI plus a JSON HTTP API, bound to localhost, with live refresh.
- The tool is a thin layer over the existing toolchain — it shells out to the `workmux` CLI (`list`, `status --json`, `capture`, `open`, `close`, `remove`, `send`) and to `tmux` for cross-project discovery. It does not modify workmux.

Non-goals (v1): branch merging, creating new worktrees, multi-user or network authentication, and remote/SSH targets.

## Capabilities

### New Capabilities

- `worktree-inventory`: Cross-project discovery and a unified read model of all workmux worktrees and their active agents (status, branch, git state, live output).
- `worktree-lifecycle`: Open or close a worktree's tmux window, and remove a worktree (worktree + window + branch) with confirmation.
- `agent-interaction`: Send a prompt to a running agent and view its live terminal output.
- `web-dashboard`: Deliver the overview and controls as a single Go binary with an embedded web UI and a JSON API, bound to localhost, with live refresh.

### Modified Capabilities

(none — this is a greenfield project with no existing specs)

## Impact

- **New code**: a Go module `gittreemux` — a `serve` CLI command, an HTTP server, an embedded web UI, and adapters that shell out to `workmux`, `tmux`, and `git`.
- **Runtime dependencies**: the `workmux` CLI and `tmux` must be installed, with a tmux server running, on the local machine; `git` supplies worktree/branch metadata.
- **APIs**: a new local JSON HTTP API under `/api/...` (inventory, per-worktree detail, actions) plus the embedded HTML/JS dashboard.
- **Systems touched**: the tmux server (read windows, open/close), workmux (read status, send, remove), and git (worktree/branch state). Read-mostly; the only destructive action is `remove`, guarded by an explicit confirmation.
- **Security**: binds to `127.0.0.1` by default; single-user with no auth in v1.
