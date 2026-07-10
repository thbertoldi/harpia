## ADDED Requirements

### Requirement: Custom roster is the effective project surface
The project configuration SHALL select `orchestrator` as the default primary agent, SHALL set `zai-coding-plan/glm-5.2` as the main model, and SHALL set `opencode/mimo-v2.5-free` as the lightweight model. It MUST disable built-in `build`, `plan`, `explore`, and `general` roles while retaining required hidden system agents.

#### Scenario: Start OpenCode without an explicit agent
- **WHEN** OpenCode starts in the repository without an agent override
- **THEN** it resolves `orchestrator` as the primary agent and does not expose the disabled built-in development roles

#### Scenario: Generate lightweight session metadata
- **WHEN** OpenCode performs a lightweight title or summary task
- **THEN** it uses the configured free lightweight model instead of falling back to the main workhorse

### Requirement: Curated skill surface
The project-wide Skill permission SHALL deny unmatched skills and SHALL allow only the Harpia skill, the tracked `openspec-*` skills, and explicitly reviewed engineering skills relevant to Harpia's stack. BMAD, WDS, and other externally discovered skills MUST NOT be loadable unless a later reviewed configuration change adds them.

#### Scenario: Discover an unapproved external skill
- **WHEN** OpenCode discovers a skill that does not match an allowlisted name or pattern
- **THEN** the Skill tool denies loading it even if its file remains visible to discovery diagnostics

#### Scenario: Invoke the canonical change workflow
- **WHEN** an agent loads any tracked `openspec-*` skill or the `harpia` skill
- **THEN** the Skill permission allows the operation

### Requirement: Independent verification role
The roster SHALL expose a cheap `verifier` subagent that cannot edit, delegate, load skills, access GitHub tools, or access external directories. Its shell permission MUST deny by default and allow only fixed repository lint, test, OpenSpec, and roster-validation gates. The coordinator MUST request independent verification evidence after writer self-verification and before declaring implementation complete.

#### Scenario: Verify an implementation independently
- **WHEN** the coordinator asks `verifier` to run an applicable allowlisted gate
- **THEN** the verifier reports the exact command, exit status, and relevant unaltered output without editing or attempting to fix the implementation

#### Scenario: Attempt an unapproved verifier command
- **WHEN** the verifier attempts a shell command outside its fixed gate allowlist
- **THEN** OpenCode denies the command instead of asking for or silently granting broader access

### Requirement: Deterministic roster policy validation
The repository SHALL provide `mise run opencode-validate` to validate project configuration parsing, required and disabled loaded roles, effective defaults, permission invariants, dangerous Git allow rules, stale role/model references, OpenSpec configuration shape, adapter presence, and OpenCode CLI/plugin version equality. The deterministic gate MUST NOT require provider credentials; authenticated model-catalog validation MAY be an explicit local extension.

#### Scenario: Validate a clean checkout
- **WHEN** a developer or CI runs `mise run opencode-validate` with the pinned local tools
- **THEN** it succeeds only when static policy and OpenCode's pure resolved roster satisfy all tracked invariants

#### Scenario: Introduce dangerous or stale configuration
- **WHEN** a change silently allows a Git mutation, references a missing role, reintroduces a banned model alias, or drifts the plugin from the pinned CLI version
- **THEN** the validator exits nonzero with a specific invariant failure

#### Scenario: Check authenticated model catalogs
- **WHEN** a developer explicitly runs the model-catalog extension with configured provider credentials
- **THEN** every configured project model is checked against the available catalog without making catalog access a CI prerequisite

### Requirement: Shared executor command policy and aligned packages
Safe Git inspection and common repository verification commands SHALL have one project-level permission definition inherited by `implementer`, `terra`, and `scut`; role-specific additions MAY remain local. The OpenCode CLI version in `mise.toml` and tracked `@opencode-ai/plugin` dependency MUST be identical and exact.

#### Scenario: Update a common verification gate
- **WHEN** a maintainer changes a verification command shared by writer roles
- **THEN** one project-level policy update changes the inherited permission without editing three duplicated lists

#### Scenario: Upgrade OpenCode tooling
- **WHEN** the pinned OpenCode CLI version changes without the tracked plugin version changing to the same release
- **THEN** deterministic roster validation fails before the drift reaches normal development
