## ADDED Requirements

### Requirement: Version-addressable composable LinkedIn post
The system SHALL define `ArtifactRef` with artifact id, artifact version id, artifact type key,
and content hash, and SHALL define `LinkedInPost` as one typed Artifact containing a
`LinkedInPostDraft`, an optional `CarouselDraft`, and zero or more image ArtifactRefs.

#### Scenario: Author step creates the initial post version
- **WHEN** `author-content` completes for a LinkedIn Content Studio execution
- **THEN** it creates a `LinkedInPost` Artifact and an initial ArtifactVersion containing text
- **AND** the step output identifies that artifact id, version id, artifact type key, and content hash

#### Scenario: Enrichment preserves one post identity
- **WHEN** an active carousel or image enrichment completes
- **THEN** it creates a new ArtifactVersion under the same `LinkedInPost` Artifact id
- **AND** its output ref names the new version and includes all previously accepted post content

### Requirement: Carousel publish document is an immutable derivative
When a `LinkedInPost` carries a carousel, the system SHALL create a tenant-scoped immutable
`LinkedInCarouselDocument` Artifact containing the deterministic PDF bytes derived from the
reviewed carousel and SHALL retain its ArtifactRef and content hash in the carousel payload.

#### Scenario: Carousel candidate carries uploadable approved bytes
- **WHEN** `draft-carousel` produces a candidate post version
- **THEN** the carousel preview data and a PDF document ArtifactRef are contained in that version
- **AND** the document ArtifactRef content hash identifies the exact bytes to upload

#### Scenario: Text-only post has no document derivative
- **WHEN** a final `LinkedInPost` has no carousel
- **THEN** it contains no carousel document ArtifactRef
- **AND** no document bytes are created or loaded for publishing

### Requirement: Optional enrichment is identity-shaped
The template validator SHALL reject an optional middle PlanStep unless its input ArtifactType and
output ArtifactType are identical. For a valid excluded optional enrichment, the workflow SHALL
alias the upstream ArtifactRef as the skipped step output.

#### Scenario: Invalid optional type transition is rejected
- **WHEN** a template declares an optional middle step whose input ArtifactType differs from its output ArtifactType
- **THEN** template loading fails before the template can be seeded or configured

#### Scenario: Excluded carousel aliases the authored post
- **WHEN** carousel authoring is not included in a LinkedIn Content Studio execution
- **THEN** `draft-carousel` has a SKIPPED StepExecution whose input and output ArtifactRefs are equal
- **AND** `generate-image` or `publish` resolves that aliased `LinkedInPost` ref as its input

### Requirement: Artifact versions are the revision substrate
The system SHALL create an ArtifactVersion under the same subject Artifact for each Review/Revision
candidate and SHALL preserve prior versions and their content hashes.

#### Scenario: Revision preserves prior candidate
- **WHEN** an overseer requests a revision with feedback
- **THEN** the resumed specialist produces a new ArtifactVersion under the reviewed `LinkedInPost` Artifact
- **AND** the prior candidate remains readable by its original ArtifactRef
