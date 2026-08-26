# worktree-inventory Delta

## ADDED Requirements

### Requirement: Bound per-project read time
The system SHALL bound how long a single project's state can be read during an inventory query. A read that exceeds its deadline SHALL be treated, for that query only, as a failure isolated to that project; the deadline SHALL NOT allow one slow or wedged project to hold up the whole inventory indefinitely.

#### Scenario: One project's read exceeds its deadline
- **WHEN** reading one project takes longer than the configured read deadline while others complete in time
- **THEN** the inventory returns the readable projects and flags that project with a readable error indicating it could not be read in time, rather than waiting for it to finish

#### Scenario: A wedged external command cannot hang a query
- **WHEN** an external command used to read one project stops making progress past its deadline
- **THEN** the system abandons that read (the underlying process does not outlive the query) and reports the failure on that project only

### Requirement: Reuse recent results briefly
The system SHALL be able to reuse recently computed results — the set of discovered projects, tool availability, and per-project records — for a short window after they were last read, so that repeated queries (for example frequent UI polling or an action immediately following a refresh) do not all pay the cost of a full re-read. A result older than its reuse window SHALL be treated as stale and re-read.

#### Scenario: Repeated queries within the reuse window
- **WHEN** an inventory is requested again shortly after a previous request, inside the reuse window
- **THEN** the system may answer from the recent results instead of re-reading every project, and the response reflects state at least as fresh as the end of that window

#### Scenario: Stale result outside the reuse window
- **WHEN** an inventory is requested after a project's last successful read has aged past its reuse window
- **THEN** that project is re-read before being included in the response

## MODIFIED Requirements

### Requirement: Isolate per-project read failures
The system SHALL surface a readable error for any project whose state cannot be read — including a read abandoned because it exceeded its deadline — and SHALL continue to return the worktrees it was able to read instead of failing the entire inventory.

#### Scenario: One project is unreadable
- **WHEN** one project cannot be read but the others can
- **THEN** the inventory returns the readable projects and flags the failed project with an error
