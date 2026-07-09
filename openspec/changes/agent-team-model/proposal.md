## Why

The LinkedIn content plan exposed a wrong model: **one narrow SKU per step**, each capability
a separate agent, publish split per format. That fragments the catalog (a new SKU per
capability), makes binding rigid (a step names a SKU, not the capability it needs), and
**blocks configuration** — the assistant binds *every* step before the run, so any step whose
SKU has no installable executor (e.g. image generation) makes the plan unable to reach
RUNNABLE. It also split "publish" per format when carousel/image are really *content inside
one post*.

This change realigns the executor/agent model per **ADR-018** (folded into the constitution
§4/§7.2/§13): **multi-capable agents organized as a team**.

## What Changes

- **Agents become multi-capable and tiered.** A single "LinkedIn Content Specialist" carries
  several capabilities (adapt text, author carousel, tune voice, anti-jargon); each role
  exists in **Júnior / Pleno / Sênior** tiers with their own graph, tools, and
  cost-per-execution. Predefined platform agents are the defaults.
- **Plan steps declare required capabilities**, not a hardcoded SKU. `default_executor_sku_key`
  becomes a hint; `ExecutorRequirement.required_capabilities` is the contract.
- **The platform recommends an agent team** — a minimal set of agents (role × tier) whose
  combined capabilities cover every step. The user accepts or swaps members ("choose the team
  that runs my plan").
- **Capability-based binding.** A `SlotBinding` is chosen so the bound agent satisfies the
  step's required capabilities. A multi-capable agent can fill several steps.
- **Opt-in capabilities don't block.** A capability the user did not select (e.g. image
  generation) is excluded from the run and needs no binding — so an optional capability never
  blocks configuration.
- **One unified content post.** The LinkedIn output is a single post artifact that may carry a
  text body, an optional carousel (markup slides), and optional generated image(s). **One
  publish step.** Carousel/image are *content*; the output shape is a content choice, not
  separate execution branches.
- **Image generation is an opt-in capability** (an Image Generator role/agent), chosen during
  configuration, never a mandatory branch.

## Capabilities

### New Capabilities
- `agent-team-recommendation`: given a plan's steps and their required capabilities, recommend
  a minimal agent team (role × tier) covering them; surface it for the user to accept or swap.
- `tiered-agent-catalog`: the agent catalog is organized as **role × tier**, multi-capable,
  seeded declaratively; predefined platform agents are the suggested defaults.

### Modified Capabilities
- `capability-based-binding`: plan steps declare `required_capabilities`; `SlotBinding` matches
  by capability coverage; capabilities the user did not opt into are excluded from the run and
  need no binding (no longer blocks configuration).
- `unified-content-post`: the LinkedIn post is one composable artifact (text + optional
  carousel + optional image); one publish step consumes it (replaces the per-format publish
  split and the `content_output_format` branch selector).
- `conversational-plan-proposal`: the configuration assistant asks which optional capabilities
  to include (e.g. "generated images?") and recommends the team.

## Impact

- **Proto:** `ExecutorRequirement` already has `required_capabilities`; add a **seniority
  tier** field on `ExecutorSKU`/manifest and a **capability opt-in** signal on the
  configuration (which optional capabilities the user included). The unified post may add a
  composable `LinkedInPost` carrying optional carousel/image parts (revising the per-format
  artifacts).
- **Control-plane:** a **team-recommendation** concern (capability-coverage → minimal team);
  the agent **catalog seeder** redefined to role × tier, multi-capable; the configuration
  assistant binds by capability and treats image/etc. as opt-in; one publish step.
- **Agent-runtime:** agents reorganized into tiered manifests with multiple capabilities
  (e.g. a Sr LinkedIn Specialist graph that adapts *and* authors carousels *and* tunes voice);
  the narrow `linkedin-carousel-senior`/`linkedin-voice-senior` split is consolidated.
- **Frontend:** a "choose your team" surface (recommended team + swap members/tiers); the
  configuration no longer forces binding for opt-out capabilities; one post preview.
- **Docs:** ADR-018 + constitution amendment (done); revises `rich-linkedin-content`.

## Non-goals

- No marketplace / runtime agent CRUD (predefined seed agents only; the catalog service is
  deferred per §13).
- No automatic tier selection tuning/ML — recommendation is rule-based (capability coverage +
  sensible tier default) for now.
- No multi-channel fan-out beyond LinkedIn (the unified post is LinkedIn-scoped here).
- No interactive carousel/PDF editor — carousel is a previewable markup draft (HTML/CSS→PDF
  rendering is an additive future layer over the same `CarouselDraft`).

## Relationship to `rich-linkedin-content`

`rich-linkedin-content` is **revised onto this model** (its narrow per-capability SKUs,
separate publish steps, and `content_output_format` branch selector are superseded). Its
`CarouselDraft`/`ImageAsset` protos and carousel agent graph are reused as *content* produced
by multi-capable agents. This change should land the model; `rich-linkedin-content` is then
re-authored as the first plan on it (tracked as a follow-up, or folded into this change's
later tasks).
