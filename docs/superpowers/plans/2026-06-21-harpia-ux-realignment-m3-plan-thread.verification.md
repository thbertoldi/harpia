# M3 Verification — 2026-06-21

Branch: `feat/ux-realignment-m3-plan-thread`
Plan: `docs/superpowers/plans/2026-06-21-harpia-ux-realignment-m3-plan-thread.md`

## Automated checks (executed)

| Check | Result | Notes |
|---|---|---|
| `frontend: npm run test` | ✅ 239/239 passing | New tests: lib/chat/client.test.ts (2), lib/plans/thread.test.ts (6) |
| `frontend: npm run check` | ✅ 10 errors, 0 new | Trunk baseline |
| `frontend: npm run lint` | ✅ clean | |
| `control-plane: go test ./...` | ✅ all passing | New: internal/chat (7 tests), internal/plans/thread_handler (3) |
| `control-plane: go vet ./...` | ✅ clean | |
| Frontend dev server boots | ✅ Vite ready | |
| Route /plans/configurations/[id] resolves | ✅ 302→/login | Authenticated render not tested via curl |
| `proto: buf generate && buf lint` | ✅ clean | New chat.v1 package + 3 PlanService RPCs |

## Route smoke (curl, unauthenticated)

```
302 → /
302 → /inbox
302 → /plans/configurations/test-id
```

## Browser-driven verification (manual — required before merge)

Run `cd frontend && npm run dev` with Tilt up so the backend is reachable. Use `devLogin('Leader')`.

| Step | Expected |
|---|---|
| Visit a PlanConfiguration's thread URL: `/plans/configurations/<configId>` | Page renders. DAG mini-map at top. First message: "Configuration saved." (or "Configuration updated." for updated ones). HintBanner visible. Composer at bottom. |
| Type a message in the composer and press send | Message appears in the thread as a right-aligned bubble. HintBanner can be dismissed (persists across reloads via localStorage). |
| Run a plan (use existing wizard) | Live updates: RUN_STARTED system event appears, then STEP_BOUND for each step, then RUN_COMPLETED or RUN_FAILED. |
| Raise an elicitation on a step | ELICITATION_RAISED card appears in the thread inside the execution section. |
| Answer the elicitation (via the existing detail page or via inbox) | ELICITATION_ANSWERED card appears below the raised one. |
| Click "Open thread" on an inbox elicitation row | Navigates to /plans/configurations/<configId>#m-elicitation-<id>; thread scrolls to that message, pulses briefly. |
| Reload the thread page | Messages persist (loaded from chat_messages table). |
| Most recent execution section is expanded by default; older ones collapsed | Verified visually. |

## Commits

```
a3cd6ec fix(ux-m3): resolve merge regressions in runtime, main, and lint
b23a4ba feat(ux-m3): inbox 'Open thread' deep-links to plan thread
e8a30c4 feat(ux-m3): /plans/configurations/[id] route — plan thread page
3f5b25b feat(ux-m3): add thread.* i18n keys for plan thread surface
7cca649 feat(ux-m3): add lib/components/thread — chat surface components
b20eae1 feat(ux-m3): emit ELICITATION_ANSWERED / APPROVAL_RAISED / APPROVAL_DECIDED chat messages
03c0564 feat(ux-m3): add buildThreadSections helper for execution grouping
10e6d30 fix(ux-m3): plumb AbortSignal through watch* RPCs and the inbox page
9eb3708 feat(ux-m3): emit RUN/STEP/ELICITATION_RAISED chat messages from runtime
68d14f4 feat(ux-m3): write CONFIGURATION_SAVED message on configuration create/update
23d394f feat(ux-m3): add three PlanService chat-thread RPC handlers
89feb66 feat(ux-m3): add internal/chat store with typed payload helpers
7bbf799 feat(ux-m3): expose plan_configuration_id on Elicitation/Approval RPCs and InboxItem
047d973 feat(ux-m3): add lib/chat — types, client, watch stream
d8dc902 feat(ux-m3): add PlanDagMiniMap compact DAG component
91729f9 feat(ux-m3): add chat_messages table migration
b70209b feat(ux-m3): add three PlanService RPCs for plan thread messages
ce6dc50 feat(ux-m3): add harpia.chat.v1 proto package with ThreadMessage
2a38050 docs(ux-m3): write M3 plan-thread implementation plan
39dd272 docs(ux-m3): M3 plan-thread implementation design spec
```

## Status

**Automated portion: ✅ complete.** Manual browser steps required before merge.

## Orchestrator notes

- T3 migration RLS adapted to `current_tenant_id()` per canonical project pattern (brief used `app.tenant_id`).
- T8 cherry-pick required conflict resolution; T6 had accidentally truncated thread_handler tests (restored in a3cd6ec).
- Parallel worktree commits stripped of Co-Authored-By trailers before integration.
