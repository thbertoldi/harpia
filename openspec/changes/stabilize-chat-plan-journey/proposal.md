## Why

The chat-first plan journey has enough pieces in place to run a plan, but the experience still feels unfinished: catalog content leaks in English, execution and artifact surfaces are visually disconnected from the conversation, and approval work appears in notifications without an actionable in-thread decision point. This change stabilizes the existing journey before adding the next set of capabilities.

## What Changes

- Resolve PlanTemplate names, input labels, PlanStep titles/descriptions, proposal actions, post-create actions, and execution step labels through locale-aware catalog/i18n keys instead of raw backend strings.
- Make the `Run now` execution card align with the same conversation width as the other structured cards.
- Reconcile artifact preview behavior with the reference chat/artifact mock: artifacts should be launched from inline chat cards and preview in one canonical sheet/panel state, not remain disconnected in a sticky rail.
- Make approval requests actionable where the work happens: when a running execution reaches an approval gate, the chat thread should show an approval card with context, preview access, approve/reject actions, and synchronized inbox state.
- Improve inbox approval rows so they show meaningful context rather than a generic notification.
- Keep all user-facing copy in flat `translate()` keys for `en` and `pt-BR`.
- Capture richer LinkedIn content generation as a separate follow-up rather than silently expanding this stabilization scope.

## Capabilities

### New Capabilities

- `in-thread-approval`: Execution approval requests are rendered as actionable chat cards and stay synchronized with the inbox approval lifecycle.

### Modified Capabilities

- `conversational-plan-proposal`: Proposal and post-create actions must render localized plan/catalog labels and localized action copy.
- `conversational-configuration`: Configuration review/binding surfaces must render localized PlanTemplate, PlanStep, and input metadata.
- `chat-thread`: Structured execution cards, approval cards, and artifact cards must align visually with the conversation column and preserve chronological context.
- `artifact-side-preview`: Artifact preview must be launched from inline artifact cards and use one canonical preview panel state rather than a sticky, disconnected rail.
- `runs-panel`: Execution/inbox links back to a plan run must land the user in the origin thread with the relevant plan/execution context selected.

## Impact

- Frontend components: `frontend/src/routes/chat/[threadId]/+page.svelte`, `frontend/src/lib/components/thread/PlanProposalCard.svelte`, `BindingMatrixCard.svelte`, `PlanExecutionCard.svelte`, `ThreadMessage.svelte`, `SystemEventCard.svelte`, `ApprovalRefCard.svelte`, and artifact preview components under `frontend/src/lib/components/artifacts/`.
- Frontend helpers/i18n: `frontend/src/lib/catalog-i18n.ts`, plan/thread view-model helpers, inbox aggregator helpers, and `frontend/src/lib/i18n/{en,pt-BR}.json`.
- Backend if needed: approval payload shaping in control-plane thread/watch messages and inbox aggregation payloads, but no schema migration is expected before v1.
- Tests: frontend logic tests for locale resolution, execution/card sizing behavior via class/view-model assertions where available, artifact panel state, approval aggregation, and hardcoded-copy coverage; backend tests only if approval payloads need additional context.
