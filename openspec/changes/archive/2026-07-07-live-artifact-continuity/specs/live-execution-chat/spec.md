## MODIFIED Requirements

### Requirement: Live execution progress card
The chat thread SHALL render one collapsible progress card per `PlanExecution` section,
derived solely from the existing execution event stream, showing each `PlanStep`'s status
and an overall progress bar. When collapsed and running, the card header SHALL remain
informative: it SHALL surface the current running step's title/detail and a `done/total`
progress indication without requiring the user to expand the card.

#### Scenario: Collapsed running card shows the current step
- **WHEN** an execution card is collapsed and one of its steps is `running`
- **THEN** the header shows the current running step's title/detail
- **AND** the header shows a `done/total` progress indication

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
- **AND** the collapsed header no longer shows a current running step

#### Scenario: Run failure marks the failing step
- **WHEN** a section contains `RUN_FAILED` while a step is `running`
- **THEN** that running step is marked `failed`
- **AND** earlier steps remain `done`
- **AND** the card shows a localized failure caption with the danger token
