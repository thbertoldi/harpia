## Why

ADR-012 positions UI surfaces as **"browsers over the Artifact stream"**, and artifacts are
the primary output of every plan. Today the chat's artifact affordance is weak: only three
preview renderers exist (`text`, `list`, `json` — `frontend/src/lib/artifacts/preview.ts`),
there is no `html`/`markdown`/`image` rendering, and the preview lives in a persistent
right-column swap (`ConversationalWorkspace.svelte:74-104`), not a transient panel launched
from the conversation. `ARTIFACT_CREATED/UPDATED` events are not richly rendered into a
first-class inline card. A reference design (Fable "Forge Agent") shows the target: an inline
artifact card that opens a transient side panel with preview/code tabs, a version badge,
copy, and reload — a genuinely useful preview of what the agent produced.

## What Changes

- Add a transient **artifact preview slide-over** (`ArtifactPreviewSheet`) that reuses the
  existing bespoke right-side slide-over pattern (`frontend/src/lib/components/canvas/CanvasDrawer.svelte`),
  launched from the chat thread (not the persistent rail). Header carries type icon, title,
  version badge, and filename; tabs switch **Preview** and **Code**; actions include copy
  source and (for HTML) reload; a footer status bar shows mime + size.
- Add a **renderer registry** so preview is extensible by `ArtifactType`. Extend the preview
  client model (`ArtifactPreviewKind`) and `formatArtifactPreview` to include `html`,
  `markdown`, and `image`, keeping `text`/`json`/`list`. Renderers: `html` → sandboxed iframe,
  `markdown` → rendered markdown, `image` → `<img>`, plus a line-numbered **code view** with
  minimal highlighting for any type.
- Add an inline **`ArtifactCard` chat message** tied to `ARTIFACT_CREATED/UPDATED` events that
  opens the slide-over on click (new branch in `ThreadMessage.svelte`).
- Extend the artifact preview proto with server-side preview variants
  (`html_preview`, `markdown_preview`, `image_preview`) so the server remains the preview
  authority (per ADR-012), selected by `ArtifactType`.
- Keep the existing `/artifacts/[artifactId]` full-detail route as the "open full" target,
  linked from the slide-over.

## Capabilities

### New Capabilities
- `artifact-preview-panel`: The chat launches a transient artifact preview slide-over with
  preview/code tabs, version badge, copy, and reload, driven by an extensible per-`ArtifactType`
  renderer registry backed by server-side preview variants; `ARTIFACT_CREATED/UPDATED` events
  render as inline cards that open it.

## Impact

- Proto: add `html_preview`/`markdown_preview`/`image_preview` to the `PreviewArtifactResponse`
  oneof (`proto/harpia/artifacts/v1/artifacts.proto:228-233`); regenerate. Additive, pre-v1.
- Backend: the artifact preview service returns the correct variant by `ArtifactType`
  (`control-plane` artifact preview handler).
- Frontend: new `ArtifactPreviewSheet.svelte` (reusing `CanvasDrawer`), renderer registry +
  renderers (`ArtifactPreview.svelte` / `preview.ts`), inline `ArtifactCard` message +
  `ThreadMessage.svelte` routing, markdown renderer (introduce or reuse if present).
- Tokens: consumes ADR-016 type/status accents; MUST NOT land before ADR-016.
- Tests: Go tests for preview-variant selection; frontend tests for the registry mapping,
  sandbox attributes, and card→sheet launch; copy in `en` and `pt-BR`.
