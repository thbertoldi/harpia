## ADDED Requirements

### Requirement: Final Artifact Side Preview
The system SHALL provide a split-pane or side-drawer in the conversation view (`/chat/[threadId]`) to display the final artifact produced by a plan execution.

#### Scenario: View final artifact
- **WHEN** a plan execution completes and produces a final artifact
- **THEN** the artifact is displayed in the side preview
- **AND** the user can continue the conversation in the main chat area

### Requirement: Suppress Intermediate Artifacts
The conversation stream SHALL NOT promote intermediate artifacts into the UI.

#### Scenario: Intermediate artifact produced
- **WHEN** a plan execution produces an intermediate artifact
- **THEN** the artifact is NOT displayed in the side preview
- **AND** the chat stream only shows a progress message

### Requirement: Closable Preview
The side preview SHALL be closable by the user.

#### Scenario: Close side preview
- **WHEN** the user clicks the close button on the side preview
- **THEN** the side preview is hidden
- **AND** the chat area expands to fill the available space
