## 1. Tiered, multi-capable agent catalog (D1)

- [ ] 1.1 Add a **seniority tier** field to the agent/SKU model (proto `ExecutorSKU`/manifest metadata: `JUNIOR|PLENO|SENIOR`) + an explicit **capabilities** list on the SKU (manifest already has `capabilities[]`; surface it on the SKU compatibility metadata). Regenerate. Verify: `cd proto && buf lint`.
- [ ] 1.2 Consolidate the narrow SKUs: replace `linkedin-voice-senior` + `linkedin-carousel-senior` with a **LinkedIn Content Specialist** role carrying `{linkedin-content-adaptation, carousel-authoring, tone-matching, anti-ai-jargon}`, in 3 tiers (Jr/Pleno/Sr); keep the News Writer and add an **Image Generator** role (opt-in, 1+ tier). Update `executors.DefaultCatalogSeeds` (Go seeder, §13). Verify: `cd control-plane && go test ./internal/executors/...`.
- [ ] 1.3 Author the consolidated agent manifests under `agents/<role>/<tier>.yaml` (3 tiers for the specialist, reusing the existing voice + carousel graphs as capabilities; the Sr specialist adapts text AND authors carousels). Update the agent-runtime loaders/registry/worker dispatch. Verify: `cd agent-runtime && ruff check src/` + the registry smoke + agent tests.

## 2. Capability-based requirements + opt-in (D2, D4)

- [ ] 2.1 Mark capabilities as **opt-in vs required** (e.g. `image-generation` opt-in) on the manifest/template; thread an **included-capabilities** signal onto the PlanConfiguration (which opt-ins the user selected). Proto + regenerate. Verify: `cd proto && buf lint`.
- [ ] 2.2 Update the configuration assistant so a step requires a binding ONLY if its required capability is in the run's included set; opted-out steps are excluded (need no binding). This is the configuration-blocker fix. Verify: `cd control-plane && go test ./internal/planassistant/...`.
- [ ] 2.3 Update the RUNNABLE invariant (constitution §7.1): a configuration is RUNNABLE iff every step **that will run** is bound. Verify with a planassistant test that an opted-out image step does not block RUNNABLE.

## 3. Team recommendation (D3)

- [ ] 3.1 Implement `agent-team-recommendation` (control-plane): given a plan's steps + included capabilities, compute a minimal covering set of role × tier agents (fewest agents, default tier per role), respecting entitlements/installations and flagging gaps. Verify: `cd control-plane && go test` with unit tests for cover/flag/swap.
- [ ] 3.2 Expose the recommendation to the configuration flow (a prompt/card: "recommended team" + accept/swap). Verify: `cd control-plane && go test ./internal/planassistant/...`.

## 4. Unified content post + single publish (D5)

- [ ] 4.1 Define a composable **LinkedInPost** artifact (text body + optional `CarouselDraft` + optional `ImageAsset`(s)) in proto; validation + preview (text+carousel markdown; image when present). Reuse the existing `CarouselDraft`/`ImageAsset` payloads. Verify: `cd control-plane && go test ./internal/artifacts/...`.
- [ ] 4.2 Collapse the per-format publish steps into **one publish** that consumes the unified post; remove `publish-post`/`publish-carousel`. Verify: `cd control-plane && go test ./internal/plans/... ./internal/workflow/...`.

## 5. Engine: opt-out-capability skip (replaces format skip) (D6)

- [ ] 5.1 Replace `shouldSkipStepForFormat` (artifact-type inference) with **opt-out-capability skipping**: a step is skipped when its required capability was opted out. Reuse the SKIPPED status + retry-aware logic (already built). Verify: `cd control-plane && go test ./internal/workflow/... ./internal/plans/...`.
- [ ] 5.2 Retire/repurpose the `content_output_format` behavior-policy branch selector (pre-v1 break). Verify build + tests.

## 6. Re-author `linkedin-content-studio` on the model (D6)

- [ ] 6.1 Rewrite `linkedin-content-studio.yaml`: shared fetch-news → write-draft head; a single LinkedIn Content Specialist step that produces the unified post (text, +carousel when chosen); ONE publish; image-generation as an opt-in capability step. Verify: `cd control-plane && go test ./internal/plans/...` (real embedded template seed test).
- [ ] 6.2 Update i18n (en + pt-BR) for the revised plan/steps/inputs (role names, tiers, opt-in image, unified post). Verify: `cd frontend && bun run lint && bun run check && bunx vitest run src/lib/i18n/`.

## 7. Frontend: choose-your-team + opt-in + unified preview

- [ ] 7.1 A "choose your team" surface: recommended team (role × tier) with accept/swap; binding driven by capability coverage. Verify: `cd frontend && bun run check` + component/view-model tests.
- [ ] 7.2 Configuration asks which opt-in capabilities to include (e.g. "generated images?"); opted-out steps hidden/excluded. Verify: `cd frontend && bunx vitest run`.
- [ ] 7.3 Unified post preview (text + carousel markdown; image when present). Verify: `cd frontend && bunx vitest run src/lib/artifacts/preview.test.ts`.

## 8. Verification

- [ ] 8.1 `openspec validate agent-team-model --strict`.
- [ ] 8.2 `cd proto && buf lint`; `cd control-plane && go test ./...`; `cd agent-runtime && ruff check src/`; `cd frontend && bun run lint && bun run check`.
- [ ] 8.3 Manual smoke (en + pt-BR): configure `linkedin-content-studio`, pick text-only (no image) → RUNNABLE → run; then opt into images → team includes the Image Generator → run with image content in the post.
