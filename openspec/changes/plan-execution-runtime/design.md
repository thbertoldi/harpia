## Context

A `PlanConfiguration` walks a `PlanTemplate` DAG as a Temporal workflow
(`control-plane/internal/workflow/plans.go` `PlanWorkflow` → `runPlanWorkflow`), dispatching
each step to an executor by kind (`runExecutorActivity`): integration steps hit the Go
`IntegrationRegistry` (RSS, LinkedIn), agent steps route to the Python agent-runtime worker on
`harpia-agent-task-queue`. `CreatePlanExecution` (`control-plane/internal/plans/handler.go`)
validates RUNNABLE-readiness, snapshots the config+template, and starts the workflow.
Scheduling is real Temporal Schedules (`control-plane/internal/plans/schedule.go`).

The missing piece is upstream of all this: a config's `SeedArtifacts`, `SlotBindings`, and
`PlanBehaviorPolicies` are stored verbatim from the create/update request
(`buildConfigurationFromRequest`), while `parameter_values_json` is stored as an independent
blob. `runtimeMappings` — the template's declaration of how each input parameter projects onto
those three fields — are validated at seed time but never applied. The chat flow therefore
either sends empty bindings (generic path → fails to run) or relies on a hardcoded per-template
TypeScript materializer (`frontend/src/lib/plans/linkedin-template-inputs.ts`).

## Goals / Non-Goals

**Goals:**

- One generic, tested backend function that projects `parameter_values` + template
  `runtimeMappings` into `SeedArtifacts` / `SlotBindings` / `PlanBehaviorPolicies`, so any
  template (not just LinkedIn) is executable from `parameter_values` alone.
- Rolling date-range presets resolve to concrete dates at each run's start.
- Missing required seeds fail as a pre-flight error, not a mid-run crash.
- `weekly-newsletter-linkedin` runs end-to-end and on schedule.

**Non-Goals:**

- Generic agent executor / manifest-driven agent input plumbing (separate follow-up).
- New SKUs, templates, or execution-surface redesign.
- Migration shims (pre-v1).

## Decisions

### 1. The backend is authoritative for seeds/bindings/policies

`CreatePlanConfiguration` and `UpdatePlanConfiguration` compute `SeedArtifacts`,
`SlotBindings`, and `PlanBehaviorPolicies` from `(template, parameter_values)` and ignore any
client-supplied values for those three fields. The frontend sends `parameter_values` only.
This removes the per-template TS materializer and makes every declarative template executable.

Alternative considered: keep the client authoritative and add generic materialization only as
a fallback. Rejected — it preserves the LinkedIn special-case and lets a client desync bindings
from parameters.

### 2. Split materialization into a pure part and a resolver part

- **Pure** (`materialize.go`, no I/O, table-testable): from `runtimeMappings` +
  `parameter_values`, build
  - `SEED_ARTIFACT`: group mappings by `(stepKey, inputName)`; for each group, build one seed
    whose `LiteralJson` is a JSON object populated at each mapping's `jsonPath`
    (e.g. `write-draft` / `harpia.internal.ContentPreferences` ← `$.topic`, `$.language`,
    `$.tone`, `$.audience`, `$.topics_to_avoid`). A single-value mapping whose `jsonPath` is
    empty or `$` sets the whole literal (e.g. `fetch-news` / `date_range` ← the date-range
    value verbatim, including a `{"preset":…}` object).
  - `BEHAVIOR_POLICY`: set the policy named by `policyKey` (e.g. `publish_approval_mode`) from
    the parameter value.
  - `SLOT_BINDING` (parameter-driven): the parameter value is an `ExecutorInstallation` id
    (e.g. `source_group`) → a `SlotBinding{stepKey, installationId}`.
- **Resolver** (needs the executor repo): for every template step **without** a parameter
  `SLOT_BINDING`, resolve the binding from the step's `default_executor_sku_key` → the tenant's
  enabled `ExecutorInstallation` for that SKU. Agent steps additionally get
  `manifest_id` / `manifest_version` from the installation. If a required step has no resolvable
  installation, surface a clear `FailedPrecondition` (the config stays DRAFT / not RUNNABLE).

The date-range literal shape must match what the executor reads: protojson of
`harpia.artifacts.v1.DateRange` (`{"startDate":"YYYY-MM-DD","endDate":"YYYY-MM-DD"}`), the same
shape `artifacts.ValidatePayload(TypeKeyDateRange, …)` accepts.

### 3. Rolling date presets resolve per run, not at config time

The stored `date_range` seed literal keeps the preset (`{"preset":"last_7_days"}`) so the
window is not frozen. At execution start — in `CreatePlanExecution`'s snapshot build and in the
scheduled `PlanScheduledExecution` path — a resolver rewrites any preset seed literal to a
concrete `DateRange` for that run. Implement `ResolveDateRangePreset(preset string, now time.Time)`
in Go mirroring `frontend/src/lib/plans/template-inputs.ts` `resolveDateRangePreset`
(`last_7_days` = the seven days ending yesterday). Keep the two in sync; a shared test vector
(`2026-07-01` → `2026-06-24`…`2026-06-30`) guards both.

Defense in depth: `parseDateRange` (RSS handler) returns a clear error if it ever receives an
unresolved preset.

### 4. Pre-flight seed readiness

Extend `ValidateConfigurationForExecution` (`validation.go`) to check, for each step whose
`input_artifact_type` is set and which has no upstream producer in the DAG, that a matching
seed exists in `SeedArtifacts`. This mirrors the workflow's `stepInputArtifacts` guard but runs
before the workflow starts, so a misconfigured plan fails at `CreatePlanExecution` with an
actionable message.

### 5. Agent steps depend on the Python worker (deployment, not code)

The reference plan's agent steps use `newsletter-writer-senior` and `linkedin-voice-senior`,
both supported by `agent-runtime/src/harpia_agents/temporal/worker.py`. Milestone E ensures that
worker is deployed and polling `harpia-agent-task-queue` in dev. Generalizing beyond the two
manifests is explicitly out of scope here.

## Risks / Trade-offs

- **Removing the client materializer touches the legacy LinkedIn matrix flow**
  (`BindingMatrixCard.svelte`). It must switch to sending `parameter_values`, or be retired.
  Covered by task 1.5 + its test.
- **Two implementations of the date preset** (Go + TS) can drift. Mitigated by a shared,
  documented test vector in both suites.
- **Agent worker not running** makes agent steps hang. Task 4.1 makes this a verification gate,
  not a silent failure.

## Migration

None. Pre-v1: existing configs can be recreated. No shim.

## Open Questions

- Should materialization run only at config save, or also lazily at `CreatePlanExecution` for
  configs created before this change? Decision for the implementer: run at save; re-materialize
  in `CreatePlanExecution` if `SeedArtifacts`/`SlotBindings` are empty but `parameter_values`
  is present (cheap safety net, keeps old drafts runnable).
