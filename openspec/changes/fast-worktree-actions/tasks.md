## 1. Timeout-aware execution layer (internal/exec)

- [x] 1.1 Add `exec.RunCtx(ctx, dir, name string, args ...string) Result` using `os/exec.CommandContext`; on context expiry kill the process and set a new `Result.TimedOut bool` (keep `Err` reserved for launch failures so `workmux.ErrNotInstalled` mapping is unaffected); keep existing `Run` as a no-deadline wrapper over it
- [x] 1.2 Unit tests: a command that exceeds its context deadline returns `TimedOut=true` with the process killed; a normal command is unchanged

## 2. Workmux client timeouts (internal/workmux)

- [x] 2.1 Give `Client` a `Timeout time.Duration` field (zero = no deadline) and route every operation (`list`, `status`, `capture`, `open`, `close`, `remove`, `send`, `--version`) through `exec.RunCtx`; map `TimedOut` to a descriptive error ("timed out after <d>")
- [x] 2.2 Unit tests: with a stub binary that sleeps, a short timeout yields the timed-out error and does not report "not installed"; existing stub-binary tests still pass

## 3. Scoped project read + faster inventory (internal/discover)

- [x] 3.1 Add `Discoverer.Project(ctx, name)` — fresh cache hit returns as-is; stale hit re-reads only that root via `readProject`; no hit runs `discoverRoots()` once and reads the matching root; otherwise "project not found"
- [x] 3.2 Run per-root `workmux list` and `workmux status --git` concurrently in `readProject`, preserving current error semantics (failed list → empty project with that error; failed status → flag error, keep listed worktrees)
- [x] 3.3 Cache the discovery root set + tmux availability and the workmux version check for one TTL window so repeated inventories skip re-probing; run all discovery probes (`tmux list-panes -s`, per-pane `git rev-parse`) under the request context with the client timeout
- [x] 3.4 Raise the default per-project cache TTL from 2 s to 5 s (flag already exists) and unit-test: scoped read returns cached fresh result without re-reading; stale result is re-read once; unknown project errors

## 4. API surface (internal/api, cmd/workmux-explorer)

- [x] 4.1 Switch `findProject` to the scoped `Discoverer.Project` lookup (handlers keep their status-code behavior: 404 unknown project/worktree, 409 preconditions); update the `inventoryProvider` test seam so existing handler tests compile and pass
- [x] 4.2 Add route `GET /api/projects/{project}` (`handleProject`) returning `{ "project": <Project> }` or a readable 404; unit test for both outcomes
- [x] 4.3 Wire a new `-timeout` flag (default 60 s) in `cmd/workmux-explorer` into the workmux client and discovery, and update usage text

## 5. Focus activation hardening (internal/focus)

- [x] 5.1 Replace the per-level `ps` spawns in `topLevelComm` with a single `ps -eo ppid=,pid=,comm=` table read + in-memory walk under the same depth bound; apply a fixed internal deadline to the activation chain
- [x] 5.2 Update focus unit tests for the tabular `ps` form and add a case where the chain exceeds the depth bound

## 6. Instant post-action UI update (web/app.js)

- [x] 6.1 On successful remove: drop the worktree from `lastInv`, re-render immediately, then fetch the single-project endpoint and merge its fresh record into `lastInv` (re-select another project if this one vanished); on failure keep current alert behavior
- [x] 6.2 Apply the same immediate-re-render + targeted single-project reconciliation to open/close/focus/send success paths; leave background polling untouched

## 7. Verification

- [x] 7.1 `go build ./...`, `go vet ./...`, `gofmt -l .` (empty), `go test ./...` all clean
- [x] 7.2 Live smoke on a spare port (`serve -listen 127.0.0.1:8788`): time `GET /api/projects/{project}` and an action against one project while the slowest repo is present — confirm scoped endpoints return in seconds, full inventory stays bounded by `-timeout`, and no workmux subprocess outlives its deadline

## 8. Review fixes

- [x] 8.1 Drop the addressed project's cached record after a successful remove/open/close (`Discoverer.Invalidate`; `Invalidate` added to the `projectSource` seam and its test fake; the remove and open/close handlers call it only on success) so the UI's post-action reconciliation reads post-mutation state instead of a still-fresh pre-mutation snapshot that resurrected removed worktrees for up to the TTL — unit-tested in discover (force re-read within TTL) and api (invalidation on success, none on failure)
- [x] 8.2 Restore cross-platform builds of `internal/exec`: the process-group deadline kill moved behind `//go:build unix` with a no-op fallback, so `GOOS=windows go build ./...` passes again (the unconditional `Setpgid`/`syscall.Kill` broke Windows)
- [x] 8.3 Bound the focus fallback's `tmux list-panes` scan with the request context plus a fixed 15 s internal deadline, so a wedged tmux server cannot hang a focus request
- [x] 8.4 `tmux.ListPanesCtx` accepts a query that exits 0 even when the deadline fired in the same instant instead of discarding the valid output as `ErrNotRunning`; first hermetic tests for the tmux package (parse, no-server, timeout)
- [x] 8.5 The web UI matches the reconciled row by project root (name fallback) so two repositories sharing a base name cannot splice into each other's row
