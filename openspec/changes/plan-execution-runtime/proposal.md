## Why

Milestone E is "a PlanConfiguration created in chat actually runs end-to-end, and on
schedule." Today it does not, for one structural reason: **nothing on the server turns a
template's `input_parameters` + `runtimeMappings` + the stored `parameter_values_json` into
the config's `SeedArtifacts` / `SlotBindings` / `PlanBehaviorPolicies`.** `runtimeMappings`
are only *validated* (`control-plane/internal/plans/template_seed.go` `validateRuntimeMapping`),
never *applied*. The only code that produces those fields is a hardcoded, template-specific
TypeScript function (`frontend/src/lib/plans/linkedin-template-inputs.ts`
`materializeLinkedInInputValues`) that reimplements the LinkedIn template's mapping by hand.

Consequences observed while shipping the chat-first refinements:

- A config created from the generic chat flow carries `parameter_values` but empty
  `SeedArtifacts`, so `fetch-news` fails at run time
  (`control-plane/internal/workflow/plans.go` `stepInputArtifacts`: "requires input artifact
  type … but no seed artifact was configured").
- The weekly newsletter's `date_range` default is a rolling preset
  (`{"preset":"last_7_days"}`), but there is no backend resolver — the RSS executor
  (`control-plane/internal/executors/integrations/rss/fetch.go` `parseDateRange`) requires
  concrete `startDate`/`endDate`. A scheduled plan would freeze one week or fail.
- Execution-time validation (`ValidateConfigurationForExecution`) checks slot bindings only;
  missing seeds surface late as a workflow failure instead of a pre-flight error.

The Temporal workflow engine, worker, scheduling (real Temporal Schedules), RSS + LinkedIn
integration handlers, the Python agent activity (for the two shipped manifests), RUNNABLE
slot-binding validation, and the execution snapshot **already exist and work**. Milestone E
closes the gap between a configured plan and a running one.

## What Changes

- Add a **server-authoritative configuration materializer**: given a `PlanTemplate` and a
  config's `parameter_values`, expand `runtimeMappings` (targets `SEED_ARTIFACT`,
  `SLOT_BINDING`, `BEHAVIOR_POLICY`, with `jsonPath` / `policyKey` / `stepKey` / `inputName`)
  into `SeedArtifacts`, parameter-driven `SlotBindings`, and `PlanBehaviorPolicies`.
- Resolve **default slot bindings** for steps that have no parameter mapping, from each
  step's `default_executor_sku_key` → the tenant's `ExecutorInstallation` for that SKU
  (agent steps also get `manifest_id` / `manifest_version`).
- Make the backend the single source of truth: `CreatePlanConfiguration` /
  `UpdatePlanConfiguration` compute these fields from `parameter_values`; the frontend sends
  `parameter_values` only, and the bespoke frontend materializer is removed.
- Resolve **rolling date-range presets at each run's start** (mirror the frontend
  `resolveDateRangePreset` in Go), so a scheduled weekly plan always covers the previous
  seven days.
- Add **pre-flight seed readiness** to `ValidateConfigurationForExecution` so a config
  missing a required seed fails fast with a clear message instead of failing mid-run.
- Prove the reference plan (`weekly-newsletter-linkedin`) runs end-to-end from
  `parameter_values` → RUNNABLE → `PlanExecution` → artifacts, and on schedule with a fresh
  rolling window.

## Capabilities

### New Capabilities

- `plan-execution-runtime`: server-side expansion of `parameter_values` into config bindings,
  seeds, and policies via template `runtimeMappings`; default slot-binding resolution from
  `default_executor_sku_key`; execution-time rolling date-range resolution; pre-flight seed
  readiness validation; and verified end-to-end execution of a configured plan.

### Modified Capabilities

- `conversational-slot-binding`: slot bindings and seeds become a server-computed projection
  of `parameter_values` + template metadata rather than client-supplied structures, so the
  chat flow persists only `parameter_values`.

## Impact

- Backend plan configuration + execution: `control-plane/internal/plans/handler.go`,
  `control-plane/internal/plans/validation.go`, and new
  `control-plane/internal/plans/materialize.go` (+ tests).
- Backend rolling-date resolution: new util under `control-plane/internal/plans/` and its use
  in the execution snapshot path (`control-plane/internal/plans/execution_snapshot.go` /
  `control-plane/internal/plans/handler.go` `CreatePlanExecution`, and the scheduled path
  `control-plane/internal/workflow/plans.go` `PlanScheduledExecution`).
- Frontend: `frontend/src/lib/plans/linkedin-template-inputs.ts`,
  `frontend/src/lib/plans/plan-configuration.ts`, `frontend/src/lib/plans/assistant.ts`,
  `frontend/src/lib/components/thread/BindingMatrixCard.svelte` stop sending
  `seedArtifacts`/`slotBindings`/`behaviorPolicies` and send `parameter_values` only.
- Deployment: the Python agent-runtime worker (`agent-runtime/`) must poll
  `harpia-agent-task-queue` in dev (`mise run dev` / Tilt) or agent steps hang.
- No production migration/deprecation shim; pre-v1.

## Non-Goals

- Generalizing the Python agent activity beyond the two shipped manifests
  (`newsletter-writer-senior`, `linkedin-voice-senior`). The reference plan uses only these;
  a generic agent executor + manifest-driven input plumbing is tracked as a separate
  follow-up.
- New ExecutorSKUs / templates (blog, email, X) — blocked on integrations that do not exist.
- Any UI redesign of execution / artifact / audit surfaces beyond what verification needs.
