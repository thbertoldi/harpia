## MODIFIED Requirements

### Requirement: Extensible per-ArtifactType renderer registry
Artifact preview SHALL be rendered by a registry keyed by preview kind, supporting at least `html`, `markdown`, `image`, `text`, `json`, and `list`, with the server selecting the variant by `ArtifactType`. The registry SHALL cover the `CarouselDraft` and `ImageAsset` artifact types by mapping them onto the existing `markdown_preview` and `image_preview` variants respectively, without introducing a new preview transport variant.

#### Scenario: HTML artifact renders in a sandboxed iframe
- **WHEN** the preview variant is `html_preview`
- **THEN** the Preview tab renders the HTML in an `<iframe>` using `srcdoc`
- **AND** the iframe `sandbox` attribute does NOT include `allow-scripts`

#### Scenario: Markdown artifact renders as formatted text
- **WHEN** the preview variant is `markdown_preview`
- **THEN** the Preview tab renders headings, quotes, lists, and code blocks as formatted markdown

#### Scenario: Carousel draft renders via markdown preview
- **WHEN** the artifact type is `CarouselDraft`
- **THEN** the server returns a `markdown_preview` rendering each slide as a heading-and-body section with any referenced images shown inline
- **AND** the sheet renders it through the existing markdown renderer without a new preview variant

#### Scenario: Image asset renders via image preview
- **WHEN** the artifact type is `ImageAsset`
- **THEN** the server returns an `image_preview` carrying a resolved URL or inline fallback plus alt text
- **AND** the sheet renders it through the existing image renderer

#### Scenario: Unknown or empty preview degrades gracefully
- **WHEN** the preview variant is unhandled or empty
- **THEN** the sheet shows an empty/placeholder state instead of erroring
