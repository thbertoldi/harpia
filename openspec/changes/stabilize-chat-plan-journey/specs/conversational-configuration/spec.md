## ADDED Requirements

### Requirement: Localized catalog metadata in configuration cards
Configuration review, binding, overseer, policy, and execution preparation surfaces SHALL render PlanTemplate, PlanStep, and input metadata through locale-aware catalog/input keys before falling back to backend strings.

#### Scenario: Binding matrix renders localized step names
- **WHEN** the active locale is `pt-BR`
- **AND** the binding matrix shows the `fetch-news` PlanStep for `weekly-newsletter-linkedin`
- **THEN** the row displays the Portuguese localized step title rather than `Fetch News`

#### Scenario: Input labels render localized names
- **WHEN** a configuration card renders a template input such as `source_group` or `approval_mode`
- **THEN** the label resolves through the configured locale's flat input/catalog translation key
