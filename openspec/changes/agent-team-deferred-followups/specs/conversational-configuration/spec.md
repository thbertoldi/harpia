# conversational-configuration

## ADDED Requirements

### Requirement: Recommended team surface

The PlanConfiguration assistant SHALL surface the team recommendation (`RecommendTeam` result) as a conversational card during configuration, letting the user accept the recommended team or swap an agent's tier or identity without leaving the thread.

#### Scenario: Recommended team offered during configuration

- GIVEN a PlanConfiguration whose steps have a coverable team recommendation
- WHEN the assistant reaches team selection
- THEN it renders a "recommended team" card naming each step's recommended executor and tier
- AND the user can accept the whole team or swap an individual agent/tier in-thread

#### Scenario: Composable publish artifact

- GIVEN a plan that opted into carousel and/or images
- WHEN the publish step runs
- THEN it produces a single composable LinkedInPost artifact carrying text plus the opted-in carousel and images together
