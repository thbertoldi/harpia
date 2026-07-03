## ADDED Requirements

### Requirement: Chat-launched transient artifact preview
The chat thread SHALL launch a transient slide-over artifact preview (overlaying the chat column, which remains full-width when closed) when an artifact is selected, rather than requiring navigation to a separate route.

#### Scenario: Inline card opens the slide-over
- **WHEN** an `ARTIFACT_CREATED` or `ARTIFACT_UPDATED` event renders in the thread
- **THEN** it appears as an inline `ArtifactCard`
- **AND** selecting it opens the `ArtifactPreviewSheet` over the chat column without leaving the thread

#### Scenario: Only one artifact is previewed at a time
- **WHEN** an artifact is already open in the sheet and the user selects another
- **THEN** the sheet swaps to the newly selected artifact
- **AND** the previously active card is no longer marked active

### Requirement: Extensible per-ArtifactType renderer registry
Artifact preview SHALL be rendered by a registry keyed by preview kind, supporting at least `html`, `markdown`, `image`, `text`, `json`, and `list`, with the server selecting the variant by `ArtifactType`.

#### Scenario: HTML artifact renders in a sandboxed iframe
- **WHEN** the preview variant is `html_preview`
- **THEN** the Preview tab renders the HTML in an `<iframe>` using `srcdoc`
- **AND** the iframe `sandbox` attribute does NOT include `allow-scripts`

#### Scenario: Markdown artifact renders as formatted text
- **WHEN** the preview variant is `markdown_preview`
- **THEN** the Preview tab renders headings, quotes, lists, and code blocks as formatted markdown

#### Scenario: Unknown or empty preview degrades gracefully
- **WHEN** the preview variant is unhandled or empty
- **THEN** the sheet shows an empty/placeholder state instead of erroring

### Requirement: Preview and code tabs with actions
The preview sheet SHALL provide Preview and Code tabs, a copy-source action, and a reload action for HTML previews, plus a version badge and footer status.

#### Scenario: Code tab shows source
- **WHEN** the user selects the Code tab
- **THEN** a line-numbered source view of the artifact renders with minimal token highlighting

#### Scenario: Copy and reload actions
- **WHEN** the user activates Copy
- **THEN** the artifact source is copied and a "Copied" feedback is shown
- **WHEN** the artifact is HTML and the user activates Reload
- **THEN** the preview iframe re-mounts via a changed key

#### Scenario: Header and footer metadata
- **WHEN** the sheet renders an artifact
- **THEN** the header shows a token-coded type icon, title, version badge, and filename
- **AND** the footer shows the mime type and size

### Requirement: Server is the preview authority
The backend preview service SHALL return the appropriate preview variant per `ArtifactType`, keeping client-side payload-shape coupling out of the preview path.

#### Scenario: Variant selection by ArtifactType
- **WHEN** the client requests a preview for an artifact whose `ArtifactType` is HTML-like
- **THEN** the response carries the `html_preview` variant
- **AND** the client renders it without inspecting the raw payload shape

### Requirement: Motion-safe and localized preview UI
The preview sheet and inline card SHALL use only reduced-motion-aware opacity/transform transitions and SHALL resolve all user-facing copy through flat translation keys in both supported locales.

#### Scenario: Slide-over and generating states are motion-safe
- **WHEN** the sheet opens or an artifact is generating
- **THEN** only opacity or transform transitions animate
- **AND** the generating skeleton respects reduced-motion preferences

#### Scenario: Copy resolves in supported locales
- **WHEN** the sheet and card render in `en` or `pt-BR`
- **THEN** every label, action, status word, and aria-label resolves through flat `translate()` keys
- **AND** no user-facing string is a raw literal
