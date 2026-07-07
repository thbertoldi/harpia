# live-execution-chat Specification

## Purpose
TBD - created by archiving change live-execution-chat. Update Purpose after archive.
## Requirements
### Requirement: Live execution progress card
The chat thread SHALL render one collapsible progress card per `PlanExecution` section, derived solely from the existing execution event stream, showing each `PlanStep`'s status and an overall progress bar.

#### Scenario: Running step shows progress
- **WHEN** a thread section for `executionId` contains `RUN_STARTED` and `STEP_STARTED` for `step_key="draft"`
- **THEN** the execution card shows step `draft` as `running`
- **AND** the progress bar reflects `done/total` with the running step counted as half
- **AND** the card header shows the running step's detail text

#### Scenario: Step completion flips the next state
- **WHEN** a section contains `STEP_STARTED` for `step_key="draft"` followed by `STEP_BOUND` for `step_key="draft"`
- **THEN** step `draft` is shown as `done`
- **AND** if a later `STEP_STARTED` for the next step exists, that step is `running`

#### Scenario: Run completion settles in-flight steps
- **WHEN** a section contains `RUN_COMPLETED`
- **THEN** every step still marked `running` is forced to `done`
- **AND** the execution card header reports the plan as completed

#### Scenario: Run failure marks the failing step
- **WHEN** a section contains `RUN_FAILED` while a step is `running`
- **THEN** that running step is marked `failed`
- **AND** earlier steps remain `done`
- **AND** the card shows a localized failure caption with the danger token

### Requirement: Event folding is order-tolerant
The view model SHALL fold events in `sequence_number` order and SHALL tolerate late or out-of-order arrival without leaving a step permanently stuck.

#### Scenario: Late completion settles a step
- **WHEN** `STEP_STARTED` arrives before a preceding step's `STEP_BOUND` due to ordering
- **THEN** processing in `sequence_number` order still yields a consistent `done`/`running` assignment
- **AND** no step remains `running` after `RUN_COMPLETED` or `RUN_FAILED`

#### Scenario: Unknown step key degrades gracefully
- **WHEN** a `STEP_STARTED` references a `step_key` not present in the resolved template step list
- **THEN** the card does not throw
- **AND** the execution still reports an overall running/completed/failed summary

### Requirement: Header execution-status pill
The chat header SHALL surface whether any execution in the thread is running.

#### Scenario: Any running execution shows executing state
- **WHEN** at least one execution section is in the `running` state
- **THEN** the header status pill shows "Executing…" in the energy accent

#### Scenario: No running execution shows ready state
- **WHEN** no execution section is running
- **THEN** the header status pill shows "Ready" in a neutral tone

### Requirement: Motion-safe and localized affordances
The execution UI SHALL use only reduced-motion-aware opacity/transform transitions and SHALL resolve all user-facing copy through flat translation keys in both supported locales.

#### Scenario: Typing and generating affordances are motion-safe
- **WHEN** the assistant is thinking or a step is generating
- **THEN** the typing-dots indicator and generating skeleton animate only opacity or transform
- **AND** both are suppressed or softened under reduced-motion preferences

#### Scenario: Copy resolves in supported locales
- **WHEN** the execution card and status pill render in `en` or `pt-BR`
- **THEN** every label, status word, failure caption, and aria-label resolves through flat `translate()` keys
- **AND** no user-facing string is a raw literal

