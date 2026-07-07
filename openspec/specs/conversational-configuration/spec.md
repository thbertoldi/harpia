# conversational-configuration Specification

## Purpose
TBD - created by archiving change stabilize-user-journey. Update Purpose after archive.
## Requirements
### Requirement: Conversational Composer
The system SHALL replace the static "leave a note" box with a conversational composer that accepts free-text input for configuring plans.

#### Scenario: Enter free-text configuration
- **WHEN** the user types a configuration request in the composer
- **THEN** the assistant processes the text to fill smart defaults

### Requirement: Clarifying Questions
The assistant SHALL ask at most 1-2 clarifying questions if it is unsure about the configuration based on the user's free-text input.

#### Scenario: Assistant asks clarifying question
- **WHEN** the user's input is ambiguous or missing required information
- **THEN** the assistant responds with a clarifying question in the chat stream

### Requirement: Structured Approval Card
The system SHALL present a structured approval card with pre-filled configuration details (sources, schedule, tone, etc.) before executing a plan.

#### Scenario: View approval card
- **WHEN** the assistant has gathered sufficient configuration details
- **THEN** it presents a structured approval card in the chat stream

### Requirement: Explicit Approval for Execution
A plan SHALL NOT transition to RUNNABLE or SCHEDULED status without explicit user interaction with the structured approval card.

#### Scenario: Approve plan execution
- **WHEN** the user clicks "Approve" or "Run" on the structured approval card
- **THEN** the plan transitions to the appropriate execution state

### Requirement: Centralized Frontend Configuration Flow Model

The conversational PlanConfiguration frontend SHALL expose a pure TypeScript flow model that represents the configuration assistant path and produces render-ready view-model decisions for proposal, binding, overseer, policies, matrix review, and runnable states.

#### Scenario: Components consume flow helpers
- **WHEN** a conversational configuration component needs to decide which assistant state or summary to render
- **THEN** the decision is derived from a shared frontend flow helper rather than duplicated in the component body

#### Scenario: Flow helpers are independently testable
- **WHEN** the golden configuration path is exercised in unit tests
- **THEN** the tests can validate proposal, binding, overseer, policy, matrix, and runnable decisions without mounting Svelte components

### Requirement: Typed Payload Parser Boundary

The conversational PlanConfiguration frontend SHALL parse assistant prompt payload JSON through typed parser helpers before components render payload content.

#### Scenario: Valid prompt payload is parsed once
- **WHEN** a component receives a `BINDING_STEP`, `OVERSEER_STEP`, `POLICIES_STEP`, or `BINDING_MATRIX` assistant payload
- **THEN** it uses the shared typed parser for that payload state before reading rows, focused steps, fields, or options

#### Scenario: Malformed prompt payload is contained
- **WHEN** an assistant prompt payload is invalid JSON, has the wrong state, or omits required arrays
- **THEN** parser tests cover the failure mode
- **AND** the component renders the existing safe fallback instead of throwing during normal rendering

### Requirement: Mutation Helper Hygiene

Frontend PlanConfiguration mutation helpers SHALL map to a backend mutation or perform meaningful domain transformation; no helper SHALL exist solely to rename arguments and forward to another helper.

#### Scenario: Domain helper is retained
- **WHEN** a helper preserves existing configuration fields, transforms parameter values, appends domain events, or hides transport details for a backend mutation
- **THEN** the helper remains available and has focused unit test coverage

#### Scenario: Pass-through helper is removed
- **WHEN** a helper only forwards to another helper without validation, transformation, event construction, or durable domain language
- **THEN** callers use the underlying helper directly
- **AND** tests assert the remaining behavior rather than the deleted indirection

### Requirement: Frontend Configuration Flow Guide

The frontend SHALL include a concise maintainer guide for the conversational PlanConfiguration flow that identifies key files, the golden path, parser boundaries, mutation-helper rules, and verification commands.

#### Scenario: Agent locates the correct edit point
- **WHEN** an implementer or AI agent needs to change conversational configuration behavior
- **THEN** the guide identifies where state/view-model logic, payload parsing, mutations, Svelte rendering, and tests belong

#### Scenario: Guide includes verification commands
- **WHEN** the guide describes implementation workflow
- **THEN** it lists the focused Vitest commands and whole-frontend lint/check commands expected for this surface

