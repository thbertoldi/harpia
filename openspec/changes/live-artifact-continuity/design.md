# Design — live-artifact-continuity

## Context

Source of the idea: a reference prototype (Fable "Forge Agent" chat + artifact mock,
reviewed 2026-07-07). It is a *coding agent* clone, so most of it does not transfer to
Harpia's operations domain. The transferable insight is **continuity**: the plan step,
the inline card, and the side panel should behave as one synchronized signal about a
single artifact moving from "being produced" → "ready".

What deliberately does **not** transfer (and is out of scope here):

- Syntax-highlighted code view / source tab — our artifacts are typed payloads, not files.
- HTML `srcdoc` sandboxed `<iframe>` rendering — same reason.
- Asset/image generation and executor changes — separate idea log entry.

## Current behavior (baseline)

- `frontend/src/routes/chat/[threadId]/+page.svelte` auto-opens the panel only once the
  final artifact **exists**, via `shouldAutoOpenFinalArtifact(artifactId, dismissedId)`
  (`frontend/src/lib/artifacts/preview.ts`). Result: the panel pops in abruptly at the end.
- `ArtifactCard.svelte` renders title + status label + an `Eye` open button. It has no
  notion of "this is the artifact currently open" or "this artifact is still generating".
- `ArtifactPreviewSheet.svelte` body shows a plain "Loading…" string while `busy`; the
  footer shows only generating/ready; the header has no version badge.
- `PlanExecutionCard.svelte` is `initiallyCollapsed`; the `live-execution-chat` spec already
  requires the header to show the running step's detail, but the collapsed card is silent.
- `buildExecutionViewModel` (`frontend/src/lib/plans/execution-view.ts`) computes a
  `state` (`running`/…) but does not expose which step is currently running for header use.

## Approach

### 1 + 4 — Early open + skeleton (the core continuity move)

Drive panel-open off *execution progress*, not artifact existence. Add a predicate that
answers "is the emphasized/final artifact currently being produced?" from the execution
view model (the producing step is `running` and no ready final artifact exists yet). When
true, the chat route sets `previewOpen = true` with an explicit `generating` flag so the
sheet renders the skeleton body. When the real artifact arrives, the existing
`finalArtifact` derivation + auto-open effect take over and the body swaps to the ready
render. Dismissal memory (`previewDismissedFinalArtifactId`) must still suppress re-open
for an artifact the user closed — the early-open path checks the same memory.

Skeleton: a small stack of shimmer bars (opacity/transform animation only, reduced-motion
aware). This is a body state of `ArtifactPreviewSheet`, gated by the generating flag, and
reuses the token accents already in the footer status dot.

### 2 — Inline card ↔ panel sync

Thread two optional inputs into `ArtifactCard`: `active` (its id === the panel's active id
while open) and `generating` (its id === the id being produced). `active` → accent
border/ring + the open affordance reads "Open" instead of "Preview"; `generating` → the
`Eye` swaps for a spinner and the status line reads a generating caption. The chat route is
the single owner of `activeArtifactId` + the generating id and passes both down, mirroring
how it already owns the canonical panel state.

### 3 — Collapsed execution header

Expose `currentStep` (title + detail) on the execution view model when `state === "running"`.
`PlanExecutionCard`'s collapsed header renders it plus the existing `done/total` and a thin
progress bar, so the card is informative without expansion. This realigns the shipped card
with the existing `live-execution-chat` requirement rather than inventing new behavior.

### 5 — Version / iteration badge

Our artifacts have no code-style `v1/v2`. Derive an iteration indicator from what we do
have: (a) content revisions via `saveTextArtifactVersion` / `contentHash` history for
editable drafts, and (b) the ordinal of the producing execution among repeated runs of the
same configuration for a given artifact type. The badge is presentational; if neither
signal is available it is omitted (no "v1" noise on first drafts). Shown on both the inline
card and the sheet header for consistency.

## Risks / trade-offs

- **False-early open.** If the producing-step detection is wrong, the panel could open for a
  run that never yields the emphasized artifact. Mitigation: only trigger for the step that
  maps to a final artifact type (reuse `finalArtifactTypeKeys`), and always fall back to the
  existing existence-based open.
- **Flicker on fast runs.** A run that completes almost instantly could show skeleton →
  content in one frame. Acceptable; the swap is opacity-only and reduced-motion safe.
- **Iteration semantics ambiguity.** Deliberately conservative: omit the badge unless a real
  revision/repeat signal exists, rather than fabricate a version number.

## Out of scope / follow-ups

- Code/source tab and iframe rendering remain unshipped by choice; if ever needed they are a
  separate change, not this one.
- Richer LinkedIn artifact formats (carousel/image) are tracked in `docs/notes/product-ideas.md`.
