# Harpia MVP Roadmap

**Companion to the [Platform Constitution](harpia-platform.md).** This document turns the
constitution's **content-depth-first** scope decision ([§12](harpia-platform.md#12-mvp-scope))
into a sequenced build with concrete deliverables per phase.

**Last updated:** 2026-07-07 · **Target:** Aiuna Growth Front demo, ~2026-08-22 (≈6 weeks).

> Each phase below is written to be **executor-cold**: a phase graduates into one or more
> OpenSpec changes (`openspec/`), and each change carries self-contained tasks with file
> paths and per-task verification. This roadmap is the index; the OpenSpec changes are the
> briefs.

---

## 1. Current reality (verified 2026-07-07)

The platform is a working **plan → executor → artifact → approval engine** proven on a
single creator-economy content chain. Everything that exists:

| Layer | What exists |
|---|---|
| **PlanTemplates (2)** | `news-digest-draft` (fetch-news → write-draft); `weekly-newsletter-linkedin` (fetch-news → write-draft → adapt-for-linkedin → publish-linkedin). |
| **ExecutorSKUs (4)** | `rss-news-feed` (int), `linkedin-publish` (int), `newsletter-writer-senior` (agent), `linkedin-voice-senior` (agent). |
| **ArtifactTypes (5)** | `DateRange`, `NewsList`, `TextDraft`, `LinkedInPostDraft`, `PublishConfirmation`. |
| **Agents (2)** | Single-node LangGraphs, `allowed_tool_ids: []`, model `deepseek-v4-flash`. No tool use, no multi-step reasoning. |
| **Integrations** | RSS (4 seeded feed-group presets) + LinkedIn (dev sandbox, `approval_only`). |
| **Data model** | plans/executions/steps, artifacts, executors, threads, chat, approvals, elicitations, budget/usage. **No** CRM, offer, opportunity, goal, KB, dashboard, or brand-profile tables. |

**PRD coverage:** 0 of 34 fully, 4 partially (N07 approval, N08 autonomy policy, N12 action
log, M04 LinkedIn text only), 30 not addressable. See the constitution's scope section for
the deferral list.

---

## 2. Build sequence (overview)

```text
Phase 0  Cleanup & correctness        (unblocks everything; retire contradictions/dead code)
Phase 1  Resource layer               (brand kit / company profile; the shared layer)
Phase 2  Browser surfaces             (recent Conversations + Artifact Library)
Phase 3  Rich content production      (carousels, images, elicitation, adversarial review, single preview)
```

Phase 1 is the keystone; Phase 3 is the visible demo and *consumes* Phase 1. Phase 2 is
independent and low-risk (can slot in parallel). Phase 0 comes first because the
thread-resolver bug and the LinkedIn-hardcoded input model actively block clean work.

---

## 3. Phase 0 — Cleanup & correctness

**Goal:** trunk is clean, single-sourced, and free of the contradictions that make the next
phases risky. Full detail + `file:line` evidence in the **[Cleanup Backlog](cleanup-backlog.md)**.

Ordered work (→ OpenSpec change `phase0-consolidation-cleanup`):
1. **C1 (bug):** collapse the two owning-thread resolvers into one; delete the config-id
   fallback. *Verify:* runtime RUN_*/STEP_* events and approval/elicitation events for one
   configuration land on the same thread in an e2e run.
2. **D1:** delete the orphaned PlanCanvas cluster (~8 components + `canvas-state.ts` +
   SettingsDrawer/CanvasDrawer + tests). *Verify:* `bun run build` + `bun run check` (no new
   baseline errors); grep shows zero dangling imports.
3. **C2/DUP1:** drive `BindingMatrixCard` from the generic `TemplateInputParameter[]`;
   delete the LinkedIn-hardcoded input model. *Verify:* configuring `weekly-newsletter-linkedin`
   still works; a second template's inputs render with no code change.
4. **D2/D3/D4:** delete orphaned `ArtifactRail`/`TextArtifactEditor`, dead e2e
   plan-thread mock branches, and stale proto/doc comments.

**Also in Phase 0 (doc-level, this effort):** proto `subtask_*` → `step_*` renames (C4,
pre-v1 free), AGENTS.md wording fixes, ADR banners. Tracked in the backlog.

---

## 4. Phase 1 — Resource layer (the shared layer)

**Goal:** a tenant can create, upload, and version **Resources** (brand kit, company/brand
profile), and content agents consume them. Delivers PRD **N01** and the cross-plan-reuse
backbone. Implements constitution [§8](harpia-platform.md#8-the-resource-layer-the-shared-layer).

**New domain / data:**
- `resources` model: id, tenant/workspace/brand/user scope, `resource_type`, version,
  provenance (origin PlanExecution/StepExecution/artifact/approver), permissions, backing
  artifact ref. Reuses Garage tenant-safe storage.
- Proto: generalize ADR-014's `MemoryResource` into `Resource` in `harpia.artifacts.v1`;
  both creation paths — `CreateResource`/`UploadResourceVersion` (direct upload/authoring)
  and `PromoteArtifactToResource` (from a plan output) — plus `ListResources`, `GetResource`,
  and resource-version RPCs.

**New ArtifactTypes:** `CompanyProfile` (N01: tone of voice, audience, services,
differentiators, communication limits), `BrandKit` (logos, colors, fonts references,
do/don't), `DesignSystem` (optional, later). All JSON Schema.

**New executor / agent capability:** a memory-backed tool (MCP or control-plane adapter)
that lets an AgentExecutor read bound Resources, declared in `allowed_tool_ids` and narrowed
by `MemoryBinding`; wire `MemoryUsageRecord` audit.

**Binding:** support `MemoryBinding` on PlanConfiguration/ExecutorInstallation and
`SeedArtifactBinding` from a Resource; freeze resolved bindings into the execution snapshot.

**Two creation paths (both governed):**
- **Direct upload/authoring** — the *primary* path for branding the user feeds the platform
  (upload a brand kit, author a company profile). Validated against the Resource type schema,
  versioned, permissioned, Área-gated, audited; provenance `user-upload`.
- **Promotion from a plan output** — "Save as Resource" on an artifact, controlled
  (human-approved), with provenance to the origin run; not silent accumulation.

**OpenSpec changes:** `resource-model-and-promotion`, `brand-kit-resources`,
`agent-resource-consumption`.

**Verify:** create a CompanyProfile Resource; configure `weekly-newsletter-linkedin` with a
MemoryBinding to it; run; `MemoryUsageRecord` shows the writer agent read it and the draft
reflects the brand voice.

---

## 5. Phase 2 — Browser surfaces

**Goal:** the two "browsers over streams" the user asked for. Independent, low-risk;
reuses existing streams.

**Recent Conversations:** a Home/sidebar surface listing recent Threads with status
(running / needs-attention / completed / archived), resume in one click, and search.
Reuses `ThreadService.ListThreads`; add title/status derivation and a search filter.

**Artifact Library (`/artifacts`):** browse and **search** the artifact stream (by type,
plan, date, thread), open the canonical `ArtifactPreviewSheet`, and **promote to Resource**
(the Phase 1 action surfaced here). This is also the Resource management surface — one
browser, per constitution §8.

**OpenSpec changes:** `recent-conversations`, `artifact-library`.

**Verify:** after several runs, the library lists and filters artifacts; the recent-chats
surface resumes a thread; promoting an artifact creates a Resource visible in the same
surface.

---

## 6. Phase 3 — Rich content production (the demo)

**Goal:** turn the LinkedIn generator into a genuine creator workflow — the visible demo.
Delivers PRD **M04** depth (+ **M02/M03** partials). Consumes Phase 1 Resources so output
is brand-aware. Implements the product-idea log's "Rich LinkedIn content plan variants."

**Capabilities to add:**
1. **Format choice + carousels.** During conversational configuration, the assistant asks
   for the output shape (text post / carousel / image-backed). New ArtifactType
   `CarouselDeck` (ordered slides: heading + body + optional image ref). New agent step
   `adapt-for-carousel: TextDraft → CarouselDeck`.
2. **Images.** New ArtifactType `ImageAsset` and an image-generation **ExecutorSKU**
   (integration) whose installation carries provider/config. New optional step
   `generate-image: (brief|CarouselDeck) → ImageAsset`. (Evaluate the "Creator Kit"
   deterministic-renderer direction from the idea log for brand-safe assets.)
3. **Stronger elicitation.** Generalize the writer/adapter elicitation beyond
   tone/topics: hook strength, anti-AI-jargon rules, audience, format — asked in-thread.
4. **Adversarial review inside the graph.** Add a reviewer/critic node to the content agent
   graph (Reflection/Actor-Critic pattern): draft → adversarial critique → revise, before
   the artifact is surfaced. Keep it inside the agent step (one StepExecution), bounded, not
   a new human gate.
5. **One single preview artifact.** Consolidate the final output into a single previewable
   artifact (text + carousel + image assembled) so the thread's side preview shows exactly
   what will publish — no scattered intermediates.

**New/changed:** ArtifactTypes `CarouselDeck`, `ImageAsset`, a composite `ContentPreview`;
ExecutorSKU for image gen; a richer content agent manifest (multi-node graph with a critic);
a new/upgraded template `linkedin-content-studio`.

**OpenSpec changes:** `content-format-and-carousel`, `image-generation-executor`,
`adversarial-review-node`, `unified-content-preview`, `rich-elicitation`.

**Verify:** from chat, request a carousel about a topic in pt-BR; the plan elicits format +
hook; the graph self-critiques and revises; a single preview shows the branded carousel
(using the Phase 1 brand kit); approval publishes to the LinkedIn sandbox.

---

## 7. Deferred (post-MVP)

Kept out of the 6-week window, with rationale in the constitution: all Sales plans
(V02–V08), CRM/customer/history (N02/N03), offer catalog as a full plan (N04), pipeline
(N05), reporting/dashboards/indicators (N09/N10/M08/V08), paid media (M07), strategic
diagnosis agents (M01), multi-channel publishing (Instagram/blog/email/X), full
catalog-authoring service + marketplace, the **credit/subscription monetization layer**
(constitution §14), overseer re-delegation, and adaptive→PlanExecution migration. Each
becomes its own spec → plan cycle after the MVP demo. The credit layer's hooks (Budget
capability, SKU price metadata, approval-card cost summary) already exist, so nothing in the
MVP is blocked by deferring it.

---

## 8. Cross-cutting reminders

- Every phase ships `en` + `pt-BR` copy and catalog localization keys.
- New templates need new ExecutorSKUs/ArtifactTypes — recombining the existing four is not
  enough (constitution §13).
- Add a test that validates the **real embedded template YAML** at boot (a typo currently
  fails only at API startup).
- Wire the classifier / new agents to tenant LLM budget controls before heavy use.
- Keep feed/OAuth/renderer config on ExecutorInstallation; keep outputs in the Artifact
  stream; gate everything by Áreas.
