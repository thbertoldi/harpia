## 1. C1 — Single owning-thread resolution

- [x] 1.1 In `control-plane/internal/plans/runtime.go`, replace all `resolveThreadID(...)` calls (lines ~125,166,208,289,362,438) with the shared `configurationThreadID(...)` used by the handlers.
- [x] 1.2 Delete `resolveThreadID` (`runtime.go:33-43`), `GetThreadIDForConfiguration` (`repository.go:586`), and the `configID.String()` fallback. Ensure the single resolver errors clearly when no owning thread resolves (no config-id fallback).
- [x] 1.3 In `proto/harpia/plans/v1/plans.proto`, drop the redundant `thread_id` field (`:148`), keeping `origin_thread_id` (`:150`) as the canonical owning thread; update `handler.go:336-337` to set only `origin_thread_id`. Regenerate Go + TS (`mise run buf-generate` or equivalent) and fix references.
- [x] 1.4 Add/extend a test asserting that a configuration producing both a `STEP_*` event and an approval/elicitation event appends both to the same thread id.
- [x] 1.5 Verify: `cd control-plane && go test ./...`; run the engineer e2e journey and confirm runtime + approval events share one thread.

## 2. D1–D4 — Dead-code removal

- [x] 2.1 Delete the PlanCanvas cluster: `PlanCanvas.svelte`, `PlanCanvasNode.svelte`, `PlanCanvasEdge.svelte`, `PlanCanvasDetailPane.svelte`, `NodeHoverCard.svelte`, `CanvasElicitationForm.svelte`, `frontend/src/lib/plans/canvas-state.ts`, `SettingsDrawer.svelte`, `CanvasDrawer.svelte`, and their `.test.ts`. Do NOT delete `CanvasApprovalForm.svelte` or `ScheduleDialog.svelte` (still imported).
- [x] 2.2 Delete orphaned `frontend/src/lib/components/artifacts/ArtifactRail.svelte` and `TextArtifactEditor.svelte` (+ tests).
- [x] 2.3 Delete the dead legacy plan-thread RPC mock branches in `frontend/e2e/engineer-journey.spec.ts:724,732,737`.
- [x] 2.4 Reword stale comments in `proto/harpia/chat/v1/chat.proto:42-44,55` (drop the "legacy plan-thread RPC" note; reference `WatchThreadMessages`); regenerate so `chat.pb.go`/`chat_pb.ts` comments follow.
- [x] 2.5 Verify: `cd frontend && bun run build && bun run check` (no new baseline errors) and `grep -rn "PlanCanvas\|ArtifactRail\|TextArtifactEditor" src e2e` returns nothing.

## 3. C2 — Generic template-driven inputs

- [x] 3.1 Rewire `frontend/src/lib/components/thread/BindingMatrixCard.svelte` to render from `TemplateInputParameter[]` (reuse `TemplateInputsForm.svelte`'s approach) instead of the LinkedIn model.
- [x] 3.2 Delete the LinkedIn-hardcoded input model: `frontend/src/lib/plans/template-inputs.ts` (`LinkedInTemplateInputValues`, `parameterValuesJson` hardcoding), `frontend/src/lib/plans/linkedin-template-inputs.ts`, and `applyLinkedInSuggestion` in `frontend/src/lib/plans/assistant.ts`. Update callers.
- [x] 3.3 Verify: configuring the LinkedIn content template still works end-to-end; a second (synthetic) template's inputs render with no code change. Run `conversational-configuration` flow-model Vitest + the engineer e2e journey.

## 4. T1 — Outcome-shaped template + date-from-schedule

- [x] 4.1 Rename the `weekly-newsletter-linkedin` template YAML/key to an outcome name (decide, e.g. `news-to-social-post`); update all references (frontend suggestions, tests, seeds, e2e) via grep for the old key.
- [x] 4.2 Remove cadence from template identity/copy; ensure cadence is expressed only via `PlanSchedule`. Update localization keys (`catalog.plan.<key>.*`) to describe the outcome.
- [x] 4.3 In `control-plane/internal/plans` (materialize/runtime), derive the `DateRange` seed at execution start from the schedule window (recurring cadence) or explicit range (one-shot), replacing the hardcoded `last_7_days` assumption.
- [x] 4.4 Verify: a ONE_SHOT config and a weekly RECURRING config of the renamed template both run; the resolved date window matches the schedule cadence. `go test ./...` + real embedded-template load at boot.

## 5. C4 — Proto noun rename

- [x] 5.1 Rename `subtask_key`/`SubtaskCostEstimate` and other `subtask_*` nouns to `step_*`/`StepCostEstimate` in `proto/harpia/budget/v1` and `proto/harpia/mcp/v1`.
- [x] 5.2 Regenerate Go + TS; update all references in `control-plane/internal/{budget,mcp}` and any frontend usage.
- [x] 5.3 Verify: `cd proto && buf lint` (buf breaking will flag the rename — expected/accepted pre-v1), `cd control-plane && go test ./...`, `cd frontend && bun run check`.

## 6. Final verification

- [x] 6.1 Run all gates: `cd frontend && bun run lint && bun run check`; `cd control-plane && go test ./...`; `cd proto && buf lint`; `cd agent-runtime && ruff check src/`.
- [x] 6.2 Run the engineer e2e journey end-to-end (configure → run → approve → artifact) on the renamed template.
- [x] 6.3 Update the cleanup backlog to mark C1, C2, C4, D1–D4, T1 done.
