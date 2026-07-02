## 1. Backend State And Payloads

- [ ] 1.1 Add `POLICIES_STEP` to `control-plane/internal/planassistant/state.go` and derive it after SlotBindings and required OverseerBindings are complete but before `BINDING_MATRIX` when `policiesSet` is false.
- [ ] 1.2 Add/update `control-plane/internal/planassistant/state_test.go` coverage for overseer-complete/policies-missing, partial policies, and policies-complete transitions.
- [ ] 1.3 Add typed chat payload support in `control-plane/internal/chat/messages.go` for `POLICIES_STEP` fields, grouped enum chip options, current values, and policy completeness.
- [ ] 1.4 Add/update `control-plane/internal/chat/messages_test.go` for deterministic `POLICIES_STEP` JSON ordering and empty-option defaults.
- [ ] 1.5 Extend `control-plane/internal/planassistant/prompts.go` to build generic publish approval and elicitation timeout prompts from PlanBehaviorPolicies/runtime-mapping conventions.
- [ ] 1.6 Add/update `control-plane/internal/planassistant/prompts_test.go` and `controller_test.go` for `OVERSEER_STEP -> POLICIES_STEP -> BINDING_MATRIX` progression and duplicate prompt suppression.

## 2. Policy Parameter Persistence

- [ ] 2.1 Add frontend helper tests proving policy chip selections update `parameterValuesJson` keys mapped to `BEHAVIOR_POLICY` runtime mappings and never send `behaviorPolicies` directly.
- [ ] 2.2 Implement a frontend assistant helper that applies one policy value at a time through `UpdatePlanConfiguration`, preserving overseer bindings and schedule while updating `parameterValuesJson`.
- [ ] 2.3 Extend `STEP_REBOUND`/system-event payload handling only as needed to record previous/new policy field values without breaking existing executor/overseer rebound consumers.
- [ ] 2.4 Add backend or frontend focused tests proving the server materializes selected policy parameter values into PlanBehaviorPolicies after update.

## 3. Conversational Policy UI

- [ ] 3.1 Add parser/types for `POLICIES_STEP` payloads near `frontend/src/lib/plans/matrix.ts` or a dedicated helper, with tests for valid and malformed payloads.
- [ ] 3.2 Implement `frontend/src/lib/components/thread/ConversationalPoliciesCard.svelte` mirroring the focused chip interaction, answered-state edit affordance, error state, and reduced-motion-safe selection behavior of existing conversational cards.
- [ ] 3.3 Route `ASSISTANT_PROMPT` with `state === "POLICIES_STEP"` in `frontend/src/lib/components/thread/ThreadMessage.svelte`.
- [ ] 3.4 Keep matrix review/fallback synchronized so selected policy values and `policies_set` display correctly after conversational policy selection.
- [ ] 3.5 Add all user-facing policy-card copy to `frontend/src/lib/i18n/en.json` and `frontend/src/lib/i18n/pt-BR.json` using flat `translate()` keys.

## 4. Verification

- [ ] 4.1 Run `openspec validate --changes conversational-policies`.
- [ ] 4.2 Run focused Go tests for `control-plane/internal/chat` and `control-plane/internal/planassistant`.
- [ ] 4.3 Run focused frontend tests with `cd frontend && bunx vitest run <changed test files>`.
- [ ] 4.4 Run `cd frontend && bun run check` and confirm only the known baseline diagnostics remain.
- [ ] 4.5 Run `cd control-plane && go test ./...`.
- [ ] 4.6 Smoke the threaded flow manually or through Playwright enough to verify `BINDING_STEP -> OVERSEER_STEP -> POLICIES_STEP -> BINDING_MATRIX` appears in one conversation.
