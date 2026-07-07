## 1. Refinement Model And Tests

- [x] 1.1 Add focused frontend tests for a proposal-refinement reducer/helper that accumulates candidate choice, audience, themes, topics to avoid, source groups, date range, and final confirmation without dropping prior turns.
- [x] 1.2 Add tests for best-match candidate selection using an explicit backend marker and highest-confidence fallback.
- [x] 1.3 Add tests that final confirmation copy resolves in `en` and `pt-BR` and reads from full localized sentence templates rather than stitched fragments.
- [x] 1.4 Add backend tests for enriched `PLAN_PROPOSED` payload fields: best candidate marker, recommendation reasons, and deterministic refinement defaults.

## 2. Backend Proposal And Recommendation Payloads

- [x] 2.1 Extend `control-plane/internal/chat/messages.go` proposal payload structures to include best-candidate metadata and refinement defaults while preserving existing JSON fields.
- [x] 2.2 Update `control-plane/internal/threads/handler.go` and classifier shaping so ranked candidates include a stable best-match marker and recommendation reasons.
- [x] 2.3 Derive deterministic defaults for audience, themes, topics to avoid, source groups, language/tone, and date range from classifier extraction, template metadata, and template options.
- [x] 2.4 Ensure proposal/refinement selections append durable `USER_SELECTION` or `ASSISTANT_PROMPT` messages with payloads that can be rendered chronologically before a PlanConfiguration exists.

## 3. Multiple Source Groups End-To-End

- [x] 3.1 Update template input helpers to represent selected source groups as a list for the news/LinkedIn flow while keeping generic parameter serialization stable.
- [x] 3.2 Implement or reuse a tenant-scoped aggregate RSS ExecutorInstallation helper that unions selected source-group feed configs and reuses an existing aggregate installation when the same selection already exists.
- [x] 3.3 Update PlanConfiguration creation/materialization so `fetch-news` receives exactly one SlotBinding pointing at the aggregate installation.
- [x] 3.4 Add backend or integration tests proving multiple source groups do not create duplicate `fetch-news` SlotBindings and feed URLs stay in ExecutorInstallation config.

## 4. Conversational UI Refinement

- [x] 4.1 Replace the replace-in-place proposal stage UI with an accumulated thread/refinement renderer that keeps proposal, selections, prompts, and confirmation visible in order.
- [x] 4.2 Highlight the best-match plan candidate with an accessible label and maintain selectable alternatives.
- [x] 4.3 Build editable chip controls for recommended themes, topics to avoid, and source groups, including custom entry and removal.
- [x] 4.4 Improve audience, tone/language, and date-range controls so the date period is labeled as the source-article publication window.
- [x] 4.5 Add all new user-facing copy to `frontend/src/lib/i18n/en.json` and `frontend/src/lib/i18n/pt-BR.json` with flat `translate()` keys.
- [x] 4.6 Apply motion only through existing opacity/transform primitives and verify reduced-motion behavior.

## 5. Post-Create Thread Continuity

- [x] 5.1 Change created-plan actions so "Ask about another plan" or "Anything else?" stays on `/chat/<threadId>` and prepares the same thread for a new request.
- [x] 5.2 Ensure creation of a DRAFT plan reloads or continues into the same thread where `BINDING_STEP` and subsequent `OVERSEER_STEP` prompts are visible.
- [x] 5.3 Ensure creation of a RUNNABLE plan offers Run now, Schedule, Review plan, Adjust configuration, and Ask about another plan from the same conversation.
- [x] 5.4 Update route logic that currently blocks proposals whenever `routeConfigurationId` exists so same-thread follow-up requests can be handled without navigating home.

## 6. Shared Plan Summary Vocabulary

- [x] 6.1 Create a shared plan-summary helper/model for identity, lifecycle status, bindings, overseers, behavior policies, schedule, source groups, seed inputs, runtime state, and actions.
- [x] 6.2 Use the shared summary vocabulary in proposal/refinement, binding/overseer cards, and at least one structured plan surface to prove cohesion.
- [x] 6.3 Add focused tests for summary formatting and action vocabulary so chat and structured surfaces cannot diverge silently.

## 7. Verification

- [x] 7.1 Run `openspec validate --changes conversational-plan-refinement`.
- [x] 7.2 Run focused frontend tests with `cd frontend && bunx vitest run <file>` for all new/changed tests.
- [x] 7.3 Run `cd frontend && bun run check` and confirm only the known baseline diagnostics remain.
- [x] 7.4 Run `cd control-plane && go test ./...`.
- [x] 7.5 Run `cd proto && buf lint`.
- [ ] 7.6 Verify a real thread manually or through an e2e-style test: user request -> best-match proposal -> refinement selections -> create plan -> SlotBinding -> OverseerBinding -> review gate, with all turns visible in one conversation.
