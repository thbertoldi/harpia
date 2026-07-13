## MODIFIED Requirements

### Requirement: Active Plan Selection
The active plan in a conversation view SHALL be strictly derived from the selected chip/tab. The
primary assistant focus SHALL be the selected PlanConfiguration and an exact execution targeted by
an event/action, or its latest non-terminal execution only as display fallback. The system SHALL
NOT store a thread-global active execution or use a latest execution to resolve an action.

#### Scenario: Select active plan
- **WHEN** the user clicks a plan chip/tab
- **THEN** that PlanConfiguration becomes the active plan
- **AND** its configuration, executions, and artifacts are the primary scope

#### Scenario: Exact execution target overrides display fallback
- **WHEN** an execution event or action supplies an execution id for the selected PlanConfiguration
- **THEN** the conversation focuses that exact execution
- **AND** it does not substitute a different newer non-terminal execution

## ADDED Requirements

### Requirement: Off-focus execution attention remains compact
The thread SHALL present pending/running executions outside the selected PlanConfiguration or exact
focused execution as compact attention. Such attention SHALL preserve navigation identity but
SHALL NOT replace the primary conversation turn or authorize an action.

#### Scenario: Another configuration needs attention
- **WHEN** a non-selected PlanConfiguration has a pending approval while the selected configuration is ready to run
- **THEN** the selected configuration remains the primary ready turn
- **AND** the approval appears as compact off-focus attention linked to its configuration and execution
