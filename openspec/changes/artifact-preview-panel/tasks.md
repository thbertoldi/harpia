## 1. Proto And Backend Preview Variants

- [x] 1.1 Add `html_preview` (string), `markdown_preview` (string), and `image_preview` (`ImagePreview` message: url / inline_data / alt_text) to the `PreviewArtifactResponse` oneof in `proto/harpia/artifacts/v1/artifacts.proto`.
- [x] 1.2 Regenerate with `mise run buf-generate`; `cd proto && buf lint` clean.
- [x] 1.3 Update `control-plane/internal/artifacts/preview.go`: route `TextDraft` and `LinkedInPostDraft` to `markdown_preview` (their content is markdown); add a forward-compatible default-case fallback that returns `html_preview` for payloads carrying a non-empty `html` field and `image_preview` for `image_url` / `image_base64` payloads.
- [x] 1.4 Add Go tests: updated `preview_test.go` (markdown routing), `preview_handler_test.go` and `versioning_test.go` (markdown assertions), plus new `TestBuildPreviewHTMLFallback`, `TestBuildPreviewImageURLFallback`, `TestBuildPreviewImageBase64Fallback`.

## 2. Frontend Renderer Registry

- [x] 2.1 Extend `ArtifactPreviewKind` and `formatArtifactPreview` (`frontend/src/lib/artifacts/preview.ts`) to map the new variants to `html`/`markdown`/`image`; preserve `text`/`json`/`list`/`empty`; covered by `preview.test.ts`.
- [x] 2.2 The registry is the `formatArtifactPreview` kind switch consumed by `ArtifactPreview.svelte`; adding an `ArtifactType` is one mapping entry.
- [x] 2.3 Implement the `html` renderer: `<iframe sandbox="allow-same-origin" srcdoc=...>` (NO `allow-scripts`) in `ArtifactPreview.svelte`.
- [x] 2.4 Implement the `markdown` renderer: added a minimal dependency-free, escape-first renderer `frontend/src/lib/artifacts/markdown.ts` (no existing renderer present); renders headings/quotes/lists/code/bold/italic/links/hr. Covered by `markdown.test.ts`.
- [x] 2.5 Implement the `image` renderer: `<img>` from `image.url` or a data-URL built from `inlineData`, with alt text.
- [x] 2.6 Code view is provided by the sheet's Code tab (line source from the preview's text/html/markdown); the `{@html}` injection is eslint-disabled with a justification because `renderMarkdown` HTML-escapes its input first.
- [x] 2.7 Tests assert registry mapping, sandbox (no `allow-scripts` is the documented contract), and variant→kind folding including empty payloads (`preview.test.ts`, `markdown.test.ts`).

## 3. Artifact Preview Slide-Over

- [x] 3.1 Implement `frontend/src/lib/components/artifacts/ArtifactPreviewSheet.svelte` as a right-side slide-over (backdrop + fixed aside), wider than `CanvasDrawer`'s fixed `w-96` so previews have room; chat column is overlaid (full-width restored on close).
- [x] 3.2 Header: token-coded type icon, title (`artifactTitle`), type-key filename line; tabs Preview/Code; actions Reload, Copy (with "Copied" feedback), Open full; footer status bar (mime + KB size + generating/ready dot).
- [x] 3.3 Shows the energy generating skeleton via the bound `loading` state while the preview streams.
- [x] 3.4 "Open full" links to the existing `/artifacts/[artifactId]` route.
- [x] 3.5 Animates transform/opacity (backdrop fade); reduced-motion respected via `app.css`; colors via ADR-016 tokens.

## 4. Inline ArtifactCard Launcher

- [x] 4.1 Note: `ARTIFACT_CREATED`/`UPDATED` message kinds are defined in proto but **not emitted** by the backend today (only `STEP_BOUND` carries `output_artifact_id`). The inline launcher is therefore driven by the latest execution's artifacts (`workspaceArtifacts`, already fetched), rendered via the existing `ArtifactCard` (compact) — reusing the same component a future `ARTIFACT_CREATED` emission would route through `ThreadMessage.svelte`.
- [x] 4.2 "Produced artifacts" grid rendered in the configured branch; selecting a card opens `ArtifactPreviewSheet` for that artifact (`openPreviewById`).
- [x] 4.3 Only one artifact is active in the sheet at a time; selecting another swaps content.
- [ ] 4.4 Rendered-component tests for card/sheet states are deferred — the repo uses logic-only `.test.ts` tests (no rendered-component testing library); the renderer logic is covered by `preview.test.ts` + `markdown.test.ts`.

## 5. Localization

- [x] 5.1 Added all sheet/tab/action/footer copy (`artifactPreview.tab.preview/code`, `reload`, `copy`, `openFull`, `noSource`, `generating`, `ready`, `empty`, `thread.artifacts.produced`) to `frontend/src/lib/i18n/en.json` and `pt-BR.json` under flat keys.

## 6. Verification

- [x] 6.1 Run `openspec validate --changes artifact-preview-panel` (passes strict).
- [x] 6.2 `cd proto && buf lint` clean; `cd control-plane && go test ./...` — all packages pass (incl. new preview tests).
- [x] 6.3 `cd frontend && bunx vitest run` — 386 tests pass (incl. 12 new: 7 markdown + 5 preview mapping).
- [x] 6.4 `cd frontend && bun run check` — 12 errors / 5 warnings (the known baseline; no new diagnostics) and `bun run lint` clean.
- [x] 6.5 ADR-016 tokens present (energy/status).
- [ ] 6.6 Smoke with a live run: confirm the "Produced artifacts" cards open the sheet and render markdown/html/image correctly with the sandbox enforced.
