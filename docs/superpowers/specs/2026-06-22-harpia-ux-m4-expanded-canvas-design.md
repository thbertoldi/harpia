# Harpia UX Realignment — M4 Expanded Canvas — Implementation Design Spec

**Date:** 2026-06-22
**Status:** Approved (brainstorm), awaiting plan
**Originating conversation:** Brainstorming session 2026-06-22 between Thiago and Claude.
**Extends:** `docs/superpowers/specs/2026-06-19-harpia-ux-realignment-design.md` (master design)

This spec resolves M4's implementation-level decisions. The user-facing UX shape is settled in the master spec — §5.3 (expanded canvas), §6 (route disposition), §9 (v1/v2 split). This document captures the route shape, real-time data source, right-pane behavior, drawer scope, and component decomposition needed to write the M4 implementation plan.

---

## 1. Scope

M4 ships the **expanded canvas** — a full-bleed graph view of a single PlanExecution rendered from the same routing namespace M3 introduced. Per master spec §5.3 it carries:

- A grid-backgrounded DAG of the plan's tasks (one node per step)
- Per-node state badges (`pending`, `running`, `awaiting elicitation`, `awaiting approval`, `done`, `failed`)
- Per-node executor and overseer indicators
- Edges labeled with the contract type (artifact type) that rides them
- A right-side detail pane (340px) that fills with the selected node's details — executor, runtime stats, input/output artifact links, and an **inline answer form** when the node is awaiting an elicitation or approval
- A top bar with `← Back to thread`, `Run history`, `Schedule`, `Settings`, and a primary `Answer ★ N` action when there are pending Human Interaction events
- Deep-link entry from notifications and from the plan thread's inline DAG mini-map

The canvas is the **operational surface** for a running plan. Elicitations and approvals are answered here (and from the inbox), not by navigating to standalone detail pages.

**M4 is universal** — like M2 and M3, not flag-gated. After it ships, the legacy `/plans/executions/[executionId]/+page.svelte` timeline view is replaced; that URL becomes a 302 redirect to the canonical canvas URL.

---

## 2. Resolved decisions

### 2.1 Route shape — `/plans/configurations/[configurationId]/canvas?run=<execId>`

The canvas extends M3's `/plans/configurations/[configId]/` namespace rather than the legacy `/plans/executions/[id]/`. The `?run=<execId>` query is **optional**:

- Without `?run=` — renders the configuration's static DAG (no execution state). The "I just configured this; here's what it looks like before any run" case.
- With `?run=` — paints execution state on top of the same DAG.

Single page, two modes. Switching between historical runs is a query-string change, not a remount.

**Legacy redirect.** `/plans/executions/[executionId]/+page.svelte` is replaced by a `+page.ts` redirect that:
1. Looks up the configuration_id for the execution (one RPC: `GetPlanExecution` already returns it per M3's proto changes).
2. 302-redirects to `/plans/configurations/<configId>/canvas?run=<execId>`.

The old timeline view is deleted; the canvas is the canonical execution view.

### 2.2 Real-time state — reuse M3 chat stream + add `STEP_STARTED` kind

The M3 chat stream (`watchPlanThreadMessages`) already streams `RUN_STARTED`, `STEP_BOUND`, `RUN_COMPLETED`, `RUN_FAILED`, `ELICITATION_RAISED`, `ELICITATION_ANSWERED`, `APPROVAL_RAISED`, `APPROVAL_DECIDED`. M4 reuses this stream, filtered to events where `executionId === <currentRun>`, and projects state changes onto canvas nodes.

The one gap: there is no explicit signal for "this step just started running" (only `STEP_BOUND` on completion). For canvas-as-operational-surface, the spinner-on-active-node UX is important. M4 adds:

- **New `ThreadMessageKind`: `STEP_STARTED`** — emitted when a step transitions to `RUNNING`. Payload mirrors `STEP_BOUND`: `{ "step_key": "...", "step_execution_id": "..." }`.
- **New workflow-activity hook** in the same Go runtime path that already emits `STEP_BOUND` (per M3's `runtime.go` pattern). Tiny addition (~30 lines spread across proto + Go).

The frontend `ChatMessageKind` enum gains the new variant. The canvas's state projection maps `STEP_STARTED` → step status `RUNNING`, `STEP_BOUND` → step status `COMPLETED`.

No new RPC. No new polling. The M3 stream becomes the single source of truth for execution state on the canvas.

### 2.3 Right-pane fully owns answering

When the user selects a node with a pending elicitation or approval, the right pane (340px) renders an **inline answer form**:

- **Approvals** — reuse `InboxApprovalEntry`'s action/form logic, extracted into a smaller `CanvasApprovalForm` component (props: `{ approvalRequestId: string }`). Renders Preview / Reject / Approve buttons; expand shows `ArtifactPreview`; reject shows reason textarea. Submitting calls `respondToApprovalRequest` (existing RPC).
- **Elicitations** — extract the form from `/plans/executions/[executionId]/elicitations/[elicitationId]/+page.svelte` into a `CanvasElicitationForm` component (props: `{ elicitationId: string }`). Renders schema-driven fields (select / textarea / text / number) plus a free-form `responseText` fallback. Submitting calls `respondToElicitation` (existing RPC).

Both extracted components are **also re-used** by the existing detail pages they came from — those pages become thin route wrappers around the same component, so no copy-paste. The detail page URLs survive as deep-link targets (notifications, emails).

Selecting a node with no pending interaction shows: executor name + tier, overseer, runtime stats (started_at, duration, status), input/output artifact links (clickable to `ArtifactPreview` modal or inline).

### 2.4 Top-bar action scope

| Button | M4 status | Implementation |
|---|---|---|
| `← Back to thread` | ✓ Ship | Link to `/plans/configurations/<configId>` |
| `Run history` | ✓ Ship | Drawer; data from existing `listPlanExecutions` RPC; row = "Run N · status · started" with click → `?run=<execId>` |
| `Settings` | ✓ Ship | Drawer hosting the existing `PlanPoliciesForm` (timeout + approval mode). Save calls `updatePlanConfiguration` (existing). |
| `Schedule` | 🚫 Deferred to M5 | Stub button: gray with `title="Coming in a later milestone"`; no drawer wired. |
| `Answer ★ N` | ✓ Ship | Primary-style button; visible only when N > 0 pending elicitations/approvals on the current run; clicking selects the first such node (paints right-pane with its form). N derived from the same M3 chat stream events. |

### 2.5 Component decomposition

A new top-level `PlanCanvas` component owns the DAG layout, node selection, and state projection. It is **not** an extension of `PlanDagDiagram` — that component stays as-is for the legacy template-detail page (static, no state). `PlanCanvas` is a separate component sharing the layout helpers from `lib/plans/artifact-flow.ts` (`orderPlanStepsLinear`, `buildLinearDagEdges`).

Decomposition:

- `PlanCanvas.svelte` — top-level layout (grid background, top bar slot, graph area, right pane slot). Owns selection state.
- `PlanCanvasNode.svelte` — single node card. Props: `{ step, status, executor, overseer, selected }`. Click → emits selection event.
- `PlanCanvasEdge.svelte` — edge connector with type label. Pure visual.
- `PlanCanvasDetailPane.svelte` — right-pane shell. Props: `{ selectedStep, execution }`. Renders the appropriate form based on selectedStep's pending state.
- `CanvasApprovalForm.svelte` — extracted from `InboxApprovalEntry`.
- `CanvasElicitationForm.svelte` — extracted from `/plans/executions/[id]/elicitations/[id]/+page.svelte`.
- `RunHistoryDrawer.svelte` — list drawer.
- `SettingsDrawer.svelte` — drawer hosting `PlanPoliciesForm`.

### 2.6 Execution state projection helper

A new `lib/plans/canvas-state.ts` exports `buildCanvasState(messages, steps): Record<stepKey, CanvasStepState>` where:

```ts
type CanvasStepState = {
  status: "pending" | "running" | "awaiting_elicitation" | "awaiting_approval" | "done" | "failed";
  stepExecutionId?: string;
  pendingElicitationId?: string;
  pendingApprovalId?: string;
  startedAt?: string;
  completedAt?: string;
  outputArtifactId?: string;
};
```

The function takes the chat messages for the current execution and the plan's steps, walks them in sequence_number order, and produces a per-step state snapshot. Pure function — unit-testable without browser. Replaces what otherwise would be ad-hoc derivation in the canvas component.

### 2.7 The M1 canvas stub

`/plans/[templateId]/canvas/+page.svelte` (still a `MilestoneStub`) is **deleted** in M4. The canonical canvas URL is `/plans/configurations/[configId]/canvas`. The `nav.planCanvas` i18n key survives (still used in the page title of the canvas).

### 2.8 AbortSignal continuity

M4 components subscribing to `watchPlanThreadMessages` and `watchPlanExecution`-like helpers continue using the `AbortController + $effect` pattern M3 established. Nothing new on the wire; just consistent usage.

---

## 3. Architecture sketch

### 3.1 Backend (small)

```
proto/harpia/chat/v1/chat.proto                    [EXTEND]
  enum ThreadMessageKind {
    ...                                            // existing
    THREAD_MESSAGE_KIND_STEP_STARTED = 12;         // new
  }

control-plane/internal/plans/runtime.go             [EXTEND]
  // Wherever the runtime transitions a step_execution to RUNNING,
  // emit a STEP_STARTED chat message — mirror the existing STEP_BOUND
  // emission pattern exactly.

control-plane/internal/chat/messages.go             [EXTEND]
  func BuildStepStartedPayload(stepKey, stepExecutionID string) string

control-plane/gen, frontend/src/lib/gen, agent-runtime/...  [REGEN]
  buf generate after the proto change.
```

No new RPCs, no new migrations, no new Go packages.

### 3.2 Frontend

```
frontend/src/lib/chat/types.ts                      [EXTEND]
  ChatMessageKind union gains "STEP_STARTED"
  KIND_FROM_PROTO / KIND_TO_PROTO maps updated

frontend/src/lib/plans/canvas-state.ts              [NEW]
  buildCanvasState(messages, steps) → Record<stepKey, CanvasStepState>
  buildCanvasState tests

frontend/src/lib/plans/canvas-state.test.ts         [NEW]

frontend/src/lib/components/canvas/                  [NEW DIRECTORY]
  PlanCanvas.svelte                                 // top-level layout
  PlanCanvasNode.svelte
  PlanCanvasEdge.svelte
  PlanCanvasDetailPane.svelte
  CanvasApprovalForm.svelte                         // extracted from InboxApprovalEntry
  CanvasElicitationForm.svelte                      // extracted from elicitation detail page
  RunHistoryDrawer.svelte
  SettingsDrawer.svelte
  CanvasTopBar.svelte                               // back / run-history / schedule (stub) / settings / answer

frontend/src/routes/plans/configurations/[configurationId]/canvas/  [NEW]
  +page.svelte
  +page.ts                                          // loads PlanConfiguration + PlanTemplate; subscribes to chat stream

frontend/src/routes/plans/executions/[executionId]/                  [MODIFY]
  +page.svelte                                      // DELETE (replaced by +page.ts redirect)
  +page.ts                                          // NEW — looks up config_id, 302-redirects to canvas URL

frontend/src/routes/plans/[templateId]/canvas/                       [DELETE]
  +page.svelte                                      // M1 stub, no longer needed

frontend/src/routes/plans/executions/[executionId]/elicitations/[elicitationId]/  [REFACTOR]
  +page.svelte                                      // thinned to wrap <CanvasElicitationForm />

frontend/src/lib/components/inbox/InboxApprovalEntry.svelte          [REFACTOR]
  // Internal form logic moves into CanvasApprovalForm; InboxApprovalEntry
  // composes CanvasApprovalForm. Behavior unchanged for inbox users.

frontend/src/lib/i18n/{en,pt-BR}.json                                [EXTEND]
  canvas.* keys: top-bar labels, drawer titles, schedule stub tooltip,
                  empty/error states, status badge labels.
```

---

## 4. v1→v2 hinges

- **Branching DAGs.** v1 plans are linear; `orderPlanStepsLinear` produces a single chain. The canvas layout assumes left-to-right linear flow. When branching arrives, the layout helper changes; the `PlanCanvas` shell, state projection, and detail pane all stay (they're keyed by step key, not by position).
- **Schedule dialog.** Stubbed button advertises the surface; M5 wires the form.
- **Drag-to-compose (M5+).** The dotted-grid background and node-position model deliberately leave room for future drag affordances. v1 nodes are positioned by the layout algorithm; v2 will allow position overrides. No structural change blocks this.
- **M5 chat configuration.** When M5 ships, the Settings drawer's form may be replaced by a chat-driven flow. The drawer abstraction survives — its contents change.

---

## 5. Out of scope

- Schedule dialog (deferred to M5).
- Branching DAG layout (v2; spec §9).
- Drag-to-rearrange / drag-to-add nodes (v2).
- Per-tenant or per-plan canvas presets (zoom, hidden steps, etc.).
- Real-time `step_execution.status` granularity beyond `STARTED` / `BOUND` (e.g., progress %).
- Backend changes to plan_executions / step_executions schemas (M3's data model is sufficient).
- Persona/admin split for the canvas (M6).

---

## 6. Implementation milestones (sketch — refined in the plan)

Approximate task decomposition:

### Backend (small)
1. Proto: add `STEP_STARTED` kind to `harpia.chat.v1.ThreadMessageKind`; regen.
2. Go: `chat.BuildStepStartedPayload(...)` + workflow hook to emit on step `RUNNING` transition.

### Frontend lib
3. `lib/chat/types.ts` — `ChatMessageKind` union + maps gain `STEP_STARTED`.
4. `lib/plans/canvas-state.ts` — `buildCanvasState` + tests.

### Frontend forms (extractions)
5. `CanvasApprovalForm.svelte` — extracted from `InboxApprovalEntry`. `InboxApprovalEntry` refactored to compose it. Behavior preserved.
6. `CanvasElicitationForm.svelte` — extracted from the elicitation detail page. Detail page refactored to compose it.

### Frontend canvas components
7. `PlanCanvasNode.svelte` (+ tests for state-to-class mapping).
8. `PlanCanvasEdge.svelte`.
9. `PlanCanvasDetailPane.svelte`.
10. `PlanCanvas.svelte` (layout + selection state).
11. `CanvasTopBar.svelte`.

### Frontend drawers
12. `RunHistoryDrawer.svelte` (uses `listPlanExecutions`).
13. `SettingsDrawer.svelte` (hosts `PlanPoliciesForm`).

### Page + integration
14. `/plans/configurations/[configurationId]/canvas/` route — page + load.
15. `/plans/executions/[executionId]/+page.ts` redirect.
16. Delete `/plans/executions/[executionId]/+page.svelte`.
17. Delete `/plans/[templateId]/canvas/+page.svelte` (M1 stub).
18. Wire the M3 thread's inline DAG mini-map `Expand ⤢` affordance to navigate to the canvas URL.
19. `canvas.*` i18n keys (lockstep en + pt-BR).

### Verify + ship
20. End-to-end verification + log.
21. Final whole-branch review.

~21 tasks. Comparable size to M3 (19 tasks). Mostly frontend; the backend addition is ~30 lines.

---

## 7. Open questions

None. All Phase 1 brainstorm decisions resolved. Items deferred by explicit decision (Schedule dialog, branching DAG layout, real-time progress percentages) are documented in §5.

---

## 8. Closes / references

- Master design spec §5.3, §6, §9 — UX shape, route disposition, v1/v2 split
- M3 design spec §2.1, §2.7 — chat stream as live-state source, AbortSignal pattern
- GitHub issues: no direct closes — M4's "expanded canvas" surface isn't tracked as a discrete ticket
