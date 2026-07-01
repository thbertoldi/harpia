# plan-template-authoring Specification

## Purpose
TBD - created by archiving change plan-template-authoring. Update Purpose after archive.
## Requirements
### Requirement: Embedded YAML template catalog
The system SHALL define PlanTemplates as embedded YAML files under `control-plane/internal/plans/templates/`, one file per template.

#### Scenario: Load valid embedded templates
- **WHEN** the API startup seed pipeline loads the embedded template catalog
- **THEN** every valid YAML file is decoded into a PlanTemplate definition containing key, name, description, vertical, version, ordered steps, edges, and input parameters

#### Scenario: Reject duplicate template keys
- **WHEN** two embedded YAML files declare the same template key
- **THEN** catalog loading fails before database writes

### Requirement: Template validation
The system SHALL validate loaded PlanTemplates before reconciliation and MUST fail startup on invalid catalog content.

#### Scenario: Reject malformed DAG
- **WHEN** a template declares a cycle, a dangling edge, or an unreachable step
- **THEN** validation fails with an error naming the affected template

#### Scenario: Reject duplicate step keys
- **WHEN** one template declares the same step key more than once
- **THEN** validation fails with an error naming the duplicated step key

#### Scenario: Reject unknown artifact type
- **WHEN** a step references an input or output ArtifactType key that is not registered
- **THEN** validation fails before reconciling that catalog

#### Scenario: Reject unknown executor SKU
- **WHEN** a step references a `default_executor_sku_key` that is not registered
- **THEN** validation fails before reconciling that catalog

#### Scenario: Reject invalid runtime mapping
- **WHEN** an input parameter runtime mapping targets an unknown step, unknown behavior policy, unknown seed field, or omits fields required for that target
- **THEN** validation fails before reconciling that catalog

#### Scenario: Reject broken linear artifact contract
- **WHEN** adjacent ordered steps have mismatched output and next input ArtifactType keys
- **THEN** validation fails before reconciling that catalog

### Requirement: Startup template reconciliation
The system SHALL reconcile the embedded YAML catalog into the existing global plan template tables at API startup.

#### Scenario: Upsert template metadata
- **WHEN** an embedded template key already exists in `plan_templates`
- **THEN** the seeder updates name, description, vertical, version, and input parameters for that key

#### Scenario: Reconcile child rows
- **WHEN** an embedded template is applied
- **THEN** its persisted steps and dependencies match the YAML definition by key, position, artifact types, executor metadata, and edges

#### Scenario: Remove absent templates
- **WHEN** a database template key is not present in any embedded YAML file
- **THEN** the seeder removes that template and its child rows from the global catalog

### Requirement: SQL template seed removal
The system SHALL keep PlanTemplate schema DDL in migrations while removing PlanTemplate row-content inserts and updates from migrations `000004`, `000011`, and `000013`.

#### Scenario: New database relies on seeder for template content
- **WHEN** migrations are applied to a fresh database
- **THEN** no PlanTemplate content rows are created by those migrations
- **AND** API startup creates the catalog from embedded YAML

### Requirement: Initial YAML catalog content
The embedded YAML catalog SHALL include the existing `weekly-newsletter-linkedin` template and at least one additional feasible template using only existing ExecutorSKUs.

#### Scenario: Existing template preserved
- **WHEN** the catalog is seeded
- **THEN** `weekly-newsletter-linkedin` exists with the four-step RSS-to-LinkedIn DAG and existing input parameters

#### Scenario: New review-only template available
- **WHEN** the catalog is seeded
- **THEN** `news-digest-draft` exists with `fetch-news` followed by `write-draft` and ends at a `TextDraft` artifact

### Requirement: Localized catalog copy
The system SHALL provide `en` and `pt-BR` i18n content keys for newly introduced template, step, and input labels.

#### Scenario: New template copy has both locales
- **WHEN** the frontend renders the new catalog template
- **THEN** `catalog.plan.<key>.name`, `catalog.plan.<key>.description`, `catalog.plan.<key>.step.<step_key>.title`, `catalog.plan.<key>.step.<step_key>.description`, and any new `plans.inputs.<param_key>.label` keys are available in both locales

