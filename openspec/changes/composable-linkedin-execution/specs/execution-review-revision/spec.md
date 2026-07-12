## ADDED Requirements

### Requirement: Review and Revision are distinct Human Interaction concepts
The system SHALL model ReviewRequest and ReviewDecision separately from ElicitationRequest and
ApprovalRequest. A ReviewRequest SHALL name its subject ArtifactRef, StepExecution, PlanExecution,
PlanStep, addressed overseer, and lifecycle status; a revision decision SHALL carry feedback.

#### Scenario: Review is not an elicitation or approval
- **WHEN** a significant content step produces a candidate
- **THEN** the system creates a ReviewRequest rather than an ElicitationRequest or ApprovalRequest
- **AND** its subject identifies the exact candidate ArtifactVersion and content hash

#### Scenario: Required review policy is declared on content steps
- **WHEN** the `linkedin-content-studio` template is loaded
- **THEN** `author-content`, `draft-carousel`, and `generate-image` declare `elicitation_mode: WHEN_REQUIRED` and `review_mode: REQUIRED`
- **AND** `publish` remains governed by its irreversible-action approval policy

### Requirement: Review decision drives a durable revision loop
The workflow SHALL pause a StepExecution after a required-review candidate, resume only on a
ReviewDecision signal for that request, and repeat review for each revised candidate.

#### Scenario: Accept completes the content step
- **WHEN** the overseer accepts a pending ReviewRequest
- **THEN** the workflow records the accepted subject ArtifactRef
- **AND** the StepExecution completes using that exact version as the downstream input

#### Scenario: Revise resumes the real specialist
- **WHEN** the overseer submits revision feedback for a pending ReviewRequest
- **THEN** the workflow resumes the same agent-backed step with that feedback rather than a stub executor
- **AND** the resulting candidate is a new ArtifactVersion and receives a new ReviewRequest

### Requirement: Elicitation precedes candidate review when context is missing
For a content step with `elicitation_mode: WHEN_REQUIRED`, the workflow SHALL persist and surface
an ElicitationRequest when the real agent requests missing context, then resume that agent before
it creates a candidate subject to review.

#### Scenario: Carousel elicitation pauses and resumes the specialist
- **WHEN** the carousel specialist cannot draft because required context is absent
- **THEN** the StepExecution enters awaiting-elicitation state and the conversation surfaces the real request
- **AND** answering its signal resumes the carousel specialist, which then produces a reviewable candidate

### Requirement: Review checkpoints surface in the execution conversation
The runtime SHALL emit review-raised and review-decided execution events and the conversation
surface SHALL render the pending review subject, accept action, and revision-feedback action using
localized copy in `en` and `pt-BR`.

#### Scenario: User revises an in-thread candidate
- **WHEN** a review-raised event for a content step reaches the owning conversation
- **THEN** the user can preview its pinned subject, accept it, or submit revision feedback without leaving the conversation
- **AND** a later review-decided event settles that exact card
