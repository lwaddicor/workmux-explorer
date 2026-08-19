## Context

See proposal.md for motivation. The relevant current state:

- The dashboard's actions run through `internal/api` handlers that shell out
  via `internal/workmux` (a thin CLI client) and `internal/tmux` (reads the live
  tmux server). `handleOpen` already calls `workmux.Client.Open`, which runs
  `workmux open <handle>` — workmux's own "open or switch to tmux window".
- `workmux.Client.Open` already performs the "switch to the correct tab" step.
  What is missing is bringing the **terminal application** that hosts the tmux
  session to the OS foreground, so the window is actually visible from the
  browser.
- `internal/exec.Run(dir, name, args...)` runs a command with no shell and
  returns `{Stdout, Stderr, ExitCode, Err}`. `internal/tmux.ListPanes()`
  exposes `Session`/`WindowName` per pane. `AgentStatus` (from `workmux
  status --json`) carries the agent's `Session` and `WindowName`.
- Constraints: Go stdlib only (no new deps), loopback-only server, hermetic unit
  tests (no real tmux / network / OS activation in tests), single self-contained
  binary.

## Goals / Non-Goals

**Goals:**
- A Focus action that switches to a worktree's already-open tmux window and
  brings the hosting terminal application to the OS foreground.
- Best-effort host-terminal detection and activation on macOS, with a clear
  report when the terminal cannot be brought forward (detached session, unknown
  app, unsupported OS).
- A Focus control in the UI shown only for open windows, and a matching JSON API
  endpoint alongside open/close/send/remove.
- Testable without a real tmux server or a real desktop session.

**Non-Goals:**
- Selecting the specific terminal app *window/tab* within a multi-tab app
  (app-specific; out of scope — activating the app plus selecting the tmux window
  is sufficient).
- Auto-attaching a detached tmux session (we report and stop).
- Linux/Windows window-manager integration in this change (the code is structured
  so a later platform can be added, but only macOS is implemented and specified).
- Any change to open/close/send/remove behavior, loopback binding, or
  dependencies.

## Decisions

- **Reuse `workmux.Client.Open` for the window switch (the "correct tab").**
  `workmux open <handle>` already resolves the window name from workmux's own
  config and switches to it, so the dashboard stays consistent with the existing
  Open action and keeps workmux as the source of truth. Alternatives considered:
  calling `tmux select-window` directly (rejected — would duplicate workmux's
  window-name/config logic in the dashboard).

- **Add a new `internal/focus` package for host-terminal detection + activation.**
  This is a distinct, macOS-specific concern; isolating it keeps `internal/api`
  and `internal/workmux` clean and makes it independently testable. It exposes,
  roughly: `ActivateSession(session string) Result` (resolve the hosting app and
  bring it forward), where `Result{App string, Activated bool, Note string}`.
  Platform gate: `runtime.GOOS == "darwin"` performs real work; other platforms
  return `Activated: false` with a note ("not supported on this platform") so the
  caller still succeeds on the window-switch step. Alternatives considered:
  putting it in `internal/tmux` (rejected — tmux package is about reading the
  live server; osascript/ps activation muddies its purpose).

- **Host-app detection: tmux client → launchd ancestor → activate.** Given the
  target session: (1) `tmux list-clients -t <session> -F #{client_pid}`; if empty,
  the session is detached and there is no host app (report and stop). (2) Take the
  client PID and walk the process parent chain (`ps -o ppid=,comm= -p <pid>`
  repeated) until the parent is `1` (launchd); the process whose parent is `1` is
  the top-level terminal application. (3) Map that `comm` to a display name (a
  small known set — Terminal, iTerm2, Ghostty, Alacritty, kitty, Warp, WezTerm,
  Hyper — with a capitalized-comm fallback) and run
  `osascript -e 'tell application "<App>" to activate'`. Rationale: this targets
  the *specific* session's terminal rather than blindly activating a guessed app,
  and the launchd-ancestor rule is terminal-agnostic. PIDs are validated to be
  positive integers before being passed to `ps` (defense in depth, matching the
  project's `ValidateName` posture).

- **Session resolution in the handler, not in `internal/focus`.** The handler has
  the worktree and tmux access; `internal/focus` stays free of workmux/discover
  dependencies. Resolution order: use `wt.Agent.Session` when the worktree has an
  agent; otherwise best-effort match the window `wm-<handle>` against
  `tmux.ListPanes()` to recover the `Session`. If no session can be determined,
  activation is skipped (reported) but the window switch still runs.

- **API surface.** New route `POST /api/projects/{project}/worktrees/{handle}/focus`
  and `handleFocus`. Behavior: 409 if the worktree's window is not open; run
  `workmux open` (window switch) — a failure here is a hard 502; then run
  `focus.ActivateSession` (best-effort); respond 200 with
  `{ok, action:"focus", project, handle, activated, app, note}`. Log the action
  via the existing `s.log` path.

- **UI.** In `web/app.js` `renderWorktree`, render a **Focus** button that is
  present only when `w.is_open` (otherwise the control is omitted), placed with
  the other window controls. Wire it in `onWorktreeClick` to call the focus
  endpoint; on a 200 where `activated` is false, surface a brief non-blocking
  notice with the `note` (e.g. "switched to the window; could not bring the
  terminal to the front: <note>"). No framework, no build step.

- **Testability.** `internal/focus` accepts an injectable command runner
  (a `func(dir, name string, args ...string) exec.Result`, defaulting to
  `exec.Run`) so unit tests can return canned `tmux`/`ps`/`osascript` output and
  assert the detected app and the `osascript` argv — mirroring how
  `internal/workmux` stubs its binary. No real tmux, desktop session, or network
  is touched in unit tests.

## Risks / Trade-offs

- [Host app detection is inherently best-effort and can misidentify the app in
  exotic setups (wrappers, tmux-in-tmux, non-standard launchers).] → The launchd
  ancestor rule and a known-app map cover the common cases; every failure degrades
  to "window switched, activation not performed" with a readable note rather than
  an error, so the core action is never broken by a detection miss.
- [The AppleScript `tell application "<App>" to activate` embeds the app name; a
  malicious or oddly-named process `comm` could break the script.] → App names
  come from `ps` (system-controlled, not API user input) and the API user only
  controls the `handle`, which is already validated by `ValidateName`; the app
  name is still sanitized/escaped when building the AppleScript string.
- [`client_pid` may point at a shell rather than the app directly.] → Resolved by
  walking up to the launchd ancestor instead of assuming a fixed depth.
- [macOS automation permissions may block `osascript` activation.] → Reported as a
  best-effort `note` (not an error); the window switch still succeeds, matching
  the spec's degradation requirement.

## Migration Plan

- Additive only: new package, new route, new button. No data migration, no change
  to existing endpoints or the loopback binding.
- Rollback = revert the commit(s); there is no persisted state introduced.

## Open Questions

- Preferred button label/position (proposed: "Focus", shown only when the window
  is open). Cosmetic; does not affect the spec or approach.
- Whether to also select the correct tmux *pane* within the window (deferred to a
  non-goal for this change; `workmux open` already lands on the configured
  focused pane).
