## ADDED Requirements

### Requirement: Execution read model exposes the frozen PlanTemplate snapshot
The `PlanExecution` read model SHALL expose `plan_template_snapshot` as the immutable PlanTemplate
captured at execution creation, in addition to its `active_step_keys` and configuration snapshot.
Every execution projection and UI consumer SHALL derive ordered steps, titles, and active rows from
that frozen template snapshot plus active keys, never from the current PlanTemplate.

#### Scenario: Read returns the original template after catalog changes
- **WHEN** an execution was created from template version N and the current template is later changed to version N+1
- **THEN** reading the execution returns its version-N `plan_template_snapshot`
- **AND** the execution's projected order and titles remain version N

#### Scenario: Proto conversion retains the frozen snapshot
- **WHEN** the server converts a persisted PlanExecution to its API response
- **THEN** `plan_template_snapshot` contains the template from the execution snapshot
- **AND** it is not omitted or replaced by a current template lookup
