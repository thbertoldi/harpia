## MODIFIED Requirements

### Requirement: Tracked OpenSpec OpenCode adapters
The repository SHALL version the five OpenSpec 1.5.0 OpenCode commands and their five matching skills for propose, explore, apply, sync, and archive workflows. Any generated cross-agent handoff MUST reference a role allowed by Harpia's custom roster. Archive adapters MUST hard-stop without a confirmation override when an artifact or task is incomplete, or when the required acceptance evidence has not been verified.

#### Scenario: Clone the repository on a new workstation
- **WHEN** a developer starts OpenCode from a clean checkout
- **THEN** all five `opsx-*` commands and all five `openspec-*` skills are available without regenerating local adapters

#### Scenario: Archive with a spec sync
- **WHEN** the archive workflow delegates a requested delta-spec sync
- **THEN** it targets Harpia's allowlisted implementation role rather than a nonexistent or disabled general-purpose role

#### Scenario: Attempt to archive an incomplete or unverified change
- **WHEN** an archive adapter finds an incomplete artifact, unchecked task, or absent verified acceptance evidence
- **THEN** it stops before moving the change and instructs the executor to complete the missing requirement without offering a confirmation override

### Requirement: Central OpenSpec authoring policy
`openspec/config.yaml` SHALL point artifact authors to root `AGENTS.md` for working rules and to the Platform Constitution for architecture and taxonomy. Its per-artifact rules MUST use only valid `spec-driven` artifact IDs and MUST require tasks to be executor-cold with explicit file paths, dependencies, acceptance criteria, and exact verification commands. Every behavior-change task list SHALL end with a checked, non-deferrable, machine-executable acceptance task carrying the exact command `mise run acceptance`.

#### Scenario: Request artifact instructions
- **WHEN** an agent requests proposal, specs, design, or tasks instructions for a change
- **THEN** OpenSpec returns the centralized project context and the rules for that artifact without copying those constraints into the artifact template

#### Scenario: Validate project policy shape
- **WHEN** an unsupported rule ID, non-string context, empty rule, or missing canonical pointer is introduced
- **THEN** roster validation fails even though OpenSpec's spec validator does not validate configuration structure

#### Scenario: Author a behavior-changing task list
- **WHEN** an executor authors tasks for a behavior change
- **THEN** the final task is non-deferrable, machine-executable, and names the exact command `mise run acceptance`

### Requirement: Continuous strict OpenSpec validation
CI SHALL install the pinned OpenSpec 1.5.0 release and run exactly `openspec validate --all --strict --no-interactive` on pull requests and trunk pushes. CI SHALL run migrations, `mise run acceptance`, and `mise run openspec-delivery-check` in a PostgreSQL-backed acceptance job using a non-superuser NOBYPASSRLS app role. Container publication MUST depend on the tooling gate that includes strict OpenSpec and roster validation and on the acceptance job.

#### Scenario: Submit an invalid active change or main spec
- **WHEN** strict OpenSpec validation finds a malformed requirement, scenario, or change artifact
- **THEN** the tooling CI job fails and container publication cannot proceed

#### Scenario: Validate all active contracts
- **WHEN** the repository's active changes and main specs are valid
- **THEN** the non-interactive strict command completes successfully without requiring user input

#### Scenario: Acceptance or delivery hygiene fails
- **WHEN** migration-backed acceptance or the archived-change delivery check exits non-zero
- **THEN** the acceptance job fails and container publication cannot start

## ADDED Requirements

### Requirement: Archived delivery evidence is enforceable
The repository SHALL provide `mise run openspec-delivery-check`, which exits non-zero for an archived change whose `tasks.md` contains an unchecked task, whose completion notes contain `agent-incapable` or unresolved `DEFERRED`, whose final acceptance task lacks the exact `mise run acceptance` command, or whose conditional/N-A task lacks checked explicit evidence in the form `N/A because no <surface> files changed`. A deferred requirement MUST be moved to a linked follow-up change before archive.

#### Scenario: Archived change retains incomplete work
- **WHEN** an archived `tasks.md` contains `- [ ]`
- **THEN** `mise run openspec-delivery-check` exits non-zero and identifies the archived change

#### Scenario: Archived change defers required behavior
- **WHEN** an archived completion note contains `agent-incapable` or an unresolved `DEFERRED` item
- **THEN** `mise run openspec-delivery-check` exits non-zero and identifies the unsupported deferral

#### Scenario: Archived conditional task is supported by evidence
- **WHEN** a checked conditional task explicitly states `N/A because no frontend files changed`
- **THEN** `mise run openspec-delivery-check` accepts that task as non-applicable

### Requirement: Deterministic production acceptance suite
The repository SHALL provide `mise run acceptance`, which runs `go test ./internal/acceptance -count=1` and fails rather than skips when `HARPIA_TEST_DATABASE_URL` is absent or is a superuser connection. The suite SHALL use real production logic and the embedded `linkedin-content-studio` PlanTemplate with deterministic `noop` image and `approval_only` LinkedIn adapters. It SHALL verify optional image/carousel steps do not require SlotBindings when opted out, image opt-in blocks RUNNABLE status until a compatible installation exists, audit Repository tenant isolation/deduplication/redaction under forced RLS, and noop image configuration/resolution without a provider key. Any unimplemented full execution scenario MUST use an explicit TODO skip referencing `enforce-acceptance-before-archive`.

#### Scenario: Optional rich-content capabilities are opted out
- **WHEN** the real embedded `linkedin-content-studio` template is materialized with `include_images=no` and `include_carousel=no`
- **THEN** its included optional capabilities exclude image generation and carousel authoring, and RUNNABLE slot-binding validation succeeds without bindings for `generate-image` or `draft-carousel`

#### Scenario: Image capability is opted in without installation
- **WHEN** the same template is materialized with `include_images=yes` and no compatible image ExecutorInstallation exists
- **THEN** RUNNABLE slot-binding validation is blocked until a compatible image installation exists

#### Scenario: Audit repository uses forced RLS and redaction
- **WHEN** the app role records a deduplicated audit event containing a secret-like diff for tenant A and lists events for tenants A and B
- **THEN** tenant A sees one redacted event, tenant B sees no event, and the duplicate write is a no-op

#### Scenario: Noop image installation requires no provider key
- **WHEN** an image ExecutorInstallation is validated with `provider: noop`
- **THEN** validation succeeds and the production resolver returns the noop provider without a configured key
