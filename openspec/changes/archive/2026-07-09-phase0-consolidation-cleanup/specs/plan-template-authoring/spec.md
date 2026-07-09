## ADDED Requirements

### Requirement: Templates are identified by outcome, not configuration axes

A PlanTemplate's identity — its key, name, and catalog copy — SHALL describe the job/outcome it
achieves and SHALL NOT encode cadence, channel, or format, which are configuration axes
(cadence via `PlanSchedule`, channel via `SlotBinding`, format via input parameters).

#### Scenario: Template key omits cadence and channel

- **WHEN** a content template is authored or renamed
- **THEN** its key and name name the outcome (e.g. source-driven social content) and do not bake
  in a cadence like "weekly" or a single channel like "linkedin"

#### Scenario: Cadence is expressed only through schedule

- **WHEN** a template is configured to run on a cadence
- **THEN** the cadence is set on the configuration's `PlanSchedule`, and neither the template key
  nor its copy asserts a fixed cadence

#### Scenario: Localization keys describe the outcome

- **WHEN** catalog localization keys (`catalog.plan.<key>.*`) are authored for a template
- **THEN** they describe the job the template does, not a one-off cadence or channel
