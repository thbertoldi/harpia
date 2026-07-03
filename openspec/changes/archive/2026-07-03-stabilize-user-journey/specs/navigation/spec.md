## ADDED Requirements

### Requirement: Locked Main Navigation
The main navigation SHALL be locked to exactly four items: Home (`/`), Gallery (`/plans`), Runs (`/runs`), and Conversation (`/chat/[threadId]`).

#### Scenario: View main navigation
- **WHEN** the user views the main navigation menu
- **THEN** only Home, Gallery, Runs, and Conversation are available

### Requirement: No Standalone Config or Canvas Routes
The system SHALL NOT provide standalone routes for plan configurations (`/plans/configurations`), plan executions (`/plans/executions`), or artifacts (`/artifacts`).

#### Scenario: Attempt to access standalone route
- **WHEN** the user attempts to navigate to `/plans/configurations`, `/plans/executions`, or `/artifacts`
- **THEN** the system redirects them to an appropriate active route (e.g., Home or a relevant Conversation)

### Requirement: No Navigate-Away Buttons
The UI SHALL NOT contain any buttons or links that navigate the user away from the conversation view to a standalone configuration or canvas page.

#### Scenario: Click configuration button
- **WHEN** the user clicks a button to configure a plan
- **THEN** the configuration happens inline within the conversation view or via a modal/drawer, without navigating away
