## ADDED Requirements

### Requirement: In-thread approval card
When a plan execution reaches a human approval gate, the system SHALL render an actionable approval card in the origin chat thread for the active PlanExecution.

#### Scenario: Approval gate appears during execution
- **WHEN** a running PlanExecution creates an approval request for a produced artifact
- **THEN** the origin chat thread displays an approval card in chronological execution context
- **AND** the card identifies the plan, execution, approval request, and artifact being approved

#### Scenario: Approval card opens artifact preview
- **WHEN** the user clicks preview from the approval card
- **THEN** the canonical artifact preview panel opens for the artifact associated with the approval request

### Requirement: Approval decision actions
The approval card SHALL allow the user to approve or reject the request from the chat thread, using the same approval request lifecycle as the inbox.

#### Scenario: Approve from chat
- **WHEN** the user approves an approval request from the in-thread approval card
- **THEN** the approval request is marked approved
- **AND** the running execution is unblocked according to its PlanBehaviorPolicies
- **AND** the inbox no longer shows the request as pending

#### Scenario: Reject from chat
- **WHEN** the user rejects an approval request from the in-thread approval card
- **THEN** the approval request is marked rejected
- **AND** the running execution follows the configured rejection behavior
- **AND** the inbox no longer shows the request as pending

### Requirement: Inbox approval context
The inbox SHALL show enough approval context for the user to understand what needs a decision without opening a blank or generic item.

#### Scenario: Approval appears in inbox
- **WHEN** an approval request is pending
- **THEN** the inbox row displays a localized approval summary derived from the request, artifact, plan, or execution context
- **AND** selecting the row navigates to the origin chat thread with the relevant plan/execution context selected
