## ADDED Requirements

### Requirement: Overseer setup remains visible after conversational creation

The PlanConfiguration assistant SHALL keep OverseerBinding setup visible in the same thread after conversational refinement and SlotBinding completion.

#### Scenario: Overseer prompt follows completed slot bindings in same thread

- **WHEN** a plan created from conversational refinement has all required SlotBindings persisted
- **AND** at least one agent-backed PlanStep still lacks an OverseerBinding
- **THEN** the next assistant prompt in the same thread has state `OVERSEER_STEP`
- **AND** it appears below the prior binding selection turn

#### Scenario: Real-thread verification covers overseer transition

- **WHEN** the conversational refinement implementation is verified
- **THEN** verification includes a real thread that creates a plan, completes SlotBindings, and observes the `OVERSEER_STEP` prompt before the review gate
