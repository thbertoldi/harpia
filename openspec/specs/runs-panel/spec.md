# runs-panel Specification

## Purpose
TBD - created by archiving change stabilize-user-journey. Update Purpose after archive.
## Requirements
### Requirement: Runs Panel Destination
The system SHALL provide a top-level `/runs` route that serves as the primary destination for viewing and managing all plan executions.

#### Scenario: Navigate to Runs
- **WHEN** the user clicks "Runs" in the main navigation
- **THEN** the system navigates to the `/runs` route

### Requirement: Group Executions by Plan
The Runs panel SHALL group all executions (in-progress, scheduled, past) by their associated PlanConfiguration.

#### Scenario: View grouped executions
- **WHEN** the user views the Runs panel
- **THEN** executions are visually grouped under their respective plans

### Requirement: Link to Origin Thread
Every plan and execution in the Runs panel SHALL provide a link back to its origin conversation thread.

#### Scenario: Navigate to origin thread
- **WHEN** the user clicks the origin thread link for a plan or execution
- **THEN** the system navigates to the `/chat/[threadId]` route for that origin thread
- **AND** the corresponding plan tab is selected

### Requirement: Inline Edits for Recurring Plans
The Runs panel SHALL allow light edits (sources, schedule, tone) and cancellation for recurring plans inline, without navigating away.

#### Scenario: Edit recurring plan schedule
- **WHEN** the user edits the schedule of a recurring plan in the Runs panel
- **THEN** the schedule is updated without leaving the `/runs` route

### Requirement: Structural Edits Redirect to Chat
The Runs panel SHALL redirect the user to the origin conversation thread for structural changes to a plan.

#### Scenario: Attempt structural edit
- **WHEN** the user attempts a structural edit on a plan in the Runs panel
- **THEN** the system navigates to the `/chat/[threadId]` route for that plan's origin thread

