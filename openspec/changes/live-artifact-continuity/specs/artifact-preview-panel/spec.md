## ADDED Requirements

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

## MODIFIED Requirements

### Requirement: Header and footer metadata
The preview sheet header SHALL show a token-coded type icon, title, and — when an iteration
signal exists — a version/iteration badge; the inline card SHALL show the same iteration
badge. The iteration indicator SHALL be derived from artifact content revisions and/or the
ordinal of the producing execution among repeated runs of the same configuration, and SHALL
be omitted when no such signal exists (no `v1` noise on first drafts).

#### Scenario: Iteration badge on a revised or repeated artifact
- **WHEN** an artifact has more than one content revision, or is produced by a repeated run of
  the same configuration for its artifact type
- **THEN** the sheet header and the inline card show a matching iteration badge

#### Scenario: No badge on a first, unrevised artifact
- **WHEN** an artifact has a single revision and no prior run for its type
- **THEN** no iteration badge is shown
