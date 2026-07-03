## ADDED Requirements

### Requirement: 1:N Thread to Plan Relationship
The conversation view (`/chat/[threadId]`) SHALL support a 1:N relationship between a thread and its spawned PlanConfigurations.

#### Scenario: View multiple plans in a thread
- **WHEN** a thread has spawned multiple plans
- **THEN** the UI displays chips or tabs at the top of the conversation view for each plan

### Requirement: Active Plan Selection
The active plan in a conversation view SHALL be strictly derived from the selected chip/tab.

#### Scenario: Select active plan
- **WHEN** the user clicks a plan chip/tab
- **THEN** that plan becomes the active plan
- **AND** the configuration, executions, and artifacts shown are scoped to that plan

### Requirement: Default Active Plan
If no plan chip/tab is explicitly selected, the system SHALL default to the most recently created plan in that thread.

#### Scenario: Default plan selection
- **WHEN** the user navigates to a thread with multiple plans without specifying a plan ID
- **THEN** the most recently created plan is automatically selected as the active plan

### Requirement: Origin Thread Source of Truth
The `origin_thread_id` field on a PlanConfiguration SHALL be the absolute source of truth for linking a plan to its conversation.

#### Scenario: Verify origin thread
- **WHEN** a plan is created within a conversation
- **THEN** its `origin_thread_id` is permanently set to that conversation's ID
