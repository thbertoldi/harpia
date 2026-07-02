## 1. Backend Assistant State

- [x] 1.1 Add failing `control-plane/internal/planassistant` tests for `OVERSEER_STEP` derivation after all SlotBindings are present, skipping integration-backed steps, skipping already-bound OverseerBindings, and falling through to `BINDING_MATRIX` when required overseers are complete.
- [x] 1.2 Add failing prompt/controller tests for `OVERSEER_STEP` payloads: focused `step_key`, current-user overseer option, `required_step_keys`, full matrix rows, current overseer ids/labels, duplicate suppression, and advancement after `STEP_REBOUND`.
- [x] 1.3 Implement `StateOverseerStep`, required-overseer derivation from `PlanTemplate.steps[].executor_requirement.executor_kind == AGENT`, and prompt building for `OVERSEER_STEP`.
- [x] 1.4 Keep existing `BINDING_STEP`, `BINDING_MATRIX`, landing idempotency, and executor-candidate loading behavior unchanged except where `OVERSEER_STEP` needs shared matrix rows.

## 2. Backend Validation

- [x] 2.1 Add failing `control-plane/internal/plans` validation tests proving DRAFT permits missing OverseerBindings, RUNNABLE rejects missing agent-step overseers, SCHEDULED rejects missing agent-step overseers, and RUNNABLE accepts complete overseers.
- [x] 2.2 Implement server-side OverseerBinding validation in Create/Update configuration paths alongside SlotBinding validation, with `FailedPrecondition` for missing required overseers on RUNNABLE/SCHEDULED.
- [x] 2.3 Ensure validation uses template agent-backed steps as the source of truth and does not require overseers for integration-backed steps.

## 3. Frontend Overseer Helpers

- [x] 3.1 Add failing frontend tests for parsing an `OVERSEER_STEP` payload and hydrating rows from `overseerBindings`.
- [x] 3.2 Add failing frontend tests that conversational overseer selection persists an OverseerBinding with `announceSaved=false`, preserves the rest of the PlanConfiguration, appends `USER_SELECTION`, and appends `STEP_REBOUND` with previous/new overseer user ids.
- [x] 3.3 Implement overseer payload parsing/hydration helpers in `frontend/src/lib/plans/matrix.ts` or a closely scoped helper module, reusing existing matrix row types where practical.
- [x] 3.4 Implement `editOverseerBinding()`, `selectOverseerOption()`, and an additive overseer-aware `STEP_REBOUND` payload path in `frontend/src/lib/plans/assistant.ts`.

## 4. Conversational Overseer UI

- [x] 4.1 Add `frontend/src/lib/components/thread/ConversationalOverseerCard.svelte` for `OVERSEER_STEP` prompts with step context, current-user overseer chip, required-step progress, selected-overseer display, and answered-message edit behavior.
- [x] 4.2 Route `ThreadMessage.svelte` to `ConversationalOverseerCard.svelte` for `ASSISTANT_PROMPT` payloads with state `OVERSEER_STEP`.
- [x] 4.3 Update `BindingMatrixCard.svelte` and shared hydration so matrix rows display saved OverseerBindings consistently after conversational selections.
- [x] 4.4 Add `en` and `pt-BR` flat translation keys for all new overseer UI copy.
- [x] 4.5 Keep UI transitions limited to existing reduced-motion-aware opacity/transform primitives.

## 5. Validation

- [x] 5.1 Run `openspec validate --changes conversational-overseer`.
- [x] 5.2 Run `cd control-plane && go test ./...`.
- [x] 5.3 Run `cd proto && buf lint`.
- [x] 5.4 Run `cd frontend && bun run check` and confirm no new errors beyond the baseline.
- [x] 5.5 Run `cd frontend && bunx vitest run <files touched>`.
- [x] 5.6 Archive `conversational-overseer` only after all gates pass.
