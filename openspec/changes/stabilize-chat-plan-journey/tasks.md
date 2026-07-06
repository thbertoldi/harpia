## 1. Locale-Aware Catalog Rendering

- [x] 1.1 Add or extend helpers in `frontend/src/lib/catalog-i18n.ts` to resolve plan name/description, step title/description, and template input labels by locale with backend-string fallbacks.
- [x] 1.2 Add missing `catalog.plan.weekly-newsletter-linkedin.*`, step, action, and input keys to `frontend/src/lib/i18n/en.json` and `frontend/src/lib/i18n/pt-BR.json` so the weekly-newsletter LinkedIn journey renders fully in both locales.
- [x] 1.3 Update `frontend/src/lib/components/thread/PlanProposalCard.svelte` so proposal candidates, selected plan titles, confirmation copy, and post-create actions use localized plan/action labels.
- [x] 1.4 Update `frontend/src/lib/components/thread/BindingMatrixCard.svelte`, `TemplateInputsForm.svelte`, and related plan-summary helpers so configuration rows and input labels use localized catalog/input metadata.
- [x] 1.5 Update `frontend/src/lib/components/thread/PlanExecutionCard.svelte` and the execution view-model inputs so step titles/descriptions use localized catalog step metadata rather than raw template titles where the template key is known.
- [x] 1.6 Add focused frontend tests for locale resolution fallbacks and for the embedded `weekly-newsletter-linkedin` path in `en` and `pt-BR`.

## 2. Structured Chat Card Alignment

- [x] 2.1 Remove or make conditional the `max-w-[85%]` bubble-width styling in `frontend/src/lib/components/thread/PlanExecutionCard.svelte` so execution cards render at full structured-card width in the configured thread branch.
- [x] 2.2 Audit structured cards under `frontend/src/lib/components/thread/` and normalize wrapper classes so proposal, binding, policies, execution, artifact, and approval cards align visually in the conversation column.
- [x] 2.3 Add a focused regression assertion where feasible for the execution card class contract, or document the manual smoke assertion in the task completion notes if component-render tests are unavailable.

## 3. Canonical Artifact Preview Flow

- [x] 3.1 Rewire `frontend/src/routes/chat/[threadId]/+page.svelte` and `frontend/src/lib/components/thread/ConversationalWorkspace.svelte` so all artifact launchers use one canonical `ArtifactPreviewSheet` state (`activeArtifactId` plus loading/ready state).
- [x] 3.2 Remove the sticky artifact rail behavior that pins artifacts at the top of the viewport; artifact access should come from inline cards, produced-artifact launchers, and an optional lightweight reopen affordance.
- [x] 3.3 Ensure `STEP_BOUND` messages with `output_artifact_id` render or feed an inline artifact card close to the producing step without duplicating independent artifact panel state.
- [x] 3.4 Keep final artifact emphasis by default when a run completes, while allowing explicit user selection of intermediate artifacts for preview.
- [ ] 3.5 Add or update frontend tests for artifact preview kind mapping/panel state, and smoke-test a live run to confirm markdown/html/image previews still render in the canonical sheet.

## 4. In-Thread Approval And Inbox Context

- [ ] 4.1 Trace the current approval request lifecycle and payloads across control-plane, chat watch messages, `frontend/src/lib/inbox/aggregator.ts`, and `frontend/src/routes/inbox/+page.svelte`; decide whether existing payloads carry enough artifact/plan context or need backend enrichment.
- [ ] 4.2 If context is missing, enrich approval request/thread payload shaping in control-plane so pending approvals expose approval request id, plan/configuration id, execution id, artifact id/title, and a localized-safe summary source.
- [ ] 4.3 Update `frontend/src/lib/inbox/aggregator.ts` and `frontend/src/lib/components/inbox/InboxRow.svelte` so inbox approval rows show meaningful localized context and link back to the origin chat thread.
- [ ] 4.4 Implement or complete `frontend/src/lib/components/thread/ApprovalRefCard.svelte` so pending approvals render in the chat thread with preview, approve, and reject actions.
- [ ] 4.5 Wire approval/rejection actions from the chat card through the same API path used by inbox decisions, then refresh/invalidate both thread and inbox state after a decision.
- [ ] 4.6 Add tests for approval aggregation, approval-card view-model/action behavior, and pending/resolved synchronization. Add backend tests if task 4.2 changes control-plane payloads.

## 5. Context-Preserving Navigation

- [ ] 5.1 Update inbox approval links so selecting an approval navigates to `/chat/[threadId]` with enough query/hash/state to select the relevant plan/execution/approval context.
- [ ] 5.2 Update `/runs` execution links, if needed, so opening a run selects the origin thread, active plan, and execution context rather than landing on an ambiguous thread state.
- [ ] 5.3 Add tests for URL/context derivation helpers or route-state handling used by inbox/runs links.

## 6. Verification

- [x] 6.1 Run `openspec validate --changes stabilize-chat-plan-journey`.
- [x] 6.2 Run focused frontend tests with `cd frontend && bunx vitest run <changed test files>`.
- [x] 6.3 Run `cd frontend && bun run lint`.
- [x] 6.4 Run `cd frontend && bun run check` and confirm only the known baseline diagnostics remain.
- [x] 6.5 If backend files changed, run `cd control-plane && go test ./...`.
- [ ] 6.6 Manual smoke: in `pt-BR`, create the weekly-newsletter LinkedIn plan, make it RUNNABLE, click run now, confirm execution card width, localized plan/step/action labels, inline artifact preview behavior, and in-thread/inbox approval behavior.

## 7. Follow-Up: Rich LinkedIn Plan Content

- [ ] 7.1 Create a separate OpenSpec exploration/proposal for a richer LinkedIn content PlanTemplate that can refine voice, anti-AI-jargon rules, post format choices, carousel drafts, image generation, and preview variants.
- [ ] 7.2 In that follow-up, decide whether new artifact fields/types are needed for carousel/image outputs and whether new ExecutorSKUs or agent manifests are required.
