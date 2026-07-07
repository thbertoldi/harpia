# Tasks — live-artifact-continuity

## 1. Early generating-state open (item 1)
- [x] 1.1 Add an early-open predicate in `frontend/src/lib/artifacts/preview.ts` that answers
  "the emphasized/final artifact is currently being produced" from the execution view model
  (producing step `running`, final artifact not yet ready), reusing `finalArtifactTypeKeys`.
- [x] 1.2 In `frontend/src/routes/chat/[threadId]/+page.svelte`, drive `previewOpen` + a
  `generating` flag off the predicate; keep `previewDismissedFinalArtifactId` suppression;
  ensure the existing final-artifact auto-open still takes over on arrival.
- [x] 1.3 Logic test: predicate opens while producing, respects dismissal, and yields to the
  existence-based open once the artifact exists.

## 2. Inline card ↔ panel sync (item 2)
- [x] 2.1 Add `active` and `generating` props to
  `frontend/src/lib/components/artifacts/ArtifactCard.svelte`; accent + "Open"/"Preview"
  affordance for active; spinner + generating caption for generating.
- [x] 2.2 Pass `activeArtifactId` and the generating id from the chat route into every
  `ArtifactCard` (inline thread cards and any workspace cards). Also extended the same
  active/generating treatment to `PlanExecutionCard`'s per-step row, since STEP_BOUND
  messages always carry a non-empty `executionId` and therefore never reach
  `SystemEventCard`'s `ArtifactCard` branch — `PlanExecutionCard`'s Eye-button row is the
  only inline artifact affordance actually visible during a live run.
- [x] 2.3 Test/assert card state classes/labels for generating and active (via the extracted
  `artifactCardStatusLabelKey` / `artifactCardOpenActionKey` pure helpers — no
  component-rendering test infra exists in this repo).

## 3. Collapsed execution header current step (item 3)
- [x] 3.1 Expose `currentStep` (title + detail) on the model from
  `frontend/src/lib/plans/execution-view.ts` when running.
- [x] 3.2 Render current step + `done/total` + thin progress bar in the collapsed header of
  `frontend/src/lib/components/thread/PlanExecutionCard.svelte`.
- [x] 3.3 View-model test: `currentStep` reflects the running step and clears on completion.

## 4. Generating skeleton (item 4)
- [x] 4.1 Add a reduced-motion-aware shimmer skeleton body state to
  `frontend/src/lib/components/artifacts/ArtifactPreviewSheet.svelte`, gated by the
  generating flag (opacity/transform only).

## 5. Version / iteration badge (item 5)
- [x] 5.1 Derive an iteration indicator (content revisions and/or repeated-execution ordinal);
  omit when no signal exists.
- [x] 5.2 Render the badge on the inline card and the sheet header.

## 6. i18n + verification
- [x] 6.1 Add flat keys for generating, open/preview, and iteration copy in
  `frontend/src/lib/i18n/{en,pt-BR}.json`; no raw literals.
- [x] 6.2 `cd frontend && bun run lint` and `bun run check` (no new baseline errors); run the
  touched vitest logic files.
- [x] 6.3 `openspec validate live-artifact-continuity --strict`.
