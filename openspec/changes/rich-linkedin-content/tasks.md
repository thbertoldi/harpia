## 1. Artifact Types (Proto + Schemas)

> **Superseded execution scope (2026-07-12):** Do not continue unfinished tasks under this
> change that rely on its branching DAG, separate format publish paths, or text-only publisher.
> [`composable-linkedin-execution`](../composable-linkedin-execution/) owns the linear
> composable-post execution, review/revision, version-pinned approval, and real document upload.
> Completed checkmarked tasks below remain historical evidence.

- [x] 1.1 Add `CarouselSlide`, `CarouselDraft`, and `ImageAsset` messages to `proto/harpia/artifacts/v1/artifacts.proto` (no new `PreviewArtifactResponse` oneof variant). Verify: `cd proto && buf lint`.
- [x] 1.2 Regenerate proto stubs for Go and TS consumers. Verify: generated files under `control-plane/internal/.../gen` and `frontend/src/lib/gen/harpia/artifacts/v1/artifacts_pb.ts` include the new messages; `cd frontend && bun run check` introduces no new baseline errors.
- [x] 1.3 Add type-key constants (`harpia.artifacts.v1.CarouselDraft`, `harpia.artifacts.v1.ImageAsset`) and JSON Schemas + registration in `control-plane/internal/artifacts/validation.go`. Verify: `cd control-plane && go test ./internal/artifacts/...`. (Note: the codebase uses proto-unmarshal validation, not a JSON-Schema registry — followed the existing `validateProtoJSON` pattern; finding F8.)

## 2. Artifact Preview Mapping

- [x] 2.1 Add `BuildPreview` cases in `control-plane/internal/artifacts/preview.go`: `CarouselDraft → markdown_preview` (slides as heading/body sections, referenced images inline) and `ImageAsset → image_preview` (`ImagePreview` from `storage_uri`/inline fallback + alt text). Verify: `cd control-plane && go test ./internal/artifacts/...`. (CarouselDraft case done; `ImageAsset → image_preview` deferred to the image-storage slice — finding F5, no PayloadStore presign primitive exists yet. The carousel markdown preview acknowledges `image_artifact_id` refs without resolving them, per decision 5.)
- [x] 2.2 Map the two new artifact-type keys to the existing `markdown`/`image` kinds in `frontend/src/lib/artifacts/preview.ts` (kinds already exist). Verify: `cd frontend && bunx vitest run src/lib/artifacts/preview.test.ts`. (Slice J2: `CarouselDraft` added to the renamed `PRIMARY_PREVIEW_ARTIFACT_TYPES` so a completed carousel surfaces as the primary preview; the proto oneof already maps CarouselDraft→markdown / ImageAsset→image, so no kind-map change was needed.)

## 3. Executors (SKUs + Agent Manifests)

- [x] 3.1 Add SKU keys `linkedin-carousel-senior` and `image-asset-generator` to `control-plane/internal/executors/catalog/catalog.go` and seed rows (input/output artifact type keys, executor kind) in `control-plane/internal/executors/seed.go`. Verify: `cd control-plane && go test ./internal/executors/...`.
- [x] 3.2 Add the carousel agent manifest `agents/linkedin-carousel-senior/0.1.0.yaml` (capability `linkedin-carousel-authoring`, `artifact_input_type: TextDraft`, `artifact_output_type: CarouselDraft`, output schema = slides). Verify: manifest loads at agent-runtime boot; `cd agent-runtime && ruff check src/`. (Slice D: manifest + `linkedin_carousel.py` runner + registry/worker wiring; 6 carousel tests + 16-test regression sweep pass.)
- [ ] 3.3 Add a new version of `agents/linkedin-voice-senior/*.yaml` extending the `system_prompt` with hook construction + an anti-AI-jargon rubric and accepting the new voice input fields; keep the SKU key `linkedin-voice-senior`. Verify: manifest loads; existing adapt-for-linkedin path still produces `LinkedInPostDraft`. (DEFERRED: quality improvement to the text branch, not blocking. The `hook_style`/`avoid_ai_jargon`/`voice_rules` inputs are collected into `ContentPreferences` but not yet consumed by the 0.1.0 voice manifest. Bumping the manifest version ripples through 3 hand-synced pins (loader default, SKU `ManifestVersion`, installations) — deferred to avoid that churn until the prompt quality is validated.)
- [ ] 3.4 Wire the `image-asset-generator` integration executor so provider/model/brand/credentials are read from the `ExecutorInstallation` config (never template/artifact). Verify: `cd control-plane && go test ./internal/executors/...`. (DEFERRED with H/I — the image path needs a real provider integration + the PayloadStore presign primitive; see task 2.1 / finding F5.)

## 4. PlanBehaviorPolicies + Template Variant

- [x] 4.1 Add the `content_output_format` field to `PlanBehaviorPolicies` in `proto/harpia/plans/v1/plans.proto` (enum: text_post / carousel / image_backed / approval_only). Decide `avoid_ai_jargon` typing per design open question #4. Verify: `cd proto && buf lint`. (Field added in Slice A; behavior-policy whitelist + materialize registered in Slice E. `avoid_ai_jargon` resolved to a two-option SELECT — decision 4, no proto BOOL add.)
- [x] 4.2 Author `control-plane/internal/plans/templates/linkedin-content-studio.yaml`: shared `fetch-news`/`write-draft` head; `adapt-for-linkedin`, `draft-carousel`, `generate-image`, and `publish-linkedin` steps; `output_format` input mapped via `BEHAVIOR_POLICY` to `content_output_format`; `hook_style`/`avoid_ai_jargon`/`voice_rules` mapped via `SEED_ARTIFACT` into `ContentPreferences`. Verify: `cd control-plane && go test ./internal/plans/...` (template seed/validation tests). (Slice F: authored as a branching DAG with three format-specific publish steps — publish-post/publish-carousel/publish-image — per the resolved OQ2; voice rules mapped to write-draft ContentPreferences alongside tone, consistent with the existing pattern.)
- [x] 4.3 Add a test that validates the real embedded `linkedin-content-studio.yaml` (not synthetic YAML) against the seeder. Verify: `cd control-plane && go test ./internal/plans/...`. (Slice F: `TestEmbeddedPlanTemplatesLoadLinkedInContentStudio` loads the real embedded template via `embeddedPlanTemplateFiles()` and asserts 8 steps / 7 edges / 3 branches at write-draft.)

## 5. Execution Engine (Format Branch Selection)

- [x] 5.1 Implement conditional branch selection in `control-plane/internal/workflow/` + `control-plane/internal/plans/`: skip output steps whose format is not the run's `content_output_format`. Verify: `cd control-plane && go test ./internal/workflow/... ./internal/plans/...`. (Slice G: format inferred from artifact types — no `format_filter` field; `STEP_EXECUTION_STATUS_SKIPPED` added; loop-top skip before retry fast-path; retry planner tolerates SKIPPED upstream; 4 workflow scenarios + retry test; backward-compat (UNSPECIFIED→run-all) confirmed; whole control-plane suite green.)
- [x] 5.2 Ensure the publish step consumes the selected branch's artifact per the design typed-contract decision (open question #2). Verify: `cd control-plane && go test ./internal/workflow/...`. (Three format-specific publish steps — publish-post/publish-carousel/publish-image — each a single typed contract, selected by the same skip mechanism; resolved OQ2.)
- [x] 5.3 Route the publish gate through the existing approval-request lifecycle when `publish_approval_mode` requires approval, previewing the selected-format artifact. Verify: `cd control-plane && go test ./internal/plans/...` (approval path tests). (No new approval path needed — the existing `requiresPublishApproval` gate covers all three publish steps; skipped steps never reach the gate. CarouselDraft preview is wired (Slice B); ImageAsset preview deferred to the image-storage slice.)

## 6. Conversational Configuration + Localization

- [x] 6.1 Extend the conversational configuration assistant to ask for `output_format` and the voice rules (reuse existing `USER_SELECTION` / seed-input handling). Verify: `cd frontend && bunx vitest run <configuration flow test>`. (Slice J1: `content_output_format` added to the planassistant policies step — `orderedPolicyKeys`, the `allFields` builder, the `policyUnset` switch, and a `contentOutputFormatValue` helper; gated by `templateDeclaresPolicy` so non-declaring templates are unchanged. 3 new tests + backward-compat assertion. Voice-rule inputs surface via the existing template-input flow.)
- [x] 6.2 Add catalog keys `catalog.plan.linkedin-content-studio.*` (name, description, suggestion, step titles/descriptions) and `plans.inputs.<output_format|hook_style|avoid_ai_jargon|voice_rules>.*` to `frontend/src/lib/i18n/content/{en,pt-BR}.json` and `frontend/src/lib/i18n/{en,pt-BR}.json`. Verify: `cd frontend && bunx vitest run` (hardcoded-copy / locale coverage tests). (Slice J2: 84 new keys, 42 per locale; both locales have identical key sets; i18n parity + content tests pass.)
- [ ] 6.3 Ensure the in-thread approval card and canonical `ArtifactPreviewSheet` render carousel/image previews (no new panel state). Verify: `cd frontend && bunx vitest run src/lib/artifacts/preview.test.ts` and a manual smoke of the approval card. (Carousel preview works end-to-end — Slice B backend + Slice J2 frontend primary-preview surfacing. Image-preview path is deferred with H/I — no ImageAsset preview/storage primitive yet. Manual approval-card smoke remains.)

## 7. Verification

- [x] 7.1 Run `openspec validate rich-linkedin-content --strict`.
- [x] 7.2 Run `cd proto && buf lint`.
- [x] 7.3 Run `cd control-plane && go test ./...`.
- [x] 7.4 Run `cd frontend && bun run lint`, `cd frontend && bun run check` (no new baseline errors), and `cd frontend && bunx vitest run` for changed test files.
- [x] 7.5 Run `cd agent-runtime && ruff check src/`.
- [ ] 7.6 Manual smoke in `pt-BR` and `en`: configure `linkedin-content-studio`, pick each output format, confirm per-format preview before publish, and confirm in-thread approval when required. (Manual, agent-incapable. text_post + carousel are unit/integration-covered end-to-end; image_backed is deferred pending H/I.)
