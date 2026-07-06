## MODIFIED Requirements

### Requirement: Localized user-facing copy

All user-facing copy in the proposal flow SHALL be provided via flat `translate()` keys in both `en` and `pt-BR`, including strings that were previously hardcoded. Dynamic PlanTemplate names and descriptions SHALL resolve through locale-aware catalog keys before falling back to backend display strings.

#### Scenario: Copy resolves in both locales

- **WHEN** the proposal flow renders in `en` or `pt-BR`
- **THEN** every label, prompt, and message resolves to a translated string with no missing key

#### Scenario: Candidate plan name resolves by locale

- **WHEN** a proposal candidate has `template_key=weekly-newsletter-linkedin`
- **AND** the active locale is `pt-BR`
- **THEN** the candidate card renders the Portuguese catalog plan name rather than the raw backend English name

### Requirement: Post-create confirmation with contextual actions

On successful creation the card SHALL confirm the plan was created and SHALL offer next actions selected from the saved configuration's authoritative status. Action labels SHALL resolve through localized copy keys, and plan names in the confirmation SHALL resolve through localized catalog metadata.

#### Scenario: Runnable plan offers run and schedule

- **WHEN** the saved configuration's status is RUNNABLE
- **THEN** the celebration offers localized actions for run now, schedule, review plan, adjust configuration, and ask about another plan as applicable to the current flow
- **AND** activating run now starts an execution in the same configured thread so the user can watch it

#### Scenario: Draft plan offers finish-setup

- **WHEN** the saved configuration still needs binding (status is not RUNNABLE)
- **THEN** the celebration offers localized finish-setup and follow-up actions
- **AND** finish setup keeps the user in the configured thread where binding continues

#### Scenario: Schedule opens the schedule dialog

- **WHEN** the user activates the localized schedule action from the celebration
- **THEN** the existing schedule dialog opens for the created configuration
