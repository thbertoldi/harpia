## Context

This design answers the deferred `stabilize-chat-plan-journey` tasks 7.1 (a richer LinkedIn content plan) and **7.2** (whether new artifact fields/types, `ExecutorSKU`s, or agent manifests are required). It is grounded in what exists today:

- The template `control-plane/internal/plans/templates/news-to-social-post.yaml` is a fixed four-step DAG (`fetch-news → write-draft → adapt-for-linkedin → publish-linkedin`) with `input_parameters` mapped via `runtimeMappings` onto `SEED_ARTIFACT`, `SLOT_BINDING`, and `BEHAVIOR_POLICY` targets.
- Four `ExecutorSKU`s exist (`control-plane/internal/executors/catalog/catalog.go`): `rss-news-feed`, `linkedin-publish`, `newsletter-writer-senior`, `linkedin-voice-senior`.
- Five artifact-type keys exist (`control-plane/internal/artifacts/validation.go`): `DateRange`, `NewsList`, `TextDraft`, `LinkedInPostDraft`, `PublishConfirmation`.
- The preview stack **already supports** image/markdown/html: `PreviewArtifactResponse` has `image_preview` (with an `ImagePreview` message), `markdown_preview`, and `html_preview`; the frontend registry (`frontend/src/lib/artifacts/preview.ts`) already maps `image`/`markdown`/`html`/`text`/`list` kinds; the canonical `ArtifactPreviewSheet` was made canonical by `stabilize-chat-plan-journey`.
- The constitution (§7) states directly that **format is a configuration axis** ("post / carousel / image-backed"), that rich content is roadmap Phase 3 / M04, and that new formats "need new channel executors + artifact types."

Constraints (from `AGENTS.md` / constitution):

- Ubiquitous language is authoritative; UI surfaces browse the Artifact stream (no out-of-band CRUD).
- Feed/OAuth/renderer/provider config lives on `ExecutorInstallation`, never on templates or artifacts.
- Pre-v1: protos, artifact-type keys, SKU keys, and template seeds may change freely; no migrations or shims.
- Keep infrastructure out of domain packages (hexagonal / ports & adapters).

## Goals / Non-Goals

**Goals:**

- Decide the artifact model for carousel and image outputs (7.2 part 1).
- Decide the `ExecutorSKU` and agent-manifest set (7.2 part 2).
- Decide how output-format choice is modeled (new steps, `input_parameters`, `runtimeMappings`, and whether this is a new template or a variant).
- State preview/renderer implications, separating settled decisions from open questions for human review.

**Non-Goals:** interactive carousel editor; multi-channel fan-out; Resource promotion; memory capture; video.

## Decisions

### 3.1 ArtifactType / proto decisions

**Decision: add two new typed `ArtifactType`s — `CarouselDraft` and `ImageAsset` — and no new preview transport.**

New proto messages in `proto/harpia/artifacts/v1/artifacts.proto`:

```proto
message CarouselSlide {
  string heading = 1;
  string body = 2;
  string image_artifact_id = 3; // optional ImageAsset ref for image-backed slides
  string alt_text = 4;
}

message CarouselDraft {
  string title = 1;
  string hook = 2;          // opening/cover-slide hook
  repeated CarouselSlide slides = 3;
  string caption = 4;       // accompanying post caption
  repeated string hashtags = 5;
}

message ImageAsset {
  string prompt = 1;        // generation prompt / brief (provenance)
  string mime_type = 2;     // e.g. image/png
  int32 width = 3;
  int32 height = 4;
  string alt_text = 5;
  string caption = 6;
  // Bytes live in Garage; the Artifact.storage_uri/content_hash address the payload,
  // consistent with every other ArtifactType. No inline bytes on the artifact.
}
```

New artifact-type key constants in `control-plane/internal/artifacts/validation.go`
(`harpia.artifacts.v1.CarouselDraft`, `harpia.artifacts.v1.ImageAsset`) with JSON Schemas registered on the same path as the existing five types.

**Flow through the Artifact stream.** Both are ordinary typed step outputs: a step's
`output_artifact_type` is `CarouselDraft` or `ImageAsset`, the runtime creates the Artifact
via `CreateArtifactWithPayload` (payload JSON in Garage, metadata in Postgres), and the same
`ListArtifacts` / preview / approval surfaces browse them. `CarouselSlide.image_artifact_id`
references an `ImageAsset` Artifact by id — carousels compose images by reference, not by
embedding bytes, so image assets stay first-class, individually previewable, and reusable.

**ArtifactVersion.** No change to versioning. Regeneration ("try another hook", "redraw slide
3") and human edits create new `ArtifactVersion` rows under the same Artifact, exactly as
`TextDraft`/`LinkedInPostDraft` do today; approval and publish act on the current version. The
existing `SaveTextArtifactVersion` covers text-shaped edits; carousel/image edits use the
generic `CreateArtifactWithPayload` version path (payload replaced, `source_version_id` set).

**Preview.** **No new `PreviewArtifactResponse` oneof variant.** `CarouselDraft` renders via
the existing `markdown_preview` (each slide as a `## heading` + body section, with referenced
images shown inline); `ImageAsset` renders via the existing `image_preview` (`ImagePreview`
carrying a signed Garage URL or inline fallback + `alt_text`). Only new `BuildPreview` cases
in `control-plane/internal/artifacts/preview.go` are required. Rationale: the transport and the
frontend renderer registry already exist — adding a new oneof variant would be gratuitous
churn against a stack that already speaks image and markdown.

*Trade-off:* markdown is a lossy carousel preview (no slide-pager UX). Accepted for the first
cut; a dedicated slide viewer is listed as an open question, not a blocker — it is additive
frontend polish over the same `CarouselDraft` payload.

### 3.2 ExecutorSKU / agent manifest decisions

**Decision: two new `ExecutorSKU`s, one new agent manifest, and one *extended* existing manifest.**

| Concern | Decision | Kind |
|---|---|---|
| Stronger hook + anti-AI-jargon voice for the **text post** | **Reuse `linkedin-voice-senior`**; ship a **new manifest version** with hook/anti-jargon system-prompt rules and new input fields. No new SKU. | Agent |
| **Carousel** authoring (`TextDraft → CarouselDraft`) | **New SKU `linkedin-carousel-senior`** + **new agent manifest**. | Agent |
| **Image/asset** generation (→ `ImageAsset`) | **New SKU `image-asset-generator`**. | Integration |

- **Voice writer — extend, don't fork.** The stronger-hook / anti-AI-jargon requirement is a
  quality change to the *same* contract (`TextDraft → LinkedInPostDraft`). Model it as a new
  **version** of `agents/linkedin-voice-senior/*.yaml` (bump from `0.1.0`) whose
  `system_prompt` adds explicit hook construction and a banned-phrase / anti-AI-jargon rubric,
  and whose `input_schema` accepts the new voice fields (see §3.3). The `ExecutorSKU` key stays
  `linkedin-voice-senior`, so no SlotBinding or seed data churn. Adding a whole new SKU for a
  prompt-quality bump would fragment the catalog for no contract change.

- **Carousel writer — new SKU + manifest.** A carousel is a *different typed output*
  (`CarouselDraft`, not `LinkedInPostDraft`), so it is a genuinely new `(input → output)`
  contract and therefore a new `ExecutorSKU` (`linkedin-carousel-senior`) with its own agent
  manifest (`agents/linkedin-carousel-senior/*.yaml`, capability `linkedin-carousel-authoring`,
  `artifact_input_type: TextDraft`, `artifact_output_type: CarouselDraft`). This follows the
  constitution rule that a materially new output shape needs a new SKU + ArtifactType.

- **Image generator — new integration-kind SKU.** Image generation calls an **external
  provider** (or a hosted render service) with credentials and brand config. Per the boundary
  rule, that config belongs on the **`ExecutorInstallation`** (provider, model, brand assets,
  allowed sizes, API key/OAuth) — never on the template or artifact. That makes it an
  `EXECUTOR_KIND_INTEGRATION` SKU (`image-asset-generator`), mirroring how `linkedin-publish`
  and `rss-news-feed` carry OAuth/feed config on their installations.

  *Trade-off / alternative:* image generation could instead be an `EXECUTOR_KIND_AGENT` with an
  image tool in `allowed_tool_ids`. Rejected as the default because credentials + brand config
  fit the installation-config boundary more cleanly as an integration, and it keeps the
  agent-runtime free of provider SDKs. This is flagged as an open question (§3.4) since it
  depends on how the agent-runtime's tool bridge evolves (see the `Creator Kit` idea).

### 3.3 Output-format modeling

**Decision: a new *variant* `PlanTemplate` `linkedin-content-studio`, with format modeled as a `PlanBehaviorPolicies` selection, voice rules as `SEED_ARTIFACT` inputs, and no new runtime-mapping target.**

- **Variant template, not a mutation of `news-to-social-post`.** Keep `news-to-social-post`
  as the simple, stable approval-only journey the MVP relies on. Introduce
  `linkedin-content-studio` sharing the neutral head (`fetch-news → write-draft → TextDraft`)
  and diverging into format-specific output steps. Rationale: the DAG and input set differ
  materially, and per constitution §12 a genuinely richer plan warrants new SKUs + ArtifactTypes
  (which we are adding). A variant lets the assistant match richer intent without destabilizing
  the existing flow. (Pre-v1 we *could* evolve the one template in place; a variant is chosen
  for journey safety, not compatibility.)

- **New `PlanStep`s** in the variant (all present in the DAG; the engine runs only the
  format-selected branch):
  - `adapt-for-linkedin` (`TextDraft → LinkedInPostDraft`, SKU `linkedin-voice-senior`) — text-post branch.
  - `draft-carousel` (`TextDraft → CarouselDraft`, SKU `linkedin-carousel-senior`) — carousel branch.
  - `generate-image` (`TextDraft → ImageAsset`, SKU `image-asset-generator`) — image-backed branch (feeds the post/carousel that references it).
  - `publish-linkedin` (publish step) accepts the selected branch's output (see typed-contract note below).

- **Format axis = `PlanBehaviorPolicies`.** Add an `output_format` `input_parameter`
  (`TEMPLATE_INPUT_PARAMETER_TYPE_SELECT`: `text_post` | `carousel` | `image_backed_post` |
  `approval_only`) mapped via the **existing** `TEMPLATE_INPUT_RUNTIME_TARGET_BEHAVIOR_POLICY`
  to a new policy `content_output_format`. Rationale: format is a governed, plan-level behavior
  (like `publish_approval_mode`) that the execution engine reads to select which output branch
  runs — it is not a seed payload and not a slot identity. This reuses the existing runtime
  target; **no new `TemplateInputRuntimeTarget` enum value is needed.** It does require a new
  field on `PlanBehaviorPolicies` (`content_output_format`) in `plans.proto`.

- **Voice rules = `SEED_ARTIFACT` inputs** (extend the existing `ContentPreferences` seed
  pattern already used for theme/tone/audience): new `input_parameters` `hook_style` (SELECT),
  `avoid_ai_jargon` (BOOL — note: needs a new `TEMPLATE_INPUT_PARAMETER_TYPE_BOOL`, or model as
  a two-option SELECT to avoid a proto add; see open questions), and `voice_rules` (TEXTAREA),
  each mapped via `TEMPLATE_INPUT_RUNTIME_TARGET_SEED_ARTIFACT` into
  `harpia.internal.ContentPreferences` (`$.hook_style`, `$.avoid_ai_jargon`, `$.voice_rules`).
  The extended `linkedin-voice-senior` / `linkedin-carousel-senior` manifests consume them.

- **`approval_only`** selects the plain `text_post` branch with `publish_approval_mode =
  REQUIRE_APPROVAL` and no image/carousel steps — i.e. it is the current `news-to-social-post`
  behavior expressed as a format choice, so the two templates converge behaviorally for that
  option.

- **Engine work — conditional branch selection.** The Temporal engine today walks a static
  DAG. Selecting one output branch by `content_output_format` requires the engine to **skip
  steps whose format is not selected**. This is the one net-new execution-engine capability
  and is called out as a task and an open question (fan-out vs skip vs assistant-picked
  sub-template).

- **Typed-contract note (publish input).** The publish step must accept whichever branch ran.
  Settled recommendation: keep publish typed by making `publish-linkedin` a channel publish that
  accepts a **format-discriminated LinkedIn asset** — for the first cut, keep three explicit
  typed publish inputs (post / carousel / image-backed) selected with the branch, rather than a
  single untyped blob. Whether to collapse these into one `LinkedInPublishable` union is an
  open question (§3.4), consistent with the constitution's channel-discriminated-family caution
  against both catalog explosion and untyped blobs.

### 3.4 Renderer / preview implications

**Settled:**

- Reuse the canonical `ArtifactPreviewSheet` and the existing preview-kind registry; no new
  panel state machine and no new `PreviewArtifactResponse` variant.
- `CarouselDraft → markdown_preview`; `ImageAsset → image_preview`. Only new `BuildPreview`
  cases (control-plane) + the two new type keys in the frontend kind map are required.
- The selected format is previewed **before** publish; when `publish_approval_mode =
  REQUIRE_APPROVAL`, the in-thread approval card opens the same sheet for the format artifact
  (the `artifact-preview-panel` renderer-registry requirement is MODIFIED to name the two new
  types).
- All new copy is specified as `en` + `pt-BR` keys (see tasks); JSON files are not edited here.

**Open questions for human review:**

1. **Conditional execution mechanism** — skip-non-selected-branch (recommended for first cut)
   vs multi-format fan-out (enable N formats from one `TextDraft`) vs assistant-selected
   per-format sub-template. Affects engine scope.
2. **Publish typing** — three typed publish inputs (recommended) vs one `LinkedInPublishable`
   discriminated union. Affects proto + publisher.
3. **Image executor kind** — integration SKU with provider config on `ExecutorInstallation`
   (recommended) vs agent-with-image-tool. Affects agent-runtime tool bridge.
4. **`avoid_ai_jargon` typing** — add `TEMPLATE_INPUT_PARAMETER_TYPE_BOOL` vs model as a
   two-option SELECT (no proto change). Cosmetic but decides a proto add.
5. **Carousel preview fidelity** — ship markdown-as-slides first (recommended) vs invest in a
   dedicated slide-pager preview component now.

## Resolved decisions (acting senior; @hard-problem/@reviewer-senior unavailable)

GPT/Gemini-backed agents were unavailable for this pass, so the orchestrator (GLM-5.2)
resolved the five open questions and verified them against a full surface map of the
engine, seeder, agent-runtime, and storage layers. Each call is reversible pre-v1.

1. **Conditional execution mechanism → SKIP non-selected branch, format inferred from
   artifact types (no step-level field).** Add `STEP_EXECUTION_STATUS_SKIPPED = 7` to
   `StepExecutionStatus` (plans.proto). A `step_executions.status` CHECK constraint does
   **not** exist, so this needs **no DB migration**. Rather than add a `format_filter`
   field to `PlanStep` (which the strict YAML loader + a `plan_template_steps` column +
   a proto field would force — finding F1), the engine **derives each step's format from
   its artifact types**: `LinkedInPostDraft → text_post`, `CarouselDraft → carousel`,
   `ImageAsset → image_backed_post`; `DateRange`/`NewsList`/`TextDraft`/`PublishConfirmation`
   are format-agnostic (always run). A step is skipped when its inferred format != the run's
   `content_output_format`. `approval_only` selects the text-post branch (LinkedInPostDraft).
   This eliminates the 4-layer `format_filter` plumbing, the DB migration, and the proto
   `PlanStep` change. The retry planner treats SKIPPED upstream steps as satisfied.

2. **Publish typing → three format-specific publish steps.** `PlanStep` is strictly
   single-contract (one `input_artifact_type_id` → one `output_artifact_type_id`,
   plans.proto:105), so `publish-post` (LinkedInPostDraft→PublishConfirmation),
   `publish-carousel` (CarouselDraft→PublishConfirmation), and `publish-image`
   (ImageAsset→PublishConfirmation) are three steps, each selected by `format_filter`.
   This honors ADR-012 and reuses the skip mechanism from decision 1; it avoids a
   `LinkedInPublishable` union for now.

3. **Image executor kind → integration SKU `image-asset-generator`**, provider/model/
   brand/credentials on `ExecutorInstallation` config (enforced at the integration
   boundary, never on template/artifact). Wired explicitly in `bootstrap.go`.

4. **`avoid_ai_jargon` typing → two-option SELECT (no proto change).** The frontend
   `TemplateInputsForm.svelte` has **no BOOL render path** — a `BOOL` type would fall
   through to a plain text input. SELECT already renders and round-trips through
   `materialize.go`. A proto BOOL add is deferred as gratuitous churn for one field.

5. **Carousel preview → markdown-as-slides first.** `CarouselDraft → markdown_preview`
   (slides as `## heading` + body). `CarouselSlide.image_artifact_id` references are
   **not resolved** in the first cut (see finding F5) — a dedicated viewer is additive.

## Critical implementation findings (surface map)

These constrain the briefs and correct three design assumptions:

- **F1 — YAML loader is strict** (`template_seed.go` `KnownFields(true)`). *Refined away:*
  the engine infers step format from artifact types (decision 1), so NO new `format_filter`
  step field, NO `plan_template_steps` column, NO proto `PlanStep` change is needed. The
  only seeder structural change required is F2.
- **F2 — Linear adjacency type-check breaks branching DAGs** (`template_seed.go:143-146`
  rejects `steps[i-1].output != steps[i].input`). A branching `linkedin-content-studio`
  DAG (three siblings all consuming `TextDraft`) **fails seeding** today. The seeder must
  be relaxed to validate input-satisfiability via **edges/DAG reachability**, not list
  adjacency. This is structural seeder work, not content.
- **F3 — Behavior-policy keys whitelisted in two places** (`template_seed.go:253-259`
  seeding + `materialize.go:165-183` runtime). `content_output_format` must be added to
  both or the template won't seed and the configuration won't materialize.
- **F4 — No SKIPPED status / retry coupling.** `StepExecutionStatus` tops out at FAILED(6);
  adding SKIPPED has no migration cost (F-…) but `buildRetryPlanWorkflowInput`
  (`runtime.go:644-663`) assumes every upstream step has a completed attempt with an output
  artifact. The skip path must be retry-aware.
- **F5 — ImageAsset preview has no storage primitive.** `BuildPreview(typeKey, payload)`
  sees only the JSON payload; the forward-compatible image fallback looks for
  `image_url`/`image_base64` in the payload, but the `ImageAsset` proto carries neither.
  `PayloadStore` (`garage.go`) has **no presign method**. Two viable paths: (a) add
  `PresignGet` to `PayloadStore` and inject `storageURI` into `BuildPreview`; (b) inline
  image bytes via `Get` into `ImagePreview.inline_data`. **Deferred to its own slice** —
  CarouselDraft preview (F5 applies to `image_artifact_id` refs only) lands first.
- **F6 — Agent manifests hand-wired in 3 places** (`agents/<id>/<ver>.yaml` + Python
  loader constant pinned to `0.1.0` + `registry._RUNNERS`/`_MANIFEST_MODEL_IDS` +
  `worker.py` if/elif dispatch + result-type switch). No auto-discovery. A version bump
  ripples into the SKU seed `ManifestVersion` and existing installations.
- **F7 — Integration handlers explicitly listed** in `bootstrap.go:40-47` (not
  discovered); `image-asset-generator` must be added there + `DefaultConfigValidators`.
- **F8 — validation.go uses proto-unmarshal checks, not a JSON-Schema registry.** Task 1.3
  wording ("JSON Schemas + registration") does not match the code; new types follow the
  existing `validateProtoJSON` pattern.

## Implementation order (dependency-ordered slices)

A proto + regen · B Go validation + CarouselDraft preview · C executor catalog/seed ·
D agent manifests · E behavior-policy whitelist (2 places) · F template + seeder
relaxation + format_filter plumbing · G engine conditional skip · H image storage/preview ·
I image-asset-generator integration handler · J frontend i18n + preview map + config ask ·
K verification. Slices F/G/H are the hard+sensitive core and ship last behind tests.

## Risks / Trade-offs

- **Engine branch selection is the real new backend work.** Everything else is additive
  content/schema; conditional step skipping touches the execution core → keep it behind the
  `content_output_format` policy and cover it with runtime tests before wiring the UI.
- **Markdown carousel preview is lossy** → acceptable first cut; dedicated viewer is additive.
- **Image provider config drift** → enforce that provider/model/brand/credentials live only on
  `ExecutorInstallation`; template and artifact carry only the neutral brief/prompt.
- **Catalog fragmentation** → mitigated by extending `linkedin-voice-senior` rather than adding
  a voice SKU, and by keeping a single `linkedin-content-studio` variant rather than one
  template per format.
