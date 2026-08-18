# web-dashboard Specification

## ADDED Requirements

### Requirement: Sort the worktree grid

The web UI SHALL let the user choose the order in which the selected repository's worktrees are shown in the grid. The control SHALL offer at least the following options: created (newest first), created (oldest first), name (A→Z), and name (Z→A). "Created" SHALL order by the worktree's creation time; "name" SHALL order by the worktree handle. The default order SHALL be created, newest first. The chosen order SHALL be remembered across page loads. Changing the order SHALL re-render the grid without a page reload, and SHALL NOT alter the order of worktrees in the JSON API response.

#### Scenario: Choose newest created first
- **WHEN** the user selects the "created (newest first)" order
- **THEN** the grid shows the most recently created worktree first and the least recently created last, without a page reload

#### Scenario: Choose oldest created first
- **WHEN** the user selects the "created (oldest first)" order
- **THEN** the grid shows the least recently created worktree first and the most recently created last

#### Scenario: Choose by name
- **WHEN** the user selects the "name (A→Z)" order
- **THEN** the grid lists the worktrees in ascending order of their handle, and selecting "name (Z→A)" lists them in descending order

#### Scenario: Default order on first visit
- **WHEN** the user opens the dashboard for the first time with no saved preference
- **THEN** the grid shows worktrees ordered by creation time, newest first

#### Scenario: Preference persists across reloads
- **WHEN** the user selects an order and then reloads the page
- **THEN** the grid is again shown in the previously selected order

#### Scenario: API order is unaffected
- **WHEN** the user has selected any grid order
- **THEN** a request to the JSON inventory API still returns worktrees in the server's own order, unchanged by the user's grid preference
