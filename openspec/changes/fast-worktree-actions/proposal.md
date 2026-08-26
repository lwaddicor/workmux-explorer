# Proposal: fast-worktree-actions

## Why

Every dashboard action (open/close/focus/send/remove) currently rebuilds the full
cross-project inventory before doing anything, so a single slow or wedged project on
the machine blocks all actions for tens of seconds to minutes. Measured live: one
inventory request took 60+ s because `workmux list` in a ~290k-file docs worktree
takes ~50 s per cold scan; the UI then waits for a *second* full inventory after each
action before re-rendering, and unbounded subprocess execution lets wedged workmux
processes (observed alive 6+ hours) make things worse. Actions on one worktree must
not depend on the read latency of every other project.

## What Changes

- **Scoped action lookup**: open/close/focus/send/remove resolve their target project
  by re-reading only that one project root, instead of rebuilding the whole inventory.
- **Bounded subprocess execution**: all external commands (workmux reads/actions, git
  discovery probes, tmux queries, ps, osascript) run under a timeout; a timed-out or
  wedged read degrades to a per-project error instead of hanging requests and leaking
  long-lived processes. Read timeouts are configurable via a serve flag.
- **Faster full inventory**: `workmux list` and `workmux status --git` for one project
  run concurrently; the discovered root set, workmux version check, and per-project
  results are cached briefly with a TTL that is not shorter than the UI's minimum poll
  interval.
- **New single-project API endpoint** (`GET /api/projects/{project}`) returning just
  that project's record, so the UI can refresh one repository after an action instead
  of the whole machine.
- **Instant post-action UI update**: on remove (and other actions) the affected card is
  updated/removed from the current view immediately and reconciled with a targeted
  single-project fetch; full inventory polling continues in the background.
- **Cheaper focus activation**: the terminal-app process-tree walk uses one `ps` table
  snapshot instead of one `ps` spawn per ancestor level.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `worktree-lifecycle`: actions (open, close, focus, send, remove) are scoped to the
  addressed project and complete without waiting for reads of unrelated projects; a
  single-project record is retrievable over the API.
- `worktree-inventory`: per-project reads are bounded in time — a read that exceeds its
  deadline is flagged on that project instead of blocking the whole inventory, and
  repeated queries reuse recent results (roots, version, per-project) briefly.
- `web-dashboard`: after an action, the grid reflects the change immediately for the
  affected worktree and reconciles via a targeted refresh, without waiting for a full
  machine-wide rescan; polling continues to reflect live state as before.

## Impact

- **Code**: `internal/exec` (timeout-aware execution), `internal/workmux` (per-command
  timeouts), `internal/discover` (scoped project read, root/version caching, concurrent
  per-root reads, TTL default), `internal/api` (handlers use scoped lookup; new
  single-project route), `cmd/workmux-explorer` (timeout flag), `web/app.js` (instant
  post-action update + targeted refresh).
- **API**: additive — one new GET route; existing routes keep their shapes.
- **Behavior**: full inventory can take up to ~one read-timeout instead of unbounded;
  a project whose read exceeds the deadline shows a readable error for that poll cycle
  only (previously it blocked every request indefinitely). No breaking changes, no new
  dependencies.
- **Out of scope**: workmux itself (its `--no-optional-locks` per-worktree git-status
  cost on huge repos is an upstream concern; noted here as the observed worst case but
  not modified by this change).
