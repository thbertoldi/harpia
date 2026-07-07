## MODIFIED Requirements

### Requirement: Alternative-plan selection

When more than one candidate is available, the proposal flow SHALL let the user choose among them and SHALL highlight the best matching candidate while keeping alternatives visible.

#### Scenario: Multiple candidates offer a picker

- **WHEN** more than one candidate is returned, or no single candidate clears the confidence threshold
- **THEN** the card presents a selectable list of candidate plans
- **AND** selecting one enters the confirmation step for that candidate

#### Scenario: Best candidate is highlighted in the picker

- **WHEN** more than one candidate is returned with compatibility data
- **THEN** the highest-compatibility candidate is visually identified as the best match
- **AND** lower-compatibility candidates remain selectable without being hidden

#### Scenario: No candidates offers a fallback

- **WHEN** the proposal returns zero candidates
- **THEN** the card explains no plan matched and links to browse templates

### Requirement: Conversational confirmation before the form

The proposal flow SHALL lead with an assistant sentence restating the request and SHALL continue into accumulated refinement turns with confirm and adjust affordances, rather than immediately presenting the inputs form or replacing prior turns.

#### Scenario: Single confident candidate shows confirmation first

- **WHEN** exactly one candidate is returned above the confidence threshold
- **THEN** the card shows the restatement sentence with "Yes, create it" and "Adjust" actions
- **AND** the inputs form is not shown until the user confirms or adjusts

#### Scenario: Confirm with all required inputs pre-filled creates immediately

- **WHEN** the user activates "Yes, create it"
- **AND** every required input parameter already has a value
- **THEN** the plan is created without showing the inputs form
- **AND** the card transitions to the post-create celebration

#### Scenario: Confirm with missing required inputs reveals conversational refinement

- **WHEN** the user activates "Yes, create it"
- **AND** at least one required input parameter has no value
- **THEN** the assistant appends refinement prompts for the missing setup values
- **AND** the plan is created only after the user answers required prompts and confirms the final summary

#### Scenario: Adjust opens the full refinement path

- **WHEN** the user activates "Adjust"
- **THEN** the assistant appends the full refinement path for editing before creation

### Requirement: Post-create confirmation with contextual actions

On successful creation the thread SHALL confirm the plan was created and SHALL offer next actions selected from the saved configuration's authoritative status without navigating away from the thread.

#### Scenario: Runnable plan offers run and schedule

- **WHEN** the saved configuration's status is RUNNABLE
- **THEN** the celebration offers "Run now", "Schedule", "Review plan", "Adjust configuration", and "Ask about another plan"
- **AND** "Run now" starts an execution and keeps the user in the configured thread to watch it

#### Scenario: Draft plan offers finish-setup

- **WHEN** the saved configuration still needs binding (status is not RUNNABLE)
- **THEN** the celebration offers "Finish setup", "Schedule", "Review plan", "Adjust configuration", and "Ask about another plan"
- **AND** "Finish setup" keeps the user in the configured thread where binding continues

#### Scenario: Schedule opens the schedule dialog

- **WHEN** the user activates "Schedule" from the celebration
- **THEN** the existing schedule dialog opens for the created configuration

#### Scenario: Ask about another plan stays in chat

- **WHEN** the user activates "Ask about another plan" from the celebration
- **THEN** the route remains on the current chat thread
- **AND** no navigation to `/` occurs
