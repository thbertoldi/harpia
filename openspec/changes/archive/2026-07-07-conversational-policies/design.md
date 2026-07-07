## Context

The current plan-configuration assistant derives DRAFT state in this order:
unbound executor SlotBindings, missing agent OverseerBindings, then `BINDING_MATRIX`.
`BINDING_MATRIX` receives a `policies_set` flag and the frontend matrix card includes
policy controls, but policies are not collected as an assistant turn. That is inconsistent
with the chat-first model and makes the final review gate carry both review and data-entry
responsibilities.

Lane A made the backend authoritative for materialized PlanConfiguration fields:
clients send `parameter_values_json`, and the server expands template runtime mappings into
SeedArtifacts, SlotBindings, and PlanBehaviorPolicies. C1 should use that contract rather
than sending `behaviorPolicies` directly from the frontend.

## Goals / Non-Goals

**Goals:**
- Add a deterministic `POLICIES_STEP` assistant state after required overseers and before
  `BINDING_MATRIX`.
- Collect `publish_approval_mode` and `elicitation_timeout_behavior` through enum-backed
  chips that are not LinkedIn-specific.
- Persist selections by updating parameter values whose template runtime mappings target
  `BEHAVIOR_POLICY`.
- Keep the thread transcript durable: selected chips produce `USER_SELECTION`; saved policy
  changes produce `STEP_REBOUND` or a policy-equivalent rebound payload.
- Keep the matrix review synchronized and let it remain the edit-all fallback.

**Non-Goals:**
- Scheduling, status promotion to RUNNABLE/SCHEDULED, and "Revise" handoff are C2.
- MemoryResource capture for reusable preferences is logged separately and not implemented
  in this change.
- No new proto fields or migrations. The existing assistant payload JSON may evolve because
  Harpia is pre-v1.
- No new policy fields beyond `publish_approval_mode` and `elicitation_timeout_behavior`.

## Decisions

1. **Represent policies as their own assistant state.**

   Add `StatePoliciesStep = "POLICIES_STEP"` and derive it when all SlotBindings and
   required OverseerBindings are complete but `policiesSet(config.behavior_policies)` is
   false. This keeps the flow linear:
   `BINDING_STEP* -> OVERSEER_STEP* -> POLICIES_STEP -> BINDING_MATRIX`.

   Alternative considered: keep policies inside `BINDING_MATRIX`. That preserves existing
   UI but prevents policies from being an auditable conversational turn and delays the
   assistant's next prompt until the user finds controls in a review card.

2. **Use parameter values as the write surface.**

   The frontend helper should update `parameterValuesJson` keys associated with policy
   runtime mappings. For current templates, `approval_mode` maps to
   `publish_approval_mode`; C1 should add or reuse an `elicitation_timeout_behavior`
   parameter mapping. The server materializer computes `PlanBehaviorPolicies`; the client
   does not send `behaviorPolicies` directly.

   Alternative considered: call `UpdatePlanConfiguration` with `behaviorPolicies`. That
   conflicts with Lane A's backend-authoritative contract and would reintroduce template-
   specific client materialization.

3. **Prompt payload is explicit and generic.**

   Add a typed assistant payload with:
   - `state: "POLICIES_STEP"`
   - `fields`: the required policy field descriptors in order
   - `current_values`: current enum values from `behavior_policies` or parameter values
   - `options`: enum chip options grouped by field
   - `policies_set`

   The frontend should parse this payload in `plans/matrix.ts` or a sibling helper and
   render one compact card. Persisting one field should leave the card live until both
   required fields are set; once complete, `NextTurn` advances to `BINDING_MATRIX`.

4. **Reuse the existing transcript mechanics.**

   Policy chip selection appends `USER_SELECTION` with the selected field/value and appends
   a rebound-style system event that records previous and new values. If the existing
   `STEP_REBOUND` payload shape is too executor/overseer-specific, extend it with optional
   policy fields while preserving existing consumers.

5. **Localize all visible copy through flat keys.**

   All policy prompt labels, option labels, descriptions, progress text, selected-value
   text, and error copy must live in both `frontend/src/lib/i18n/en.json` and
   `frontend/src/lib/i18n/pt-BR.json`.

## Risks / Trade-offs

- **Template missing runtime mapping** -> the card could offer chips that cannot persist
  into materialized policies. Mitigation: derive policy fields from template input
  parameters with `BEHAVIOR_POLICY` mappings and fall back only to known default keys when
  the template is the shipped weekly newsletter template.
- **Duplicate prompts after each field save** -> `NextTurn` may emit another identical
  policy prompt. Mitigation: payload includes current values; controller duplicate
  suppression keeps byte-identical prompts from repeating, while changed payloads render
  the updated card.
- **Matrix and policy card drift** -> both surfaces may show different values. Mitigation:
  hydrate from the current PlanConfiguration plus `parameter_values_json`; add tests for
  summary/matrix helpers.
- **User-facing copy in payload text is backend-generated English** -> backend prompt text
  is already English today. Mitigation: keep frontend labels localized; defer backend prompt
  localization to a broader assistant copy pass.

## Migration Plan

This is pre-v1. Ship as a forward-only behavior change:
1. Add backend state/payload support and tests.
2. Add frontend card/helper/i18n support and focused tests.
3. Keep existing matrix policy controls working as a fallback.
4. Validate `conversational-policies` and run backend/frontend gates.
