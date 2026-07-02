## Context

`conversational-slot-binding` restored the per-step chat flow for executor SlotBindings, then falls through to `BINDING_MATRIX` once every PlanStep has an executor. The matrix payload already carries `current_overseer_id` and `current_overseer_label`, but the UI treats the overseer cell as display-only. ADR-012 requires one OverseerBinding per agent-backed step and says RUNNABLE/SCHEDULED validation fails if an agent-backed step lacks an overseer.

The current backend state machine has only `AWAITING_TEMPLATE`, `BINDING_STEP`, `BINDING_MATRIX`, and `SAVED`. The frontend already has a good pattern for conversational choices: parse an assistant payload, render chips, persist through `UpdatePlanConfiguration`, append `USER_SELECTION`, append `STEP_REBOUND`, and let `STEP_REBOUND` trigger `NextTurn`.

## Goals / Non-Goals

**Goals:**

- Add a conversational state that asks for one missing OverseerBinding at a time after all executor SlotBindings are complete.
- Scope the prompt to agent-backed PlanSteps only, using the template's executor requirement as the source of truth.
- Offer the current signed-in tenant user as the MVP overseer option and keep the payload extensible for future tenant-user choices.
- Persist overseer choices through the existing `UpdatePlanConfiguration` RPC with `announceSaved=false`.
- Append `USER_SELECTION` and `STEP_REBOUND` after successful overseer selection, then advance to the next missing overseer or the existing matrix review/save gate.
- Make matrix rows reflect saved OverseerBindings.
- Enforce backend promotion validation for missing OverseerBindings on RUNNABLE/SCHEDULED configurations.

**Non-Goals:**

- No agent-as-overseer, team delegation, or tenant-user picker beyond the current user option.
- No behavior-policy generalization.
- No schedule-in-chat work.
- No new proto fields unless implementation proves the existing JSON payload and PlanConfiguration fields insufficient.
- No database migration.

## Decisions

1. **Introduce `OVERSEER_STEP` between `BINDING_STEP` and `BINDING_MATRIX`.**
   - `DeriveState()` continues to prioritize missing executor bindings first.
   - Once all PlanSteps have SlotBindings, it finds the first agent-backed PlanStep without an OverseerBinding and returns `OVERSEER_STEP` with that step key.
   - Once required overseers are bound, it returns the existing `BINDING_MATRIX` state.
   - Alternative considered: fold overseer selection into `BINDING_MATRIX`. That would preserve the current flat form, but it would not satisfy the conversational-first direction and would hide the missing validation invariant until save.

2. **Derive required overseer steps from the PlanTemplate, not persisted SlotBinding kind.**
   - The current frontend `editBinding()` path sends only `executorInstallationId`; it does not populate `executorKind`.
   - Template `PlanStep.executor_requirement.executor_kind == AGENT` is stable for deciding which steps can elicit and therefore need a human overseer.
   - Alternative considered: query selected ExecutorInstallations to infer agent-backed rows. That is more dynamic, but it adds catalog lookups to state derivation and diverges from the existing `getOverseerRequiredSteps()` helper.

3. **Use a dedicated overseer prompt payload.**
   - Add a payload builder for `OVERSEER_STEP` containing `state`, `step_key`, `options`, `rows`, and `required_step_keys`.
   - `options` contains overseer choices for the focused step. MVP emits one option for the current user: `{ id, label, value }`, where `value` is the user id.
   - `rows` reuses `AssistantMatrixRow` so the card can show step context and the matrix can stay synchronized.
   - Alternative considered: reuse `BINDING_STEP` payload with different option semantics. A distinct state keeps frontend parsing and `STEP_REBOUND` payload interpretation unambiguous.

4. **Mirror the existing frontend selection helper pattern.**
   - Add `editOverseerBinding()` to upsert an OverseerBinding while preserving seed artifacts, SlotBindings, policies, schedule, and parameter values.
   - Add `selectOverseerOption()` to append `USER_SELECTION`, call `editOverseerBinding()`, then append `STEP_REBOUND`.
   - Extend `STEP_REBOUND` payloads additively with `previous_overseer_user_id` and `new_overseer_user_id` for overseer changes. Existing executor fields remain for executor rebinding.
   - Alternative considered: introduce a new thread message kind for overseer assignment. The existing generic "step rebound" event already represents step-level configuration changes and avoids proto churn.

5. **Make promotion validation server-side.**
   - Add validation that rejects RUNNABLE/SCHEDULED when any required agent-backed PlanStep lacks an OverseerBinding.
   - Run that validation from Create/Update paths alongside SlotBinding validation.
   - DRAFT remains permissive so the assistant can save partial configuration as the user progresses.
   - Alternative considered: rely on frontend validation only. That would violate ADR-012 and allow API clients to create invalid runnable configurations.

6. **Keep `BINDING_MATRIX` as the review/save gate.**
   - The matrix continues to summarize executors, overseers, policies, and schedule before promotion.
   - Its overseer cell should display the saved binding for each row. Editing all overseers in the matrix can be minimal for this slice: at least keep display synchronized and use the conversational path for missing assignments.
   - Alternative considered: remove matrix review after overseer completion. That belongs to a later schedule/promote slice because policies and scheduling are still not conversational.

## Risks / Trade-offs

- [Risk] The MVP option list has only the current user, so the interaction can feel ceremonial. -> Mitigation: still required because it persists real OverseerBindings and unlocks correct RUNNABLE validation; payload shape leaves room for tenant users.
- [Risk] `STEP_REBOUND` payload becomes polymorphic. -> Mitigation: add overseer fields without removing executor fields, and keep renderer text generic enough for both.
- [Risk] Duplicate prompt suppression may suppress an overseer prompt after local matrix edits. -> Mitigation: `OVERSEER_STEP` payload includes rows and required step keys, so persisted overseer changes alter the next payload or advance state.
- [Risk] Backend validation may reveal existing invalid RUNNABLE/SCHEDULED configs. -> Mitigation: pre-v1 permits schema/protocol hardening; DRAFT remains permissive and users can repair via chat.

## Migration Plan

This is a pre-v1 code-only change. Deploy backend and frontend together. Existing DRAFT configurations with complete SlotBindings and missing agent-step OverseerBindings will enter `OVERSEER_STEP` on the next assistant turn. Existing configurations already promoted without overseers may fail future update/execution validation until repaired.

Rollback is code rollback only. Persisted OverseerBindings are normal PlanConfiguration data, and additive `STEP_REBOUND` payload fields are ignored by older readers.

## Open Questions

- None for MVP. Broader tenant-user choice and agent-as-overseer remain explicitly post-MVP/future-slice work.
