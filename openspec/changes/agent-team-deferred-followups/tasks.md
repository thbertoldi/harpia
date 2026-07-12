# Tasks: agent-team-deferred-followups

## 1. Team-recommendation UI

- [ ] 1.1 Surface the existing `RecommendTeam` result as a conversational "recommended team" card (accept / swap tier / swap agent) in the PlanConfiguration assistant thread. **Depends on:** none. **Acceptance:** Given a plan with coverable steps, when the assistant reaches team selection, then it renders the recommended team and lets the user accept or swap without leaving the thread. **Focused verification:** `cd frontend && bunx vitest run <card test>`; `cd control-plane && go test ./internal/planassistant`.
- [ ] 1.2 i18n (en + pt-BR) for all new copy via flat `translate()` keys.

## 2. Composable LinkedInPost artifact

- [ ] 2.1 Introduce a composable `LinkedInPost` artifact (text + optional carousel + optional images) and re-author the `publish` step over it. **Depends on:** none. **Acceptance:** Given an opted-in carousel and image, when the plan publishes, then a single `LinkedInPost` artifact carries all three and previews correctly. **Focused verification:** `cd control-plane && go test ./internal/plans ./internal/artifacts`; `cd frontend && bun run check`.

## 3. Verification

- [ ] 3.1 Run `mise run acceptance` and confirm no regression in the LinkedIn opt-out scenario.
- [ ] 3.2 Whole-surface gates from AGENTS.md (`go test ./...`, `bun run lint`, `bun run check`, `ruff check src/`, `buf lint`).
