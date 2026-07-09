# tiered-agent-catalog

## Purpose
The agent catalog is organized as **role × tier**, multi-capable, seeded declaratively;
predefined platform agents are the suggested defaults.

## ADDED Requirements

### Requirement: Agents are multi-capable
Each agent manifest SHALL declare a set of `capabilities`; a single agent MAY satisfy several
plan steps.

#### Scenario: specialist carries multiple capabilities
- GIVEN the LinkedIn Content Specialist agent
- THEN its capabilities include `linkedin-content-adaptation`, `carousel-authoring`,
  `tone-matching`, and `anti-ai-jargon`

### Requirement: Seniority tiers
Each role SHALL exist across tiers (Júnior / Pleno / Sênior), each a distinct manifest with
its own graph, tool access, and cost-per-execution.

#### Scenario: three tiers per role
- GIVEN the LinkedIn Content Specialist role
- THEN there are Júnior, Pleno, and Sênior manifests, each with a distinct graph and
  cost-per-execution

### Requirement: Declarative seed
Platform agents SHALL be seeded declaratively at startup (Go seeder, not migrations), mirroring
`EnsureCatalog` / `EnsurePlanTemplates` (constitution §13).

#### Scenario: boot seeds the team catalog
- GIVEN a fresh boot
- WHEN the catalog seeder runs
- THEN all predefined role × tier agents are present and idempotent on re-run

### Requirement: Consolidate the narrow SKUs
The narrow per-capability SKUs (`linkedin-voice-senior`, `linkedin-carousel-senior`) SHALL be
consolidated into the multi-capable role × tier agents; their graphs are reused as the
specialist's capabilities.

#### Scenario: voice + carousel become one specialist
- GIVEN the consolidated catalog
- THEN adaptation (was `linkedin-voice-senior`) and carousel authoring (was
  `linkedin-carousel-senior`) are capabilities of the LinkedIn Content Specialist, not
  separate agents
