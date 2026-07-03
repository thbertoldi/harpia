## Context

Artifacts are central (ADR-012). The preview path today is:

- Proto `PreviewArtifactResponse` oneof has only `text_preview` / `list_summary` /
  `json_preview` (`proto/harpia/artifacts/v1/artifacts.proto:228-233`).
- Frontend `formatArtifactPreview` (`frontend/src/lib/artifacts/preview.ts`) maps those to
  `text`/`list`/`json`, rendered in `ArtifactPreview.svelte` (`text`→`<pre>`, `list`→titles,
  `json`→`<pre>`).
- The chat shows artifacts via a persistent `ArtifactRail` + `ArtifactDetailPanel` right-column
  swap inside `ConversationalWorkspace.svelte:74-104`, plus timeline "Open" buttons
  (`PlanActivityTimeline.svelte:35-43`). `ARTIFACT_CREATED/UPDATED` events are not rendered as
  first-class inline cards.
- A standalone library exists at `/artifacts` and `/artifacts/[artifactId]` (versions sidebar,
  optional `TextArtifactEditor`).
- There is no shadcn-svelte, but a bespoke right-side slide-over (`CanvasDrawer.svelte`) and a
  centered modal (`ScheduleDialog.svelte`) already exist.

So: the renderer set is minimal, the panel model is a persistent column rather than a
transient launch, and artifact events aren't first-class chat citizens.

## Goals / Non-Goals

**Goals:**
- A transient, chat-launched preview slide-over (preview/code tabs, version badge, copy,
  reload) reusable from any chat thread.
- An extensible per-`ArtifactType` renderer registry; add `html`, `markdown`, `image` while
  keeping `text`/`json`/`list`.
- A first-class inline `ArtifactCard` message for `ARTIFACT_CREATED/UPDATED` that opens the
  sheet.
- Keep the server as the preview authority (ADR-012) via new preview variants.

**Non-Goals:**
- In-place artifact editing inside the sheet (the `/artifacts/[id]` editor remains the edit
  surface).
- Replacing the persistent `ArtifactRail` in the configured workspace (it stays as a
  secondary affordance; the slide-over is the chat-launched primary).
- Installing shadcn-svelte (decided against; reuse the bespoke `CanvasDrawer`).
- Live co-editing / multi-user cursors on a preview.
- Video or arbitrary file mime previewing (only `html`/`markdown`/`image`/`text`/`json`/
  `list` for now).

## Decisions

1. **Transient slide-over reusing `CanvasDrawer`, not a new primitive.**

   Build `ArtifactPreviewSheet` on the existing right-side slide-over
   (`CanvasDrawer.svelte`), launched from chat and overlaying the chat column. The chat column
   stays full-width when closed. Reusing the bespoke drawer avoids introducing shadcn-svelte
   just for this.

   Alternative considered: upgrade the persistent right column. Rejected: the user-chosen
   direction is a Fable-style transient panel launched from the conversation.

2. **Server-side preview variants; client renderer registry maps them.**

   Extend the `PreviewArtifactResponse` oneof with `html_preview` (string), `markdown_preview`
   (string), and `image_preview` (a small message holding a URL or inline bytes). The backend
   preview handler selects the variant by `ArtifactType`. The frontend keeps a registry that
   maps preview kind → renderer component, so adding an `ArtifactType` later is one entry.

   Alternative considered: render HTML/markdown client-side from raw `GetArtifactPayload`
   bytes. Rejected: couples the client to payload shapes and bypasses ADR-012's "preview is a
   server capability". Additive proto fields are cheap pre-v1.

3. **HTML preview is sandboxed and script-restricted by default.**

   Render `html_preview` in an `<iframe sandbox="allow-same-origin">` (no `allow-scripts`) so
   styling renders but scripts cannot execute. A reload control re-mounts the iframe via a
   changing `key`. CSP: the frame source is `srcdoc` only; no remote frame loading.

   Alternative considered: `allow-scripts` for "live app" artifact types. Deferred — only
   opt-in per explicit, allowlisted `ArtifactType` in a later change; never the default.

4. **Code view is type-agnostic and line-numbered.**

   A single code view renders the source (from the raw payload or a `text`-equivalent variant)
   with line numbers and minimal token highlighting (tags/strings) — reuse the approach, not
   React code, from the reference. No full syntax-highlighting dependency.

   Alternative considered: a heavyweight highlighting library. Rejected: extra dependency for
   marginal gain; minimal highlighting suffices for a preview.

5. **Inline `ArtifactCard` is a new dispatch branch, not a new message kind.**

   Route `ARTIFACT_CREATED`/`ARTIFACT_UPDATED` in `ThreadMessage.svelte` to an `ArtifactCard`
   (type icon, title, version, filename + line count, "Preview"/"Open" affordance) that opens
   the slide-over on click. The card also shows a generating state while an artifact is still
   streaming (energy skeleton). No proto message-kind change.

   Alternative considered: open the persistent rail. Rejected: that's already the configured-
   mode affordance; the chat-launched sheet is the new primary.

6. **Localization and motion.**

   All sheet/card copy (tab labels, "Copy", "Copied", "Reload", "Generating…", "Open full",
   size/mime footer) via flat keys in `en` and `pt-BR`. Slide-over animates transform/opacity
   only (consistent with `CanvasDrawer`); reduced-motion-safe.

## Risks / Trade-offs

- **Untrusted HTML in `srcdoc`.** Even sandboxed, `allow-same-origin` without `allow-scripts`
  is safe for static styling; the risk is if a future type needs scripts. Mitigation: scripts
  are opt-in per allowlisted type only (decision 3); document the sandbox contract in the spec.
- **Large payloads.** Big HTML/markdown could jank the iframe or code view. Mitigation: the
  footer shows size; cap code-view virtualization if needed (deferred; preview payloads are
  bounded today).
- **Proto additive churn.** New oneof variants are additive but every consumer of
  `PreviewArtifactResponse` must handle the new cases. Mitigation: the `default` branch
  already returns `empty`; update `formatArtifactPreview` and add tests.
- **Markdown renderer choice.** Introducing one is a new dependency. Mitigation: prefer a
  minimal, dependency-light renderer; confirm none already exists in the frontend before
  adding.

## Migration Plan

Pre-v1; forward-only:
1. Proto: add preview variants; `buf generate`; backend handler returns the right variant by
   `ArtifactType`; Go tests.
2. Frontend: renderer registry + `html`/`markdown`/`image` renderers + code view; update
   `formatArtifactPreview`.
3. Frontend: `ArtifactPreviewSheet` (on `CanvasDrawer`) + inline `ArtifactCard` +
   `ThreadMessage.svelte` routing + i18n.
4. Lint/typecheck/test gates; confirm ADR-016 tokens are present.
