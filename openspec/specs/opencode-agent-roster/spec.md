# opencode-agent-roster Specification

## Purpose
Define Harpia's project-scoped OpenCode agent roster, routing, model selection, review layers, canonical guidance, and least-privilege operating boundaries.
## Requirements
### Requirement: Frontier planning and expert roles
The project OpenCode roster SHALL expose `planner` as a primary system-development planning agent and `sol` as a directly queryable read-only expert. Both roles MUST use `openai/gpt-5.6-sol`; they MUST use the highest configured reasoning effort, and only the planner MAY request approval to write planning artifacts.

#### Scenario: Plan system development
- **WHEN** a user selects `planner` and asks for a system-development plan
- **THEN** the planner researches the repository, distinguishes facts from assumptions and human-owned decisions, and returns an executor-cold plan with explicit file paths, dependencies, acceptance criteria, and verification commands before implementation

#### Scenario: Query the strongest expert
- **WHEN** a user or coordinator invokes `@sol` with one focused hard problem
- **THEN** Sol returns a bounded, implementation-ready decision with rationale, edge cases, and proof steps without editing files or delegating further

### Requirement: Complementary implementation executors
The roster SHALL retain GLM-5.2 `implementer` as the default workhorse and SHALL expose GPT-5.6 Terra as an additional executor for higher-judgment, model-diverse, or recovery work. The coordinator MUST assign exactly one executor to any overlapping file set and MUST route unresolved design decisions to `@sol` before implementation.

#### Scenario: Route a normal brief
- **WHEN** the coordinator receives a complete normal implementation brief
- **THEN** it selects either `@implementer` or `@terra`, gives that executor a self-contained brief, and does not assign another writer to overlapping files

#### Scenario: Escalate an unresolved design decision
- **WHEN** an executor discovers that its brief requires a material architectural or product decision
- **THEN** it stops without guessing and returns a focused question for the coordinator to resolve with the user or `@sol`

### Requirement: Large-context GLM exploration
The `explorer` role SHALL use `zai-coding-plan/glm-5.2`, SHALL remain unable to edit files, and SHALL report conclusions with `path:line` evidence, applicable project constraints, and explicit search bounds. The tracked project roster MUST NOT retain Gemini as an unused explorer fallback.

#### Scenario: Map an unfamiliar subsystem
- **WHEN** the coordinator invokes `@explorer` to map repository behavior
- **THEN** the explorer reads broadly, returns a concise evidence-backed synthesis, and identifies material areas it did not cover

### Requirement: Layered independent review
Every implementation diff SHALL receive the fast reviewer, and sensitive diffs MUST additionally receive `reviewer-senior` using `deepseek/deepseek-v4-pro`. Fast review MUST NOT suppress mandatory senior review. Uncontested findings MUST return to the owning executor; adjudication SHALL occur only for an explicit reasoned disagreement and SHALL use GPT-5.6 Sol.

#### Scenario: Review a sensitive change
- **WHEN** a diff touches authentication, authorization, tenancy, secrets, database schemas, workflow semantics, deployment, public contracts, or a materially risky cross-service boundary
- **THEN** the coordinator runs both fast and senior review before declaring the task complete

#### Scenario: Handle an uncontested serious finding
- **WHEN** the senior reviewer identifies a serious issue and the executor accepts it
- **THEN** the coordinator returns the finding to that executor for correction without invoking adjudication

#### Scenario: Adjudicate a real dispute
- **WHEN** an executor and reviewer provide conflicting reasoned positions on correctness
- **THEN** `@adjudicator` renders a decisive Sol-backed verdict, while escalating human value decisions rather than deciding them

### Requirement: Canonical project guidance
All project agents SHALL treat root `AGENTS.md` as the working guide and `docs/architecture/harpia-platform.md` as the authoritative source before product-domain modeling. ADRs SHALL be treated as dated rationale where the Constitution supersedes them. OpenCode configuration and skills MUST NOT inject or duplicate stale architecture that conflicts with those sources.

#### Scenario: Model a Harpia domain change
- **WHEN** any planning, expert, implementation, exploration, or review role reasons about Harpia domain concepts
- **THEN** it uses the Constitution's six bounded contexts, plan-centric taxonomy, current design-token governance, and current verification gates

#### Scenario: Load project instructions
- **WHEN** OpenCode starts in the repository
- **THEN** root `AGENTS.md` supplies canonical working instructions without the obsolete `README.md` being added as a competing instruction source

### Requirement: Least-privilege agent operation
Git mutations and GitHub MCP actions SHALL require approval or be denied, while safe read-only Git inspection MAY be allowlisted. The coordinator MUST have an explicit project-subagent allowlist. Subagents MUST NOT delegate further; read-only roles MUST NOT edit files; and writer roles MUST be limited to in-scope edits and documented verification commands.

#### Scenario: Attempt an unapproved repository mutation
- **WHEN** an agent attempts to commit, push, reset, clean, or perform a GitHub write without user approval
- **THEN** OpenCode denies the action or prompts the user instead of silently allowing it

#### Scenario: Attempt nested delegation
- **WHEN** a subagent attempts to invoke another subagent
- **THEN** its task permission denies the invocation

#### Scenario: Run an applicable verification gate
- **WHEN** a writer completes an in-scope change
- **THEN** it can execute the relevant commands required by `AGENTS.md` and reports results and material caveats before completion

### Requirement: Resolvable and maintainable roster
Every configured model ID MUST appear in the locally available OpenCode catalog. Model choices SHALL live in agent frontmatter, while routing prose SHALL refer to stable role names and capabilities so a future model upgrade does not require rewriting workflow semantics.

#### Scenario: Resolve the updated roster
- **WHEN** OpenCode loads the project agent configuration
- **THEN** `planner`, `sol`, and `terra` resolve with their intended model families, `hard-problem` is absent, and no stale GPT-5.5, Gemini explorer, or `deepseek-reasoner` reference remains

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
