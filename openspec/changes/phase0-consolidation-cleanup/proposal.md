## Why

The Phase 0 consolidation ([Platform Constitution](../../../docs/architecture/harpia-platform.md),
[Cleanup Backlog](../../../docs/architecture/cleanup-backlog.md)) identified contradictions,
duplication, and dead code that make every later phase riskier. The highest-value items are a
**latent correctness bug** (a PlanConfiguration's owning thread is resolved two different ways,
so an autonomous run's events can split onto a different thread than its approvals) and two
**genericity blockers** (a frontend LinkedIn-hardcoded input model that duplicates the server
materializer and blocks a second template, and an example template that bakes cadence/channel/
format into its identity). Clearing these now unblocks the Resource layer and rich-content work.

## What Changes

- **C1 (bug):** Collapse the two owning-thread resolvers (`configurationThreadID` vs
  `resolveThreadID`) into one; delete the legacy config-id fallback so runtime `RUN_*`/`STEP_*`
  events and assistant/approval/elicitation events for a configuration always target the same
  thread. **BREAKING** (pre-v1): removes the config-id-as-thread fallback and one of the two
  owning-thread proto fields.
- **C2/DUP1:** Drive `BindingMatrixCard` from the generic `TemplateInputParameter[]` (as
  `TemplateInputsForm` already does) and delete the LinkedIn-hardcoded input model; the server
  owns materialization so a second template configures with no code change.
- **T1 (template factoring):** Rename `weekly-newsletter-linkedin` to an outcome name; move
  cadence out of template identity into `PlanSchedule`, and derive the `DateRange` seed from the
  schedule window instead of a hardcoded `last_7_days` (constitution §7.6).
- **D1:** Delete the orphaned PlanCanvas cluster (~8 components + `canvas-state.ts` +
  `SettingsDrawer`/`CanvasDrawer` + tests); the chat page's `previewOpen`/`activeArtifactId`
  machine is the single artifact-preview state.
- **D2/D3/D4:** Delete orphaned `ArtifactRail`/`TextArtifactEditor`, dead e2e plan-thread RPC
  mock branches, and stale proto/doc comments referencing removed RPCs.
- **C4:** Rename legacy `subtask_*` proto nouns (`subtask_key`, `SubtaskCostEstimate`, …) to
  `step_*` in `harpia.budget.v1`/`harpia.mcp.v1` for taxonomy consistency. **BREAKING** (pre-v1).

## Capabilities

### New Capabilities
<!-- none — this is a consolidation/cleanup change; no new capability is introduced -->

### Modified Capabilities
- `plan-execution-runtime`: single, unambiguous owning-thread resolution for a
  PlanConfiguration (C1); the run's `DateRange` seed is derived from the schedule/run window
  rather than a hardcoded preset (T1).
- `conversational-configuration`: template inputs are rendered and materialized generically
  from `TemplateInputParameter[]` — no per-template (LinkedIn-specific) hardcoding on the
  frontend (C2).
- `plan-template-authoring`: a PlanTemplate's identity is its outcome; cadence/channel/format
  are configuration axes, not baked into the template key or copy (T1).

## Impact

- **control-plane:** `internal/plans/{handler,runtime,repository}.go` (thread resolver),
  `internal/plans/materialize.go` + template YAML (date-from-schedule, rename),
  `internal/{budget,mcp}` proto-facing code (C4).
- **proto:** `harpia/plans/v1` (drop one owning-thread field), `harpia/budget/v1` +
  `harpia/mcp/v1` (`subtask_*` → `step_*`), `harpia/chat/v1` comment cleanup. Regenerate Go + TS.
- **frontend:** delete PlanCanvas cluster, `ArtifactRail`, `TextArtifactEditor`; rewire
  `BindingMatrixCard` to `TemplateInputParameter[]`; delete the LinkedIn input model and dead
  e2e mock branches.
- **Verification gates:** `go test ./...`, `bun run lint`/`check`, `buf lint`/`buf breaking`
  (breaking is expected/accepted pre-v1), e2e engineer journey.
- **Docs:** closes backlog items C1, C2, C4, D1–D4, T1.
