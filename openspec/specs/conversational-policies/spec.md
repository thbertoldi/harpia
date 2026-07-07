# conversational-policies Specification

## Purpose
TBD - created by archiving change conversational-policies. Update Purpose after archive.
## Requirements
### Requirement: Focused behavior-policy prompt
The PlanConfiguration assistant SHALL emit a focused conversational behavior-policy prompt for a DRAFT configuration when every PlanStep has an executor SlotBinding, every required agent-backed PlanStep has an OverseerBinding, and at least one required PlanBehaviorPolicies field is unset.

#### Scenario: Policies are prompted after overseers
- **WHEN** a DRAFT PlanConfiguration has all SlotBindings
- **AND** every required OverseerBinding is set
- **AND** `publish_approval_mode` is unset
- **THEN** the next assistant prompt has state `POLICIES_STEP`
- **AND** the prompt includes the publish approval mode choices

#### Scenario: Partial policy state remains in policies prompt
- **WHEN** a DRAFT PlanConfiguration has `publish_approval_mode` set
- **AND** `elicitation_timeout_behavior` is unset
- **THEN** the next assistant prompt has state `POLICIES_STEP`
- **AND** the prompt includes the current publish approval mode value
- **AND** the prompt still offers elicitation timeout behavior choices

#### Scenario: Complete policies reach review gate
- **WHEN** every SlotBinding, required OverseerBinding, and required PlanBehaviorPolicies field is set
- **THEN** the next assistant prompt has state `BINDING_MATRIX`
- **AND** the matrix payload reports policies as set

### Requirement: Policy prompt payload
The behavior-policy assistant prompt SHALL carry generic policy field descriptors and enum chip options rather than template-specific copy.

#### Scenario: Payload contains required policy fields
- **WHEN** the assistant emits a `POLICIES_STEP` prompt
- **THEN** the payload includes `fields` for `publish_approval_mode` and `elicitation_timeout_behavior`
- **AND** each field includes stable option ids, labels, and values
- **AND** the payload includes the current value for any already-selected policy field

#### Scenario: Payload derives from runtime mappings
- **WHEN** a PlanTemplate declares input parameters with `BEHAVIOR_POLICY` runtime mappings
- **THEN** the policy prompt maps chip selections back to those parameter keys
- **AND** no client-side `PlanBehaviorPolicies` materialization is required

### Requirement: Conversational policy selection persists and advances
Selecting a behavior-policy chip SHALL persist the selected value through `UpdatePlanConfiguration`, append `USER_SELECTION`, append a rebound/audit message, and advance the assistant once all required policies are set.

#### Scenario: First policy selection updates parameter values
- **WHEN** the user selects `require_approval` for publish approval mode
- **THEN** the frontend updates `parameter_values_json` with the parameter key mapped to `publish_approval_mode`
- **AND** the frontend does not send `behaviorPolicies` directly
- **AND** the thread contains a `USER_SELECTION` message for the selected chip

#### Scenario: Second policy selection advances to matrix
- **WHEN** the user selects the final missing policy value
- **THEN** `UpdatePlanConfiguration` persists the new parameter values
- **AND** the server materializes `PlanBehaviorPolicies`
- **AND** the assistant emits the `BINDING_MATRIX` review prompt

#### Scenario: Re-selecting a policy records previous and new values
- **WHEN** the user edits an already selected policy value
- **THEN** the persisted parameter value is replaced
- **AND** the thread records the previous and new policy values in a durable system message

### Requirement: Matrix policy fallback stays synchronized
The existing matrix review surface SHALL remain a synchronized edit-all fallback for behavior policies.

#### Scenario: Conversational selection updates matrix policy summary
- **WHEN** the user sets behavior policies through `POLICIES_STEP`
- **THEN** the subsequent matrix review displays the selected policy values
- **AND** the matrix save gate treats policies as complete

#### Scenario: Matrix edit uses same persistence path
- **WHEN** the user changes a behavior policy from the matrix fallback
- **THEN** the update writes the same `parameter_values_json` keys used by the conversational policy card
- **AND** the server remains authoritative for the materialized PlanBehaviorPolicies

### Requirement: Localized and motion-safe policy UI
The conversational behavior-policy UI SHALL localize all user-facing copy and SHALL use only reduced-motion-aware opacity or transform transitions.

#### Scenario: Copy resolves in supported locales
- **WHEN** the conversational policy card renders in `en` or `pt-BR`
- **THEN** every prompt label, option label, description, progress label, selected-value label, and fallback label resolves through flat translation keys

#### Scenario: Policy chip motion respects constraints
- **WHEN** the user selects a policy chip
- **THEN** the UI uses existing reduced-motion-aware motion primitives
- **AND** no transition animates color, font, or layout dimensions

