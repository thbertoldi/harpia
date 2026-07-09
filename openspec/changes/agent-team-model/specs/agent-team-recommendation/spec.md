# agent-team-recommendation

## Purpose
Given a plan's steps and their required capabilities (minus capabilities the user opted out
of), recommend a minimal **agent team** (role × tier) whose combined capabilities cover every
step, and surface it for the user to accept or swap.

## ADDED Requirements

### Requirement: Cover all required capabilities
The system SHALL compute a recommended team such that every step's `required_capabilities`
(ignoring opted-out capabilities) is satisfied by at least one agent in the team.

#### Scenario: single multi-capable agent covers several steps
- GIVEN a plan whose steps require `linkedin-content-adaptation` and `carousel-authoring`
- WHEN the system recommends a team
- THEN it recommends a single LinkedIn Content Specialist (which carries both capabilities)
- AND does not recommend two separate agents

#### Scenario: missing capability is flagged
- GIVEN a step requires a capability no entitled agent provides
- WHEN the system recommends a team
- THEN it flags the uncovered capability as a gap (not silently satisfied)

### Requirement: Prefer fewer agents and sensible tiers
The recommendation SHALL prefer the fewest agents that cover the required capabilities, with a
default tier per role (Sênior for quality-critical roles, Júnior for high-volume/cheap ones).

#### Scenario: tier default applies
- GIVEN a covered team
- WHEN no tier preference is expressed
- THEN each role is recommended at its default tier

### Requirement: User accepts or swaps
The recommendation SHALL be advisory; the user MAY accept it, swap a member's tier, or substitute an
alternative agent that still satisfies the step's required capabilities.

#### Scenario: user swaps tier
- GIVEN a recommended team with a Sênior specialist
- WHEN the user swaps to Pleno for that role
- THEN the binding updates to the Pleno installation and the plan remains valid

### Requirement: Respect entitlements and installations
The recommendation SHALL prefer entitled, already-installed agents, then entitled-not-installed
(prompt setup), and flag unentitled gaps.

#### Scenario: unentitled role
- GIVEN a required capability whose role is not entitled to the tenant
- WHEN the system recommends a team
- THEN it flags the role as needing entitlement before the plan can be RUNNABLE
