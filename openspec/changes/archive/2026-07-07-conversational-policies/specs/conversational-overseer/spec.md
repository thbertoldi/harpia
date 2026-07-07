## MODIFIED Requirements

### Requirement: Focused OverseerBinding prompt

The PlanConfiguration assistant SHALL emit a focused conversational OverseerBinding prompt for a DRAFT configuration when every PlanStep has an executor SlotBinding and at least one agent-backed PlanStep has no OverseerBinding.

#### Scenario: First missing overseer is prompted

- **WHEN** a DRAFT PlanConfiguration has every PlanStep bound to an ExecutorInstallation
- **AND** the first agent-backed PlanStep without an OverseerBinding is `write-draft`
- **THEN** the next assistant prompt has state `OVERSEER_STEP`
- **AND** the prompt focuses `write-draft`
- **AND** the prompt options include the current tenant user as an overseer candidate

#### Scenario: Non-agent steps are skipped

- **WHEN** a DRAFT PlanConfiguration has integration-backed PlanSteps and agent-backed PlanSteps
- **THEN** the assistant does not ask for OverseerBindings on integration-backed PlanSteps

#### Scenario: Existing overseer bindings are skipped

- **WHEN** the first agent-backed PlanStep already has an OverseerBinding
- **THEN** the next `OVERSEER_STEP` assistant prompt focuses the next agent-backed PlanStep without an OverseerBinding in template order

#### Scenario: All required overseers reach policy gate

- **WHEN** every agent-backed PlanStep has an OverseerBinding
- **AND** at least one required PlanBehaviorPolicies field is unset
- **THEN** the next assistant prompt has state `POLICIES_STEP`

#### Scenario: All required overseers and policies reach review gate

- **WHEN** every agent-backed PlanStep has an OverseerBinding
- **AND** every required PlanBehaviorPolicies field is set
- **THEN** the next assistant prompt has state `BINDING_MATRIX`
- **AND** the existing review and save flow remains the follow-on path
