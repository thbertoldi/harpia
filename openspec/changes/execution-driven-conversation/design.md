## Context

Constitution §9 defines a Conversation as lifecycle assistance over multiple PlanConfigurations
and their PlanExecutions. The existing `planassistant.DeriveState` nevertheless mixes the DRAFT
configuration reducer with non-DRAFT landing behavior, while runtime cards are independently
folded from thread events. That leaves no authoritative, exact execution turn, and treating an
execution as thread-global would break the thread 1:N configuration / configuration 1:M execution
model and selected-plan focus.

The completed `composable-linkedin-execution` change already made executions snapshot-based,
version-pinned, and capable of elicitation, review/revision, and approval checkpoints. Its review
handler currently creates a second terminal-write owner: the browser RPC marks the review decided
before signaling Temporal, then the resolution activity attempts the same write and only then
emits `REVIEW_DECIDED`. This change makes the event driver depend only on durable, single-owner
interaction transitions.

## Goals / Non-Goals

**Goals:**

- Project exactly one selected PlanConfiguration and, when targeted, exactly one PlanExecution
  into a deterministic conversation turn without introducing `Thread.active_execution_id`.
- Keep configuration prompting strictly DRAFT-only and preserve the explicit save → RUN boundary.
- Surface execution progress, its one pending interaction, and artifact previews from immutable,
  version-pinned, tenant-authorized state.
- Make browser actions request-ID based and server-scoped so concurrent executions with matching
  step keys cannot cross-route a human decision.
- Fix review decision persistence/event ownership before execution prompts consume review-decided
  transitions.

**Non-Goals:**

- Redesigning the LinkedIn DAG, agents, approval, or publish path delivered by
  `composable-linkedin-execution`.
- Autonomous run, retry, acceptance, approval, or rejection by the assistant.
- Removing the configuration phase; adding structured-preview/Flint; adding a user-facing budget
  field; or preserving old prompt payloads with a migration shim.

## Decisions

### 1. Wrap the configuration reducer with a conversation-level driver

`control-plane/internal/planassistant/state.go` SHALL rename `DeriveState` to
`DeriveConfigurationState`, remove its unused messages argument and `SAVED` state, and return
configuration prompts only for DRAFT configurations. A new pure conversation driver SHALL select
one `ConversationTurn` and `ExecutionAssistantView` after resolving the selected configuration,
its execution snapshot, StepExecutions, durable pending interactions, and relevant thread event.

The driver chooses, in order: configuration phase for DRAFT without a targeted execution; ready
phase for RUNNABLE/SCHEDULED without an execution; execution phase for one exact execution; then
terminal execution phase to preserve the just-finished turn before returning to ready. It SHALL
emit these `ConversationTurnKind` values: `CONFIGURING`, `READY_TO_RUN`,
`WAITING_FOR_SCHEDULE`, `CONFIGURATION_DISABLED`, `CONFIGURATION_ARCHIVED`, `EXECUTION_QUEUED`,
`EXECUTION_RUNNING`, `EXECUTION_AWAITING_ELICITATION`, `EXECUTION_AWAITING_REVIEW`,
`EXECUTION_AWAITING_APPROVAL`, `EXECUTION_FAILING`, `EXECUTION_FAILED`, `EXECUTION_COMPLETED`,
`EXECUTION_CANCELLED`, and `EXECUTION_NEEDS_ATTENTION`.

Alternative rejected: extending `DeriveState` with execution cases. That makes a configuration
reducer responsible for runtime identity and obscures the distinct configuration/execution
lifecycle.

### 2. Resolve focus deterministically; never persist a thread-global execution

The target resolver SHALL use, in order: (1) the exact `execution_id` carried by the initiating
event or action; (2) the latest non-terminal execution of the selected PlanConfiguration only for
display; (3) the selected configuration. A latest non-terminal result is never legal action
resolution. Every card and command remains pinned to the configuration, execution, StepExecution,
request, and artifact version from which it was projected. The selected configuration remains the
ADR-017 focus; executions of other configurations are compact attention, not active state.

Alternative rejected: `Thread.active_execution_id` or an inferred global latest execution. Both
would conflate concurrent executions and deny the selected-plan 1:N/1:M topology.

### 3. Persist an execution prompt as its own message kind

Add `THREAD_MESSAGE_KIND_EXECUTION_PROMPT = 29`, never reusing `ASSISTANT_PROMPT`; configuration
selection continues to treat `ASSISTANT_PROMPT` as the latest unanswered configuration prompt.
`ExecutionPromptPayload` SHALL carry only stable identity and projection data:
`configuration_id`, `plan_execution_id`, turn `state`, `step_execution_id`, `plan_step_key`,
completed/active counts, an optional pending-interaction pointer, latest `ArtifactRef`, and action
IDs. Backend-authored labels are forbidden. Its dedupe fingerprint is execution id + state + step
execution id + interaction kind + request id + subject artifact-version id.

The event driver projects execution-transition notifications into this prompt and deduplicates
replays. It selects `EXECUTION_NEEDS_ATTENTION` rather than guessing when durable state is
contradictory, including more than one pending interaction in one linear execution.

Alternative rejected: overloading configuration prompts. It would corrupt the unanswered-config
prompt rule and make a runtime checkpoint indistinguishable from a DRAFT question.

### 4. The frozen template is the execution read-model authority

Add `PlanExecution.plan_template_snapshot = 12`. `executionToProto` SHALL return the immutable
template held in the execution snapshot rather than discard it. Every execution turn/card derives
titles, ordered rows, and active rows from this frozen template plus frozen `active_step_keys`,
never the current template or configuration.

Alternative rejected: resolving titles/order from the current PlanTemplate. A later catalog edit
would rewrite the history and visible progress of an in-flight or completed run.

### 5. Human actions are exact requests; Temporal owns durable resolution

The browser invokes existing command RPCs using only its projected request/action IDs; it never
calls `SignalWorkflow`, supplies a replacement execution/step identity, or resolves a request by
“latest.” The handler authorizes tenant/user access, loads the request under RLS, derives its
execution and StepExecution server-side, and signals the exact workflow identity. The mapping is:

| Transition | User action | Command / signal |
| --- | --- | --- |
| `ELICITATION_RAISED` | Answer | existing elicitation response command → `ElicitationResponseSignal` |
| `REVIEW_RAISED` | Accept or revise | `RespondToReviewRequest` → `ReviewDecisionSignal` |
| `APPROVAL_RAISED` | Approve or reject | existing approval response command → `ApprovalDecisionSignal` |
| `STEP_FAILED` | Retry | `RetryPlanExecution`; create a new execution from the failed snapshot and reuse upstream outputs |
| `STEP_FAILED` | Run again | `CreatePlanExecution`; take a fresh current-configuration snapshot |
| ready configuration | Run | `CreatePlanExecution`; snapshot then Temporal start |
| no focused non-terminal execution | Reconfigure | strict `UpdatePlanConfiguration`; stage edits locally and do not demote RUNNABLE or clone a configuration |

For reviews specifically, `RespondToReviewRequest` SHALL authorize and submit the exact Temporal
signal but SHALL not call `MarkReviewDecided`. `ResolveReviewRequestActivity` is the sole owner of
terminal review persistence and `REVIEW_DECIDED` emission. Duplicate signals remain safe because
the workflow validates both `step_execution_id` and `review_request_id`.

Alternative rejected: handler-side terminal persistence followed by workflow persistence. It
creates an observable no-op second write and makes the event's causality unreliable.

### 6. Projection invariants are enforced in the pure model and adapter boundaries

The pure projection SHALL enforce one pending interaction per execution; more than one returns
`EXECUTION_NEEDS_ATTENTION` and suppresses unsafe action selection. Interaction lookup, events,
and signals SHALL require both execution and StepExecution identity so same-key concurrent runs
cannot collide. Review and approval previews SHALL use the request's
`subject_artifact_ref.artifact_version_id`; `STEP_BOUND` previews SHALL use its complete output
ref. Revision settles only its old request; a replacement candidate creates a new review id and
artifact version. Projection reads are RLS-scoped, and an action card is never an authorization
grant.

## Risks / Trade-offs

- **A terminal prompt may be replaced too quickly by Ready** → retain the terminal execution turn
  as the immediately finished turn before the next driver pass selects Ready.
- **Event replay can duplicate execution prompts** → persist and compare the full deterministic
  fingerprint, not presentation text.
- **A display fallback can accidentally authorize an action** → keep fallback code in projection
  reads only; command handlers load by request ID and reject missing/past-terminal state.
- **Two pending rows expose corrupt durable state** → fail safe as `EXECUTION_NEEDS_ATTENTION`,
  with no auto-resolution.
- **Proto/read-model change can leave clients partially generated** → regenerate checked-in
  consumers together and verify the exact snapshot reaches Go and TypeScript views.

## Migration Plan

1. Ship the breaking proto/read-model contracts and pure reducer/turn tests first; pre-v1 has no
   compatibility shim for old prompt payloads.
2. Make Temporal review resolution the sole terminal persistence owner, then wire durable
   interaction/execution transition reads and prompt emission.
3. Ship the UI only after exact action routing and frozen-snapshot projection tests prove
   concurrency isolation.
4. Roll forward only: existing prompt payloads are not translated. If a release must be reverted,
   restore the prior application version before emitting new execution prompts; immutable
   execution snapshots and events remain audit records.

## Open Questions

None. The approved design fixes the focus, identity, ownership, and run-boundary semantics.
