# conversational-plan-refinement Specification

## Purpose
TBD - created by archiving change conversational-plan-refinement. Update Purpose after archive.
## Requirements
### Requirement: Conversation accumulates during plan refinement

The system SHALL present plan proposal refinement as accumulated assistant and user turns in the thread rather than replacing a proposal card's visible content with each step.

#### Scenario: Candidate choice remains visible after next prompt

- **WHEN** a user selects a candidate plan from a `PLAN_PROPOSED` message
- **THEN** the thread contains a visible user selection turn for that candidate
- **AND** the next assistant refinement prompt appears below it without removing the original proposal or selection

#### Scenario: Refinement answers remain visible through creation

- **WHEN** the user answers audience, theme, source group, date range, and confirmation prompts
- **THEN** each answer remains visible in chronological order after the PlanConfiguration is created
- **AND** the created plan's setup prompt appears below the refinement transcript in the same route

### Requirement: Best matching plan is highlighted

The system SHALL identify the highest-compatibility candidate in a proposal and visually distinguish it from alternatives without hiding alternatives.

#### Scenario: Highest-confidence candidate is marked as best match

- **WHEN** a `PLAN_PROPOSED` payload contains multiple candidates with confidence scores
- **THEN** the candidate with the highest compatibility is labeled as the best match
- **AND** other candidates remain selectable

#### Scenario: Backend best-match marker wins over UI inference

- **WHEN** a `PLAN_PROPOSED` payload includes an explicit best-candidate marker
- **THEN** the UI highlights that marked candidate even if candidate order changes

### Requirement: Assistant recommends configurable setup values

Before creating a PlanConfiguration, the assistant SHALL recommend setup values based on the selected PlanTemplate, classifier extraction, template options, and the user's request.

#### Scenario: Recommendations are shown as editable chips

- **WHEN** a selected plan needs theme, audience, topics-to-avoid, source groups, tone, language, or date-range values
- **THEN** the refinement flow presents recommended values as selectable chips or concise controls
- **AND** the user can remove recommendations
- **AND** the user can add custom values where the input accepts multiple values

#### Scenario: Audience is recommended before creation

- **WHEN** the selected template contains an audience input
- **THEN** the assistant recommends an audience or persona before the plan is created
- **AND** the final parameter values include the accepted or edited audience

#### Scenario: Topics to avoid are recommended before creation

- **WHEN** the selected template contains a topics-to-avoid input
- **THEN** the assistant recommends topics to avoid as removable chips
- **AND** the user can add additional topics to avoid

#### Scenario: Date range is explained as publication window

- **WHEN** the selected template contains a date-range input for source articles
- **THEN** the assistant labels the period as the publication window for source articles
- **AND** the selected date range is serialized into the plan's parameter values

### Requirement: Multiple source groups materialize into one executable binding

The system SHALL allow selecting multiple source groups for a news-based plan and SHALL materialize those selections into one tenant-scoped ExecutorInstallation used by the source-fetch PlanStep.

#### Scenario: Multiple selected groups create one aggregate installation

- **WHEN** the user selects more than one source group before creating a news-based PlanConfiguration
- **THEN** the system creates or reuses one aggregate RSS ExecutorInstallation containing the union of feed configuration from the selected groups
- **AND** the created PlanConfiguration has one SlotBinding for `fetch-news` pointing to that aggregate installation

#### Scenario: Feed URLs remain installation configuration

- **WHEN** a PlanConfiguration is created from selected source groups
- **THEN** feed URLs are stored in ExecutorInstallation config
- **AND** the PlanTemplate and Artifact payloads do not store source feed URLs as ad hoc business data

### Requirement: Final confirmation is natural language

The final pre-create confirmation SHALL read as a complete localized sentence summarizing the selected plan and key setup values.

#### Scenario: Confirmation avoids stitched fragments

- **WHEN** the user reaches final confirmation
- **THEN** the assistant presents one complete sentence or short paragraph using localized copy templates
- **AND** the sentence includes the plan, audience, themes, source groups, and publication window when those values are available

#### Scenario: Confirmation resolves in supported locales

- **WHEN** the final confirmation renders in `en` or `pt-BR`
- **THEN** every visible string resolves through flat translation keys
- **AND** no fallback key or mixed-language fragment is displayed

### Requirement: Post-create continuation stays in the thread

After plan creation, the system SHALL keep the user in the same chat thread and present contextual next actions instead of navigating to the home screen.

#### Scenario: Ask about another plan does not navigate home

- **WHEN** the user activates "Ask about another plan" or "Anything else?" after creation
- **THEN** the route remains `/chat/<threadId>`
- **AND** the conversation is ready for another user request in the same thread

#### Scenario: Created runnable plan offers operational actions

- **WHEN** the created PlanConfiguration is RUNNABLE
- **THEN** the assistant offers Run now, Schedule, Review plan, Adjust configuration, and Ask about another plan actions

#### Scenario: Created draft plan continues setup

- **WHEN** the created PlanConfiguration is still DRAFT
- **THEN** the assistant shows the next setup prompt in the same thread
- **AND** the user can continue SlotBinding and OverseerBinding without switching mental models

### Requirement: Shared plan summary vocabulary

The system SHALL expose a shared plan-summary representation for chat, lists, configuration/settings, execution, artifacts, and audit surfaces.

#### Scenario: Same plan identity appears across surfaces

- **WHEN** a PlanConfiguration is shown in chat and in a structured surface
- **THEN** both surfaces use the same plan identity fields: template name, configured title or intent, and lifecycle status

#### Scenario: Same configuration state appears across surfaces

- **WHEN** a PlanConfiguration has executor bindings, overseer bindings, behavior policies, schedule, source groups, or seed inputs
- **THEN** chat and structured surfaces use the same labels and value formatting for those concepts

#### Scenario: Same controls are available from summary surfaces

- **WHEN** a PlanConfiguration summary is rendered with sufficient permissions
- **THEN** run, schedule, revise, inspect artifacts, and inspect audit controls are represented with the same action vocabulary across surfaces

