## ADDED Requirements

### Requirement: SlotBinding setup continues from refinement transcript

The PlanConfiguration assistant SHALL continue SlotBinding setup in the same chat thread after a plan is created from conversational refinement.

#### Scenario: Draft plan shows first binding prompt below creation

- **WHEN** a plan is created from a refinement transcript and the resulting PlanConfiguration is DRAFT with missing SlotBindings
- **THEN** the next `BINDING_STEP` assistant prompt appears in the same `/chat/<threadId>` route
- **AND** the refinement transcript remains visible above the binding prompt

#### Scenario: Binding summary uses shared plan vocabulary

- **WHEN** a `BINDING_STEP` or `BINDING_MATRIX` assistant prompt renders a PlanConfiguration summary
- **THEN** it uses the shared plan-summary labels for plan identity, status, executor bindings, overseer bindings, schedule, policies, source groups, and seed inputs
