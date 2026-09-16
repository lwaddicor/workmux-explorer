# web-dashboard Delta

## MODIFIED Requirements

### Requirement: Refresh to reflect live state
The system SHALL refresh the presented inventory so that changes to running worktrees and agents appear without requiring a full page reload. After an action on one worktree (for example removing it), the dashboard SHALL immediately update the affected part of the view from the action's outcome, then reconcile with a targeted refresh of only the addressed project — it SHALL NOT need to wait for a machine-wide rescan before showing the change.

#### Scenario: An agent changes status
- **WHEN** a worktree's agent transitions status, for example from working to done
- **THEN** the dashboard reflects the new status on its next refresh

#### Scenario: A worktree is removed from the dashboard
- **WHEN** the user confirms removal of a worktree and the server reports that it was removed
- **THEN** the grid no longer shows that worktree immediately, without waiting for a machine-wide rescan to complete

#### Scenario: An action changes a worktree's window state
- **WHEN** the user opens or closes a worktree's window from the dashboard and the server reports success
- **THEN** the affected worktree's card reflects the new open/closed state as soon as it is reconciled with its project's fresh record
