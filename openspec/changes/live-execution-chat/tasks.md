## 1. Execution View Model

- [x] 1.1 Add `buildExecutionViewModel(group, orderedSteps)` in `frontend/src/lib/plans/execution-view.ts` returning per-step status (`pending|running|done|failed`), execution state (`idle|running|completed|failed`), running step, `doneCount`, `total`, and `progress`.
- [x] 1.2 Implement the folding rules from design §2 in `sequenceNumber` order: `STEP_STARTED`→running (settling any prior running step), `STEP_BOUND`→done, `RUN_FAILED`→running step failed, `RUN_COMPLETED`→settle remaining running→done.
- [x] 1.3 Add unit tests covering: idle, single-step running, sequential done progression, failure mid-run, run-completed settle, two-group isolation, and step-before-run.
- [x] 1.4 Degrade gracefully when the ordered step list is empty (summary-only `degraded` state) and when step keys are unknown — tested.

## 2. PlanExecutionCard UI

- [x] 2.1 Implement `frontend/src/lib/components/thread/PlanExecutionCard.svelte`: collapsible header (`done/total` badge, current-step/final-state subtitle), progress bar (energy gradient while running, status-done when complete, danger when failed), and a step timeline with status icons + connector line.
- [x] 2.2 Render status icons purely with ADR-016 tokens (`--color-status-running/done/pending`, `--color-danger`, `--color-energy`); no raw hex.
- [x] 2.3 Expand the latest execution by default; collapse prior executions (`initiallyCollapsed` prop).
- [x] 2.4 Show a `failed` step with the danger token and a localized failure subtitle; earlier steps remain `done`.
- [x] 2.5 Behavior covered by the view-model unit tests (`execution-view.test.ts`) — the repo uses logic-only `.test.ts` tests (no rendered-component testing library is installed), so the card is presentational over the tested view model.

## 3. Header Status Pill And Affordances

- [x] 3.1 Add an `anyExecutionRunning` derived in `frontend/src/routes/chat/[threadId]/+page.svelte`; render an "Executing…" (energy, pulsing dot) / "Ready" (neutral) status pill in the configured branch.
- [x] 3.2 Replace the "Thinking about a plan…" CSS pulse with a reduced-motion-safe typing-dots indicator (animate transform/opacity only).
- [x] 3.3 Running steps show the energy spinner inside `PlanExecutionCard` (animate transform only; respect the `app.css` reduced-motion guard).
- [x] 3.4 New execution cards enter with the existing reduced-motion-safe `chatEnter` rise transition.

## 4. Localization

- [x] 4.1 Add all `PlanExecutionCard`, status-pill, typing, and failure copy to `frontend/src/lib/i18n/en.json` and `pt-BR.json` under flat `thread.execution.*` / `thread.status.*` keys.
- [x] 4.2 Step titles render from the template step `title` (dynamic, not a literal); localized `catalog.plan.<key>.step.<step_key>.*` resolution is a future refinement.

## 5. Verification

- [x] 5.1 Run `openspec validate --changes live-execution-chat` (passes strict).
- [x] 5.2 Run `cd frontend && bunx vitest run` — 374 tests pass (including 14 new `execution-view` tests).
- [x] 5.3 Run `cd frontend && bun run check` — 12 errors / 5 warnings (the known baseline; no new diagnostics).
- [x] 5.4 Run `cd frontend && bun run lint` (prettier + eslint, whole project) — clean.
- [x] 5.5 ADR-016 tokens are merged in this same pass; the card consumes `--color-status-*` / `--color-energy`.
- [ ] 5.6 Smoke the configured-thread flow manually with a live run: observe pending→running→done per step, the header pill, and failure rendering.
