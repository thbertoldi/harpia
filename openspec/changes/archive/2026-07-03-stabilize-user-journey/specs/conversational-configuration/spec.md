## ADDED Requirements

### Requirement: Conversational Composer
The system SHALL replace the static "leave a note" box with a conversational composer that accepts free-text input for configuring plans.

#### Scenario: Enter free-text configuration
- **WHEN** the user types a configuration request in the composer
- **THEN** the assistant processes the text to fill smart defaults

### Requirement: Clarifying Questions
The assistant SHALL ask at most 1-2 clarifying questions if it is unsure about the configuration based on the user's free-text input.

#### Scenario: Assistant asks clarifying question
- **WHEN** the user's input is ambiguous or missing required information
- **THEN** the assistant responds with a clarifying question in the chat stream

### Requirement: Structured Approval Card
The system SHALL present a structured approval card with pre-filled configuration details (sources, schedule, tone, etc.) before executing a plan.

#### Scenario: View approval card
- **WHEN** the assistant has gathered sufficient configuration details
- **THEN** it presents a structured approval card in the chat stream

### Requirement: Explicit Approval for Execution
A plan SHALL NOT transition to RUNNABLE or SCHEDULED status without explicit user interaction with the structured approval card.

#### Scenario: Approve plan execution
- **WHEN** the user clicks "Approve" or "Run" on the structured approval card
- **THEN** the plan transitions to the appropriate execution state
