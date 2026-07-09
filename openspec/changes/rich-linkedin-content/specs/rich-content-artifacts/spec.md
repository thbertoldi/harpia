## ADDED Requirements

### Requirement: Carousel draft artifact type

The system SHALL define a `harpia.artifacts.v1.CarouselDraft` `ArtifactType` carrying an ordered set of slides (heading, body, optional image reference, alt text) plus a hook, caption, and hashtags, produced as a typed `PlanStep` output into the Artifact stream.

#### Scenario: Carousel draft flows through the Artifact stream
- **WHEN** the carousel branch step completes
- **THEN** a `CarouselDraft` Artifact is created with metadata in the control plane and payload in object storage
- **AND** it is listed and previewable through the same artifact surfaces as other artifact types

#### Scenario: Slides reference image assets by id
- **WHEN** a carousel slide is image-backed
- **THEN** the slide references an `ImageAsset` Artifact by id rather than embedding image bytes

### Requirement: Image asset artifact type

The system SHALL define a `harpia.artifacts.v1.ImageAsset` `ArtifactType` whose payload bytes live in object storage (addressed by the Artifact `storage_uri`/`content_hash`) and whose metadata carries the generation brief, mime type, dimensions, alt text, and caption.

#### Scenario: Image asset created as a typed output
- **WHEN** the image-generation step completes
- **THEN** an `ImageAsset` Artifact is created with the image bytes in object storage
- **AND** the generation prompt/brief is retained on the artifact metadata for provenance

### Requirement: Rich content versioning

Regenerations and edits of carousel and image artifacts SHALL create new `ArtifactVersion` records under the same Artifact, and approval/publish SHALL act on the current version.

#### Scenario: Regenerated carousel adds a version
- **WHEN** the user regenerates a carousel draft
- **THEN** a new `ArtifactVersion` is created under the same `CarouselDraft` Artifact
- **AND** the prior version is preserved and referenced as the source version

### Requirement: Executors for rich content outputs

The system SHALL provide the `ExecutorSKU`s that produce the new artifact types: a carousel-writing agent SKU (`TextDraft → CarouselDraft`) and an image/asset-generation integration SKU (→ `ImageAsset`) whose provider, model, brand, and credential configuration lives on its `ExecutorInstallation`.

#### Scenario: Carousel SKU produces a carousel draft
- **WHEN** the carousel step's slot is bound to the carousel-writer installation
- **THEN** running the step produces a `CarouselDraft` artifact

#### Scenario: Image provider config stays on the installation
- **WHEN** the image-generation SKU is configured
- **THEN** its provider, model, brand assets, and credentials are held on the `ExecutorInstallation`
- **AND** neither the `PlanTemplate` nor the produced Artifact carries that configuration
