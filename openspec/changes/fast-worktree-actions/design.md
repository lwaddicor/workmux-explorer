## Context

See proposal.md for motivation and measurements. The relevant current state:

- Every handler in `internal/api` resolves its target via `findProject`, which calls
  `Discoverer.Inventory` — a full cross-project rebuild: `tmux list-panes -s`, one
  sequential `git rev-parse --git-common-dir` per pane, `workmux --version`, then
  per root (concurrency-bounded) `workmux list --json` followed by *sequential*
  `workmux status --json --git`. Results are cached per root for only 2 s while the
  UI polls every 4 s by default, so action requests almost always miss the cache.
- `internal/exec.Run` is an unbounded `cmd.Run()`: no deadline anywhere, so a wedged
  subprocess hangs its request indefinitely and outlives it (observed: explorer-spawned
  `workmux list --json` processes alive for hours, holding repo locks that make every
  fresh workmux command in those repos wait ~45–50 s).
- `web/app.js` awaits a full inventory (`load()`) after every action before re-rendering.
- `internal/focus.topLevelComm` spawns one `ps -o ppid=,comm= -p <pid>` per ancestor
  level (up to 64 sequential spawns) to walk the process tree.
- Constraints: Go stdlib only, loopback-only server, hermetic unit tests (no real tmux,
  network, or OS activation), single embedded web UI with no build step.

## Goals / Non-Goals

**Goals:**
- Actions on one worktree resolve and execute against that project alone; unrelated
  projects' read latency cannot delay them.
- Every external command runs under a deadline; expired reads degrade to a per-project
  error, and expired processes are killed rather than leaking.
- Full inventory becomes bounded (~one read-timeout worst case) and cheaper in the common
  case (concurrent per-root reads; brief reuse of roots/version/per-project results).
- UI reflects an action's outcome immediately for the affected worktree, reconciling via
  a targeted single-project fetch instead of a machine-wide rescan.

**Non-Goals:**
- Changing `workmux` itself: its per-worktree cold `git status --porcelain` cost in huge
  repos (the observed ~50 s worst case) is upstream; this change bounds and isolates the
  dashboard's exposure to it, not removes it.
- Push-based updates (SSE/WebSocket); polling stays the refresh mechanism.
- New dependencies, public binding, or any UI framework/build step.

## Decisions

- **Timeouts at the exec layer.** Add `exec.RunCtx(ctx, dir, name, args ...string)
  Result` built on `os/exec.CommandContext`; when the context expires the process is
  killed and the result carries a new `TimedOut bool` (kept distinct from `Err`, which
  still means "could not start", so existing `workmux.ErrNotInstalled` mapping stays
  correct). Existing `Run` becomes a no-deadline wrapper. New serve flag `-timeout`
  (default 60 s) wires the deadline into: workmux reads (`list`, `status`, `capture`)
  and actions (`open`, `close`, `remove`, `send`), discovery probes (`git rev-parse`,
  `tmux list-panes`/`list-clients`), and the version check. Rationale for one knob: a
  single predictable number to tune; 60 s sits above the measured ~50 s worst-case cold
  read, so even pathological repos stay representable in the inventory instead of
  degrading permanently, while still bounding what used to be unbounded waits. The focus
  activation chain (tmux/ps/osascript) uses a fixed internal 15 s deadline regardless of
  the flag — macOS automation-permission prompts can block `osascript` until a human
  answers, and a focus click should not wait a full minute for that. On Unix the
  deadline kill targets the command's own process group (a fresh `Setpgid` group,
  SIGKILL on expiry) so no descendant outlives the query; on platforms without
  process groups the direct process is killed, and the exec package keeps
  cross-compiling. The focus fallback pane scan (`tmux list-panes` in `handleFocus`)
  runs under the request context with the same fixed 15 s bound, so a wedged tmux
  server cannot hang a focus click either.

- **Scoped project read in discover.** New method on `*Discoverer`,
  `Project(ctx context.Context, name string) (*workmux.Project, error)`: (1) match
  `name` against cached entries by project name or root path — fresh within TTL → return
  as-is; stale → re-read just that one root via the existing `readProject` and store.
  (2) No cache hit → run `discoverRoots()` once, find the matching root, read it.
  (3) Otherwise error "project not found". This replaces the body of
  `findProject`; handlers keep their exact status-code behavior (404 unknown project /
  worktree, 409 precondition failures). Worst case for an action is one discovery pass +
  one project read (~1–2 s typical) instead of all projects; common case (a poll happened
  recently) is cache-only.

- **Faster full inventory.** Per root, run `workmux list` and `workmux status --git` in
  parallel goroutines and join the results (semantics preserved: a failed `list` yields
  an empty project with that error; a failed `status` flags the error but keeps the
  listed worktrees). Cache the discovery root set + tmux availability, and the workmux
  version check, for a short window (same TTL) so repeated inventories skip re-probing.
  Bump the default `-cache-ttl` from 2 s to 5 s — it must not be shorter than the UI's
  poll interval or actions will always miss; the flag already exists and users can tune
  both sides independently.
- **Post-mutation invalidation.** A successful remove/open/close drops the addressed
  project's cached record before the handler responds (a new `Discoverer.Invalidate`,
  exposed on the `projectSource` seam the handlers use). Without it the UI's post-action
  reconciliation `GET` is answered from the still-fresh pre-action snapshot: with the
  default 5 s TTL and 4 s poll a poll that ran moments before the action keeps a removed
  worktree's card resurrected — and a window-state badge reverted — for up to the TTL,
  deterministically, so the targeted fetch that is supposed to confirm the action instead
  undoes the visible change. Time-based reuse stays in force for ordinary reads; a
  mutation invalidates on top of the TTL (see the inventory delta spec).

- **Single-project API endpoint.** New route `GET /api/projects/{project}` →
  `handleProject` using `Discoverer.Project`; returns `{ "project": <Project> }` (same
  envelope style as `handleWorktree`) or 404 with a readable error when the project is
  unknown. Additive; existing routes unchanged. The UI uses it for post-action
  reconciliation, and it also makes single-project output/worktree fetches fast since
  they share the scoped lookup.

- **Instant post-action UI update.** In `web/app.js`: on a successful action, mutate
  `lastInv` in memory (remove: drop the worktree record; open/close/focus/send: nothing
  structural) and re-render immediately — no network round-trip for the visible change.
   Then fetch the single-project endpoint and merge that project's fresh record into
   `lastInv`, re-rendering again so server truth wins. The merge and the 404-drop match
   the existing row by root (falling back to name), so two repositories sharing a base
   name cannot splice into each other's row. The fresh record is post-mutation because
   the server invalidates the project's cache on success, so a removed card does not
   return from the reconciliation; a card re-appears only if the removal genuinely raced
   with something restoring the worktree, and a fully vanished project is dropped from
   the sidebar with selection falling through to the next. Background full-inventory
   polling is untouched. No new dependencies or build step.

- **One `ps` snapshot for the focus tree walk.** `topLevelComm` runs a single
  `ps -eo ppid=,pid,comm=` table read once and walks parent links in memory under the
  same depth bound (64). The injectable `Runner` signature is unchanged; existing focus
  tests are updated to supply the tabular form.

## Risks / Trade-offs

- [Killing a timed-out *action* command could in theory interrupt mid-mutation.] →
  Actions use the generous default (60 s) and are quick tmux/git operations; on timeout
  the user sees an explicit error and can retry, which is strictly better than today's
  unbounded hang plus leaked processes.
- [A 5 s cache TTL means the UI can be up to ~5 s stale between polls.] → Polling still
  refreshes at its own interval; staleness is bounded and user-tunable on both sides via
  existing flags. Today's effective behavior (2 s TTL, 4 s poll) already interleaved
  full re-reads, so perceived freshness does not regress in practice.
- [Parallel `list`+`status` per root doubles peak concurrent subprocesses.] → Bounded by
  the existing `-concurrency` flag (default 8 roots × 2 = ≤16 local processes); negligible
  on a workstation.
- [The scoped lookup's name matching must stay consistent with full-inventory naming or
  actions could target the wrong root when two projects share a base name.] → Reuse the
  exact existing match rule (name, root path, base of root) and keep `findProject`'s
  first-match ordering; ambiguity is no worse than today's behavior.
- [Optimistic UI removal can briefly show state that differs from the server.] → The
  immediate target fetch reconciles within ~1–2 s and re-render restores truth on any
  race; polling remains the backstop.

## Migration Plan

- Additive only: new method, new route, one new flag with a safe default, UI behavior
  change behind no config. No persisted state, no data migration.
- Rollback = revert; nothing else to clean up.

## Open Questions

- Whether `-timeout` should also scale the action deadline (currently fixed at the same
  value as reads). Kept simple: one knob applies to both; revisit only if users report
  slow removes timing out.
