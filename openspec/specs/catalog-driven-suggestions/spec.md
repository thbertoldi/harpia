# catalog-driven-suggestions Specification

## Purpose
TBD - created by archiving change catalog-driven-suggestions. Update Purpose after archive.
## Requirements
### Requirement: Catalog-driven suggestion chips
The home empty state SHALL render suggestion chips derived from the live `PlanTemplate` catalog via `ListPlanTemplates`, rather than from hardcoded strings.

#### Scenario: Seeded catalog yields template chips
- **WHEN** the home empty state loads and `ListPlanTemplates` returns templates
- **THEN** the chips are derived from the catalog
- **AND** each chip label resolves from `catalog.plan.<key>.name`
- **AND** each chip carries a send-ready prompt from `catalog.plan.<key>.suggestion`

#### Scenario: Empty or loading catalog shows a generic fallback
- **WHEN** `ListPlanTemplates` returns no templates or has not yet resolved
- **THEN** a localized generic fallback set of prompts is shown
- **AND** the empty state is never blank

#### Scenario: Stable ordering without layout shift
- **WHEN** the catalog stream resolves after first paint
- **THEN** chips appear in a stable catalog order
- **AND** the chip count does not cause visible layout shift

### Requirement: Bare-thread empty state is actionable
The config-less chat thread empty state SHALL render the same catalog-driven suggestion chips so an empty thread is immediately actionable.

#### Scenario: Empty thread shows chips
- **WHEN** a thread has no messages and no attached PlanConfiguration
- **THEN** the catalog-driven chips render above the composer
- **AND** selecting a chip seeds the thread via the existing create/append path

### Requirement: Selection launches the existing flow
Selecting any suggestion chip SHALL create or seed a thread using the existing path, leaving the proposal/classification backend unchanged.

#### Scenario: Chip selection creates a thread
- **WHEN** the user selects a suggestion chip on home
- **THEN** a thread is created with the chip's prompt as the initial message
- **AND** the user is routed to `/chat/[threadId]`

### Requirement: Localized and token-consistent chips
Suggestion chips SHALL resolve all label and prompt copy through flat translation/content keys in both supported locales and SHALL use semantic tokens with the energy accent on interaction.

#### Scenario: Copy resolves in supported locales
- **WHEN** the chips render in `en` or `pt-BR`
- **THEN** labels, prompts, fallback prompts, and section headings resolve through flat keys
- **AND** no user-facing string is a raw literal

#### Scenario: Vertical maps to an icon with a default
- **WHEN** a template has a `vertical` with no mapped icon
- **THEN** a default icon is used
- **AND** the chip still renders and is selectable

