## ADDED Requirements

### Requirement: Artifact types declare an allowlisted structured renderer
The Artifact contract SHALL expose `preview_renderer_key` and `preview_renderer_version` on an `ArtifactType` and on ArtifactType registration. The global ArtifactType catalog SHALL persist those fields. A non-empty declaration SHALL match a platform-registered renderer key and supported version; a tenant or Artifact payload SHALL NOT register a renderer, executable, component URL, or module.

#### Scenario: Registered tabular type declares its renderer
- **WHEN** the platform seeds or registers the `TabularData` ArtifactType
- **THEN** its ArtifactType metadata declares `table.v1` and its supported renderer version
- **AND** a client retrieving the ArtifactType receives the same declaration

#### Scenario: Unknown renderer declaration is rejected
- **WHEN** an ArtifactType registration supplies a renderer key or version not in the platform registry
- **THEN** the control plane rejects the registration as invalid
- **AND** it persists no ArtifactType with that declaration

### Requirement: Structured preview envelope is server-authoritative
`PreviewArtifactResponse` SHALL support a `structured_preview` `oneof` variant containing `renderer_key`, `renderer_version`, validated `spec_json`, and zero or more `ArtifactRef` assets. Existing fixed preview variants SHALL remain readable during this pre-v1 cut. The control plane SHALL select the preview from the ArtifactType declaration and the requested immutable ArtifactVersion, never from a client payload-shape guess.

#### Scenario: Registered projector returns a validated structured preview
- **WHEN** a caller previews an ArtifactVersion whose ArtifactType declares a registered structured renderer
- **THEN** the response contains `structured_preview` with that renderer key and version
- **AND** its `spec_json` and assets have passed the projector's renderer-specific validation

#### Scenario: Requested version remains the preview subject
- **WHEN** a caller requests a non-current ArtifactVersion of an Artifact whose current version later changes
- **THEN** the projector receives and previews the requested immutable version
- **AND** it does not resolve the Artifact's newer current payload

### Requirement: Preview projector registry is platform-owned and has a safe fallback
The artifacts adapter SHALL resolve a declared renderer only through a finite platform-registered `PreviewProjector` registry. Each projector SHALL produce only its own allowlisted renderer key/version and JSON data. An ArtifactType without an available projector SHALL use the retained fixed-preview handling when applicable and SHALL otherwise return a JSON preview; it SHALL NOT fail solely because its type is unknown to the structured registry.

#### Scenario: Unprojected unknown type degrades to JSON
- **WHEN** a valid JSON Artifact payload has no declared or registered structured projector and no retained fixed preview applies
- **THEN** `PreviewArtifact` returns a JSON preview of that payload
- **AND** it returns no renderer code, URL, or unvalidated structured spec

#### Scenario: Projector cannot impersonate another renderer
- **WHEN** a platform projector emits a renderer key or version different from its registry registration
- **THEN** the control plane rejects the projection as invalid
- **AND** it returns no preview response containing that output

### Requirement: TabularData is a validated typed Artifact payload
The platform SHALL provide `harpia.artifacts.v1.TabularData` as a reusable ArtifactType with ordered typed columns, ordered scalar rows, truncation and pagination metadata, a source fingerprint, and an RFC 3339 `as_of` value. Each column SHALL have a unique non-empty key, label, and closed semantic type; each row SHALL match column count and declared scalar semantics. Nested values, executable values, invalid freshness metadata, and payloads beyond configured preview limits SHALL be rejected.

#### Scenario: Valid tabular snapshot projects as a table
- **WHEN** a plan creates a valid `TabularData` ArtifactVersion with typed columns and rows
- **THEN** the platform persists the immutable tenant-owned snapshot
- **AND** previewing it returns a validated `table.v1` structured preview preserving column and row order

#### Scenario: Invalid typed row is rejected
- **WHEN** a `TabularData` payload contains a row whose value count or scalar type conflicts with its declared columns
- **THEN** Artifact creation or versioning rejects the payload as invalid
- **AND** no object or ArtifactVersion is persisted for that payload

### Requirement: ChartSpec pins and validates its tabular input
The platform SHALL provide `harpia.artifacts.v1.ChartSpec` as a reusable ArtifactType containing a required pinned `ArtifactRef` to `TabularData` and a compact Flint semantic chart specification. The control plane SHALL verify the referenced Artifact ID, immutable version ID, type key, content hash, and caller tenant before persistence. It SHALL validate chart fields and encodings against the referenced tabular schema and reject URL-backed data, scripts, functions, arbitrary renderer options, or references to nonexistent columns.

#### Scenario: Chart preview uses its pinned data version
- **WHEN** a valid `ChartSpec` references a tenant-owned `TabularData` version and that tabular Artifact later receives a newer version
- **THEN** its `chart.flint.v1` preview contains data derived from the pinned referenced version
- **AND** it does not substitute the newer tabular version

#### Scenario: Cross-tenant chart reference is denied
- **WHEN** a caller creates or versions a `ChartSpec` with a `TabularData` ArtifactRef owned by another tenant
- **THEN** the control plane rejects the request without disclosing the referenced payload
- **AND** it persists no ChartSpec ArtifactVersion

### Requirement: Structured preview assets remain tenant-safe
The control plane SHALL validate every `StructuredPreview.assets` reference against the preview subject's tenant and immutable identity before returning it. It SHALL expose assets only as verified `ArtifactRef` values and SHALL NOT include object-store paths, credentials, downstream endpoints, or a directly usable foreign-tenant URL in a structured preview.

#### Scenario: Projected asset from another tenant is rejected
- **WHEN** a projector attempts to include an asset ArtifactRef that cannot be resolved under the preview caller's tenant
- **THEN** the preview request fails safely without returning the foreign asset reference or payload
- **AND** the owner tenant's Artifact remains undisclosed
