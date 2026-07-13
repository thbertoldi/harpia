## Why

The conversation currently has configuration prompts and execution cards, but no single
conversation-level turn model that deterministically projects a selected PlanConfiguration and
its exact PlanExecution. As a result, execution interactions can be confused with configuration
state or resolved through ambiguous "latest" state, which violates the thread 1:N configuration
and configuration 1:M execution model.

This change makes execution a first-class conversational phase while retaining the explicit
configuration-to-run boundary and the immutable execution snapshot needed for accurate, safe
human interaction.

## What Changes

- **BREAKING**: Replace the broad `DeriveState` configuration/execution reducer with the
  DRAFT-configuration-only `DeriveConfigurationState`; non-DRAFT lifecycle and execution turns
  move to a new conversation-level driver.
- Add a typed `EXECUTION_PROMPT` thread message and a deterministic `ConversationTurn` /
  `ExecutionAssistantView` projection that chooses configuration, ready, execution, or terminal
  execution phases without a thread-global active execution.
- Add the frozen `PlanTemplate` snapshot to the `PlanExecution` read model and require execution
  turns/cards to derive order, titles, and active rows from that snapshot plus `active_step_keys`.
- Make execution interaction projection and actions exact, tenant-scoped, version-pinned, and
  safe for concurrent executions with identical step keys.
- Correct review decision ownership so the Temporal resolution activity, rather than the API
  handler, is the sole terminal persistence and `REVIEW_DECIDED` event owner.
- Update the conversation UI to render execution prompts and scoped human-interaction actions,
  including compact off-focus attention for other configurations/executions in the thread.
- Clarify Constitution §9: an execution is “active” only as conversational focus over an exact
  PlanExecution, never as singleton Thread state.
- Reconcile the completed `composable-linkedin-execution` implementation checkbox evidence if
  its task file still marks completed work as unfinished.

## Capabilities

### New Capabilities
- `execution-driven-conversation`: Deterministic conversation turns for configuration and exact
  execution focus, execution prompt payloads, pinned interaction actions, and concurrency-safe
  execution invariants.

### Modified Capabilities
- `live-execution-chat`: Derive execution cards from runtime events, immutable execution
  snapshots, and durable pending-interaction projection rather than events alone.
- `chat-thread`: Define primary selected-configuration/exact-execution focus and compact
  off-focus execution attention in a multi-plan conversation.
- `plan-execution-runtime`: Expose the frozen template snapshot in every execution read model,
  alongside its existing frozen active graph.

## Impact

- Contracts: `proto/harpia/chat/v1/chat.proto`, `proto/harpia/plans/v1/plans.proto`, generated
  Go and frontend clients, and thread-message persistence/projection.
- Control plane: `internal/planassistant`, `internal/chat`, `internal/plans`, and
  `internal/workflow`, including tenant-scoped execution/interaction reads and Temporal signals.
- Frontend: thread message folding, execution conversation cards/actions, frozen-snapshot view
  derivation, localized `en` and `pt-BR` copy, and focused/off-focus rendering.
- Documentation: `docs/architecture/harpia-platform.md` §9 and the OpenSpec task evidence for
  `composable-linkedin-execution`.
