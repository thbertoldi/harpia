## 1. Backend Assistant State

- [x] 1.1 Add failing `planassistant` tests for `BINDING_STEP` derivation, prompt payload rows/options, advancement through unbound steps, and fall-through to `BINDING_MATRIX` when all steps are bound.
- [x] 1.2 Implement `BINDING_STEP` state derivation and prompt building with shared rows/options payload.
- [x] 1.3 Keep landing idempotency and existing `BINDING_MATRIX` review/save behavior unchanged.

## 2. Frontend Binding Helpers

- [x] 2.1 Add failing frontend tests for parsing a conversational binding payload and hydrating rows from SlotBindings.
- [x] 2.2 Add failing frontend tests that a conversational selection persists the SlotBinding with `announceSaved=false`, appends `USER_SELECTION`, and appends `STEP_REBOUND` with previous/new installation ids.
- [x] 2.3 Implement the parser and selection helper while preserving the existing `editBinding()` matrix path.

## 3. Conversational Binding UI

- [x] 3.1 Add the `ConversationalBindingCard.svelte` renderer for `BINDING_STEP` prompts with compatible-installation chips, progress, and a collapsible "Edit all" fallback matrix.
- [x] 3.2 Route `ThreadMessage.svelte` to the new card and keep `BINDING_MATRIX` routed to the existing `BindingMatrixCard.svelte`.
- [x] 3.3 Add `en` and `pt-BR` flat translation keys for all new user-facing copy.

## 4. Validation

- [x] 4.1 Run `openspec validate --changes conversational-slot-binding`.
- [x] 4.2 Run `cd control-plane && go test ./...`.
- [x] 4.3 Run `cd proto && buf lint`.
- [x] 4.4 Run `cd frontend && bun run check` and confirm no new errors beyond the baseline.
- [x] 4.5 Run `cd frontend && bunx vitest run <files touched>`.
- [x] 4.6 Archive `conversational-slot-binding` only after all gates pass.
