## Why

The current LinkedIn journey (`news-to-social-post`) is a single, fixed shape: fetch news → write a neutral draft → adapt to one text post → publish. It cannot express the outputs creators actually ask for — a stronger-hook text post, a carousel outline, or an image-backed post — and it cannot ask, in conversation, which of those the user wants. The `stabilize-chat-plan-journey` change deliberately deferred this richer content work (its tasks 7.1/7.2) so the stabilization pass stayed a UI/localization layer.

This change is that deferred exploration. It designs a richer LinkedIn content capability where the output **format** is a first-class configuration axis (constitution §7: *format is a configuration choice — post / carousel / image-backed*), voice quality is governed (stronger hook, anti-AI-jargon rules), and the chosen format is previewed in the thread before publish, with in-thread approval when `PlanBehaviorPolicies` require it. It also settles task 7.2: which new `ArtifactType`s, `ExecutorSKU`s, and agent manifests the build needs.

Harpia is pre-v1, so protos, artifact-type keys, executor-SKU keys, and template seeds may change freely — no migrations or compatibility shims are in scope.

## What Changes

- Introduce a **richer LinkedIn content `PlanTemplate` variant** (`linkedin-content-studio`) that reuses the neutral `fetch-news → write-draft` head (producing a channel-neutral `TextDraft`) and diverges into format-specific output steps.
- Make **output format** a conversational configuration choice — **text post**, **carousel outline**, **image-backed post**, or **approval-only publish** — modeled as a `PlanBehaviorPolicies` selection so the assistant can ask for it and the engine can act on it.
- Add **governed voice rules**: stronger-hook style and an explicit **anti-AI-jargon** toggle plus free-form voice rules, fed to the writer/adapter as `input_parameters` mapped into its seed `ContentPreferences`.
- Add two new **`ArtifactType`s** — `CarouselDraft` and `ImageAsset` — flowing through the same Artifact stream, versioned via `ArtifactVersion`, previewed via the **existing** `PreviewArtifactResponse` variants (`markdown_preview`, `image_preview`).
- **Preview the selected format before publish** in the canonical artifact preview panel, and route the publish gate through the existing **in-thread approval** card when `publish_approval_mode = REQUIRE_APPROVAL`.
- Add the executors the new outputs require: a **carousel writer** `ExecutorSKU` + agent manifest, and an **image/asset-generation** `ExecutorSKU` (`ExecutorInstallation` carries provider/model/brand config and credentials). Extend — not fork — the existing `linkedin-voice-senior` manifest for the stronger-hook / anti-AI-jargon voice rules.
- Specify all new user-facing copy as flat `translate()` keys and catalog keys for `en` + `pt-BR` (`catalog.plan.linkedin-content-studio.*`, `plans.inputs.<key>.*`). This change does not edit the JSON files.

## Capabilities

### New Capabilities

- `rich-linkedin-content`: A LinkedIn content plan whose output format is a conversational configuration axis (text / carousel / image-backed / approval-only), with governed voice rules, per-format preview before publish, and format-aware in-thread approval.
- `rich-content-artifacts`: Typed `CarouselDraft` and `ImageAsset` artifacts (schemas, versioning, preview mapping) and the new `ExecutorSKU`s that produce them.

### Modified Capabilities

- `artifact-preview-panel`: The renderer registry must cover the new `CarouselDraft` and `ImageAsset` `ArtifactType`s, mapped onto the existing `markdown_preview` / `image_preview` variants.

## Impact

- **Proto:** new messages `CarouselDraft`, `CarouselSlide`, `ImageAsset` in `proto/harpia/artifacts/v1/artifacts.proto`; no new `PreviewArtifactResponse` oneof variant. Possible new `PublishApprovalMode`/behavior-policy field for `content_output_format` in `proto/harpia/plans/v1/plans.proto` (see design §3.3).
- **Control-plane:** new artifact-type key constants + JSON Schemas (`control-plane/internal/artifacts/validation.go`), new `BuildPreview` cases (`control-plane/internal/artifacts/preview.go`), new SKU keys (`control-plane/internal/executors/catalog/catalog.go`) + seed rows (`control-plane/internal/executors/seed.go`), a new template seed (`control-plane/internal/plans/templates/linkedin-content-studio.yaml`), and format-aware step selection in the execution engine (`control-plane/internal/workflow/`, `control-plane/internal/plans/`).
- **Agent runtime:** new `agents/linkedin-carousel-senior/*.yaml` manifest; a new version of `agents/linkedin-voice-senior/*.yaml` with hook / anti-AI-jargon rules; image-generation executor wiring for the image SKU.
- **Frontend:** catalog + input i18n keys under `frontend/src/lib/i18n/content/{en,pt-BR}.json` and `frontend/src/lib/i18n/{en,pt-BR}.json`; preview kind coverage in `frontend/src/lib/artifacts/preview.ts` (kinds already exist — carousel/image just need the new type keys to map). No new panel state machine (reuses the canonical `ArtifactPreviewSheet`).
- **Docs:** forward link from the `Rich LinkedIn content plan variants` idea-log entry to this change.

## Non-goals

- No interactive slide/carousel *editor* — carousel output is a previewable draft Artifact, not a design surface (that is the separate `Creator Kit` idea).
- No general multi-channel fan-out (Instagram/X/blog); this stays LinkedIn-only. The channel-discriminated artifact family (§7) is out of scope beyond LinkedIn.
- No promotion of these outputs to `Resource`s and no memory capture of voice preferences (separate `Proactive memory capture` idea).
- No new scheduling/cadence behavior; cadence stays in `PlanSchedule` / `PlanConfiguration.kind`.
- No video generation.
