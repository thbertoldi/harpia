## MODIFIED Requirements

### Requirement: Final Artifact Side Preview
The system SHALL provide a closeable side preview panel in the conversation view
(`/chat/[threadId]`) to display artifacts produced by a plan execution, with the final
artifact emphasized by default. The panel SHALL open **early**, in a generating state, as
soon as the step that produces the emphasized/final artifact enters `running`, and SHALL
swap to the ready artifact when it arrives — rather than opening only after the artifact
exists.

#### Scenario: Preview opens while the final artifact is being produced
- **WHEN** the step that produces the emphasized/final artifact enters the `running` state
  and no ready final artifact exists yet
- **THEN** the side preview opens in a generating state
- **AND** it swaps to the ready artifact once the artifact is produced

#### Scenario: View final artifact
- **WHEN** a plan execution completes and produces a final artifact
- **THEN** the artifact is available from the chat thread and displayed in the side preview
- **AND** the user can continue the conversation in the main chat area

#### Scenario: Dismissal is respected across the early open
- **WHEN** the user has closed the emphasized final artifact
- **THEN** neither the early generating-state open nor the on-arrival open re-opens the panel
  for that same artifact
