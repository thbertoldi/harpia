## ADDED Requirements

### Requirement: Structured cards align with conversation width
Structured cards in the chat thread SHALL use the same full conversation card width, distinct from narrower free-form message bubbles.

#### Scenario: Execution card after run now
- **WHEN** the user starts a PlanExecution from a configured thread using run now
- **THEN** the resulting execution card aligns with the other structured plan/configuration cards in the conversation
- **AND** it does not render as a narrow assistant chat bubble

### Requirement: Chronological execution context
The chat thread SHALL keep execution, artifact, and approval events visually connected to the conversation chronology.

#### Scenario: Artifact generated during execution
- **WHEN** a StepExecution produces an artifact
- **THEN** the thread displays an inline artifact launcher at the relevant point in the execution context
- **AND** opening the artifact does not remove the user from the chat thread

#### Scenario: Approval generated during execution
- **WHEN** a PlanExecution waits for approval
- **THEN** the approval card appears in the thread near the execution it belongs to
