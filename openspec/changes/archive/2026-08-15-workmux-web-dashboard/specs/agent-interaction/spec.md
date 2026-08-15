## Purpose

Let the user interact with a worktree's running agent from the dashboard by sending it a prompt and by viewing the agent's live terminal output.

## ADDED Requirements

### Requirement: Send a prompt to a running agent
The system SHALL allow the user to send a prompt or instruction to a worktree's running agent.

#### Scenario: Send a prompt to an active agent
- **WHEN** the user sends a prompt to a worktree that has a running agent
- **THEN** the system delivers the prompt to that agent

#### Scenario: Send to a worktree with no running agent
- **WHEN** the user sends a prompt to a worktree that has no running agent
- **THEN** the system reports that there is no agent available to receive the prompt

### Requirement: View live agent output
The system SHALL allow the user to view the recent terminal output of a worktree's agent, and the view SHALL reflect output the agent produces after it is first loaded.

#### Scenario: View current agent output
- **WHEN** the user opens the output view for a worktree with a running agent
- **THEN** the system shows the agent's most recent terminal lines

#### Scenario: Output updates over time
- **WHEN** the agent produces new output while the user is viewing
- **THEN** the output view reflects the newly produced lines on its next refresh
