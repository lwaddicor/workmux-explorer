# web-dashboard Specification

## Purpose

Deliver the cross-project workmux overview and its controls as a local web application that a single user can run and use from a browser.

## Requirements

### Requirement: Single self-contained binary
The system SHALL be delivered as a single executable that, when run, makes the dashboard available without a separately installed frontend or a build step.

#### Scenario: Start the dashboard
- **WHEN** the user starts the dashboard from the single binary
- **THEN** the dashboard becomes available in a local browser with no additional installation

### Requirement: Serve a web UI and a JSON API
The system SHALL serve both an interactive web UI and a machine-readable JSON HTTP API over localhost. The web UI SHALL present the cross-project overview as a two-pane layout: a sidebar listing all repositories and a worktree grid showing the worktrees of the selected repository.

#### Scenario: Browse the dashboard in a browser
- **WHEN** the user opens the dashboard URL in a local browser
- **THEN** the web UI renders a sidebar listing all repositories and a grid of the selected repository's worktrees

#### Scenario: Switch repositories from the sidebar
- **WHEN** the user selects a different repository in the sidebar
- **THEN** the worktree grid updates to show only that repository's worktrees without a full page reload

#### Scenario: Repository summary in the sidebar
- **WHEN** the sidebar renders a repository entry
- **THEN** the entry shows the repository name together with a summary of its worktree count and active-agent count

#### Scenario: Query the JSON API
- **WHEN** a client requests the inventory over the HTTP API
- **THEN** the system returns the worktree inventory as JSON

### Requirement: Bind to localhost by default
The system SHALL, by default, listen only on the local loopback interface so the dashboard is not exposed to the network.

#### Scenario: Default bind address
- **WHEN** the dashboard starts with no explicit host override
- **THEN** it listens on the loopback interface (127.0.0.1) only

### Requirement: Refresh to reflect live state
The system SHALL refresh the presented inventory so that changes to running worktrees and agents appear without requiring a full page reload.

#### Scenario: An agent changes status
- **WHEN** a worktree's agent transitions status, for example from working to done
- **THEN** the dashboard reflects the new status on its next refresh

### Requirement: Degrade gracefully when dependencies are unavailable
The system SHALL report a clear, readable status when the required local tooling (workmux, tmux) is missing or not running, rather than failing silently.

#### Scenario: tmux server not running
- **WHEN** the tmux server is not running when the dashboard is queried
- **THEN** the dashboard reports that no running worktrees were found and surfaces a readable reason
