## 1. Inventory And Baseline

- [x] 1.1 Inspect the conversational configuration surface and record the current edit points in implementation notes: `frontend/src/lib/plans/matrix.ts`, `frontend/src/lib/plans/assistant.ts`, `frontend/src/lib/components/thread/AssistantPromptCard.svelte`, `ConversationalBindingCard.svelte`, `ConversationalOverseerCard.svelte`, `ConversationalPoliciesCard.svelte`, `BindingMatrixCard.svelte`, and `ThreadMessage.svelte`.
- [x] 1.2 Search `frontend/src/lib/plans` and `frontend/src/lib/components/thread` for helpers that only forward to another helper without validation, transformation, event construction, or durable domain language; list any candidates before editing.
- [x] 1.3 Run the existing focused baseline tests before extraction: `cd frontend && bunx vitest run src/lib/plans/matrix.test.ts src/lib/plans/assistant.test.ts`.

## 2. Pure Configuration Flow Module

- [x] 2.1 Add `frontend/src/lib/plans/configuration-flow.ts` with pure exported types/helpers for the conversational PlanConfiguration frontend flow; the module MUST NOT import Svelte components, Svelte stores, or RPC clients.
- [x] 2.2 Add `frontend/src/lib/plans/configuration-flow.test.ts` covering the golden path from proposal/configuration draft through `BINDING_STEP`, `OVERSEER_STEP`, `POLICIES_STEP`, `BINDING_MATRIX`, and runnable/review-ready states using plain TypeScript fixtures.
- [x] 2.3 Move duplicated render-decision or progress-summary logic from conversational cards into `configuration-flow.ts` only when it can be expressed as pure input -> output behavior; leave local UI state such as saving flags in the Svelte components.
- [x] 2.4 Update Svelte call sites to consume the extracted helpers without changing visible behavior. Because this touches `.svelte` files, use the Svelte MCP/code-writer validation required by repo instructions before finalizing implementation.

## 3. Typed Payload Parser Boundary

- [x] 3.1 Keep existing typed parser exports in `frontend/src/lib/plans/matrix.ts` unless the implementation shows a clear split is needed; if split, mechanically move them to `frontend/src/lib/plans/configuration-payloads.ts` and preserve existing public behavior.
- [x] 3.2 Ensure `AssistantPromptCard.svelte`, `ConversationalBindingCard.svelte`, `ConversationalOverseerCard.svelte`, `ConversationalPoliciesCard.svelte`, `BindingMatrixCard.svelte`, and `ThreadMessage.svelte` do not read raw assistant prompt payload JSON fields directly except through shared typed parser helpers or local safe-fallback wrappers around those helpers.
- [x] 3.3 Extend `frontend/src/lib/plans/matrix.test.ts` or add `configuration-payloads.test.ts` for any parser behavior added or moved, including invalid JSON, wrong `state`, missing required arrays, and valid focused prompt payloads.
- [x] 3.4 Preserve existing malformed-payload UI behavior: parser errors may be caught by component-local safe wrappers, but normal component rendering MUST NOT throw for invalid assistant payload JSON.

## 4. Mutation Helper Hygiene

- [x] 4.1 Update `frontend/src/lib/plans/assistant.ts` so exported helpers either map to a backend mutation or perform meaningful domain work such as preserving configuration fields, transforming parameter values, appending domain events, or hiding transport details.
- [x] 4.2 Delete any remaining no-op pass-through helpers discovered in task 1.2 and update all callers to use the underlying helper directly.
- [x] 4.3 Keep or add focused `frontend/src/lib/plans/assistant.test.ts` coverage for the remaining behavior only; do not add tests that exist solely to assert deleted indirection.

## 5. Frontend Agent Guide

- [x] 5.1 Add `frontend/src/lib/plans/README.md` documenting the conversational PlanConfiguration golden path, key files, parser boundary, mutation-helper rule, and where new state/view-model logic belongs.
- [x] 5.2 Include verification commands in the guide: focused Vitest files for the touched helpers, `cd frontend && bun run lint`, and `cd frontend && bun run check` with the known baseline noted until it is fixed.
- [x] 5.3 Reference OpenSpec change `ai-friendly-configuration-flow` in the guide so future agents can trace the rationale.

## 6. Verification

- [x] 6.1 Run `openspec validate --changes ai-friendly-configuration-flow`.
- [x] 6.2 Run focused frontend tests: `cd frontend && bunx vitest run src/lib/plans/configuration-flow.test.ts src/lib/plans/matrix.test.ts src/lib/plans/assistant.test.ts` plus any parser test file added in task 3.
- [x] 6.3 Run `cd frontend && bun run lint`.
- [x] 6.4 Run `cd frontend && bun run check` and confirm any diagnostics are limited to the known baseline, with no new diagnostics in files touched by this change.
