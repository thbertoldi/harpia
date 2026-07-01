# Path B Chat Thread Foundation Verification

**Date:** 2026-06-30
**Commit range:** `3aa412f` (feat: add thread service proto) → `0fe4fd5` (feat: emit plan runtime messages to threads)

## Commands

- `cd control-plane && go test ./internal/threads ./internal/chat ./internal/plans ./cmd/api -count=1`
- `cd proto && buf lint`
- `cd frontend && bunx vitest run src/lib/chat/client.test.ts` (project test runner is Vitest; `bun test` uses Bun's runner, which lacks `vi.hoisted`)
- `cd frontend && bun run check`

## Results

- **Backend:** PASS, exit 0.
  ```
  ok  github.com/harpia/control-plane/internal/threads
  ok  github.com/harpia/control-plane/internal/chat
  ok  github.com/harpia/control-plane/internal/plans
  ok  github.com/harpia/control-plane/cmd/api
  ```
- **Proto:** `buf lint` PASS, exit 0 (no output).
- **Frontend focused:** PASS, exit 0 — `src/lib/chat/client.test.ts` 2/2 tests.
- **Frontend check:** `bun run check` reports 12 errors / 5 warnings across 9 files,
  exit 1. **All are pre-existing baseline failures; none are in files modified by
  this plan.** Failing files:
  - `src/lib/auth-roles.test.ts` (6)
  - `src/lib/plans/artifact-flow.ts` (1)
  - `src/lib/components/canvas/ScheduleDialog.svelte` (1)
  - `src/routes/+layout.svelte` (2)
  - `src/routes/plans/[templateId]/+page.svelte` (2)

  Path B modified/created files (`lib/rpc.ts`, `lib/chat/*`, `lib/plans/plan-configuration.ts`,
  `routes/chat/[threadId]/*`, `routes/plans/configurations/[configurationId]/+page.ts`,
  `routes/new/+page.svelte`) type-check clean.

## Deviations from plan

- **Task 5, Step 1 (b):** The planned `handler_test.go` create-config assertion assumed
  a "fake repository pattern in that file" that does not exist — `PlanHandler` holds a
  concrete `*Repository`. Replaced with a DB-free test
  (`TestCreatePlanConfiguration_RequiresThreadID`) that verifies the new required-`thread_id`
  contract, which is validated before any repository access.
- **Task 8, Step 1:** The planned `runtime_test.go` fake-config assertion assumed a
  fake-based runtime harness that does not exist (`RuntimeRepository` holds a concrete
  `*Repository` requiring a database). The runtime code change (resolve configuration id →
  owning thread id, with fallback) was implemented and verified by build + existing plans
  tests; end-to-end coverage lands with the dev-DB integration path.

## Manual Smoke

- `/new` creates a thread and routes to `/chat/[threadId]`.
- Legacy `/plans/configurations/[configurationId]` redirects to the owning
  thread when `thread_id` exists.
- Thread messages stream through `ThreadService`.
