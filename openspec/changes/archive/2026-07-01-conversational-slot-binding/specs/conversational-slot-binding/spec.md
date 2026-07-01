## ADDED Requirements

### Requirement: Focused SlotBinding prompt

The PlanConfiguration assistant SHALL emit a focused conversational SlotBinding prompt for a DRAFT configuration when at least one PlanStep has no bound ExecutorInstallation.

#### Scenario: First unbound step is prompted

- **WHEN** a DRAFT PlanConfiguration has multiple PlanSteps and the first unbound PlanStep is `fetch-news`
- **THEN** the next assistant prompt has state `BINDING_STEP`
- **AND** the prompt focuses `fetch-news`
- **AND** the prompt options contain the compatible ExecutorInstallation candidates for `fetch-news`

#### Scenario: Bound steps are skipped

- **WHEN** a DRAFT PlanConfiguration has an existing SlotBinding for the first PlanStep
- **THEN** the next `BINDING_STEP` assistant prompt focuses the next unbound PlanStep in template order

#### Scenario: All steps bound reaches review gate

- **WHEN** every PlanStep in a DRAFT PlanConfiguration has a SlotBinding with an ExecutorInstallation
- **THEN** the next assistant prompt has state `BINDING_MATRIX`
- **AND** the existing save, policy, and landing flow remains the follow-on path

### Requirement: Shared conversational and matrix payload

The focused SlotBinding assistant prompt SHALL carry both the focused step data and the full matrix rows in one payload.

#### Scenario: Payload feeds chips and fallback matrix

- **WHEN** the assistant emits a `BINDING_STEP` prompt
- **THEN** the payload includes the focused `step_key`
- **AND** the payload includes `options` for that focused step
- **AND** the payload includes `rows` for every PlanStep with current executor bindings and compatible options

#### Scenario: Existing binding is reflected in both surfaces

- **WHEN** a PlanStep is already bound to an ExecutorInstallation
- **THEN** the row for that PlanStep carries the bound installation id
- **AND** any fallback matrix control for that row displays the same installation as selected

### Requirement: Conversational installation selection persists and advances

Selecting an ExecutorInstallation chip SHALL persist a SlotBinding through `UpdatePlanConfiguration`, append `USER_SELECTION`, append `STEP_REBOUND`, and advance the assistant to the next unbound PlanStep.

#### Scenario: First-time selection creates SlotBinding

- **WHEN** the user selects an installation chip for an unbound PlanStep
- **THEN** the frontend persists a SlotBinding for that PlanStep and installation using `UpdatePlanConfiguration` with `announceSaved=false`
- **AND** the thread contains a `USER_SELECTION` message for the selected chip
- **AND** the thread contains a `STEP_REBOUND` message with an empty previous installation id and the selected new installation id

#### Scenario: Rebinding replaces SlotBinding

- **WHEN** the user revisits a previously bound PlanStep and selects a different installation
- **THEN** the existing SlotBinding for that PlanStep is replaced with the new installation id
- **AND** the `STEP_REBOUND` message records both the previous and new installation ids

#### Scenario: Next prompt sees persisted binding

- **WHEN** a conversational selection is saved
- **THEN** the assistant derives the next turn from the updated PlanConfiguration
- **AND** it focuses the next unbound PlanStep or falls through to `BINDING_MATRIX` when no unbound steps remain

### Requirement: Collapsible edit-all fallback stays synchronized

The conversational binding card SHALL keep a collapsible "Edit all" matrix fallback that reads and writes the same SlotBinding data as the conversational chips.

#### Scenario: Chip selection updates fallback row

- **WHEN** the user selects an installation chip for the focused PlanStep
- **THEN** the fallback matrix row for that PlanStep reflects the selected installation without requiring a page reload

#### Scenario: Fallback selection updates conversational progress

- **WHEN** the user changes an executor from the fallback matrix
- **THEN** the same SlotBinding upsert path is used
- **AND** the conversational binding card's bound-step progress reflects the changed binding

### Requirement: Localized and motion-safe binding UI

The conversational SlotBinding UI SHALL localize all user-facing copy and SHALL use only reduced-motion-aware opacity or transform transitions.

#### Scenario: Copy resolves in supported locales

- **WHEN** the conversational SlotBinding card renders in `en` or `pt-BR`
- **THEN** every prompt label, button label, progress label, and fallback label resolves through flat translation keys

#### Scenario: Selection motion respects constraints

- **WHEN** the user selects an installation chip
- **THEN** the UI uses existing reduced-motion-aware motion primitives
- **AND** no transition animates color, font, or layout dimensions
