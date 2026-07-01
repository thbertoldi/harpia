## ADDED Requirements

### Requirement: Curated RSS preset installations
The system SHALL seed curated RSS source groups as tenant-scoped `rss-news-feed` ExecutorInstallations whose config JSON contains `{feeds:[...]}`.

#### Scenario: Seed dev tenant RSS presets
- **WHEN** the API startup seed pipeline runs for the dev tenant
- **THEN** named RSS preset installations exist for Tech/startup, Business, Marketing/creator, and Brazil/pt-BR using only verified live RSS/Atom feed URLs

#### Scenario: Reconcile existing preset installation
- **WHEN** a preset installation already exists for the tenant
- **THEN** startup updates its enabled state, connection status, and config JSON to match the curated preset definition

### Requirement: RSS presets stay out of templates
The system SHALL keep RSS feed URLs out of PlanTemplate definitions and artifact seed payloads.

#### Scenario: Template source group maps to slot binding
- **WHEN** a template includes a `source_group` input parameter
- **THEN** its runtime mapping targets the `fetch-news` slot binding instead of embedding feed URLs in template content

#### Scenario: Preset installation is selectable
- **WHEN** configuration chat or UI offers compatible installations for an RSS-backed fetch step
- **THEN** the seeded preset installations are available through the existing installation selection path
