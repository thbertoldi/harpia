## Context

The conversational PlanConfiguration path has grown across proposal cards, focused binding/overseer/policy cards, matrix review, assistant mutation helpers, and payload parser helpers. Recent work made configuration selections server-authoritative, but the frontend still lacks one explicit map of the assistant flow and the criteria for adding or deleting helper functions.

The current parser boundary is mostly in `frontend/src/lib/plans/matrix.ts`, while mutation helpers live in `frontend/src/lib/plans/assistant.ts`. The implementation should strengthen these existing seams instead of introducing a separate abstraction stack.

## Goals / Non-Goals

**Goals:**

- Make the PlanConfiguration assistant flow discoverable through a small pure TypeScript module and documentation.
- Keep Svelte conversational cards focused on rendering, local UI state, and invoking explicit mutations.
- Keep payload parsing typed, tested, and outside component bodies except for local safe-fallback wrappers.
- Define and apply a rule for deleting thin pass-through mutation helpers.
- Preserve current user-facing behavior.

**Non-Goals:**

- Rebuild the chat UI or replace existing conversational cards.
- Change backend PlanConfiguration state derivation, proto messages, ConnectRPC contracts, or database schema.
- Introduce a rendered-component testing framework.
- Archive existing OpenSpec changes or complete manual smoke checks unrelated to this refactor.

## Decisions

### Decision: Add a Pure `configuration-flow` Module

Create `frontend/src/lib/plans/configuration-flow.ts` for durable frontend flow/view-model helpers. The module should not call RPC clients, read Svelte stores, or import Svelte components. It should consume already-fetched templates, configurations, messages, and parsed payloads, then return simple objects that components can render.

Alternatives considered:

- Put flow helpers in each component. This keeps files local but preserves today's ambiguity and duplicated decisions.
- Move everything into `matrix.ts`. That file already owns matrix payload parsing and hydration; adding full flow policy there would make the name misleading.

### Decision: Keep Parser Ownership Near Existing `matrix.ts` Unless a Split Becomes Obvious

The initial implementation should standardize parser exports and tests where they already exist. If `matrix.ts` becomes too broad, split parser code into `configuration-payloads.ts` as a mechanical move with no behavior change.

Alternatives considered:

- Create a new parser module immediately. This is clean, but it risks churn without a current size or ownership problem.

### Decision: Mutation Helpers Must Either Map to a Backend Operation or Add Domain Behavior

`frontend/src/lib/plans/assistant.ts` should expose helpers that correspond to backend mutations (`submitConfigurationSelection`, `UpdatePlanConfiguration`, thread event append paths) or helpers that perform a meaningful transformation. A function that only renames arguments and forwards to another helper should be deleted or inlined.

Alternatives considered:

- Keep semantic wrappers for every UI card. This can read nicely in call sites, but it gives future agents multiple equivalent APIs and encourages stale unused arguments.

### Decision: Document the Golden Path for Agents

Add a short frontend guide near the plan/chat code that names the golden flow, key files, mutation rules, parser boundary, and verification commands. The document should be terse and operational, not an architecture essay.

Alternatives considered:

- Rely only on `AGENTS.md` and ADRs. Those are useful, but they operate at a broader altitude than the files agents modify for this flow.

## Risks / Trade-offs

- Refactor churn across Svelte cards -> Keep changes extraction-only and verify with focused tests plus lint.
- A new module could become a dumping ground -> Limit it to pure flow/view-model helpers and document non-goals in the module guide.
- Tests could lock in implementation details -> Prefer tests around observable helper outputs, parser failure modes, and mutation behavior rather than component internals.
- Existing baseline type errors may obscure regressions -> Run `bun run check` and record that only the known baseline remains; prioritize removing the baseline in a separate change.

## Migration Plan

1. Add pure helper tests first for the expected golden path and edge cases.
2. Extract existing flow decisions into `configuration-flow.ts` without changing behavior.
3. Standardize parser use from conversational cards and keep malformed-payload fallback behavior intact.
4. Delete no-op mutation wrappers, if any remain, and update call sites/tests.
5. Add the frontend guide and verification notes.
6. Roll back by reverting the extraction commit; no persisted data or API compatibility path is required.

## Open Questions

- Should parser exports stay in `matrix.ts` permanently, or should the implementation split them once the flow helper lands?
- Should the frontend type-check baseline cleanup be a prerequisite for archive, or tracked as a separate OpenSpec change?
