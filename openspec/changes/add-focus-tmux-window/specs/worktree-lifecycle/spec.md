## ADDED Requirements

### Requirement: Focus an already-open worktree's tmux window
The system SHALL allow the user to focus a worktree whose tmux window is already
open, so that the terminal hosting that window becomes immediately usable.
Focusing a worktree SHALL switch to that worktree's tmux window (selecting its
tab) and SHALL bring the terminal application hosting that tmux session to the
foreground of the operating system. The system SHALL report the outcome of the
bring-to-front step, distinguishing a successful activation from a best-effort
case in which the terminal could not be brought forward. The focus action SHALL
target only a worktree whose window is open.

#### Scenario: Focus brings an open window's terminal to the front
- **WHEN** the user selects focus for a worktree whose window is open and a
  terminal is attached to its tmux session
- **THEN** the system selects that worktree's tmux window, brings the hosting
  terminal application to the foreground, and reports that the terminal was
  brought to the front

#### Scenario: Focus degrades when no terminal is attached
- **WHEN** the user selects focus for a worktree whose tmux session has no
  attached terminal
- **THEN** the system still selects that worktree's tmux window and reports that
  it could not bring a terminal to the foreground

#### Scenario: Focus is unavailable for a closed window
- **WHEN** the user requests focus for a worktree whose window is not open
- **THEN** the system reports that the window is not open and performs no
  bring-to-front
