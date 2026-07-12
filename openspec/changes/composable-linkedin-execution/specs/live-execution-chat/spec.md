## ADDED Requirements

### Requirement: Execution cards render the frozen active graph
The live execution card SHALL obtain its ordered steps from the immutable PlanExecution snapshot's
active graph and SHALL not render optional enrichments excluded from that execution.

#### Scenario: Image opt-out has no image row
- **WHEN** an execution snapshot excludes image generation
- **THEN** its execution card has no `generate-image` row
- **AND** later template/configuration changes do not add that row

### Requirement: Execution cards surface declared human checkpoints
The live execution card SHALL show pending elicitation, review, and approval checkpoints for its
own execution and route actions to their scoped requests with localized `en` and `pt-BR` copy.

#### Scenario: Review replaces running content status
- **WHEN** a content step has produced a candidate and is awaiting review
- **THEN** its execution card identifies the review checkpoint and exposes preview, accept, and revise actions
- **AND** it does not report the step as completed until the candidate is accepted
