## Context

ADR-012 defines chat as the PlanConfiguration assistant, but the current `planassistant` state machine collapses every DRAFT configuration into `BINDING_MATRIX`. The matrix payload already carries all PlanStep rows, compatible ExecutorInstallation options, current SlotBindings, overseer defaults, and policy readiness; the missing piece is a focused conversational turn that asks for one unbound PlanStep at a time.

The chat message kinds needed for this flow already exist: `USER_SELECTION` and `STEP_REBOUND`. The frontend already has `selectChip()` and `editBinding()`, but `selectChip()` is not used by the matrix, and `editBinding()` intentionally suppresses thread history. This change should wire those existing paths into a conversational SlotBinding flow without changing proto contracts.

## Goals / Non-Goals

**Goals:**

- Guide DRAFT PlanConfigurations through one unbound SlotBinding at a time.
- Offer only compatible ExecutorInstallation options for the focused PlanStep, including seeded RSS preset installations returned by `CandidatesForStep()`.
- Keep a collapsible "Edit all" matrix fallback mounted in the same prompt and backed by the same payload rows.
- Persist chip and matrix selections through the existing `UpdatePlanConfiguration` RPC with `announceSaved=false`.
- Append `USER_SELECTION` for a user chip pick and `STEP_REBOUND` after the SlotBinding is persisted, then advance to the next unbound PlanStep.
- Preserve the existing matrix save/policies/landing flow after every step has an executor.

**Non-Goals:**

- No overseer assignment changes; existing overseer display remains read-only.
- No behavior-policy generalization; the LinkedIn suggestion path remains as-is.
- No schedule changes; the post-save `ScheduleDialog` remains as-is.
- No promotion changes; the existing save action still controls RUNNABLE/SCHEDULED status.
- No proto changes unless implementation proves the existing JSON payload and message kinds insufficient.

## Decisions

1. **Introduce `BINDING_STEP` as the focused DRAFT state.**
   - When a DRAFT PlanConfiguration has at least one PlanStep without a SlotBinding executor installation, `DeriveState()` returns `BINDING_STEP` with the first unbound step key in template order.
   - When all PlanSteps are bound but status is still DRAFT, `DeriveState()` returns the existing `BINDING_MATRIX` state so the current review/save gate remains unchanged.
   - Alternative considered: keep `BINDING_MATRIX` state and infer the focused row on the frontend. That would blur the distinction between question turns and review turns and make duplicate suppression harder to reason about.

2. **Use one prompt payload shape for both conversational and matrix UI data.**
   - `BINDING_STEP` payload includes `state`, `step_key`, `options`, `policies_set`, and `rows`.
   - `options` duplicates the focused row's compatible installations for the chip renderer; `rows` remains the source for the collapsible matrix fallback.
   - `BINDING_MATRIX` payload remains compatible with the existing matrix parser and continues carrying `policies_set` and `rows`.
   - Alternative considered: emit separate prompt and matrix messages. That would introduce synchronization risk and extra durable chat noise.

3. **Keep persistence in `editBinding()` and add an explicit conversational wrapper.**
   - Add a frontend helper that calls `selectChip()` for chip history, `editBinding()` for the SlotBinding upsert, then appends `STEP_REBOUND` with previous and new installation IDs.
   - Matrix fallback selections use the same binding persistence helper and the same local hydration behavior, but only chip selections append `USER_SELECTION`.
   - `AppendPlanThreadMessage` already triggers `NextTurn` on `USER_SELECTION`, but that is too early if the SlotBinding has not yet persisted. The helper will append `USER_SELECTION` after the successful update, so `NextTurn` sees the saved binding and emits the next prompt.
   - Alternative considered: add a backend RPC that atomically appends selection, persists binding, and emits rebound. That is cleaner long-term but unnecessary for this pre-v1 UI-only orchestration and would expand backend API surface for this change.

4. **Render a dedicated `ConversationalBindingCard`.**
   - `ThreadMessage.svelte` routes `ASSISTANT_PROMPT` with state `BINDING_STEP` to the new card.
   - The card shows the assistant text, focused PlanStep title/contract, compatible installation chips, progress, and a collapsible "Edit all" matrix fallback.
   - The fallback uses the same rows parsed from the `BINDING_STEP` payload and calls the same binding upsert path as conversational chips.
   - User-facing copy is added to `en` and `pt-BR` flat translation files.

## Risks / Trade-offs

- [Risk] Appending `USER_SELECTION` after persistence reverses the intuitive order of "user clicked, then save" inside the helper. → Mitigation: the visible chat order still shows the selection before the server-emitted next assistant prompt because `AppendPlanThreadMessage` triggers `NextTurn` immediately after the persisted configuration is available.
- [Risk] `STEP_REBOUND` can be noisy if every first-time binding emits it. → Mitigation: the change explicitly requires it for selections; the text/payload includes both previous and new IDs, with previous empty on first bind.
- [Risk] Existing matrix code is large and includes policies/save concerns. → Mitigation: keep the review `BINDING_MATRIX` card unchanged and implement a focused, smaller fallback editor for `BINDING_STEP` rows rather than refactoring the whole matrix.
- [Risk] Duplicate suppression can suppress needed prompts if payloads do not reflect updated bindings. → Mitigation: `BINDING_STEP` payload rows include current bindings, so each persisted SlotBinding changes the payload and advances the `step_key`.

## Migration Plan

This is a pre-v1 change with no database migration and no proto migration. Deploy backend and frontend together. Existing DRAFT configurations with unbound steps will enter `BINDING_STEP` on their next assistant turn; existing DRAFT configurations with all steps bound continue to see the matrix review/save gate.

Rollback is code rollback only. Persisted SlotBindings are unchanged domain data, and `USER_SELECTION` / `STEP_REBOUND` messages already use existing message kinds.

## Open Questions

- None. Ambiguity on prompt state naming is resolved as `BINDING_STEP` for focused conversational turns and `BINDING_MATRIX` for the existing review/save gate.
