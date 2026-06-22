# M5 Verification — 2026-06-22

Branch: `feat/ux-realignment-m5-chat-config`
Plan: `docs/superpowers/plans/2026-06-22-harpia-ux-realignment-m5-chat-config.md`

## Automated checks (executed)

| Check | Result | Notes |
|---|---|---|
| `control-plane: go build ./...` | ✅ clean | Tasks 7–9 |
| `control-plane: go test ./...` | ✅ all passing | Includes planassistant, plans, executors |
| `frontend: npx vitest run` | ✅ 254 passing | cost (4), assistant (2), hardcoded-copy (2), existing suite |
| `frontend: npx vitest run src/lib/i18n/hardcoded-copy.test.ts` | ✅ PASS | Task 22 lockstep en + pt-BR |
| `control-plane API health (:8080)` | ⚠️ unreachable | `curl` returned 000 — stack not fully up in this session |
| `frontend dev (:5173)` | ⚠️ partial | HTTP 302 (server responding); auth session required for golden path |
| `tilt up` | ⚠️ blocked | Port 10350 already in use by another Tilt instance; did not attach to running stack |

## Browser-driven verification (manual — BLOCKED this session)

**Status: BLOCKED** — control-plane API was not reachable and an authenticated dev session was not available. Re-run with `tilt up` (or attach to the existing Tilt instance on port 10350) plus `cd frontend && npm run dev` and `devLogin('Leader')`.

| # | Scenario | Expected | Observed |
|---|---|---|---|
| 1 | `/new` greets and shows templates | Greeting, composer, gallery | ⏸ Not run (blocked) |
| 2 | Template pick → thread | Navigate to `/plans/configurations/<id>`; `CONFIGURATION_STARTED` then `ASSISTANT_PROMPT` | ⏸ Not run |
| 3 | Walk bindings | Each chip click advances assistant prompts | ⏸ Not run |
| 4 | Save promotes to RUNNABLE | Saved ack + top-bar `Runnable` | ⏸ Not run |
| 5 | Edit pencil | `STEP_REBOUND` + binding persists on reload | ⏸ Not run |
| 6 | Schedule dialog | `SCHEDULE_SET`, status `Scheduled`, cron persisted | ⏸ Not run |
| 7 | Wizard redirects | `/plans/<id>/configure` → 302 `/new?template=<id>` | ⏸ Not run |
| 8 | Sidebar “Your plans” | List + navigation | ⏸ Not run |
| 9 | Cost pill | `R$ X / run` matches bound executors | ⏸ Not run |

## Commits (Tasks 7–24)

```
c4b49ec feat(ux-m5): wire planassistant.Controller in main + adapter types
e5e26f6 feat(ux-m5): UpdatePlanConfiguration validates cron, emits SCHEDULE_SET, reconciles SCHEDULED status
c17193e feat(ux-m5): AppendPlanThreadMessage triggers planassistant.NextTurn on USER_SELECTION
c1af67f feat(ux-m5): extend ChatMessageKind with 5 assistant message kinds
fe94d0a feat(ux-m5): lib/plans/cost — computeRunCost pure helper
d48ed37 feat(ux-m5): lib/plans/assistant — selectChip + editBinding helpers
a10c5d4 feat(ux-m5): AssistantPromptCard — chip group with edit pencil
57831d8 feat(ux-m5): ConfirmCard, TemplatePickerCard, dispatch + icons for new kinds
d494d06 feat(ux-m5): PlanCostPill + PlanThreadTopBar
7d31026 feat(ux-m5): ScheduleDialog + wire schedule button + cost pill into CanvasTopBar
ec93b51 feat(ux-m5): YourPlansList sidebar entries for operator persona
d2c7a0d feat(ux-m5): /new launcher with template gallery + auto-commit
8bea5f6 feat(ux-m5): plan thread mounts top bar + cost pill + schedule dialog; routes assistant kinds
8f4ad2f feat(ux-m5): canvas page wires ScheduleDialog + cost pill
cd77cd8 chore(ux-m5): delete wizard routes + 302 to /new; retarget template CTA
9ef8549 feat(ux-m5): i18n keys for assistant, confirm, cost, schedule, sidebar.yourPlans, new
```

## Deviations noted during implementation

| Item | Detail |
|---|---|
| `main.go` path | Plan cites `control-plane/cmd/control-plane/main.go`; actual wiring in `control-plane/cmd/api/main.go` |
| Task 21 file deletions | Wizard `.svelte` removals landed in commit `ec93b51` (Task 17) before redirect `+page.ts` files in `cd77cd8` |
| `assistant.test.ts` mocks | Used `vi.hoisted()` so Vitest mock factories compile (same assertions as plan) |
| `AssistantConfigurationStore` | Added `Executors ExecutorLookup` field to enrich slot bindings with SKU/kind on selection |
| `ListCompatibleInstallationsForStep` | Implemented on `executors.Repository` (`plan_step_compat.go`); loads SKU via `ExecutorSKUID` for catalog options |
| Schedule side-effect tests | Placeholder tests remain `t.Skip` per plan (harness not generalized) |
| Task 24 push/PR | Skipped per branch instructions (no push, no PR) |

## Review notes (Task 24 — inline)

- **CRITICAL:** none identified in automated pass.
- **IMPORTANT:** wire schedule side-effect integration tests when handler test harness exists; validate cron *before* `UpdateConfiguration` persists invalid expressions (plan inserts validation post-persist).
- **NOTE:** `configurationFromProto` preserves identity/timestamps from existing row — correct for partial proto mutations.

## Status

**Automated portion: ✅ complete.** Manual browser golden path: **BLOCKED** — re-verify with full Tilt stack + dev login before merge.
