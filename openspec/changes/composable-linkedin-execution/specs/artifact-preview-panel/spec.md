## ADDED Requirements

### Requirement: Composable LinkedInPost preview is authoritative before approval
The canonical artifact preview panel SHALL render a `LinkedInPost` through server-selected preview
data, showing its text, carousel slides when present, and referenced images when present, without
the client reconstructing content from unrelated artifact cards.

#### Scenario: Final carousel post previews as one subject
- **WHEN** the preview panel opens the version-pinned `LinkedInPost` subject of a pending approval
- **THEN** it renders the post text and ordered carousel slides together
- **AND** the content shown is sourced from that exact artifact version

#### Scenario: Text-only post degrades without rich sections
- **WHEN** the preview panel opens a `LinkedInPost` without carousel or images
- **THEN** it renders the post text without empty carousel or image placeholders

### Requirement: Review and approval cards preview pinned versions
The review and approval conversation cards SHALL request preview data for their stored
subject ArtifactRef version, not for the Artifact's mutable current version.

#### Scenario: Card opens an older pinned candidate
- **WHEN** a pending review or approval references a non-current ArtifactVersion
- **THEN** selecting Preview opens that referenced version
- **AND** the panel identifies its version and content state consistently with the card
