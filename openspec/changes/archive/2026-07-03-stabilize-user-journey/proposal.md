## Why

The system is suffering from "model whiplash" because we are trying to build the new conversational model (ADR-017) while the old standalone surfaces still exist. The state heuristics are guessing which one is active, leading to an incoherent user journey. We need a Stabilization Sequence to freeze the invariants and establish a single operational contract before delegating further work to subagents.

## What Changes

- **BREAKING**: Delete the old standalone routes (`/plans/configurations`, `/plans/executions`, `/artifacts`).
- Update the chat UI to support `Thread 1:N PlanConfiguration` using chips/tabs at the top.
- Build the `/runs` route to group executions by plan.
- Implement a split-pane/side-drawer in the chat for the *final* artifact.
- Replace the "leave a note" box with a real conversational composer and a structured approval card.

## Capabilities

### New Capabilities
- `runs-panel`: The new top-level destination for viewing and managing all executions, grouped by plan.
- `artifact-side-preview`: The split-pane/side-drawer in the chat for viewing the final artifact of a plan.
- `conversational-configuration`: The hybrid conversational composer and structured approval card for configuring plans.

### Modified Capabilities
- `chat-thread`: Updated to support 1:N PlanConfigurations with chips/tabs, and strictly derive the active plan from the selected tab.
- `navigation`: Locked to exactly 4 items: Home, Gallery, Runs, and Conversation.

## Impact

- **Frontend Routes**: Removal of `/plans/configurations`, `/plans/executions`, `/artifacts`. Addition of `/runs`.
- **Frontend Components**: Updates to `ChatPage`, `ThreadComposer`, `PlanProposalCard`, `ArtifactPreviewSheet`.
- **Data Model**: `Thread.active_plan_configuration_id` is superseded by a 1:N relationship. `origin_thread_id` becomes the absolute source of truth for a plan's origin.
