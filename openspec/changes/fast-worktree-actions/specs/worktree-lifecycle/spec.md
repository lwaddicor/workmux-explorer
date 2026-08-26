# worktree-lifecycle Delta

## ADDED Requirements

### Requirement: Scope actions to the addressed project
A dashboard action on one worktree (open, close, focus, send, remove) SHALL be resolved and executed against that worktree's own project only. The system SHALL NOT require reading, or wait for, the state of unrelated projects before performing the action. The preconditions the action checks (for example an open window for focus, a running agent for send, uncommitted changes when warning on removal) are evaluated from the addressed project's own state.

#### Scenario: Unrelated project is slow to read
- **WHEN** the user performs an action on a worktree in one project while another project's state takes longer than its read deadline or cannot be read
- **THEN** the action proceeds against the addressed project and returns its result, without being delayed by or failing because of the other project

#### Scenario: Addressed project cannot be read
- **WHEN** the user performs an action on a worktree whose own project state cannot be read in time or at all
- **THEN** the action fails with a readable error naming that project's problem, and no destructive step is attempted

### Requirement: Retrieve a single project's record
The system SHALL let a client retrieve the unified record of one addressed project — its worktrees and their agent state — without requiring construction of the full cross-project inventory.

#### Scenario: Fetch one project by name
- **WHEN** a client requests the record of a known project by its name or root path
- **THEN** the system returns that project's worktree records, including open-window and uncommitted-changes state and any active agents

#### Scenario: Unknown project
- **WHEN** a client requests the record of a project that is not discovered on this machine
- **THEN** the system reports that no such project exists rather than returning an empty or whole-machine result
