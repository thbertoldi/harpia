## Why

ADR-012 requires a PlanConfiguration to include OverseerBindings before it can become RUNNABLE, but the current chat flow stops at executor binding and leaves overseer assignment as a read-only matrix stub. `conversational-overseer` is the next slice of the fully conversational configuration epic: after agent-backed steps have executors, the assistant should ask who oversees them and persist valid OverseerBindings in chat.

## What Changes

- Add a conversational overseer prompt after SlotBindings are complete and before the existing matrix review/save gate.
- Focus only agent-backed PlanSteps that require an OverseerBinding, using the current SlotBindings and template metadata to determine the required steps.
- Offer MVP overseer choices as chips: the current signed-in tenant user as the primary option, with room in the payload shape for tenant users once identity lookup is available.
- Persist overseer selections through the existing `UpdatePlanConfiguration` RPC with `announceSaved=false`.
- Append `USER_SELECTION` and `STEP_REBOUND` thread history for conversational overseer selections, including previous/new overseer user ids.
- Keep the existing matrix review/save fallback, but make its overseer cell reflect saved OverseerBindings instead of a read-only display stub.
- Enforce the ADR-012 promotion invariant: RUNNABLE/SCHEDULED configurations with agent-backed steps MUST fail validation when required OverseerBindings are missing.
- Keep behavior policies, schedule, and promotion flow unchanged for later OpenSpec changes.

## Capabilities

### New Capabilities

- `conversational-overseer`: Covers guided OverseerBinding prompts for agent-backed steps, overseer prompt payloads, USER_SELECTION/STEP_REBOUND history, persistence through UpdatePlanConfiguration, synchronized matrix display, and promotion validation for required overseers.

### Modified Capabilities

- None.

## Impact

- Backend assistant state derivation, prompt payload builders, promotion validation, and assistant turn creation in `control-plane/internal/planassistant`, `control-plane/internal/plans`, and `control-plane/internal/chat`.
- Frontend thread rendering, new overseer selection helper/card, existing matrix display, and i18n files under `frontend/src/lib/components/thread`, `frontend/src/lib/plans`, and `frontend/src/lib/i18n`.
- Focused backend/frontend tests for overseer prompt derivation, payload content, persistence, thread history, and matrix synchronization.
- Proto files are expected to remain unchanged because PlanConfiguration already carries OverseerBindings and chat message kinds already cover selections/rebounds.
