## Context

This change executes Phase 0 of the consolidation
([roadmap §3](../../../docs/architecture/mvp-roadmap.md), [cleanup backlog](../../../docs/architecture/cleanup-backlog.md)).
It is deliberately a *cleanup + correctness* change: it removes contradictions, duplication,
and dead code so later phases (Resource layer, rich content) start from a clean base. Pre-v1,
so breaking proto/route changes are acceptable with no migration shim.

Current state, per the verified audit:
- A PlanConfiguration's owning thread is resolved two ways with two different fallbacks
  (`configurationThreadID` reads `origin_thread_id` else `thread_id`; `resolveThreadID` reads
  `thread_id` only and falls back to `configID.String()`). Masked today only because both
  proto fields get the same value at creation.
- A LinkedIn-hardcoded frontend input model duplicates the server's generic materializer.
- The example template `weekly-newsletter-linkedin` bakes cadence/channel/format into identity;
  its `DateRange` is a hardcoded `last_7_days` preset.
- A large orphaned PlanCanvas cluster and a few other orphaned components/mocks exist.
- New protos still carry legacy `subtask_*` nouns.

## Goals / Non-Goals

**Goals:**
- One owning-thread resolver; no config-id-as-thread fallback (C1).
- Frontend template inputs driven generically from `TemplateInputParameter[]` (C2).
- Example template renamed to its outcome; cadence via schedule; `DateRange` derived from the
  schedule window (T1).
- Remove dead code: PlanCanvas cluster, `ArtifactRail`, `TextArtifactEditor`, dead e2e mocks,
  stale comments (D1–D4).
- Proto `subtask_*` → `step_*` in budget/mcp (C4).

**Non-Goals:**
- Multi-channel publishing / the channel-discriminated artifact family (post-MVP; constitution §7.6).
- Any Resource-layer, rich-content, or browser-surface work (later phases).
- Changing the execution engine, Temporal topology, or the adaptive path.

## Decisions

- **Collapse to `configurationThreadID` as the single resolver.** Route runtime event emission
  (`runtime.go`) through the same resolver used by assistant/approval/elicitation handlers;
  delete `resolveThreadID` and its `configID.String()` fallback and `GetThreadIDForConfiguration`.
  *Why:* `configurationThreadID` already encodes the correct precedence (`origin_thread_id` else
  `thread_id`); the runtime path is the buggy outlier. *Alternative rejected:* keep both and make
  them agree — leaves two code paths to drift again.
- **Prefer one owning-thread field.** Since ADR-017 guarantees `origin_thread_id` is always set,
  keep `origin_thread_id` as the canonical owning-thread and drop the redundant `thread_id`
  field from `plans.proto` (pre-v1). *Alternative:* keep both — perpetuates the ambiguity.
- **Reuse `TemplateInputsForm` for `BindingMatrixCard`.** Both should consume
  `TemplateInputParameter[]`; delete `template-inputs.ts` LinkedIn model,
  `linkedin-template-inputs.ts`, and `applyLinkedInSuggestion`. *Why:* the server materializer is
  already generic; the frontend model is pure duplication that blocks a second template.
- **Rename template + derive date window from schedule.** Rename `weekly-newsletter-linkedin`
  to an outcome key; move cadence to `PlanSchedule`; resolve the `DateRange` seed at run start
  from the schedule cadence (recurring) or explicit range (one-shot). *Why:* removes false
  specificity now, cheaply, without the post-MVP channel work.
- **Delete dead clusters wholesale.** PlanCanvas + `canvas-state.ts` + `SettingsDrawer`/
  `CanvasDrawer` (keep the still-live `CanvasApprovalForm`/`ScheduleDialog`), `ArtifactRail`,
  `TextArtifactEditor`, dead e2e branches, stale comments.
- **Rename proto nouns.** `subtask_key`/`SubtaskCostEstimate`/etc. → `step_*` in
  `harpia.budget.v1`/`harpia.mcp.v1`; regenerate Go + TS and fix references.

## Risks / Trade-offs

- **Thread-resolver change touches emission for existing runs** → land C1 first, verify with an
  e2e run that produces both a `STEP_*` event and an approval, asserting one thread id.
- **Template rename breaks references to the old key** (frontend suggestions, tests, seeds) →
  grep for `weekly-newsletter-linkedin` and update all; the existing e2e journey is the safety net.
- **Proto breaking changes** (`buf breaking` will flag them) → expected and accepted pre-v1;
  regenerate clients in the same change so the tree stays green.
- **Deleting the LinkedIn input model could regress the golden config path** → the
  `conversational-configuration` flow-model tests and the engineer e2e journey must pass after
  rewiring to `TemplateInputParameter[]`.

## Migration Plan

Pre-v1, no data migration. Suggested landing order (each independently verifiable):
1. C1 resolver + proto field drop → regenerate → `go test ./...` + e2e thread assertion.
2. D1–D4 dead-code deletion → `bun run build`/`check`, grep for dangling imports.
3. C2 input-model dedup → conversational-config tests + e2e journey.
4. T1 rename + date-from-schedule → one-shot and weekly runs, window matches cadence.
5. C4 proto noun rename → regenerate → `buf lint`, `go test`, `bun run check`.

## Open Questions

- Outcome name for the renamed template (e.g. `news-to-social-post` vs `source-driven-post`) —
  pick during implementation; must update all references and localization keys.
- Whether the dropped `thread_id` proto field number should be reserved vs reused (pre-v1: reuse
  is fine, but reserving avoids confusion in any cached generated clients).
