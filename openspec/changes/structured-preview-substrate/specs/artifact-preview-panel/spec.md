## MODIFIED Requirements

### Requirement: Extensible per-ArtifactType renderer registry
Artifact preview SHALL be rendered by an allowlisted registry keyed by a server-selected structured `renderer_key` and supported renderer version. The registry SHALL support at least `table.v1`, `chart.flint.v1`, `carousel.v1`, `document.html`, `markdown`, `image`, and a JSON fallback; retained fixed variants SHALL normalize into that registry where compatible. The client SHALL not inspect a raw Artifact payload shape to choose a renderer, and an Artifact SHALL never select an arbitrary component URL, JavaScript module, or executable renderer.

#### Scenario: HTML artifact renders in a sanitized and CSP-constrained iframe
- **WHEN** the preview renderer key is `document.html`
- **THEN** the Preview tab renders server-sanitized HTML in an `<iframe>` using `srcdoc`
- **AND** the iframe sandbox attribute does NOT include `allow-scripts` and its CSP blocks network and form egress

#### Scenario: Markdown artifact renders as formatted text
- **WHEN** the preview renderer key is `markdown` or a retained `markdown_preview` is normalized into the registry
- **THEN** the Preview tab renders headings, quotes, lists, and code blocks as formatted markdown

#### Scenario: Unknown or empty preview degrades gracefully
- **WHEN** the renderer key is unhandled, its version/spec is invalid, or the preview is empty
- **THEN** the sheet shows an inspectable JSON fallback or an empty/placeholder state instead of erroring

### Requirement: Server is the preview authority
The backend preview service SHALL select and validate a structured renderer variant from the ArtifactType declaration and requested ArtifactVersion, keeping client-side payload-shape coupling out of the preview path. The frontend SHALL only dispatch a server response through its allowlisted renderer map and SHALL use JSON fallback for unknown structured responses.

#### Scenario: Variant selection by ArtifactType
- **WHEN** the client requests a preview for an ArtifactVersion whose ArtifactType declares `chart.flint.v1`
- **THEN** the response carries a validated `structured_preview` with renderer key `chart.flint.v1`
- **AND** the client renders it without inspecting the raw Artifact payload shape
