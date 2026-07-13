## ADDED Requirements

### Requirement: Client preview rendering uses an allowlisted component registry
The frontend SHALL normalize `structured_preview` responses through a statically linked renderer-key registry. The registry SHALL contain local renderers for `table.v1`, `chart.flint.v1`, `carousel.v1`, `document.html`, `markdown`, and `image`; it SHALL NOT dynamically import or execute an Artifact-supplied URL, package, module, script, or component name.

#### Scenario: Registered renderer renders a structured table
- **WHEN** the frontend receives a valid `table.v1` structured preview
- **THEN** it resolves the local table renderer from the allowlisted registry
- **AND** it renders no remote code or network-selected component

#### Scenario: Artifact-supplied module is not executed
- **WHEN** a structured preview spec includes a URL, module-like string, or unknown renderer key
- **THEN** the frontend does not import, fetch, or execute it
- **AND** it renders the safe JSON fallback instead

### Requirement: Unknown or malformed structured previews fall back to inspectable JSON
The frontend SHALL parse and validate the declared renderer key, supported version, and `spec_json` before dispatching to a renderer. An unknown key/version, invalid JSON, invalid renderer-specific shape, or renderer failure SHALL show a JSON fallback with no uncaught UI error.

#### Scenario: Future renderer key degrades safely
- **WHEN** the frontend receives `renderer_key = "map.v2"` that is not in its registry
- **THEN** the preview sheet remains usable
- **AND** it displays the structured preview data as JSON rather than a blank or crashing renderer

### Requirement: Table previews are native and accessible
The `table.v1` renderer SHALL render validated tabular data using a semantic HTML table with a caption, column headers, scoped header cells, and declared column/row order. It SHALL expose localized truncation/pagination state and source freshness metadata when present, without changing the data snapshot or fetching a live source.

#### Scenario: Truncated table announces available metadata
- **WHEN** a table preview declares that its rows are truncated and supplies pagination and `as_of` metadata
- **THEN** the renderer shows the rows in a native table
- **AND** it presents localized truncation, pagination, and freshness information to visual and assistive-technology users

### Requirement: Flint charts render only validated inline tabular data
The `chart.flint.v1` renderer SHALL statically use the lockfile-pinned `flint-chart` and ECharts dependencies to compile the validated semantic chart spec and its pinned inline tabular data. It SHALL not pass a URL-backed data source, arbitrary backend option, script, or function to Flint or ECharts. The chart SHALL have a localized accessible name and provide the same pinned data through the native table renderer as its non-visual equivalent.

#### Scenario: Chart renders the pinned tabular snapshot locally
- **WHEN** the frontend receives a valid `chart.flint.v1` preview with inline rows derived from its pinned TabularData ArtifactVersion
- **THEN** it compiles that data locally through Flint to an ECharts option and renders the chart
- **AND** it performs no network request for chart data or renderer code

#### Scenario: Invalid chart spec does not reach the chart engine
- **WHEN** a chart preview has an invalid semantic field, unsupported version, or URL-backed data declaration
- **THEN** the frontend does not call Flint or ECharts with that input
- **AND** it renders the JSON fallback

### Requirement: HTML document previews are sanitized and CSP-constrained
The `document.html` renderer SHALL display only server-sanitized HTML in a `srcdoc` iframe with no `allow-scripts` sandbox token and a deny-by-default Content Security Policy. The policy SHALL block network connections, forms, nested frames, base URLs, and external script execution; the sanitizer SHALL remove scripts, event handlers, forms, unsafe URL schemes, and active/embedded content before rendering.

#### Scenario: Malicious document markup cannot execute or exfiltrate
- **WHEN** a document Artifact contains script tags, event handlers, form actions, meta refresh, or external resource URLs
- **THEN** the previewed document contains none of the disallowed active markup after sanitization
- **AND** its iframe CSP blocks network and form egress even if markup bypasses a sanitizer rule

### Requirement: Existing compatible previews migrate through the registry incrementally
The frontend SHALL normalize existing markdown, image, and carousel preview variants through the corresponding registered renderer where their current server contract maps without loss. Fixed variants that do not yet map cleanly SHALL remain functional during this change and SHALL not be silently reinterpreted by client payload-shape heuristics.

#### Scenario: Existing markdown response uses the registered renderer
- **WHEN** the preview service returns a retained markdown fixed variant
- **THEN** the frontend normalizes it to the registered markdown renderer
- **AND** markdown formatting remains equivalent to the existing preview behavior
