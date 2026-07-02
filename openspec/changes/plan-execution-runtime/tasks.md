> Executor-cold briefs. Each task is self-contained: exact file paths + how to verify. No
> assumed conversation context. Backend is Go (`cd control-plane`), frontend is
> SvelteKit + bun (`cd frontend`). Do not add a `Co-Authored-By: Claude` trailer. Pre-v1: no
> migrations/shims. Design + rationale: `design.md`; contract: `specs/plan-execution-runtime/spec.md`.

## 1. Server-side configuration materializer (backend)

- [x] 1.1 (TDD) Write `control-plane/internal/plans/materialize_test.go` for a **pure**
  materializer that takes a `*plansv1.PlanTemplate` and a `map[string]any` of parameter values
  and returns `([]*plansv1.SeedArtifactBinding, []*plansv1.SlotBinding, *plansv1.PlanBehaviorPolicies)`.
  Cover, using the real `weekly-newsletter-linkedin` mappings as fixtures:
  (a) five `SEED_ARTIFACT` mappings targeting `(write-draft, harpia.internal.ContentPreferences)`
  at `$.topic/$.language/$.tone/$.audience/$.topics_to_avoid` compose ONE seed whose
  `LiteralJson` is `{"topic":…,"language":…,"tone":…,"audience":…,"topics_to_avoid":…}`;
  (b) `date_range` `SEED_ARTIFACT` on `(fetch-news, date_range)` with empty/`$` jsonPath sets the
  whole `LiteralJson` verbatim (including a `{"preset":"last_7_days"}` object);
  (c) `source_group` `SLOT_BINDING` on `fetch-news` produces `SlotBinding{StepKey:"fetch-news",
  InstallationId:<value>}`;
  (d) `approval_mode` `BEHAVIOR_POLICY` sets `publish_approval_mode` from the value;
  (e) unknown/absent parameters are skipped without error.
  Verify: `cd control-plane && go test ./internal/plans/ -run Materialize` (RED).
- [x] 1.2 Implement the pure materializer in new file `control-plane/internal/plans/materialize.go`
  to pass 1.1. Group `SEED_ARTIFACT` mappings by `(stepKey, inputName)`; build each seed literal
  by setting each mapping's `jsonPath` (support dotted `$.a.b`; treat `""`/`"$"` as the whole
  value). Reuse `runtimeMappings` off `template.InputParameters[i].RuntimeMappings`. Verify: same
  command GREEN, plus `go build ./...`.
- [x] 1.3 (TDD then impl) Add default slot-binding resolution:
  `ResolveDefaultSlotBindings(ctx, tenantID, template, executorRepo)` that, for every template
  step **without** a binding produced in 1.2, looks up the tenant's enabled
  `ExecutorInstallation` whose SKU key equals the step's `default_executor_sku_key`; integration
  steps bind by installation id; agent steps also set `ManifestId`/`ManifestVersion` from the
  installation. Missing installation for a required step returns an error the caller maps to
  `connect.CodeFailedPrecondition`. Put it in `materialize.go`; test with a fake executor lookup
  in `materialize_test.go`. Verify: `go test ./internal/plans/`.
- [x] 1.4 Wire materialization into config writes. In
  `control-plane/internal/plans/handler.go` `CreatePlanConfiguration` and
  `UpdatePlanConfiguration` (see `buildConfigurationFromRequest`), after parsing
  `parameter_values`, compute `SeedArtifacts` + parameter `SlotBindings` + `BehaviorPolicies`
  via 1.2 and merge the default bindings from 1.3, and use those instead of the request-supplied
  `seed_artifacts`/`slot_bindings`/`behavior_policies`. Keep storing `parameter_values`. Add a
  handler-level test (`handler_test.go`) proving a create with only `parameter_values` yields a
  config whose `fetch-news` has a slot binding and a `date_range` seed, and whose `write-draft`
  has a `ContentPreferences` seed. Verify: `go test ./internal/plans/`.
- [x] 1.5 Make the frontend send `parameter_values` only. In
  `frontend/src/lib/plans/plan-configuration.ts`, `frontend/src/lib/plans/assistant.ts`, and
  `frontend/src/lib/components/thread/BindingMatrixCard.svelte`, stop passing
  `seedArtifacts`/`slotBindings`/`behaviorPolicies` on create/update. Remove the now-dead
  `materializeLinkedInInputValues` path in
  `frontend/src/lib/plans/linkedin-template-inputs.ts` (keep only value (de)serialization used
  for form re-hydration). Update/trim the affected tests. Verify:
  `cd frontend && bunx vitest run src/lib/plans/plan-configuration.test.ts src/lib/plans/assistant.test.ts src/lib/plans/linkedin-template-inputs.test.ts` and `bun run check` (no new baseline diagnostics).

## 2. Execution-time rolling date-range resolution (backend)

- [x] 2.1 (TDD) Add `ResolveDateRangePreset(preset string, now time.Time) (*artifactsv1.DateRange, bool)`
  in a new `control-plane/internal/plans/daterange.go` (+ `daterange_test.go`). `last_7_days` →
  the seven days ending yesterday, formatted `2006-01-02`. Use the shared vector
  `now=2026-07-01 → startDate=2026-06-24, endDate=2026-06-30` (must match
  `frontend/src/lib/plans/template-inputs.ts` `resolveDateRangePreset`). Unknown preset → `(nil,false)`.
  Verify: `cd control-plane && go test ./internal/plans/ -run DateRange`.
- [x] 2.2 Resolve presets at each run's start. In the snapshot build used by
  `control-plane/internal/plans/handler.go` `CreatePlanExecution` (`buildPlanExecutionSnapshot`
  in `control-plane/internal/plans/execution_snapshot.go`), for any seed whose `LiteralJson`
  parses to `{"preset":<p>}`, rewrite it to the concrete `DateRange` protojson
  (`{"startDate":…,"endDate":…}`) via 2.1 using the current time; leave concrete literals
  unchanged. Do the same for the scheduled path so recurring runs roll forward
  (`control-plane/internal/workflow/plans.go` `PlanScheduledExecution` /
  `CreateScheduledPlanExecutionActivity`). Add a test asserting a preset seed becomes concrete
  dates in the snapshot. Verify: `go test ./internal/plans/... ./internal/workflow/...`.
- [x] 2.3 Defensive guard: in
  `control-plane/internal/executors/integrations/rss/fetch.go` `parseDateRange`, return a clear
  error if `startDate`/`endDate` are empty but a `preset` field is present (i.e. an unresolved
  preset reached the executor). Add a unit test. Verify: `go test ./internal/executors/...`.

## 3. Pre-flight execution readiness (backend)

- [x] 3.1 (TDD then impl) Extend `control-plane/internal/plans/validation.go`
  `ValidateConfigurationForExecution` to verify seed readiness: for each template step whose
  `input_artifact_type` is non-empty and which has no upstream producer edge, require a matching
  seed in the config's `SeedArtifacts`; otherwise return `connect.CodeFailedPrecondition` with a
  message naming the step and artifact type. Mirror the producer/seed logic already in
  `control-plane/internal/workflow/plans.go` `stepInputArtifacts`. Add cases to
  `validation_test.go` (missing seed → error; seeded → ok). Verify: `go test ./internal/plans/`.

## 4. Agent execution path (deployment + verification)

- [x] 4.1 Ensure the Python agent-runtime worker runs in dev. Confirm `mise run dev` / Tilt
  starts a worker from `agent-runtime/` polling `harpia-agent-task-queue`
  (`control-plane/internal/workflow/tasks.go` `AgentTaskQueueName`). If it is not started, add it
  to the dev topology (Tiltfile / deploy manifests under `deploy/dev/`). Document the requirement
  in `agent-runtime/README.md` (or the dev docs). Verify: with `mise run dev` up, a plan
  execution reaching `write-draft` dispatches to the Python worker (Temporal UI shows the
  activity on `harpia-agent-task-queue`) rather than failing with the Go stub
  (`control-plane/internal/workflow/plans.go` `RunAgentActivity`).

## 5. End-to-end verification

- [x] 5.1 Add an integration-style test that drives `weekly-newsletter-linkedin` from
  `parameter_values` only: create config (status RUNNABLE) → assert materialized seeds/bindings
  (task 1) → `CreatePlanExecution` → the workflow walks `fetch-news → write-draft →
  adapt-for-linkedin → publish-linkedin` producing the expected artifact types
  (`NewsList → TextDraft → LinkedInPostDraft → PublishConfirmation`). Use existing test doubles:
  the executor contract harness (`control-plane/internal/executors/contracttest/harness.go`) and
  fake LLM/LinkedIn where a live call would otherwise be needed. Verify:
  `cd control-plane && go test ./...`.
- [x] 5.2 Add a scheduled-run test: set `PlanSchedule` (weekly cron) → confirm the scheduled
  execution path resolves a fresh rolling `date_range` for the run (task 2), i.e. two runs on
  different `now` values produce different concrete windows. Verify: `go test ./internal/plans/... ./internal/workflow/...`.
- [ ] 5.3 Manual dev walkthrough (record result in the PR): `mise run dev`, create the weekly
  newsletter via chat to RUNNABLE, Run now, and confirm artifacts appear for each step and the
  published/approval step gates as configured.

## 6. Verification gates

- [x] 6.1 `openspec validate --changes plan-execution-runtime`.
- [x] 6.2 `cd control-plane && go test ./...`.
- [x] 6.3 `cd frontend && bunx vitest run <changed test files>`.
- [x] 6.4 `cd frontend && bun run check` — only the known baseline diagnostics remain.
- [ ] 6.5 `cd proto && buf lint` (run if any proto changed; none expected).
