## Why

Harpia already emits a rich execution event stream into the chat thread — `STEP_STARTED`,
`STEP_BOUND` ("StepExecution completed, output artifact produced", `chat.proto:162`),
`RUN_STARTED/COMPLETED/FAILED` — but renders it as a flat sequence of one-line
`SystemEventCard` entries (`ThreadMessage.svelte:101`). There is no unified, live view of a
`PlanExecution` and its `StepExecution`s, no progress indicator, and no global "an execution
is running" affordance. A reference design (Fable "Forge Agent") shows the target experience:
a collapsible card that tracks each step pending → running → done with a progress bar and a
connector timeline, plus a live header status pill.

This change makes execution **legible** without any backend work, by deriving a step-status
view model from the existing event stream.

## What Changes

- Add a `PlanExecutionCard` rendered in the configured-thread chat branch: a collapsible card
  with a step timeline (status icon + connector line), a header showing `done/total` and the
  currently-running step's detail, and a gradient progress bar.
- Add a frontend view-model builder that folds the per-`executionId` event stream
  (`RUN_STARTED`, `STEP_STARTED`, `STEP_BOUND`, `RUN_COMPLETED`, `RUN_FAILED`, plus
  `ARTIFACT_CREATED/UPDATED` produced by a step) into per-step statuses
  (`pending | running | done | failed`), reusing the `executionId` grouping already produced
  by `buildThreadSections` (`frontend/src/lib/plans/thread.ts`).
- Add a header **status pill** to the chat header that shows "Executing…" (energy accent) when
  any execution in the thread is running, and "Ready" otherwise.
- Replace the existing CSS "Thinking about a plan…" pulse (`+page.svelte:348-355`, `449-460`)
  with a reduced-motion-safe **typing-dots indicator**, and add an energy-accent
  **generating skeleton** for in-progress executions.
- Add a reduced-motion-safe message **rise** transition for new chat entries.
- All new copy goes through flat `translate()` keys in `en` and `pt-BR`.

## Capabilities

### New Capabilities
- `live-execution-chat`: The chat thread renders a live, collapsible per-`PlanExecution`
  progress card derived from the existing execution event stream, a header execution-status
  pill, and reduced-motion-safe typing/generating/rise affordances.

## Impact

- Frontend: new `PlanExecutionCard.svelte` + a view-model helper (near
  `frontend/src/lib/plans/thread.ts`); chat header changes in
  `frontend/src/routes/chat/[threadId]/+page.svelte`; message dispatcher
  (`ThreadMessage.svelte`) unchanged (the card is rendered above/outside the per-message list,
  driven by the section view model, not as a new message kind).
- Tokens: consumes ADR-016 `--color-status-running/done/pending` and the energy gradient; this
  change MUST NOT land before ADR-016.
- Tests: frontend unit tests for the view-model builder (event → step-status folding,
  including failure and multi-execution threads); component tests for the card states; the
  existing `hardcoded-copy` test enforces no stray literals.
- No proto, backend, migration, or runtime changes. No new message kinds.
