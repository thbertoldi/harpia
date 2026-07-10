## ADDED Requirements

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
