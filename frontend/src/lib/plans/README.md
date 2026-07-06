# Conversational PlanConfiguration Flow

This folder owns the frontend logic for the chat-first PlanConfiguration path. See OpenSpec change `ai-friendly-configuration-flow` for the rationale.

## Golden Path

1. A user message produces a `PLAN_PROPOSED` card.
2. Confirming a proposal creates or links a PlanConfiguration in the same thread.
3. The assistant emits focused `ASSISTANT_PROMPT` states in order: `BINDING_STEP`, `OVERSEER_STEP`, `POLICIES_STEP`, then `BINDING_MATRIX`.
4. Chip selections call the server-authoritative `SubmitConfigurationSelection` path through `selectChip`.
5. Matrix/fallback edits use explicit `UpdatePlanConfiguration` helpers that preserve existing configuration fields.
6. A complete configuration reaches the review/runnable path and can execute or schedule.

## Edit Points

- `configuration-flow.ts`: pure state/view-model helpers for conversational configuration cards. Do not import Svelte components, Svelte stores, or RPC clients here.
- `matrix.ts`: typed parser and hydration helpers for `BINDING_STEP`, `OVERSEER_STEP`, `POLICIES_STEP`, and `BINDING_MATRIX` payloads.
- `assistant.ts`: frontend mutation helpers. Keep helpers that map to backend operations or perform domain transformation.
- `components/thread/*.svelte`: rendering, local UI state, and event handlers. Components should consume parsed payloads and view-model helpers rather than owning domain flow decisions.

## Parser Boundary

Assistant prompt payload JSON should be parsed through shared helpers before components read payload fields.

- Generic prompt routing: `parseAssistantPromptState`.
- Generic chip prompts: `parseGenericAssistantPromptPayload`.
- Configuration payloads: `parseBindingStepPayload`, `parseOverseerStepPayload`, `parsePoliciesStepPayload`, and `parseMatrixPayload`.

Components may keep small safe wrappers that catch parser errors and render the existing fallback UI, but they should not duplicate payload-shape parsing.

## Mutation Helper Rule

Add or keep a helper only when it maps to a backend mutation or adds meaningful domain behavior, such as preserving existing config fields, transforming `parameterValuesJson`, appending domain events, or hiding transport details.

Delete pass-through wrappers that only rename arguments and forward to another helper.

## Verification

Run focused tests for this surface:

```bash
cd frontend && bunx vitest run src/lib/plans/configuration-flow.test.ts src/lib/plans/matrix.test.ts src/lib/plans/assistant.test.ts
```

Run frontend lint:

```bash
cd frontend && bun run lint
```

Run frontend check and confirm no new diagnostics in touched files. The repo currently has a known baseline of 12 `svelte-check` errors outside this flow:

```bash
cd frontend && bun run check
```
