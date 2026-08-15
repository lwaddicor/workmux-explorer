# worktree-inventory Specification

## Purpose

Provide a single, cross-project view of every workmux worktree and its active agent running on the local machine, so the user can see everything that is going on without switching between projects or terminals.

## Requirements

### Requirement: Discover worktrees across all projects
The system SHALL discover every workmux-managed worktree and its active agent across all projects running on the local machine, not only the project from which it is started.

#### Scenario: Multiple projects with active agents
- **WHEN** the local machine has worktree agents running in two or more distinct projects
- **THEN** the inventory includes worktrees from each of those projects

#### Scenario: Started outside any workmux project
- **WHEN** the dashboard is started from a directory that is not itself a workmux worktree
- **THEN** the inventory still lists every active workmux worktree found on the machine

### Requirement: Present a unified per-worktree record
For each discovered worktree, the system SHALL present a unified record containing at least the project, handle, branch, path, whether it is the main worktree, whether its tmux window is open, whether it has uncommitted changes, and its creation time. For worktrees that have an active agent, the record SHALL additionally contain the agent status (working, done, or waiting), the agent kind, the elapsed time, and the agent's current task title.

#### Scenario: Worktree with a running agent
- **WHEN** a worktree has an active agent reporting status
- **THEN** its record includes the agent status, agent kind, elapsed time, and task title alongside the branch and path

#### Scenario: Worktree with no active agent
- **WHEN** a worktree has no active agent, such as when its tmux window is closed
- **THEN** its record still shows the branch, path, and uncommitted-changes state, and reports agent status as absent

### Requirement: Reflect live machine state
The system SHALL derive the inventory from live local state at query time, rather than requiring a persistent registry, so the view always reflects the worktrees and agents that are actually running.

#### Scenario: A worktree is removed elsewhere
- **WHEN** a worktree is removed through workmux outside the dashboard
- **THEN** a subsequent refresh no longer lists it as active

### Requirement: Isolate per-project read failures
The system SHALL surface a readable error for any project whose state cannot be read, and SHALL continue to return the worktrees it was able to read instead of failing the entire inventory.

#### Scenario: One project is unreadable
- **WHEN** one project cannot be read but the others can
- **THEN** the inventory returns the readable projects and flags the failed project with an error
