## ADDED Requirements

### Requirement: Generic template-driven configuration inputs

The conversational PlanConfiguration frontend SHALL render and materialize a template's inputs
generically from the template's `TemplateInputParameter[]` contract, with no per-template
hardcoded input model, so that adding or changing a template requires no frontend code change.
Materialization of parameter values into seed artifacts, slot bindings, and behavior policies
remains server-authoritative.

#### Scenario: Binding surface renders from the template contract

- **WHEN** a configuration surface (e.g. the binding matrix card) renders a template's inputs
- **THEN** the fields, labels, and defaults are derived from the template's
  `TemplateInputParameter[]` rather than a template-specific frontend model

#### Scenario: A second template configures with no frontend change

- **WHEN** a template other than the LinkedIn content template is configured
- **THEN** its inputs render and serialize to parameter values through the same generic path
- **AND** no template-specific frontend input model is required

#### Scenario: Server owns materialization

- **WHEN** the frontend submits parameter values for any template
- **THEN** the server computes seed artifacts, slot bindings, and behavior policies from the
  template's runtime mappings, and the frontend does not hardcode those mappings
