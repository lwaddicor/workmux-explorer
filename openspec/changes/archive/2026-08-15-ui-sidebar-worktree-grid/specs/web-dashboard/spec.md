## MODIFIED Requirements

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
