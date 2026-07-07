# Harpia Cleanup Backlog

**Companion to the [Platform Constitution](harpia-platform.md) and [MVP Roadmap](mvp-roadmap.md).**
This is the prioritized inventory of contradictions, duplication, and dead code to retire —
the substance of **[Phase 0](mvp-roadmap.md#3-phase-0--cleanup--correctness)**.

**Last updated:** 2026-07-07 · Verified against trunk with `file:line` evidence.

**Suggested order:** C1 (correctness) → D1 (largest dead cluster) → C2 (blocks
multi-template) → D2/D3/D4 → doc-level fixes.

---

## Code findings

### C1 — HIGH — Two incompatible resolvers for a PlanConfiguration's owning thread
The owning thread is resolved two ways that read different columns with different fallbacks,
so runtime events and assistant/approval/elicitation events for the **same** configuration
can land on **different** thread IDs. Masked today only because both columns get the same
value at creation.

- `control-plane/internal/plans/handler.go:1368` — `configurationThreadID()` returns
  `OriginThreadID` else `ThreadID`.
- `control-plane/internal/plans/runtime.go:33-43` — `resolveThreadID()` →
  `GetThreadIDForConfiguration()` selects `thread_id` only
  (`repository.go:586`) and falls back to `configID.String()` (legacy config-id-as-thread).
- Consumers diverge: assistant/selection/approval/elicitation use `configurationThreadID`
  (`handler.go:604`, `approvals_handler.go:156`, `elicitations_handler.go:153`); runtime
  RUN_*/STEP_* use `resolveThreadID` (`runtime.go:125,166,208,289,362,438`).
- Two proto fields back it: `proto/harpia/plans/v1/plans.proto:148` `thread_id`, `:150`
  `origin_thread_id`; both assigned the same value at `handler.go:336-337`.

**Category:** contradiction · **Fix:** collapse to a single owning-thread field + single
resolver; delete the config-id fallback; route all callers through one path.

### D1 — HIGH — Entire PlanCanvas DAG subsystem is orphaned
The visual plan-canvas UI — including its own artifact-detail/approval/elicitation panel
state machine (the "second preview-owning path") — is never mounted.

- `PlanCanvas.svelte` has **0 importers**. Its exclusive children are therefore dead:
  `PlanCanvasNode.svelte`, `PlanCanvasEdge.svelte`, `PlanCanvasDetailPane.svelte`,
  `NodeHoverCard.svelte`, `CanvasElicitationForm.svelte`.
- `frontend/src/lib/plans/canvas-state.ts` — imported only by those dead components.
- `SettingsDrawer.svelte` (0 importers) + its exclusive child `CanvasDrawer.svelte` — dead.
- **Keep** `CanvasApprovalForm.svelte` and `ScheduleDialog.svelte` — both alive
  (`inbox/InboxApprovalEntry.svelte`; 4 importers).

**Category:** dead-code · **Fix:** delete the cluster + `canvas-state.ts` +
SettingsDrawer/CanvasDrawer + tests. The chat page's `previewOpen`/`activeArtifactId`
machine (`routes/chat/[threadId]/+page.svelte:156-252`) becomes the single artifact-preview
state machine.

### C2 / DUP1 — MEDIUM — LinkedIn-hardcoded template inputs duplicate the server materializer
The server expands arbitrary template inputs generically, but the frontend carries a
parallel LinkedIn-specific model hardcoding keys/defaults/serialization for one template —
blocking any second template and risking FE/BE key drift.

- Server generic path: `control-plane/internal/plans/materialize.go:30`
  `MaterializePlanConfiguration()` iterates `InputParameters`/`RuntimeMappings`.
- Frontend already has a generic form: `frontend/src/lib/components/thread/TemplateInputsForm.svelte`
  (used by `PlanProposalCard.svelte:581`).
- **But** the duplicate hardcoded path: `frontend/src/lib/plans/template-inputs.ts:170-216`,
  `frontend/src/lib/plans/linkedin-template-inputs.ts:17,74`,
  `frontend/src/lib/plans/assistant.ts:174`, consumed by
  `frontend/src/lib/components/thread/BindingMatrixCard.svelte` (via `ThreadMessage.svelte:128`).

**Category:** duplication + contradiction · **Fix:** drive `BindingMatrixCard` from the
generic `TemplateInputParameter[]`; delete the LinkedIn-specific input model; the server
owns materialization.

### D2 — MEDIUM — Orphaned artifact components
`frontend/src/lib/components/artifacts/ArtifactRail.svelte` (0 importers) and
`TextArtifactEditor.svelte` (0 references). **Fix:** delete both + tests. (For contrast,
`ArtifactPreviewSheet` and `ArtifactCard` are live and canonical.)

### D3 — MEDIUM — Dead legacy plan-thread RPC mocks in e2e
`frontend/e2e/engineer-journey.spec.ts:724,732,737` still mock `ListPlanThreadMessages` /
`WatchPlanThreadMessages` / `AppendPlanThreadMessage`, which no longer exist in proto or
backend (`ThreadService` only has `List/Watch/AppendThreadMessages`). Live branches at
`:829,837,842` already cover the real RPCs. **Fix:** delete the three dead branches.

### D4 — LOW — Stale proto/doc comments referencing deleted RPCs
`proto/harpia/chat/v1/chat.proto:55,42-44` reference `WatchPlanThreadMessages` / "legacy
plan-thread RPCs" (mirrored in generated `chat.pb.go:376`, `chat_pb.ts:143`). **Fix:**
reword to `WatchThreadMessages`; drop the legacy note.

---

## Doc- & schema-level findings (from ADR reconciliation)

These are resolved *in text* by the constitution; the residual work is code/proto/wording.

### C4 — Legacy `subtask_*` nouns in new protos
ADR-012 deprecated "subtask," but `harpia.budget.v1` and `harpia.mcp.v1` still ship
`subtask_key` / `SubtaskCostEstimate` / `subtask` references (ADR-010, ADR-011). **Fix
(pre-v1, free to break):** rename to `step_*` / `StepCostEstimate` for consistency with the
canonical taxonomy.

### AGENTS.md wording
- "the chat interface is a PlanConfiguration assistant" reads singular/1:1 → update to the
  1:N conversation framing (constitution §9).
- "Design tokens (colors, fonts) are LOCKED" → clarify to **fonts locked, color/depth
  governed by the Harpy Eclipse token system** (constitution §10).
- Add the pointer to `docs/architecture/harpia-platform.md` as source of truth.

### ADR stamping
Add a "superseded-by-constitution" banner to each ADR header and create
`docs/adr/README.md` with the supersession map (constitution §14).

---

## Verified non-findings (checked, not flagged)
- `ArtifactPreviewSheet` ↔ `ArtifactPreview` is **not** duplication: the sheet
  `bind:`-delegates fetch state to the single child fetcher `ArtifactPreview.svelte`.
- Server Path B (thread-first) migration is otherwise complete; no config-id-keyed message
  store remains besides the C1 resolver split.
- `agent-runtime` `linkedin_voice.py` / `newsletter_writer.py` are legitimately distinct
  agents behind a registry — not duplication.
