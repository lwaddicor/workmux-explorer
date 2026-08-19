## 1. Host-terminal focus package (`internal/focus`)

- [x] 1.1 Create `internal/focus` with an injectable command runner
  (`func(dir, name string, args ...string) exec.Result`, defaulting to
  `exec.Run`) and a `Result{App string, Activated bool, Note string}` type.
- [x] 1.2 Implement `ActivateSession(session string) Result`: read the session's
  tmux client PID (`tmux list-clients -t <session> -F #{client_pid}`); if none,
  return `Activated:false` with a "detached / no attached terminal" note.
- [x] 1.3 Walk the client PID up to its launchd ancestor (`ps -o ppid=,comm= -p
  <pid>` until ppid is 1) to identify the terminal app; map known `comm` names
  (Terminal, iTerm2, Ghostty, Alacritty, kitty, Warp, WezTerm, Hyper) to display
  names with a capitalized-comm fallback; activate via
  `osascript -e 'tell application "<App>" to activate'` (darwin only — other OSes
  return `Activated:false` with a "not supported" note).
- [x] 1.4 Validate the client PID is a positive integer before calling `ps`, and
  escape the app name when building the AppleScript string (no shell, discrete
  argv).
- [x] 1.5 Add hermetic unit tests with a fake runner: attached→app detected and
  `osascript` argv asserted; detached (no client)→`Activated:false` + note;
  non-darwin→`Activated:false` + note; malformed/missing PID→graceful note.

## 2. API endpoint (`internal/api`)

- [x] 2.1 Add route `POST /api/projects/{project}/worktrees/{handle}/focus` and a
  `handleFocus` handler wired into `Routes()`.
- [x] 2.2 In `handleFocus`: resolve the target session (use `wt.Agent.Session`
  when present, else best-effort match window `wm-<handle>` via
  `tmux.ListPanes()`), then run `s.Workmux.Open(p.Root, handle)` for the window
  switch.
- [x] 2.3 Respond `409` when the worktree's window is not open; `502` when the
  window switch fails; `200` with `{ok, action:"focus", project, handle,
  activated, app, note}` for best-effort outcomes; log via the existing `s.log`.
- [x] 2.4 Add a hermetic handler test (httptest + stub workmux client + fake
  focus runner) covering the 409 (not open), 502 (switch failed), and 200
  best-effort (`activated:false` note) paths.

## 3. Web UI (`web/`)

- [x] 3.1 Add a **Focus** button in `web/app.js` `renderWorktree`, rendered only
  when `w.is_open`, and wire it in `onWorktreeClick` to POST the focus endpoint.
- [x] 3.2 On the focus response, surface a brief non-blocking notice when
  `activated` is false, showing the returned `note`.
- [x] 3.3 Style the Focus button and the notice in `web/style.css` to match
  existing controls (no framework, no build step).

## 4. Verification

- [x] 4.1 `gofmt -l .` returns nothing; `go vet ./...` passes; `go build ./...`
  succeeds.
- [x] 4.2 `go test ./...` passes with no real tmux, network, or OS activation.
- [x] 4.3 Manual smoke on a worktree port (`go run ./cmd/workmux-explorer serve -listen
  127.0.0.1:8788`): click Focus on an open worktree and confirm the terminal
  comes to the front; confirm the best-effort notice appears when it cannot.

## 5. Finish

- [x] 5.1 Commit with a conventional commit, e.g.
  `feat: add Focus action to bring an open worktree window to the front`.
