## Why

The dashboard can open and remove a worktree's tmux window, but once a window is
already open there is no way from the browser to bring that specific terminal to
the front and land on its tab. Today the user must manually switch to the
terminal app and hunt for the right window. A single "Focus" action should take an
already-open worktree and make it immediately usable: switch to its tmux window
(the correct tab) and bring the terminal application hosting it to the OS
foreground.

## What Changes

- Add a **Focus** action for a worktree whose window is already open: switch to
  that worktree's tmux window (the "correct tab") and bring the terminal
  application hosting that tmux session to the front of the OS, so it can be used
  immediately.
- Add a **Focus** button to each worktree card in the web UI. It is the primary
  "go to it" control, is distinct from "Open window" (which also handles closed
  windows), and is shown only when the worktree's window is open.
- Add a JSON API endpoint for the Focus action, alongside the existing
  open/close/send/remove endpoints.
- Best-effort host-terminal detection and activation on **macOS**: detect the
  terminal app attached to the target tmux session and activate it. If the host
  app cannot be determined (e.g. detached session, or an unsupported OS), still
  perform the tmux window switch and report the activation outcome rather than
  failing the whole action.
- No changes to loopback binding, the set of dependencies, or the existing
  open/close/send/remove behavior.

## Capabilities

### New Capabilities
- (none)

### Modified Capabilities
- `worktree-lifecycle`: add a new requirement — Focus an already-open worktree's
  tmux window (switch to its window and bring the hosting terminal to the OS
  foreground), with graceful degradation when no terminal can be brought forward.

## Impact

- `internal/workmux`: reuse `Open` (open or switch to the tmux window) for the
  "correct tab" step.
- New package `internal/focus`: detect the terminal app hosting a tmux session
  and activate it (macOS via `osascript`, best-effort; no-op activation on other
  platforms). Stdlib only.
- `internal/tmux`: expose the target session/window and the attached client tty
  for the focus step.
- `internal/api`: new `focus` handler and route; wire it into the action
  dispatcher; surface the activation outcome in the response.
- `web/`: new Focus button (HTML/JS/CSS), enabled only when the window is open;
  calls the new endpoint and surfaces the activation status.
- Tests: hermetic unit tests that stub the workmux binary and fake tmux output,
  following the approach in `internal/workmux`. No real tmux or network in unit
  tests.
- No new dependencies; Go stdlib only.
