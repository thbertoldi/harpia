## Context

The current chat-first journey is the result of several adjacent changes: conversational proposal/refinement, live execution cards, artifact preview, plan execution runtime, and policy collection. The pieces are mostly present, but they are not yet cohesive in the real user flow.

Observed gaps:

- Catalog content such as `Weekly Newsletter`, `Fetch News`, `Write Draft`, and action labels still render from raw backend/template strings in several chat cards.
- `PlanExecutionCard` uses a conversational bubble width (`max-w-[85%]`) even when it is rendered as a structured plan execution block.
- Artifact preview has competing concepts: a sticky/disconnected rail and a richer `ArtifactPreviewSheet` inspired by the reference mock.
- Approval requests appear in notification/inbox surfaces, but the user cannot make the approval decision in the same chat where the execution is already running.
- Inbox approval rows lack enough context to tell the user what they are approving.

Constraints:

- ADR-012 remains authoritative: chat is the PlanConfiguration assistant, and UI surfaces browse the Artifact stream.
- Catalog content localizes via `catalog.plan.<key>.*` and `plans.inputs.<key>.label`; user-facing copy uses flat `translate()` keys in `frontend/src/lib/i18n/{en,pt-BR}.json`.
- Design tokens are locked; changes may adjust layout, opacity, and transform only.
- Harpia is pre-v1, so no compatibility shim is needed unless persisted data requires it.

## Goals / Non-Goals

**Goals:**

- Make the existing weekly-newsletter LinkedIn journey feel coherent in Portuguese and English.
- Use one locale-aware catalog metadata path for plan names, step titles/descriptions, input labels, and plan action labels.
- Keep execution, artifacts, and approvals in chronological chat context while preserving structured controls.
- Make inbox approvals and in-thread approvals reflect the same underlying approval request state.
- Align the artifact preview implementation with the richer reference pattern: inline launcher, transient side panel, preview/code affordances, and a closeable/reopenable state.

**Non-Goals:**

- Do not add carousel/image generation to the LinkedIn plan in this stabilization change.
- Do not introduce new ExecutorSKUs or external services.
- Do not redesign locked color or typography tokens.
- Do not replace the inbox; it remains useful as a cross-thread task list.

## Decisions

### 1. Localize Dynamic Catalog Metadata At Render Boundaries

Use catalog-aware helpers near `frontend/src/lib/catalog-i18n.ts` to resolve dynamic labels from stable keys:

- Plan name: `catalog.plan.<template_key>.name` with backend display name as fallback.
- Plan description: `catalog.plan.<template_key>.description` with backend description as fallback.
- Step title/description: `catalog.plan.<template_key>.step.<step_key>.title|description` with backend step metadata as fallback.
- Input label/help: existing `plans.inputs.<key>.label` and catalog/input fallbacks.

Alternative considered: rewrite YAML seed files into locale keys only. Rejected for this stabilization because the backend still needs a useful default display string and the frontend already owns locale resolution.

### 2. Structured Chat Cards Use Full Conversation Width

Message bubbles may keep constrained widths, but structured cards that represent plan state (`PlanProposalCard`, binding/policy cards, execution cards, approval cards, artifact cards) should use the same `w-full` conversation card width. This fixes the `Run now` execution card without changing the chat layout model.

Alternative considered: make the entire chat column wider/narrower. Rejected because the bug is card-local: `PlanExecutionCard` is using bubble sizing in a structured-card context.

### 3. ArtifactPreviewSheet Becomes Canonical

Use one panel-owning state in the chat route: `activeArtifactId` plus loading/ready state. Inline artifact cards open the canonical sheet. Sticky rails or duplicated panel state should be removed or reduced to launchers that delegate to the same state.

Alternative considered: keep the sticky rail and add more controls there. Rejected because the user specifically called out that artifacts pinned at the top feel disconnected from the scroll context, and the reference mock uses inline artifact launchers plus a transient panel.

### 4. Approval Is Both Inbox Work And Chat Work

The inbox remains the aggregate task list, but a plan execution that reaches an approval gate must append/render an in-thread approval card. The card should show what is being approved, link/open the produced artifact preview, and expose approve/reject actions. Decision state must be synchronized so acting in one surface updates the other.

Alternative considered: force all approvals through `/inbox`. Rejected because it breaks the running execution mental model and makes the notification feel empty.

### 5. Rich LinkedIn Content Is A Follow-Up Change

The user's ideas for stronger LinkedIn writing, carousel choices, and generated images are valid, but they alter plan structure and potentially artifact schemas/executors. Capture them as follow-up product work rather than burying them in a UI stabilization change.

Alternative considered: add more YAML steps immediately. Rejected because new steps like carousel/image generation require artifact and agent contracts, not just labels.

## Risks / Trade-offs

- Localized dynamic metadata could fall back silently if keys are missing -> add tests that the embedded templates used in the current journey resolve `en` and `pt-BR` names/steps/actions.
- In-thread and inbox approval state can diverge -> route both surfaces through the same approval request id and invalidate/reload both data sources after a decision.
- Removing sticky artifact behavior may reduce always-visible context for some users -> keep a lightweight “reopen artifact” affordance in the header or near the latest artifact card.
- Changing artifact behavior may conflict with older `artifact-side-preview` requirements that suppress intermediate artifacts -> update the spec to allow inline artifact cards while still emphasizing the final artifact by default.
