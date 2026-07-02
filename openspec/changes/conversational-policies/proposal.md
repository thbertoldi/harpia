## Why

The chat-first configuration flow already walks the user through executor SlotBindings and
OverseerBindings, but behavior policies still appear only as part of the review matrix and
remain partly LinkedIn-shaped. This leaves a gap in the ADR-012 model: PlanBehaviorPolicies
are first-class PlanConfiguration state, so the assistant should collect them as a
conversational turn before the final review/save gate.

## What Changes

- Add a conversational behavior-policy step to the `planassistant` state machine after
  required overseers are bound and before the review matrix.
- Ask for publish approval mode and elicitation timeout behavior using enum-backed chips,
  generalized from PlanBehaviorPolicies rather than LinkedIn-specific labels.
- Persist each selection through the existing PlanConfiguration update path by updating
  `parameter_values_json` fields that materialize to `PlanBehaviorPolicies`.
- Append durable `USER_SELECTION` and `STEP_REBOUND` messages for policy choices so the
  thread remains an auditable configuration transcript.
- Add a frontend `ConversationalPoliciesCard` mirroring the focused interaction patterns of
  `ConversationalBindingCard` and `ConversationalOverseerCard`.
- Keep the binding matrix as the synchronized review/edit-all fallback once both policy
  fields are set.

## Capabilities

### New Capabilities
- `conversational-policies`: The PlanConfiguration assistant collects PlanBehaviorPolicies
  in chat using enum-backed prompt chips, persists them generically, and advances to the
  review gate only after required policy fields are set.

### Modified Capabilities
- `conversational-overseer`: After all required OverseerBindings are complete, the next
  assistant state becomes the policy prompt rather than jumping directly to the matrix
  review gate.

## Impact

- Backend: `control-plane/internal/planassistant` state derivation, prompt payloads, and
  controller tests; plan update behavior that already materializes
  `parameter_values_json` into PlanBehaviorPolicies.
- Frontend: chat thread message routing, policy card component, assistant helpers,
  matrix hydration/summary helpers, and i18n copy in `en` and `pt-BR`.
- Tests: Go unit tests for state/payload progression; frontend unit tests for parsing,
  helper persistence, and copy; focused Playwright or component-level smoke coverage if
  needed for the threaded flow.
- No schema migration or compatibility shim is required; Harpia is pre-v1 and the backend
  remains authoritative for materialized PlanConfiguration fields.
