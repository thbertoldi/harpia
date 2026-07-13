## ADDED Requirements

### Requirement: Conversation turns wrap DRAFT configuration and exact execution state
The system SHALL use a conversation-level driver to derive one `ConversationTurn` for the selected
PlanConfiguration. `DeriveConfigurationState` SHALL be a DRAFT-configuration-only pure reducer,
with no messages argument and no `SAVED` state. The driver SHALL produce exactly one of
`CONFIGURING`, `READY_TO_RUN`, `WAITING_FOR_SCHEDULE`, `CONFIGURATION_DISABLED`,
`CONFIGURATION_ARCHIVED`, `EXECUTION_QUEUED`, `EXECUTION_RUNNING`,
`EXECUTION_AWAITING_ELICITATION`, `EXECUTION_AWAITING_REVIEW`,
`EXECUTION_AWAITING_APPROVAL`, `EXECUTION_FAILING`, `EXECUTION_FAILED`,
`EXECUTION_COMPLETED`, `EXECUTION_CANCELLED`, or `EXECUTION_NEEDS_ATTENTION`.

#### Scenario: DRAFT uses configuration phase without an execution target
- **WHEN** the selected PlanConfiguration is DRAFT and no exact execution is targeted
- **THEN** the driver returns `CONFIGURING` with the result of `DeriveConfigurationState`
- **AND** it does not derive a runtime turn from another execution in the Thread

#### Scenario: Terminal execution preserves its finished turn before ready
- **WHEN** an exact focused execution has just become completed, failed, or cancelled
- **THEN** the driver returns its corresponding terminal execution turn for the just-finished turn
- **AND** a subsequent no-target derivation for its RUNNABLE or SCHEDULED configuration returns a ready turn

### Requirement: Execution focus is exact and selected-configuration scoped
The system SHALL resolve an execution target from the exact `execution_id` in an initiating event
or action, otherwise use the latest non-terminal execution of the selected PlanConfiguration only
as a display fallback, otherwise resolve the selected PlanConfiguration. It SHALL NOT store or
interpret a thread-global active execution. Latest non-terminal display fallback SHALL NOT resolve
an action.

#### Scenario: Concurrent executions with equal step keys remain isolated
- **WHEN** two executions of the selected PlanConfiguration both have a `publish` StepExecution
- **AND** an approval action identifies one approval request
- **THEN** the server derives that request's exact execution and StepExecution before signaling
- **AND** it never routes the action to the other `publish` StepExecution

#### Scenario: Off-focus execution is compact attention
- **WHEN** the selected PlanConfiguration has a focused execution and another configuration in the Thread has a pending interaction
- **THEN** the selected configuration and its exact execution remain the primary assistant focus
- **AND** the other configuration is represented only as compact attention

### Requirement: Execution prompts are typed, deduplicated, and distinct from configuration prompts
The system SHALL persist an execution turn as `THREAD_MESSAGE_KIND_EXECUTION_PROMPT = 29` with a
typed `ExecutionPromptPayload`. The payload SHALL contain configuration id, PlanExecution id,
turn state, StepExecution id, PlanStep key, completed and active counts, an optional pending
interaction pointer, latest ArtifactRef, and action IDs; it SHALL NOT contain backend-authored
labels. Its dedupe fingerprint SHALL contain execution id, state, StepExecution id, interaction
kind, request id, and subject artifact-version id. `ASSISTANT_PROMPT` SHALL remain solely the
latest unanswered configuration prompt.

#### Scenario: Replayed execution transition does not duplicate a prompt
- **WHEN** the event driver receives the same review-raised transition more than once for one execution
- **THEN** it persists at most one EXECUTION_PROMPT with the matching fingerprint
- **AND** the prompt identifies the exact review request and pinned subject version

#### Scenario: Contradictory pending interactions fail safe
- **WHEN** durable projection finds more than one pending elicitation, review, or approval interaction for one linear execution
- **THEN** the driver produces `EXECUTION_NEEDS_ATTENTION`
- **AND** it does not select an interaction action automatically

### Requirement: Execution interaction actions are exact, version-pinned, and authorized
The browser SHALL invoke existing command RPCs only with the projected request/action ID. It SHALL
NOT call Temporal directly or provide replacement execution, StepExecution, or artifact identity.
The server SHALL read requests with tenant/RLS scope, authorize the requester, derive execution and
StepExecution identity from the request, and signal the exact workflow. Review and approval
previews SHALL use `subject_artifact_ref.artifact_version_id`; `STEP_BOUND` previews SHALL use the
complete output ArtifactRef. A revision SHALL settle only its old request and SHALL create a new
review id and candidate version for the replacement.

#### Scenario: Approval preview stays pinned after a later artifact version exists
- **WHEN** an approval request names one ArtifactVersion and a later version becomes current
- **THEN** the execution prompt and Preview action load the request's pinned version and hash
- **AND** they do not load the Artifact's mutable current version

#### Scenario: Unauthorized action card cannot authorize a decision
- **WHEN** a user can observe an execution prompt but is not its assigned overseer or authorized leader
- **THEN** the command RPC rejects their request after tenant-scoped authorization
- **AND** rendering the card does not grant permission to signal the workflow

### Requirement: Review decision has one durable terminal owner
`RespondToReviewRequest` SHALL authorize and send an exact `ReviewDecisionSignal` but SHALL NOT
persist a terminal review decision. `ResolveReviewRequestActivity` SHALL be the sole owner of
terminal review persistence and `REVIEW_DECIDED` emission. The workflow SHALL validate both
`step_execution_id` and `review_request_id` so duplicate signals are safe.

#### Scenario: Duplicate review signal produces one decision event
- **WHEN** the same valid review decision signal is delivered twice for one StepExecution and review request
- **THEN** the resolution activity persists one terminal decision and emits one `REVIEW_DECIDED` event
- **AND** the duplicate signal does not decide a review for another execution

### Requirement: Explicit configuration-to-execution and retry boundaries are preserved
A DRAFT configuration SHALL be saved as RUNNABLE before the user can Run it. Run SHALL call
`CreatePlanExecution` with a fresh immutable snapshot and start Temporal. Run-again SHALL create
a fresh snapshot of the current configuration; Retry SHALL create a new execution from the failed
execution's snapshot and reuse valid upstream outputs. Reconfigure SHALL be available only with no
focused non-terminal execution, SHALL stage edits locally, and SHALL make one strict
`UpdatePlanConfiguration` without demoting RUNNABLE to DRAFT or cloning another configuration.

#### Scenario: Reconfiguration cannot mutate an in-flight execution
- **WHEN** a configuration has a focused non-terminal execution
- **THEN** Reconfigure is unavailable for that focus
- **AND** no configuration edit can alter that execution's snapshot

#### Scenario: Retry and run-again use distinct snapshots
- **WHEN** a failed execution is retried after its configuration has changed
- **THEN** Retry uses the failed execution's frozen snapshot and valid upstream outputs
- **AND** Run-again uses a new snapshot from the current configuration
