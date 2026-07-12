# conversational-slot-binding

## Purpose
Amend the archived conversational-slot-binding capability so its matrix and
review-gate requirements honor optional-capability opt-out: only steps that
**will run** participate. This aligns the spec with the Platform Constitution
(only steps that will run require bindings) and with the canonical
`planrules.StepWillRun` predicate the runtime now uses.

## MODIFIED Requirements

### Requirement: All steps bound reaches review gate

The PlanConfiguration assistant SHALL reach the `BINDING_MATRIX` review gate only when every PlanStep **that will run** has a bound ExecutorInstallation. Steps whose optional capability the user did not include SHALL NOT require a binding and SHALL NOT block the review gate.

#### Scenario: All runnable steps bound reaches review gate

- **WHEN** a DRAFT PlanConfiguration has a SlotBinding with an ExecutorInstallation for every PlanStep that will run (optional-capability steps the user did not include are excluded)
- **THEN** the next assistant prompt has state `BINDING_MATRIX`
- **AND** the existing save, policy, and landing flow remains the follow-on path

#### Scenario: Opted-out step does not block the review gate

- **WHEN** a DRAFT PlanConfiguration has an opted-out optional PlanStep (e.g. `generate-image` with `include_images=no`) that has no SlotBinding
- **AND** every PlanStep that will run has a SlotBinding
- **THEN** the next assistant prompt still reaches `BINDING_MATRIX`
- **AND** the opted-out step is not emitted as a required matrix row

### Requirement: Shared conversational and matrix payload

The focused SlotBinding assistant prompt SHALL carry both the focused step data and the full matrix rows for every PlanStep **that will run** in one payload. Steps whose optional capability is not included SHALL be omitted from `rows`.

#### Scenario: Payload feeds chips and fallback matrix

- **WHEN** the assistant emits a `BINDING_STEP` prompt
- **THEN** the payload includes the focused `step_key`
- **AND** the payload includes `options` for that focused step
- **AND** the payload includes `rows` only for every PlanStep that will run, with current executor bindings and compatible options (opted-out steps are excluded)

#### Scenario: Existing binding is reflected in both surfaces

- **WHEN** a PlanStep that will run is already bound to an ExecutorInstallation
- **THEN** the row for that PlanStep carries the bound installation id
- **AND** any fallback matrix control for that row displays the same installation as selected
