# conversational-overseer Specification

## Purpose
Define the chat-first OverseerBinding flow for PlanConfiguration setup: the assistant prompts for required human overseers on agent-backed PlanSteps, persists those bindings through the existing PlanConfiguration API, keeps the matrix review synchronized, and enforces overseer completeness before RUNNABLE or SCHEDULED promotion.
## Requirements
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

### Requirement: Overseer prompt payload

The focused OverseerBinding assistant prompt SHALL carry the focused step data, overseer options, required overseer step keys, and full matrix rows in one payload.

#### Scenario: Payload feeds chips and context rows

- **WHEN** the assistant emits an `OVERSEER_STEP` prompt
- **THEN** the payload includes the focused `step_key`
- **AND** the payload includes overseer `options` for that focused step
- **AND** the payload includes `required_step_keys` for every agent-backed PlanStep requiring an overseer
- **AND** the payload includes `rows` for every PlanStep with current executor and overseer bindings

#### Scenario: Existing overseer is reflected in rows

- **WHEN** a PlanStep already has an OverseerBinding
- **THEN** the row for that PlanStep carries the bound overseer user id
- **AND** the row label displays the selected overseer rather than the unbound default

### Requirement: Conversational overseer selection persists and advances

Selecting an overseer chip SHALL persist an OverseerBinding through `UpdatePlanConfiguration`, append `USER_SELECTION`, append `STEP_REBOUND`, and advance the assistant to the next missing overseer or review gate.

#### Scenario: First-time overseer selection creates binding

- **WHEN** the user selects an overseer chip for an agent-backed PlanStep without an OverseerBinding
- **THEN** the frontend persists an OverseerBinding for that PlanStep and user using `UpdatePlanConfiguration` with `announceSaved=false`
- **AND** the thread contains a `USER_SELECTION` message for the selected chip
- **AND** the thread contains a `STEP_REBOUND` message with an empty previous overseer user id and the selected new overseer user id

#### Scenario: Rebinding replaces overseer

- **WHEN** the user revisits a previously bound agent-backed PlanStep and selects a different overseer
- **THEN** the existing OverseerBinding for that PlanStep is replaced with the new overseer user id
- **AND** the `STEP_REBOUND` message records both the previous and new overseer user ids

#### Scenario: Next prompt sees persisted overseer

- **WHEN** a conversational overseer selection is saved
- **THEN** the assistant derives the next turn from the updated PlanConfiguration
- **AND** it focuses the next agent-backed PlanStep without an OverseerBinding or falls through to `BINDING_MATRIX` when none remain

### Requirement: Matrix overseer display stays synchronized

The matrix review and edit-all surfaces SHALL read the same OverseerBinding data as conversational overseer selections.

#### Scenario: Conversational selection updates matrix row

- **WHEN** the user selects an overseer chip for an agent-backed PlanStep
- **THEN** subsequent matrix rows for that PlanStep reflect the selected overseer user id and label without requiring manual re-entry

#### Scenario: Matrix review blocks missing required overseers

- **WHEN** a DRAFT PlanConfiguration has all executor SlotBindings but an agent-backed PlanStep lacks an OverseerBinding
- **THEN** the assistant emits `OVERSEER_STEP` before the matrix review/save gate

### Requirement: Runnable validation requires overseers

RUNNABLE and SCHEDULED PlanConfigurations SHALL require an OverseerBinding for every agent-backed PlanStep.

#### Scenario: Draft permits incomplete overseers

- **WHEN** a DRAFT PlanConfiguration has an agent-backed PlanStep without an OverseerBinding
- **THEN** create and update validation accepts the configuration as an incomplete draft

#### Scenario: Runnable rejects missing overseer

- **WHEN** a PlanConfiguration is created or updated with status RUNNABLE
- **AND** an agent-backed PlanStep lacks an OverseerBinding
- **THEN** validation fails with a precondition error

#### Scenario: Scheduled rejects missing overseer

- **WHEN** a PlanConfiguration is created or updated with status SCHEDULED
- **AND** an agent-backed PlanStep lacks an OverseerBinding
- **THEN** validation fails with a precondition error

#### Scenario: Runnable accepts complete overseers

- **WHEN** a PlanConfiguration is created or updated with status RUNNABLE
- **AND** every agent-backed PlanStep has an OverseerBinding
- **THEN** validation accepts the configuration if all other SlotBinding, policy, seed, and schedule requirements pass

### Requirement: Localized and motion-safe overseer UI

The conversational OverseerBinding UI SHALL localize all user-facing copy and SHALL use only reduced-motion-aware opacity or transform transitions.

#### Scenario: Copy resolves in supported locales

- **WHEN** the conversational OverseerBinding card renders in `en` or `pt-BR`
- **THEN** every prompt label, button label, progress label, and selected-overseer label resolves through flat translation keys

#### Scenario: Selection motion respects constraints

- **WHEN** the user selects an overseer chip
- **THEN** the UI uses existing reduced-motion-aware motion primitives
- **AND** no transition animates color, font, or layout dimensions

### Requirement: Overseer setup remains visible after conversational creation

The PlanConfiguration assistant SHALL keep OverseerBinding setup visible in the same thread after conversational refinement and SlotBinding completion.

#### Scenario: Overseer prompt follows completed slot bindings in same thread

- **WHEN** a plan created from conversational refinement has all required SlotBindings persisted
- **AND** at least one agent-backed PlanStep still lacks an OverseerBinding
- **THEN** the next assistant prompt in the same thread has state `OVERSEER_STEP`
- **AND** it appears below the prior binding selection turn

#### Scenario: Real-thread verification covers overseer transition

- **WHEN** the conversational refinement implementation is verified
- **THEN** verification includes a real thread that creates a plan, completes SlotBindings, and observes the `OVERSEER_STEP` prompt before the review gate

