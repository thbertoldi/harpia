## Why

The current plan-creation flow still feels like chat launching a form: proposal, input collection, confirmation, setup, and follow-up actions replace each other instead of accumulating as a conversation. This breaks the ADR-012 direction that chat is the PlanConfiguration assistant and makes overseer/binding progress hard to perceive after a plan is created.

## What Changes

- Add a conversational refinement phase before creating a PlanConfiguration: best-match plan highlight, candidate alternatives, assistant questions, selectable recommendations, and a natural final confirmation.
- Make proposal interactions accumulate as chat turns instead of replacing the proposal card's visible state.
- Recommend and collect audience, themes, topics to avoid, source groups, language/tone, and a clarified date-range publication window before creation.
- Support selecting multiple source groups end-to-end while preserving ADR-012 SlotBinding semantics by materializing one tenant-scoped RSS ExecutorInstallation config for the selected feed set.
- Keep the user in the same thread after creation with contextual actions to run, schedule, review, adjust configuration, or ask about another plan.
- Define one reusable plan-summary representation for chat, lists, configuration/settings, execution, artifacts, and audit surfaces.
- Include a real-thread overseer visibility check so the existing conversational-overseer flow is not hidden by the proposal-to-configuration transition.

## Capabilities

### New Capabilities

- `conversational-plan-refinement`: Covers the pre-create conversational refinement loop, recommended input chips, multi-source-group selection, final confirmation, post-create thread continuity, and shared plan-summary presentation contract.

### Modified Capabilities

- `conversational-plan-proposal`: Candidate presentation changes from a replace-in-place card to an accumulated conversational proposal with an explicit best match and natural-language confirmation.
- `conversational-slot-binding`: Slot-binding setup must be reachable from the same thread after plan creation and must use the same plan-summary vocabulary as proposal/refinement.
- `conversational-overseer`: Overseer prompts must remain visible and verifiable in the same post-create thread transition after slot bindings complete.

## Impact

- Frontend chat route and proposal components: `frontend/src/routes/chat/[threadId]/+page.svelte`, `frontend/src/lib/components/thread/PlanProposalCard.svelte`, `frontend/src/lib/components/thread/TemplateInputsForm.svelte`, and shared thread cards/helpers.
- Frontend i18n: `frontend/src/lib/i18n/en.json` and `frontend/src/lib/i18n/pt-BR.json`.
- Frontend tests for proposal/refinement state, source-group selection, plan-summary rendering, and post-create thread actions.
- Backend thread proposal/refinement handling: `control-plane/internal/threads/handler.go`, classifier payload shaping under `control-plane/internal/copilot/`, and plan-configuration creation helpers.
- Plan template input helpers for LinkedIn/news digest values and source-group materialization.
- Existing PlanConfiguration APIs are reused where possible; no production compatibility shim is required before v1.
