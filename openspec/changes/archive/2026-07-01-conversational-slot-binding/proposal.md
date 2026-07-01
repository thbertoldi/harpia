## Why

PlanConfiguration setup currently drops users into a flat binding matrix even though ADR-012 frames chat as the PlanConfiguration assistant. SlotBinding is the first unfinished step in the fully conversational configuration epic, and it needs to guide users through executor selection without losing the existing bulk-edit fallback.

## What Changes

- Add a conversational per-PlanStep binding prompt that asks who should handle the focused step and offers compatible ExecutorInstallation options as chips, including seeded RSS preset installations.
- Carry both the focused-step options and the full binding matrix rows in the assistant prompt payload so the conversational view and the collapsible "Edit all" matrix read the same source.
- Re-enable `USER_SELECTION` and `STEP_REBOUND` thread history for installation selections: user chip selection persists a SlotBinding, then the assistant records the rebound before advancing to the next unbound step.
- Keep the existing binding matrix as a collapsible fallback that uses the same `UpdatePlanConfiguration` RPC and stays in sync with conversational selections.
- Keep overseer assignment, behavior policies, schedule, and promotion behavior unchanged for later OpenSpec changes.

## Capabilities

### New Capabilities

- `conversational-slot-binding`: Covers guided SlotBinding prompts, shared prompt payloads for conversational chips and matrix rows, USER_SELECTION/STEP_REBOUND history, and sync between conversational and matrix binding surfaces.

### Modified Capabilities

- None.

## Impact

- Backend assistant state machine and message payload builders in `control-plane/internal/planassistant` and `control-plane/internal/chat`.
- Frontend thread rendering and binding helpers in `frontend/src/lib/components/thread`, `frontend/src/lib/plans`, and i18n files.
- Focused backend/frontend tests for prompt advancement, message history, persistence, and shared binding payload rendering.
- Proto files are expected to remain unchanged because the relevant chat message kinds and PlanConfiguration SlotBinding RPCs already exist.
