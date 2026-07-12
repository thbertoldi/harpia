# Tasks: agent-team-deferred-followups

## 1. Team-recommendation UI

- [ ] 1.1 Surface the existing `RecommendTeam` result as a conversational "recommended team" card (accept / swap tier / swap agent) in the PlanConfiguration assistant thread. **Depends on:** none. **Acceptance:** Given a plan with coverable steps, when the assistant reaches team selection, then it renders the recommended team and lets the user accept or swap without leaving the thread. **Focused verification:** `cd frontend && bunx vitest run <card test>`; `cd control-plane && go test ./internal/planassistant`.
- [ ] 1.2 i18n (en + pt-BR) for all new copy via flat `translate()` keys.

## 2. Composable LinkedInPost artifact

- [ ] 2.1 **SUPERSEDED (2026-07-12):** The separate unified-post/publish design is owned by [`composable-linkedin-execution`](../composable-linkedin-execution/). Do not implement this task here; that change defines the linear composable execution, review/revision, version-pinned approval, and real document upload. This unchecked entry is retained as historical deferred-scope evidence.

## 3. Verification

- [ ] 3.1 Run `mise run acceptance` and confirm no regression in the LinkedIn opt-out scenario.
- [ ] 3.2 Whole-surface gates from AGENTS.md (`go test ./...`, `bun run lint`, `bun run check`, `ruff check src/`, `buf lint`).
