## ADDED Requirements

### Requirement: Execution and approval links preserve context
Links from execution and approval surfaces SHALL return the user to the origin chat thread with the relevant plan and execution context selected.

#### Scenario: Open approval from inbox
- **WHEN** the user selects a pending approval from the inbox
- **THEN** the system navigates to the origin chat thread
- **AND** the relevant plan tab is selected
- **AND** the relevant execution or approval card is visible or addressable in the thread

#### Scenario: Open run from runs panel
- **WHEN** the user opens an execution from `/runs`
- **THEN** the system navigates to the origin chat thread for that plan
- **AND** the relevant plan tab and execution context are selected
