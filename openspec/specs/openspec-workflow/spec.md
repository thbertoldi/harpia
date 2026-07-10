# openspec-workflow Specification

## Purpose
TBD - created by archiving change harden-agent-workflow-tooling. Update Purpose after archive.
## Requirements
### Requirement: Tracked OpenSpec OpenCode adapters
The repository SHALL version the five OpenSpec 1.5.0 OpenCode commands and their five matching skills for propose, explore, apply, sync, and archive workflows. Any generated cross-agent handoff MUST reference a role allowed by Harpia's custom roster.

#### Scenario: Clone the repository on a new workstation
- **WHEN** a developer starts OpenCode from a clean checkout
- **THEN** all five `opsx-*` commands and all five `openspec-*` skills are available without regenerating local adapters

#### Scenario: Archive with a spec sync
- **WHEN** the archive workflow delegates a requested delta-spec sync
- **THEN** it targets Harpia's allowlisted implementation role rather than a nonexistent or disabled general-purpose role

### Requirement: Central OpenSpec authoring policy
`openspec/config.yaml` SHALL point artifact authors to root `AGENTS.md` for working rules and to the Platform Constitution for architecture and taxonomy. Its per-artifact rules MUST use only valid `spec-driven` artifact IDs and MUST require tasks to be executor-cold with explicit file paths, dependencies, acceptance criteria, and exact verification commands.

#### Scenario: Request artifact instructions
- **WHEN** an agent requests proposal, specs, design, or tasks instructions for a change
- **THEN** OpenSpec returns the centralized project context and the rules for that artifact without copying those constraints into the artifact template

#### Scenario: Validate project policy shape
- **WHEN** an unsupported rule ID, non-string context, empty rule, or missing canonical pointer is introduced
- **THEN** roster validation fails even though OpenSpec's spec validator does not validate configuration structure

### Requirement: Continuous strict OpenSpec validation
CI SHALL install the pinned OpenSpec 1.5.0 release and run exactly `openspec validate --all --strict --no-interactive` on pull requests and trunk pushes. Container publication MUST depend on the tooling gate that includes strict OpenSpec and roster validation.

#### Scenario: Submit an invalid active change or main spec
- **WHEN** strict OpenSpec validation finds a malformed requirement, scenario, or change artifact
- **THEN** the tooling CI job fails and container publication cannot proceed

#### Scenario: Validate all active contracts
- **WHEN** the repository's active changes and main specs are valid
- **THEN** the non-interactive strict command completes successfully without requiring user input
