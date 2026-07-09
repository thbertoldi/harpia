# plan-execution-runtime Specification

## Purpose
TBD - created by archiving change plan-execution-runtime. Update Purpose after archive.
## Requirements
### Requirement: Server materializes configuration from parameter values

The system SHALL derive a PlanConfiguration's seed artifacts, slot bindings, and behavior
policies on the server from the PlanTemplate's input-parameter runtime mappings and the stored
parameter values, so a configuration is executable when only parameter values are supplied.

#### Scenario: Seed artifacts are composed from grouped mappings

- **WHEN** several input parameters declare `SEED_ARTIFACT` runtime mappings targeting the same
  step and input name at different JSON paths
- **THEN** the server produces one seed artifact for that step and input name
- **AND** its literal payload is a JSON object with each parameter value placed at its mapped
  JSON path

#### Scenario: A parameter drives a slot binding

- **WHEN** an input parameter declares a `SLOT_BINDING` runtime mapping for a step and its value
  is an executor installation id
- **THEN** the server produces a slot binding for that step pointing at that installation

#### Scenario: A parameter drives a behavior policy

- **WHEN** an input parameter declares a `BEHAVIOR_POLICY` runtime mapping with a policy key
- **THEN** the server sets that behavior policy from the parameter value

#### Scenario: Backend is authoritative over client-supplied bindings

- **WHEN** a client creates or updates a configuration
- **THEN** the server computes seed artifacts, slot bindings, and behavior policies from the
  template and parameter values
- **AND** ignores any seed artifacts, slot bindings, or behavior policies supplied by the client

### Requirement: Default slot bindings resolve from step defaults

The system SHALL bind every template step that has no parameter-driven slot binding to the
tenant's enabled executor installation for that step's default executor SKU.

#### Scenario: Non-parameter step binds to the default SKU installation

- **WHEN** a step has a `default_executor_sku_key` and no parameter maps a slot binding to it
- **THEN** the server binds the step to the tenant's enabled installation for that SKU
- **AND** for an agent step, the binding also carries the installation's manifest id and version

#### Scenario: Missing installation blocks a runnable configuration

- **WHEN** a required step has no resolvable installation for its default SKU
- **THEN** promoting the configuration to RUNNABLE or creating an execution fails with a
  precondition error naming the step

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

### Requirement: Pre-flight execution readiness checks seeds

The system SHALL verify before starting an execution that every step requiring an input artifact
type, and having no upstream producer, has a matching seed artifact.

#### Scenario: Missing required seed fails before the workflow starts

- **WHEN** a configuration has a step that requires an input artifact type, has no upstream
  producer, and has no matching seed artifact
- **THEN** creating an execution fails with a precondition error naming the step and artifact type
- **AND** no workflow is started

#### Scenario: A fully seeded configuration passes pre-flight

- **WHEN** every input-requiring root step has a matching seed and all slot bindings are ready
- **THEN** creating an execution proceeds to start the workflow

### Requirement: Configured plan executes end to end

The system SHALL run a configured plan across its full step DAG, producing the output artifact of
each step as input to the next, from parameter values alone.

#### Scenario: Reference weekly newsletter runs to completion

- **WHEN** the `weekly-newsletter-linkedin` configuration is created from parameter values and
  made RUNNABLE, and an execution is created
- **THEN** the workflow walks fetch-news, write-draft, adapt-for-linkedin, and publish-linkedin
- **AND** each step produces its declared artifact type as input to the next step
- **AND** the publish step honors the configured approval behavior policy

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

