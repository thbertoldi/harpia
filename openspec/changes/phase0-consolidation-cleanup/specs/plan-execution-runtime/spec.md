## ADDED Requirements

### Requirement: Single owning-thread resolution for a configuration

The system SHALL resolve the thread that owns a PlanConfiguration through exactly one code
path, so that every message emitted for that configuration — runtime `RUN_*`/`STEP_*` events
and assistant/selection/approval/elicitation events alike — targets the same thread. There
SHALL be no fallback that treats the configuration id as a thread id.

#### Scenario: Runtime and interaction events share one thread

- **WHEN** a configuration produces both runtime execution events and an approval or elicitation
  event
- **THEN** all of those events are appended to the same owning thread id resolved by the single
  resolver

#### Scenario: Autonomous scheduled run targets the origin thread

- **WHEN** a scheduled execution runs with no user present
- **THEN** its `RUN_*`/`STEP_*` events and any approval it raises are appended to the
  configuration's origin thread, not to a configuration-id-derived thread

#### Scenario: No config-id thread fallback

- **WHEN** a configuration's owning thread cannot be resolved
- **THEN** the system surfaces a clear error rather than falling back to using the configuration
  id as a thread id

## MODIFIED Requirements

### Requirement: Rolling date-range presets resolve at run time

The system SHALL keep a date-range seed unresolved in configuration and resolve it to concrete
dates at the start of each execution from the plan's **schedule window** (the cadence of the
`PlanSchedule` for recurring plans, or the explicit range for one-shot runs), so cadence lives
in the schedule rather than in the template key or a hardcoded preset.

#### Scenario: A weekly schedule resolves to the previous seven days per run

- **WHEN** a recurring configuration's schedule cadence is weekly and the date-range seed is
  unresolved
- **AND** an execution starts
- **THEN** the execution's date-range seed is concrete dates for the window ending the day
  before the run, sized to the cadence
- **AND** two executions on different runs receive different concrete windows

#### Scenario: A one-shot run uses its explicit window

- **WHEN** a one-shot configuration provides an explicit date range
- **THEN** the execution uses that range without deriving one from a schedule

#### Scenario: Unresolved date range reaching an executor is rejected

- **WHEN** an executor receives a date range with no concrete start and end dates
- **THEN** the executor returns a clear error rather than silently reading an empty window
