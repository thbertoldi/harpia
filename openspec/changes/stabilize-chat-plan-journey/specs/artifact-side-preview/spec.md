## MODIFIED Requirements

### Requirement: Final Artifact Side Preview
The system SHALL provide a closeable side preview panel in the conversation view (`/chat/[threadId]`) to display artifacts produced by a plan execution, with the final artifact emphasized by default when the run completes.

#### Scenario: View final artifact
- **WHEN** a plan execution completes and produces a final artifact
- **THEN** the artifact is available from the chat thread and can be displayed in the side preview
- **AND** the user can continue the conversation in the main chat area

### Requirement: Suppress Intermediate Artifacts
The conversation stream SHALL NOT promote intermediate artifacts as the primary final output, but it MAY render inline artifact launchers for produced artifacts when they help explain execution progress or support approval.

#### Scenario: Intermediate artifact produced
- **WHEN** a plan execution produces an intermediate artifact
- **THEN** the chat stream may show a compact inline artifact launcher tied to the producing step
- **AND** the side preview does not automatically replace the final-artifact emphasis unless the user explicitly opens the intermediate artifact

## ADDED Requirements

### Requirement: Canonical artifact preview panel state
The chat route SHALL own one canonical artifact preview state for active artifact id and loading/readiness, and all artifact launchers SHALL open that same panel.

#### Scenario: Open artifact from inline card
- **WHEN** the user opens an artifact from an inline artifact card
- **THEN** the canonical artifact preview panel opens with that artifact selected
- **AND** no second independent artifact panel state is created

#### Scenario: Artifact panel scroll relationship
- **WHEN** the user scrolls the chat thread
- **THEN** artifact launchers remain in the scrolled conversation context
- **AND** the preview panel does not depend on a sticky top-of-page artifact rail to preserve access
