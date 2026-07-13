## MODIFIED Requirements

### Requirement: Live execution progress card
The chat thread SHALL render one collapsible progress card per `PlanExecution` section, derived
from its ordered runtime event stream, immutable PlanExecution template snapshot plus
`active_step_keys`, and durable pending-interaction projection. The card SHALL derive PlanStep
titles, order, and active rows from the frozen template, not the current template or
configuration, and SHALL show at most one pending elicitation, review, or approval interaction
for its exact execution. When collapsed and running, the card header SHALL remain informative: it
SHALL surface the current running step's title/detail and a `done/total` progress indication
without requiring the user to expand the card.

#### Scenario: Collapsed running card shows the current step
- **WHEN** an execution card is collapsed and one of its frozen active steps is `running`
- **THEN** the header shows the frozen step title/detail
- **AND** the header shows a `done/total` progress indication

#### Scenario: Running step shows progress
- **WHEN** a section for `executionId` contains `RUN_STARTED` and `STEP_STARTED` for `step_key="draft"`
- **THEN** the execution card shows frozen active step `draft` as `running`
- **AND** the progress bar reflects `done/total` with the running step counted as half
- **AND** the card header shows the frozen step detail text

#### Scenario: Step completion flips the next state
- **WHEN** a section contains `STEP_STARTED` for `step_key="draft"` followed by `STEP_BOUND` for `step_key="draft"`
- **THEN** step `draft` is shown as `done`
- **AND** if a later `STEP_STARTED` for the next frozen active step exists, that step is `running`

#### Scenario: Run completion settles in-flight steps
- **WHEN** a section contains `RUN_COMPLETED`
- **THEN** every frozen active step still marked `running` is forced to `done`
- **AND** the card header reports the plan as completed
- **AND** the collapsed header no longer shows a current running step

#### Scenario: Run failure marks the failing step
- **WHEN** a section contains `RUN_FAILED` while a frozen active step is `running`
- **THEN** that running step is marked `failed`
- **AND** earlier steps remain `done`
- **AND** the card shows a localized failure caption with the danger token

#### Scenario: Later template edits cannot rewrite an execution card
- **WHEN** a template title, order, or optional step participation changes after an execution was created
- **THEN** the existing card retains the frozen template's titles/order and `active_step_keys`
- **AND** only a later execution can display the changed template shape

## ADDED Requirements

### Requirement: Execution cards render the projected exact human interaction
An execution card SHALL render its durable projected pending elicitation, review, or approval only
when it belongs to that exact PlanExecution and StepExecution. Preview and action affordances
SHALL use request-pinned artifact references and request/action IDs, never a latest execution or
mutable Artifact version.

#### Scenario: Pending review is rendered from durable projection
- **WHEN** the execution driver projects one pending review for an execution
- **THEN** the card exposes Preview, Accept, and Revise for that review request
- **AND** Preview loads the review subject's pinned artifact version
