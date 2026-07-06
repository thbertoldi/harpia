## Why

Conversational PlanConfiguration behavior is increasingly spread across Svelte components, parser helpers, and mutation helpers, making future AI-assisted changes more likely to duplicate logic or pick the wrong abstraction. Centralizing the flow model and documenting helper boundaries will make the chat-first configuration path easier to inspect, test, and safely extend.

## What Changes

- Add a frontend configuration-flow module that exposes pure, tested view-model and state helpers for the PlanConfiguration assistant path.
- Standardize typed payload parsing so conversational components consume parsed payloads instead of reaching into raw JSON shapes.
- Document a frontend mutation-helper rule: keep one helper per backend mutation unless a wrapper adds domain transformation, validation, or meaningful reuse.
- Remove any remaining no-op pass-through helpers discovered in the conversational configuration path.
- Add a short frontend maintainer/agent guide for the golden configuration flow, key files, and verification commands.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `conversational-configuration`: Add maintainability requirements for centralized configuration-flow state/view-model helpers, typed payload parser boundaries, and mutation-helper hygiene.

## Impact

- Affected code: `frontend/src/lib/plans/**`, `frontend/src/lib/components/thread/**`, and frontend documentation near the plan/chat flow.
- APIs: No proto, ConnectRPC, route, or database changes expected.
- Tests: Focused Vitest coverage for pure flow helpers, payload parsers, and any remaining mutation helper behavior.
- Risks: Refactor may touch several conversational cards; scope should stay limited to extracting existing behavior and deleting no-op indirection.
