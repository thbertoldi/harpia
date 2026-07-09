## Context

ADR-018 (accepted; folded into the constitution §4/§7.2/§13) realigns the executor/agent
model from "one narrow SKU per step" to "multi-capable agents organized as a team." This
change implements that decision and revises `rich-linkedin-content` onto it.

What exists today (built under `rich-linkedin-content` + earlier work):
- Agent manifests under `agents/<id>/<ver>.yaml` declare `capabilities[]` and
  `metadata.artifact_input_type`/`artifact_output_type`; the validator enforces required
  fields and prompt-variable/schema consistency.
- The executor catalog is Go-seeded (`executors.DefaultCatalogSeeds`); each SKU has
  `CompatibilityMetadata` (input/output type keys, manifest id/version, connection type).
- `PlanStep.ExecutorRequirement` already carries `required_capabilities[]` + `executor_kind` +
  `connection_type`; `default_executor_sku_key` is a per-step hint.
- The configuration assistant (`planassistant`) binds every step (`firstUnboundStepKey`)
  before policies; binding is by SKU/installation, not capability.
- `rich-linkedin-content` split publish per format and used `content_output_format` as an
  engine branch selector (skip non-selected steps).

Constraints (constitution / AGENTS.md): ubiquitous language is authoritative; UI surfaces
browse the Artifact stream; catalog data lives in Go seeders not migrations (§13); pre-v1
break freely; keep infrastructure out of domain packages.

## Goals / Non-Goals

**Goals:** a tiered, multi-capable agent catalog; capability-based step requirements; a
team-recommendation concern; a unified LinkedIn post; image as an opt-in capability;
configuration that no longer blocks on opt-out capabilities.

**Non-Goals:** marketplace/runtime agent CRUD; ML tier selection; multi-channel fan-out;
interactive carousel/PDF editor.

## Decisions

### D1 — Agent taxonomy: role × tier, multi-capable

A platform **agent** = (role, tier). A **role** is a coherent job with a set of
**capabilities** (e.g. *LinkedIn Content Specialist* → `{linkedin-content-adaptation,
carousel-authoring, tone-matching, anti-ai-jargon}`). **Tiers** are Júnior / Pleno / Sênior —
each a distinct manifest (graph, prompt sophistication, tool access) with its own
cost-per-execution.

Proposed seed roles (refine during implementation):
| Role | Capabilities | Notes |
|---|---|---|
| News Writer | `news-synthesis` | NewsList → TextDraft (the neutral draft head). |
| LinkedIn Content Specialist | `linkedin-content-adaptation`, `carousel-authoring`, `tone-matching`, `anti-ai-jargon` | TextDraft → LinkedInPostDraft **or** CarouselDraft (content of the post). |
| Image Generator | `image-generation` | opt-in; produces ImageAsset embedded in the post/carousel. |
| RSS News Feed / LinkedIn Publish | (integrations) | unchanged integration executors. |

Each role exists in 3 tiers (Jr/Pleno/Sr) → 3 manifests each. The Sr LinkedIn Specialist
subsumes what `linkedin-voice-senior` + `linkedin-carousel-senior` did separately. This
**consolidates** the narrow SKUs.

### D2 — Step declares required capabilities; SKU is a hint

`PlanStep.ExecutorRequirement.required_capabilities[]` is the contract. Templates set
`default_executor_sku_key` only as a *default hint* (e.g. prefer the Sr tier). The
configuration no longer treats a missing specific SKU as a hard requirement — it asks "which
agent (with capability X) fills this step?".

### D3 — Team recommendation (rule-based)

New concern `agent-team-recommendation`: given a plan's steps + required capabilities (minus
opt-out capabilities), compute a **minimal covering set** of agents (role × tier) such that
every step's required capability is satisfied by some agent in the team. Rules (first cut):
- Cover all required capabilities with the fewest agents (a multi-capable agent fills many
  steps).
- Default tier = Sênior for quality-critical roles, Júnior for high-volume/cheap ones;
  user can swap tiers per role.
- Prefer entitled, already-installed agents; fall back to entitled-not-installed (prompt
  setup) and flag unentitled gaps.
The recommendation is **advisory**: the user accepts or swaps members/tiers before RUNNABLE.

### D4 — Opt-in capabilities excluded from the run

Some capabilities are **opt-in** (declared so on the template/role — e.g.
`image-generation`). During configuration the assistant asks which opt-in capabilities to
include. Steps whose *only* required capability is an opted-out one are **excluded from the
run** (they need no SlotBinding, no overseer, and the engine skips them). This is the fix for
the configuration blocker: an opt-out capability no longer forces a binding. (This generalizes
and replaces the `content_output_format` branch selector — the "what's in the post" choice is
now a capability opt-in, not an execution-topology branch.)

### D5 — Unified content post

The LinkedIn output is a single **`LinkedInPost`** artifact that composes:
- a text body (always),
- an optional **carousel** (`CarouselDraft` — markup slides),
- optional **image(s)** (`ImageAsset`, when the user opted into image generation).
**One publish step** consumes the post and publishes whatever it carries. The per-format
`publish-post`/`publish-carousel` steps and the engine skip-by-format are removed; the engine
still skips opted-out-capability steps (D4), but publish is singular.

`CarouselDraft` and `ImageAsset` protos (already added) are reused as the post's optional
content parts. Carousel preview stays markdown-as-slides now; HTML/CSS→PDF rendering is an
additive future layer.

### D6 — What this supersedes (in `rich-linkedin-content`)

- The narrow SKUs `linkedin-voice-senior` + `linkedin-carousel-senior` → consolidated into
  the **LinkedIn Content Specialist** role (× tiers).
- The separate `publish-post`/`publish-carousel` steps → **one publish** over the unified post.
- The `content_output_format` behavior-policy branch selector → **capability opt-in** (D4);
  the `ContentOutputFormat` proto enum is retired or repurposed (pre-v1, break freely).
- The engine's `shouldSkipStepForFormat` (artifact-type inference) → replaced by
  **opt-out-capability skipping** (a step is skipped when its required capability was opted
  out). The SKIPPED status + retry-aware logic (already built) is reused.

## Risks / Trade-offs

- **Taxonomy is a product judgment.** The proposed roles/capabilities are a starting point;
  refine with use. A wrong granularity (too fine → fragmentation returns; too coarse → no
  team to assemble) is the main risk.
- **Team recommendation is rule-based.** Good enough for v1; not optimal. Marked non-goal for
  ML tuning.
- **Unified post is a schema shift.** Composing text+carousel+image in one artifact touches
  the post proto + preview + publish. Pre-v1, break freely; the `CarouselDraft`/`ImageAsset`
  payloads are reused, not redefined.
- **Consolidating agents changes the catalog + entitlements.** Existing installations bound
  to the narrow SKUs need remapping during the seed revision (pre-v1: acceptable).
