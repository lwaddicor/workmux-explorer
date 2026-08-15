## 1. Project scaffold

- [x] 1.1 Initialize the Go module `gittreemux` with a `main` package exposing a `serve` subcommand stub
- [x] 1.2 Establish the layout (`cmd/`, `internal/`, `web/`) and confirm `go build ./...` produces a single binary
- [x] 1.3 Confirm `go build ./...` and `go vet ./...` pass using the standard library only (no external deps)

## 2. workmux + tmux adapters

- [x] 2.1 Add a subprocess helper that runs an external command with an explicit working directory and returns stdout/stderr/exit code, without shell interpolation
- [x] 2.2 Implement `workmux list --json` and parse it into a Worktree model
- [x] 2.3 Implement `workmux status --json --git` and parse it into an AgentStatus model
- [x] 2.4 Implement a lazy `workmux capture <name>` that returns recent agent output
- [x] 2.5 Implement action wrappers for `open`, `close`, `remove [-f]`, and `send`, each scoped to the project working directory and validating the worktree name
- [x] 2.6 Implement tmux discovery via `tmux list-panes -s` returning every window's name and current path across all sessions
- [x] 2.7 Add workmux version detection (`workmux --version`) with a clear message when workmux is missing or incompatible

## 3. Cross-project discovery + data model

- [x] 3.1 Discover project roots from tmux windows (match the workmux prefix or resolve the worktree path) and dedupe (spec: worktree-inventory)
- [x] 3.2 Merge in `workmux list` from the server's start directory as a fallback source of projects
- [x] 3.3 Build the unified per-worktree record by joining `list` with `status` per project, so active worktrees carry agent status and inactive ones do not (spec: worktree-inventory)
- [x] 3.4 Group worktrees by project and mark each as running/active vs. closed
- [x] 3.5 Isolate per-project read failures so one unreadable project is reported as an error without failing the whole inventory (spec: worktree-inventory)
- [x] 3.6 Read projects concurrently with a bounded worker pool and a brief per-project result cache

## 4. HTTP API

- [x] 4.1 Set up a stdlib `net/http` handler bound to 127.0.0.1 by default, with host/port override flags (spec: web-dashboard)
- [x] 4.2 Add `GET /api/projects` returning the cross-project inventory as JSON (spec: web-dashboard)
- [x] 4.3 Add `GET /api/projects/{project}/worktrees/{handle}` returning a single worktree record
- [x] 4.4 Add `GET /api/projects/{project}/worktrees/{handle}/output` returning recent agent output via lazy capture (spec: agent-interaction)
- [x] 4.5 Add `POST` endpoints for `open`, `close`, `send`, and `remove` that invoke the matching workmux command and return success or a structured error (spec: worktree-lifecycle, agent-interaction)
- [x] 4.6 Serve the embedded web UI and its assets at `/` (spec: web-dashboard)
- [x] 4.7 Return structured JSON errors (HTTP status + message) for missing dependencies, no running worktrees, and action failures (spec: web-dashboard)

## 5. Web UI (embedded)

- [x] 5.1 Add an `embed` static UI (HTML/CSS/JS) with a project → worktree list view (spec: web-dashboard)
- [x] 5.2 Render each worktree's branch, status, agent kind, elapsed time, task title, git state, uncommitted-changes flag, and creation time (spec: worktree-inventory)
- [x] 5.3 Add a detail/output panel that shows recent agent output and refreshes while it is open (spec: agent-interaction)
- [x] 5.4 Add action buttons for open window, close window, send prompt, and remove (spec: worktree-lifecycle, agent-interaction)
- [x] 5.5 Gate the remove action behind an explicit confirmation dialog, with a specific warning when uncommitted changes exist (spec: worktree-lifecycle)
- [x] 5.6 Poll the inventory at a configurable interval (default ~3–5s) and update the view without a full page reload (spec: web-dashboard)
- [x] 5.7 Show a clear empty/degraded state when tmux or workmux is absent or no worktrees are running (spec: web-dashboard)

## 6. Safety, robustness, verification

- [x] 6.1 Validate worktree names against a strict allowlist pattern before passing them to any workmux command
- [x] 6.2 Add a lightweight action log recording which action, which worktree, when, and the result
- [x] 6.3 Write unit tests for the `list` and `status` JSON parsers using captured sample payloads, and for worktree-name validation
- [x] 6.4 Verify end-to-end against a real tmux + workmux setup: the inventory reflects running worktrees and open/close/send/remove behave as specified
- [x] 6.5 Build the single binary and confirm it starts the dashboard on 127.0.0.1 with no external installation
