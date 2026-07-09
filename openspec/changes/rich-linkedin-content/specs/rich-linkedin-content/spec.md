## ADDED Requirements

### Requirement: Conversational output-format choice

During conversational configuration of the rich LinkedIn content plan, the system SHALL let the user choose the output format — text post, carousel outline, image-backed post, or approval-only publish — and SHALL persist the choice as a `PlanBehaviorPolicies` selection on the `PlanConfiguration`.

#### Scenario: Assistant asks for the output format
- **WHEN** the user configures a `linkedin-content-studio` plan in chat
- **THEN** the assistant offers the output-format options (text post, carousel outline, image-backed post, approval-only)
- **AND** the selected format is stored as the `content_output_format` behavior policy on the `PlanConfiguration`

#### Scenario: Approval-only maps to the simple publish path
- **WHEN** the user selects the approval-only format
- **THEN** the configuration runs the text-post branch with `publish_approval_mode` set to require approval
- **AND** no carousel or image steps are included in the run

### Requirement: Governed voice rules

The plan SHALL expose governed voice controls — a stronger-hook style, an anti-AI-jargon toggle, and free-form voice rules — as template `input_parameters` fed to the writing/adaptation step as seed `ContentPreferences`.

#### Scenario: Anti-AI-jargon rule reaches the writer
- **WHEN** the user enables the anti-AI-jargon rule during configuration
- **THEN** the value is mapped via a `SEED_ARTIFACT` runtime mapping into the step's `ContentPreferences`
- **AND** the adapting agent receives the rule in its input

#### Scenario: Voice rules are localized
- **WHEN** the voice-rule inputs render in the configuration surface
- **THEN** their labels resolve from `plans.inputs.<key>.label` in both `en` and `pt-BR`

### Requirement: Rich LinkedIn content PlanTemplate variant

The system SHALL provide a `linkedin-content-studio` `PlanTemplate` that reuses the neutral `fetch-news → write-draft` head producing a `TextDraft` and diverges into format-specific output `PlanStep`s bound to the appropriate `ExecutorSKU`.

#### Scenario: Shared neutral head
- **WHEN** the `linkedin-content-studio` template is seeded
- **THEN** it includes `fetch-news` and `write-draft` steps ending at a `TextDraft` artifact
- **AND** it includes format-specific steps for text-post, carousel, and image-backed output

#### Scenario: Format selects the executed branch
- **WHEN** a run's `content_output_format` policy selects one format
- **THEN** only that format's output step(s) execute
- **AND** the other format branches are skipped

### Requirement: Format preview before publish

The system SHALL preview the selected-format artifact in the canonical artifact preview panel before any publish step runs.

#### Scenario: Carousel previewed before publish
- **WHEN** the carousel branch produces a `CarouselDraft`
- **THEN** the artifact is previewable in the thread before publish
- **AND** the preview reflects the carousel slides

### Requirement: Format-aware in-thread approval

When `publish_approval_mode` requires approval, the system SHALL route the publish gate through the in-thread approval card, previewing the selected-format artifact.

#### Scenario: Approval card previews the format artifact
- **WHEN** a run reaches the publish gate with approval required
- **THEN** the in-thread approval card exposes a preview of the selected-format artifact (post, carousel, or image-backed)
- **AND** approve/reject decisions follow the existing approval-request lifecycle
