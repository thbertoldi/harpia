# M4 Verification — 2026-06-20

Branch: `feat/ux-realignment-m4-expanded-canvas`
Plan: `docs/superpowers/plans/2026-06-22-harpia-ux-realignment-m4-expanded-canvas.md`

## Automated checks (executed)

| Check | Result | Notes |
|---|---|---|
| `frontend: npm run test` | ✅ 247 passing | New tests: lib/plans/canvas-state (8) |
| `frontend: npm run check` | ✅ 13 errors (trunk baseline) | 0 new errors in M4 canvas files |
| `frontend: npm run lint` | ✅ clean | After fix(ux-m4) lint commit |
| `control-plane: go test ./...` | ✅ all passing | New: TestBuildStepStartedPayload |
| `control-plane: go vet ./...` | ✅ clean | |
| Frontend dev server boots | ✅ Vite ready | Port may auto-increment if 5179 busy |
| Route smoke (unauthenticated) | ⚠️ 500 | Dev server without auth session; manual Tilt+devLogin required |
| `proto: buf generate && buf lint` | ✅ clean | New STEP_STARTED enum value |

## Browser-driven verification (manual — required before merge)

Run `cd frontend && npm run dev` with Tilt up. Use `devLogin('Leader')`.

| Step | Expected |
|---|---|
| Visit `/plans/configurations/<configId>/canvas` (no run) | Canvas renders static DAG; all nodes "Pending"; right-pane empty hint. |
| Visit `/plans/configurations/<configId>/canvas?run=<execId>` | Canvas paints execution state from chat stream; running step shows spinner; completed shows checkmark. |
| Visit `/plans/executions/<execId>` directly | 302 redirects to canvas URL with run query. |
| Click a node | Right-pane fills with executor / status / artifacts; selected ring on the node. |
| Click a node awaiting elicitation | Right-pane renders CanvasElicitationForm; submit answers the elicitation; row in M3 thread shows ELICITATION_ANSWERED. |
| Click a node awaiting approval | Right-pane renders CanvasApprovalForm; Approve/Reject work; canvas state updates live. |
| Click `Answer ★ N` in top bar | First pending step is highlighted/scrolled into view. |
| Click `Run history` | Drawer opens with prior runs of this plan; clicking a row swaps `?run=` and re-renders. |
| Click `Settings` | Drawer opens with PlanPoliciesForm; saving persists. |
| Click `Schedule` | Tooltip "Coming in a later milestone"; button disabled. |
| Click `Back to thread` | Navigates to `/plans/configurations/<configId>`. |
| From thread page, click `Expand ⤢` next to mini-map | Navigates to canvas URL. |
| Run a plan from scratch | Canvas updates live as STEP_STARTED / STEP_BOUND events arrive. |

## Commits

```
871f1cf feat(ux-m4): add canvas.* i18n keys for expanded canvas surface
72e663a feat(ux-m4): wire Expand affordance from thread mini-map to canvas
fbed44b chore(ux-m4): delete M1 canvas stub
1e74889 feat(ux-m4): replace /plans/executions/[id] page with canvas redirect
492726a feat(ux-m4): add /plans/configurations/[id]/canvas route with live state
db3d141 feat(ux-m4): add SettingsDrawer hosting PlanPoliciesForm
693c664 feat(ux-m4): add CanvasDrawer chrome + RunHistoryDrawer
7b8f66e feat(ux-m4): add PlanCanvas top-level layout with node selection
33108ca feat(ux-m4): add CanvasTopBar with back, drawers, and Answer N primary
391974e feat(ux-m4): add PlanCanvasDetailPane with inline answer forms
275f0ba feat(ux-m4): add PlanCanvasEdge connector with artifact-type label
d42506b feat(ux-m4): add PlanCanvasNode component with state-driven icon and border
76663ec feat(ux-m4): extract CanvasElicitationForm from elicitation detail page
e988924 feat(ux-m4): extract CanvasApprovalForm from InboxApprovalEntry
4460532 feat(ux-m4): add buildCanvasState helper projecting chat events to step state
c8d079d feat(ux-m4): extend ChatMessageKind with STEP_STARTED
0be117b feat(ux-m4): emit STEP_STARTED chat message when step transitions to RUNNING
6ceb20d feat(ux-m4): add STEP_STARTED to harpia.chat.v1.ThreadMessageKind
```

## Known limitations / deferred

- Schedule dialog stubbed (disabled button + tooltip).
- `approvalInputArtifactByStep` empty in canvas page v1 — approval preview may lack artifact until M5.
- `Answer ★ N` sets URL hash; hash-based node selection not fully wired in PlanCanvas.
- Branching DAG layout post-v1 (linear chain assumed); parallel fan-out pointer routing uses most-recent running step.

## Review fixes (post-PR #218)

- CanvasElicitationForm submitted text uses `text-talon-gold` (locked palette).
- PlanCanvas grid uses `var(--color-obsidian)` without hex fallback.
- STEP_STARTED emission gated on `Attempt == 1` + chat de-dupe by `step_execution_id`.
- `canvas.topbar.answerPending` i18n includes ★ glyph; RunHistoryDrawer uses `triggeredAt`.
- PlanCanvasDetailPane wires `onDecided` for optimistic approval header.
- `buildCanvasState` ignores duplicate STEP_STARTED when step is awaiting interaction.

## Status

**Automated portion: ✅ complete.** Manual browser steps required before merge.
