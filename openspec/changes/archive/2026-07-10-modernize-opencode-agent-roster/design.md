## Context

OpenCode 1.17.17 currently loads eight project agents from `.opencode/agent/`: GLM-5.2 orchestrates and implements, Gemini 3.1 Pro explores, GPT-5.5 handles hard problems and adjudication, DeepSeek Reasoner performs senior review, and free Nemotron/MiMo agents provide fast review and mechanical execution. The local provider catalog confirms access to GPT-5.6 Sol and Terra, GLM-5.2, and DeepSeek V4 Pro. OpenCode automatically loads root `AGENTS.md`, but `opencode.json` additionally injects a stale `README.md`, and the Harpia skill duplicates pre-Constitution architecture.

The roster is repository development tooling, not Harpia's product-side Executor catalog. Its prompts must nevertheless honor the Platform Constitution whenever they model the product domain.

## Goals / Non-Goals

**Goals:**

- Provide a quality-first Sol planner and a separately queryable Sol expert.
- Add Terra as a second, distinct implementation choice without creating overlapping writers.
- Use GLM-5.2's large context for repository exploration and remove Gemini from the project roster.
- Preserve model-family diversity at planning, execution, and sensitive-review boundaries.
- Make routing deterministic enough to avoid skipped senior review, unnecessary adjudication, and competing edits.
- Align every role with current project guidance, verification gates, and least-privilege tool access.

**Non-Goals:**

- Change Harpia runtime agents, ExecutorSKUs, schemas, APIs, provider credentials, global OpenCode configuration, or product behavior.
- Change the default selected OpenCode primary agent or remove the inexpensive `scut`/`reviewer-fast` tiers.
- Live-benchmark paid models or guarantee future provider availability.
- Migrate the working `.opencode/agent/` directory to the newer plural path as part of this change.

## Decisions

### 1. Separate Sol planning from Sol expert consultation

Add `planner` as a primary agent using `openai/gpt-5.6-sol` with `reasoningEffort: max`. It researches and produces executor-cold development plans, may delegate read-only mapping/review, and asks before writing. Add `sol` as a read-only subagent on the same model/effort for one focused hard problem. Remove `hard-problem` instead of retaining an overlapping alias; callers move to `@sol`.

The base Sol slug plus explicit reasoning effort is preferred over OpenCode's convenience `-pro` model ID. OpenAI documents Pro as an execution mode on a base GPT-5.6 model, while the base slug is locally available and portable. A separate planner and expert prompt is preferable to one mode-switching role because primary planning and bounded subagent consultation require different permissions and deliverables.

### 2. Keep GLM as workhorse and add Terra as the alternative writer

Retain `implementer` on `zai-coding-plan/glm-5.2`. Add `terra` on `openai/gpt-5.6-terra` with high reasoning for briefs needing more judgment, model-family diversity, or a retry after a GLM stall. Orchestration selects exactly one writer for any overlapping file set; it never races GLM and Terra on the same files. Hard unresolved design goes through `@sol` before either writer edits.

This preserves the low-cost workhorse instead of replacing it wholesale, while making Terra's quality/cost balance available deliberately.

### 3. Replace Gemini exploration with GLM-5.2

Move `explorer` to `zai-coding-plan/glm-5.2`. OpenCode's model registry reports a 1,000,000-token context window, making it the strongest configured GLM option for broad repository mapping. Explorer remains read-only and returns conclusions with `path:line` evidence and explicit search bounds.

This reduces diversity between exploration and default implementation, but Sol/Terra planning and DeepSeek review retain diversity at the decision and quality gates where correlated blind spots matter most.

### 4. Preserve a layered, model-diverse review loop

Keep the free fast reviewer on every diff. Change senior review to `deepseek/deepseek-v4-pro`, avoiding the retiring `deepseek-reasoner` alias. Sensitive changes always receive both fast and senior review. An uncontested finding returns to the owning executor; `adjudicator`, upgraded to Sol with high reasoning, runs only after an executor and reviewer (or two reviewers) state an actual reasoned disagreement.

Prompts describe capabilities rather than hardcoding model names, leaving frontmatter as the model source of truth.

### 5. Make project context current and non-duplicative

Remove `instructions: ["README.md"]` because OpenCode already loads root `AGENTS.md`. Every domain-aware role reads `AGENTS.md` and treats `docs/architecture/harpia-platform.md` as authoritative before modeling; ADRs are historical rationale. Rewrite the Harpia skill as a concise router to those sources and relevant companion documents instead of duplicating architecture that can drift.

### 6. Apply least privilege to orchestration and mutation

Global Git mutations and GitHub MCP actions require approval; safe read-only Git inspection remains allowlisted. The orchestrator receives an explicit task allowlist for project subagents. Subagents cannot spawn more agents, and internal/read-only roles cannot edit files or use GitHub tools. Writers retain edit access and narrowly scoped verification commands, but Git/GitHub publication and destructive operations are never silently allowed.

## Risks / Trade-offs

- **Sol max reasoning increases latency and cost** → reserve `planner`/`@sol` for planning and genuinely hard questions; keep GLM, Terra, MiMo, and Nemotron for routine work.
- **GLM exploration and implementation share a model family** → keep independent Sol/Terra decision paths and mandatory DeepSeek senior review for sensitive work.
- **Removing `@hard-problem` breaks saved prompts** → update every repository reference and verify the old name is absent; the replacement name is intentionally direct and discoverable.
- **Provider catalogs can change** → validate exact model IDs with `opencode models` and keep model choices isolated in frontmatter.
- **Permission patterns can accidentally block useful commands** → allow the documented verification gates explicitly and leave unclassified commands approval-gated rather than broadly allowed.
- **Stale architecture can re-enter through duplicated guidance** → thin the Harpia skill and make the Constitution link, not copied prose, authoritative.

## Migration Plan

1. Add `planner`, `sol`, and `terra`; update existing prompts and routing; delete `hard-problem` only after all references move.
2. Update project-wide instructions and permissions, then thin the Harpia skill.
3. Validate JSON, model availability, resolved agent names, stale-reference absence, OpenSpec artifacts, and whitespace.
4. Roll back by restoring the prior tracked OpenCode files; no data or runtime migration is required.

## Open Questions

None. Model IDs, role boundaries, and the breaking agent rename were confirmed during planning and approved by the user.
