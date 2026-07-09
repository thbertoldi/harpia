# ADR-018: Capability-based agent teams (roles × tiers)

**Status:** Accepted
**Date:** 2026-07-09
**Deciders:** Harpia team

## Context

The first LinkedIn content plan (`rich-linkedin-content`) was built on a **one narrow SKU
per step** model: each capability became a separate agent (`linkedin-voice-senior`,
`linkedin-carousel-senior`, …), each plan step hardcoded a `default_executor_sku_key`, and
the publish step was split per format (`publish-post` / `publish-carousel` / `publish-image`).

This had three compounding problems, surfaced during smoke testing:

1. **Catalog fragmentation + rigidity.** Every new capability mints a new SKU/agent. A step
   names a *specific SKU* rather than the *capability* it needs, so agents can't be
   recombined. The catalog grows one-capability-at-a-time instead of one-agent-many-capabilities.
2. **Configuration blocks on unused executors.** The configuration assistant binds *every*
   step before the run (before the output format is even chosen). Any step whose SKU has no
   installable executor (e.g. image generation, an integration with no provider) makes the
   whole configuration unable to reach RUNNABLE.
3. **Publish split per format is wrong.** Carousel and image are *content inside one post*,
   not separate publish targets. The per-format branching (and the engine skip that came
   with it) modeled a content choice as an execution-topology choice.

The constitution (§5, §13) and ADR-012 already carry the seeds of the fix: a `PlanStep`
declares `ExecutorRequirement.required_capabilities[]`, and `default_executor_sku_key` is a
hint. The catalog data lives in Go seeders, not migrations (§13).

## Decision

Realign the executor/agent model around **multi-capable agents organized as a team**:

1. **Agents are multi-capable team members** (a "LinkedIn Content Specialist" carries
   adapt-text, author-carousel, tune-voice, anti-jargon), not one-capability specialists.
2. **Seniority tiers (Júnior / Pleno / Sênior)** per role: each tier has its own graph,
   tool access, quality, and cost-per-execution. Predefined platform agents are the defaults.
3. **A plan step declares a *required capability*** (`required_capabilities`, already in the
   proto); `default_executor_sku_key` is a hint/default, not the contract.
4. **The platform recommends a *team***: given the plan's steps and their required
   capabilities, suggest a minimal set of agents (role × tier) whose combined capabilities
   cover every step. The user accepts or swaps members.
5. **One unified content post.** The LinkedIn output is a single post artifact that may
   carry a text body, an optional carousel (markup slides), and optional generated image(s).
   **One publish step.** Carousel/image are *content*; the output "shape" is a content
   choice, not separate execution branches.
6. **Image generation is an *opt-in capability*** (an Image Generator role/agent), chosen by
   the user during configuration, never a mandatory branch.

## Rationale

Capability-matching reuses the fields ADR-012 already defines; the new work is the
recommendation concern and the catalog taxonomy, not a schema revolution. Multi-capable
tiered agents let a small team cover many steps (closer to how a human team works), keep the
catalog small, and make the user's choice about *which team at which quality/cost* — which
is the actual product decision. A unified post removes the false publish-per-format split.

### Alternatives Considered

| Alternative | Why not |
|---|---|
| Keep one-SKU-per-step; add image executor | Doesn't fix fragmentation, rigidity, or the binds-every-step blocker; catalog still grows one-capability-at-a-time. |
| Very broad "one agent does everything" | Defeats team assembly; no quality/cost tiers; no match. |
| Per-format branches + engine skip (what we built) | Models a content choice as execution topology; blocks config on unused executors; splits publish wrongly. |

## Consequences

- **Easier:** adding a new content capability = adding a capability to an existing agent (or
  a new role), not a new SKU + step + binding per capability. Smaller, recombining catalog.
  Image/carousel become opt-in content, not blockers.
- **Harder / new work:** a team-recommendation concern (capability-coverage → minimal team);
  a tiered agent taxonomy in the catalog; a composable/unified post artifact; revising the
  configuration assistant to bind by capability (and to make image genuinely opt-in).
- **Supersedes** the narrow per-capability SKUs, the separate `publish-post`/`publish-carousel`
  steps, and the `content_output_format` branch-selector built under `rich-linkedin-content`
  (the `CarouselDraft`/`ImageAsset` protos and the carousel agent graph are reused as
  *content* produced by multi-capable agents).
- **Pre-v1:** break freely; no migrations/shims for the superseded pieces.

## Next

- OpenSpec change `agent-team-model`: capability-matching + tiered agent catalog + team
  recommendation + unified post.
- Revise `rich-linkedin-content` as the first plan built on this model.
- Constitution §5 (Agent Orchestration capability) / §13 reference this decision.
