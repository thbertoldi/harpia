## ADDED Requirements

### Requirement: Execution snapshot defines the active graph
At execution creation, the system SHALL resolve optional-capability participation and persist the
resulting active PlanStep keys in the immutable PlanExecution snapshot. Runtime execution and
execution UI queries SHALL use that snapshot rather than the current template or configuration.

#### Scenario: Later configuration edit cannot change an in-flight graph
- **WHEN** an execution is created with image generation excluded
- **AND** the configuration is later changed to include image generation
- **THEN** the existing execution never runs or renders `generate-image`
- **AND** a later execution resolves its own active graph independently

### Requirement: Skipped identity steps resolve downstream input
The workflow SHALL record an excluded identity-shaped optional step as SKIPPED with the aliased
input/output ArtifactRef, satisfy downstream dependency resolution with that output, and exclude
the skipped step from the active execution graph.

#### Scenario: Carousel opt-out passes post to image or publish
- **WHEN** `draft-carousel` is excluded and `generate-image` is active
- **THEN** `generate-image` receives the exact `LinkedInPost` ref accepted from `author-content`
- **AND** the skipped carousel step is not shown as a pending or completed active row

### Requirement: Completion event is execution-scoped and version-aware
The workflow SHALL pass PlanExecutionID, PlanStepKey, output artifact id, output artifact version
id, and content hash to every normal or identity-alias completion activity. The runtime SHALL emit
the corresponding `STEP_BOUND` event on the owning conversation.

#### Scenario: Completion binds the correct preview to its execution
- **WHEN** `author-content` or an enrichment completes
- **THEN** its `STEP_BOUND` payload contains the correct execution id, step key, artifact id, version id, and content hash
- **AND** the execution card can open that output without confusing another execution's artifact

### Requirement: Version-pinned approval is enforced at the workflow boundary
The workflow SHALL reject a stale or malformed approval decision whose subject does not equal the
pending approval's artifact id, version id, type key, and content hash, and SHALL not invoke the
publisher in that case.

#### Scenario: Hash mismatch blocks external mutation
- **WHEN** an approval decision resolves to a subject whose stored content hash differs from the pending approval request
- **THEN** the publish StepExecution fails safely before its integration activity
- **AND** the LinkedIn publisher receives no call
