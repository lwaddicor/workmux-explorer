## Purpose

Let the user act on a worktree's tmux window and on the worktree itself from the dashboard, covering the safe window operations (open, close) and the destructive removal of a worktree.

## ADDED Requirements

### Requirement: Open a worktree's tmux window
The system SHALL allow the user to open, or switch to, the tmux window of a specified worktree.

#### Scenario: Open an existing worktree window
- **WHEN** the user selects open for a worktree
- **THEN** the system focuses that worktree's tmux window

### Requirement: Close a worktree's tmux window
The system SHALL allow the user to close a worktree's tmux window while preserving the worktree and its local branch.

#### Scenario: Close a worktree window
- **WHEN** the user selects close for a worktree
- **THEN** the worktree's tmux window is closed and the worktree and branch remain in place

### Requirement: Remove a worktree
The system SHALL allow the user to remove a worktree, deleting its worktree directory, its tmux window, and its local branch.

#### Scenario: Remove a worktree
- **WHEN** the user confirms removal of a worktree
- **THEN** the worktree, its tmux window, and its local branch are removed

#### Scenario: Removal requires explicit confirmation
- **WHEN** the user initiates removal of a worktree
- **THEN** the system requires an explicit confirmation before performing the destructive action

#### Scenario: Warn when uncommitted changes would be lost
- **WHEN** the user attempts to remove a worktree that has uncommitted changes
- **THEN** the system warns that uncommitted changes will be lost and requires explicit confirmation before proceeding
