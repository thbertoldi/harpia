# artifact-preview-panel Specification

## Purpose
TBD - created by archiving change artifact-preview-panel. Update Purpose after archive.
## Requirements
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

### Requirement: Inline card mirrors panel generating and active state
The inline `ArtifactCard` SHALL visibly reflect the panel's state for the artifact it
represents: a *generating* affordance while that artifact is being produced, and an
*active* affordance while that artifact is the one currently shown in the preview panel.

#### Scenario: Card shows generating while its artifact is produced
- **WHEN** the artifact an inline card represents is currently being produced by a running step
- **THEN** the card shows a generating affordance (spinner + generating caption) in place of
  the open action

#### Scenario: Active card is distinguished from the openable state
- **WHEN** the artifact an inline card represents is the one currently open in the preview panel
- **THEN** the card is marked active (accent) and its affordance reads as "Open"
- **AND** other artifact cards read as "Preview" and are not marked active

### Requirement: Generating state renders a content skeleton
The preview panel body SHALL render a content skeleton (not a bare text label) while an
artifact is generating, using reduced-motion-aware opacity/transform animation only.

#### Scenario: Skeleton while generating
- **WHEN** the panel is open for an artifact that is still generating
- **THEN** the body renders an animated content skeleton
- **AND** the animation is opacity/transform only and is softened or suppressed under
  reduced-motion preferences

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
The preview sheet SHALL provide Preview and Code tabs, a copy-source action, and a reload action for HTML previews, plus an iteration/version badge and footer status.

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
- **THEN** the header shows a token-coded type icon, title, filename, and — when an
  iteration signal exists — a version/iteration badge
- **AND** the footer shows the mime type and size

#### Scenario: Iteration badge on a revised or repeated artifact
- **WHEN** an artifact has more than one content revision, or is produced by a repeated run of
  the same configuration for its artifact type
- **THEN** the sheet header and the inline card show a matching iteration badge
- **AND** the iteration indicator is derived from artifact content revisions and/or the
  ordinal of the producing execution among repeated runs of the same configuration

#### Scenario: No badge on a first, unrevised artifact
- **WHEN** an artifact has a single revision and no prior run for its type
- **THEN** no iteration badge is shown

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

