## 1. Tiered, multi-capable agent catalog (D1)

- [x] 1.1 Add a **seniority tier** field to the agent/SKU model + explicit **capabilities** on the SKU (proto `SeniorityTier` + `CompatibilityMetadata.capabilities`/`.tier`). (Slice 1 — `a3d86c6`.)
- [x] 1.2 Consolidate the narrow SKUs into a **LinkedIn Content Specialist** role carrying `{linkedin-content-adaptation, carousel-authoring}`, multi-capable; seed it (Sênior). (Slice 2c — `ef8190a`. **Jr/Pleno tiers deferred** — Sênior only for now; Jr/Pleno are quick manifest variants once the prompt/cost tiers are decided.)
- [x] 1.3 Multi-capability manifest model (`AgentCapability` + `capability_specs`) + the specialist manifest + capability-driven runner + worker dispatch. (Slices 2a/2b — `7dc8713`, `09fa26a`.)

## 2. Capability-based requirements + opt-in (D2, D4)

- [x] 2.1 Opt-in signal `PlanConfiguration.included_optional_capabilities` + `TEMPLATE_INPUT_RUNTIME_TARGET_INCLUDED_CAPABILITY` mapping. (Slices 1 + 6 — `a3d86c6`, `287aac6`.)
- [x] 2.2 A step requires a binding ONLY if its capability is included; opted-out steps excluded. (Slice 5 — `99d53c0`.)
- [x] 2.3 RUNNABLE invariant: a configuration is RUNNABLE iff every step **that will run** is bound (opted-out steps ignored). (Slice 5 — `99d53c0`.)

## 3. Team recommendation (D3)

- [x] 3.1 `RecommendTeam` (greedy set-cover → minimal team, senior-tier preference, uncovered-gap flagging) + tests. (Slice 3 — `5cc16f9`.)
- [ ] 3.2 Surface the recommendation to the configuration flow (a "recommended team" card: accept / swap tier or agent). **DEFERRED** to follow-up change: `agent-team-deferred-followups` — the algorithm exists; the conversational/UI surfacing is out of scope here (binding works manually via the existing per-step flow).

## 4. Unified content post + single publish (D5)

- [ ] 4.1 Composable `LinkedInPost` artifact (text + optional carousel + optional images). **DEFERRED** to follow-up change: `agent-team-deferred-followups` — the first cut ships a single `publish` step over `LinkedInPostDraft` (Slice 6); carousel/image are produced as separate opt-in artifacts (previewable). The full composable post is out of scope here.
- [x] 4.2 Collapse per-format publish steps into **one publish**. (Slice 6 — `287aac6`: one `publish` step over LinkedInPostDraft.)

## 5. Engine: opt-out-capability skip (replaces format skip) (D6)

- [x] 5.1 `shouldSkipStepForOptOut` (a step is skipped when an `optional_capability` isn't included); SKIPPED status + retry-aware reused. (Slice 5 — `99d53c0`.)
- [x] 5.2 The `content_output_format` branch selector is no longer used by `linkedin-content-studio` (re-authored in Slice 6); the legacy `shouldSkipStepForFormat` path is inert (UNSPECIFIED → no skip) and kept for safety until cleanup. (Slice 6 — `287aac6`.)

## 6. Re-author `linkedin-content-studio` on the model (D6)

- [x] 6.1 Rewrite the template: specialist `author-content` → single `publish`; `draft-carousel` + `generate-image` opt-in via `optional_capabilities`; `include_carousel`/`include_images` SELECTs drive the opt-in. (Slice 6 — `287aac6`.)
- [x] 6.2 i18n (en + pt-BR) for the revised plan/steps/inputs. (Slice 7 — `a8ea95e`.)

## 7. Frontend: choose-your-team + opt-in + unified preview

- [ ] 7.1 A "choose your team" surface (recommended team, accept/swap). **DEFERRED** to follow-up change: `agent-team-deferred-followups` — see 3.2; the recommendation logic exists, the UI is out of scope here.
- [x] 7.2 Opt-in SELECTs (`include_carousel`/`include_images`) render via the existing template-input form and drive `included_optional_capabilities`. (Slices 6 + 7.)
- [x] 7.3 Carousel preview (markdown-as-slides) works (`CarouselDraft → markdown_preview`, primary-preview list). (Slices B + J2 — earlier; unchanged.)

## 8. Verification

- [x] 8.1 `openspec validate agent-team-model --strict` (see below).
- [x] 8.2 All gates green: `cd proto && buf lint`; `cd control-plane && go test ./...` (19 pkgs); `cd agent-runtime && ruff check src/` + 80 pytest; `cd frontend && bun run lint && bun run check` (0/0).
- [x] 8.3 Automated acceptance: configure `linkedin-content-studio`, opt carousel/images in/out, confirm RUNNABLE invariant honors opt-out. Covered by `mise run acceptance` scenario `TestAcceptanceLinkedInOptionalCapabilities` (change `enforce-acceptance-before-archive`), which materializes the real template and asserts opted-out steps neither bind nor block RUNNABLE, while opted-in steps do.
