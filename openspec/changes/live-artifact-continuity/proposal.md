## Why

`stabilize-chat-plan-journey` landed one canonical artifact preview panel and inline
artifact launchers, but the artifact/execution surfaces still read as *disconnected
snapshots* rather than a *live, continuous* picture of what the agent is doing. A
reference prototype (the Fable "Forge Agent" chat-with-artifact mock) makes the gap
concrete: it opens the side panel the instant the producing step starts running (as a
skeleton), keeps the inline card visibly in lock-step with the panel, surfaces the
current running step in the collapsed plan card, and badges each artifact revision.

Notably, three of these behaviors were **already specified** by earlier changes but were
dropped when the shipped implementation was simplified:

- `live-execution-chat` requires "the card header shows the running step's detail text",
  yet `PlanExecutionCard` goes quiet when collapsed.
- `artifact-preview-panel` requires a generating **skeleton** and a **version badge**,
  yet `ArtifactPreviewSheet` renders a bare "Loading…" label and no version indicator.

This change re-scopes those dropped guarantees plus two genuinely new continuity
behaviors, filtered for our domain: our artifacts are typed operations payloads
(`TextDraft`, `LinkedInPostDraft`, receipts), **not** source files — so we deliberately
do **not** adopt the prototype's code-editor / sandboxed-`<iframe>` rendering.

## What Changes

1. **Early generating-state open (new).** When the step that produces the emphasized/final
   artifact enters `running`, the canonical preview panel pre-opens in a generating state
   instead of popping in only after the artifact exists. It swaps to the ready artifact when
   it arrives, and still honors the user's dismissal memory.
2. **Inline card ↔ panel sync (new).** The inline `ArtifactCard` reflects a *generating*
   state (spinner + generating caption) while its artifact is being produced, and an
   *active/open* state (accent + "Open" affordance vs "Preview") when it is the artifact
   currently shown in the panel.
3. **Collapsed execution header shows the current step (spec realignment).** The
   `PlanExecutionCard`, even collapsed, surfaces the current running step's title/detail and
   a `done/total` progress bar so live progress is legible without expanding.
4. **Generating skeleton (spec realignment).** The preview panel body renders a
   reduced-motion-aware content skeleton while generating, not a bare text label.
5. **Version / iteration badge (spec realignment + domain fit).** The inline card and panel
   header show an iteration indicator derived from content revisions
   (`saveTextArtifactVersion` / `contentHash` history) and/or repeated executions of the same
   configuration, so users can tell "draft 2" from "draft 1".

Explicitly out of scope: code/source tabs, HTML `srcdoc` iframe rendering, and any
executor/asset-generation work (tracked separately under the "Rich LinkedIn content plan
variants" idea).

## Capabilities

### Modified Capabilities

- `artifact-side-preview`: the side preview opens early in a generating state tied to the
  producing step, rather than only after the final artifact exists.
- `artifact-preview-panel`: inline cards mirror the panel's generating/active state; the
  panel body renders a generating skeleton; the version badge carries domain iteration
  semantics.
- `live-execution-chat`: the collapsed execution card surfaces the current running step and
  progress.

## Impact

- Frontend components: `frontend/src/lib/components/artifacts/ArtifactPreviewSheet.svelte`
  (skeleton body, version badge), `frontend/src/lib/components/artifacts/ArtifactCard.svelte`
  (generating + active props/state), `frontend/src/lib/components/thread/PlanExecutionCard.svelte`
  (collapsed header), `frontend/src/routes/chat/[threadId]/+page.svelte` (auto-open effect,
  `openArtifact`, `startRun`, wiring active/generating into cards).
- Frontend helpers: `frontend/src/lib/artifacts/preview.ts`
  (`shouldAutoOpenFinalArtifact` / early-open predicate), `frontend/src/lib/plans/execution-view.ts`
  (expose current running step on the view model).
- i18n: new/adjusted flat keys in `frontend/src/lib/i18n/{en,pt-BR}.json` for generating,
  active/open, and iteration copy.
- Tokens: opacity/transform-only animation (shimmer, spinner) — design tokens stay LOCKED.
- No proto/backend/schema changes; frontend-only, pre-v1.
- Tests: logic tests for the early-open predicate, the execution view-model current-step
  field, and iteration derivation; card state (generating/active) assertions; hardcoded-copy
  coverage in `en` + `pt-BR`.
