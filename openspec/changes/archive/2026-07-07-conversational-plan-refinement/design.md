## Context

Harpia is moving toward ADR-012's model where chat is the PlanConfiguration assistant, not a launcher for disconnected forms. Today the chat route has two modes: config-less proposal messages render `PlanProposalCard`, then a created plan changes the route into `ConversationalWorkspace`. Inside `PlanProposalCard`, candidate choice, confirmation, input form, and created actions are held in local stage state, so each step replaces the previous one. This makes the experience feel modal, hides continuity with slot binding/overseer prompts, and makes the "Anything else?" action navigate back to `/`.

The user review on 2026-07-01 selected direction C: conversation plus recommendations. The experience must highlight the best matching plan, ask better setup questions, recommend values, allow chip selection plus custom input, support multiple source groups end-to-end, clarify the date range, and retain a coherent plan representation across chat, settings, execution, artifacts, and audit.

## Goals / Non-Goals

**Goals:**

- Keep one visible thread from initial request through proposal, refinement, PlanConfiguration creation, SlotBinding, OverseerBinding, review, run/schedule, and follow-up questions.
- Turn proposal setup into accumulated assistant/user turns, not a card that swaps its internal content.
- Generate recommendations for audience, themes, topics to avoid, source groups, tone/language, and date range from the proposal payload and template metadata.
- Persist multiple selected source groups by creating or reusing one tenant-scoped aggregate RSS ExecutorInstallation, then bind the `fetch-news` step to that single installation.
- Define a shared plan-summary model and component vocabulary reused by chat and structured surfaces.
- Verify that conversational-overseer remains reachable after slot bindings are completed in a real thread.
- Preserve locked design tokens and localize all copy in `en` and `pt-BR`.

**Non-Goals:**

- No new vertical CRM/ERP modules or hand-coded business workflows.
- No change to ADR-012 SlotBinding semantics; a PlanStep still binds to one ExecutorInstallation.
- No broad redesign of executions, artifacts, audit, or settings beyond introducing the shared summary contract and links/actions required for continuity.
- No production migration/deprecation shim; this is pre-v1.

## Decisions

### 1. Use the thread as the primary state timeline

Proposal/refinement interactions will append durable thread messages using existing thread message kinds where possible:

- `PLAN_PROPOSED` remains the first assistant response to a user request.
- User chip/button choices append `USER_SELECTION` with `in_response_to_message_id`, option metadata, and selected values.
- Assistant follow-up questions append `ASSISTANT_PROMPT` with refinement states such as `PLAN_REFINEMENT_AUDIENCE`, `PLAN_REFINEMENT_THEMES`, `PLAN_REFINEMENT_SOURCES`, `PLAN_REFINEMENT_DATE_RANGE`, and `PLAN_REFINEMENT_CONFIRM`.
- Plan creation appends/continues into the existing plan assistant sequence (`CONFIGURATION_STARTED`, `ASSISTANT_PROMPT`) in the same route.

Alternative considered: keep all refinement state local inside `PlanProposalCard`. That is faster but reproduces the modal feel and loses auditability, so it is only acceptable for ephemeral UI affordances while the durable transcript is being written.

### 2. Keep `PLAN_PROPOSED` as the ranked candidate payload and enrich it

`PLAN_PROPOSED` already carries summary and candidates. The payload will be extended with non-breaking JSON fields:

- `best_candidate_id` or equivalent derived best-match marker.
- Per-candidate recommendation reason and compatibility label.
- `refinement_defaults` containing suggested audience, themes, topics to avoid, source groups, language/tone, and date range.

The frontend may still derive best match from highest confidence when the backend field is absent, but backend output should be authoritative so tests do not depend on UI heuristics.

Alternative considered: introduce a new proto message/RPC for proposal refinement. The existing `payload_json` message model is already used for assistant prompts and keeps the change smaller.

### 3. Render proposal refinement with a conversation reducer plus structured chips

The UI will replace the single-stage proposal card with a conversation renderer that reads the thread messages and displays:

- Candidate shortlist with the best match visually emphasized and alternatives available.
- Assistant turns for each refinement question.
- User selections as chat bubbles or compact selection summaries.
- Chips for recommended themes, topics to avoid, and source groups; custom entries are accepted inline.
- A date-range control labeled as the source publication window.
- A final confirmation written as a full sentence assembled from localized templates and selected values, not stitched fragments.

The interaction pattern remains dense and operational. Cards are only used for individual proposal/summary blocks; no nested card stacks. Motion is limited to opacity/transform and respects reduced-motion.

### 4. Materialize multiple source groups as one aggregate RSS installation

Template inputs will move from a singular `source_group` UI value to a selected source-group list for the LinkedIn/news digest flow. Before creating the PlanConfiguration, the system will create or reuse a tenant-scoped RSS ExecutorInstallation whose config contains the union of feed URLs from the selected groups. The `fetch-news` SlotBinding points to that single aggregate installation.

This preserves ADR-012 because feed URLs remain in ExecutorInstallation config, not in PlanTemplate or Artifact payloads. It also avoids duplicate `fetch-news` SlotBindings.

Alternative considered: create multiple SlotBindings for one step. Rejected because SlotBinding points one PlanStep at one installation.

### 5. Post-create actions stay in the same route

After creation, the thread shows a contextual assistant turn with actions:

- Run now
- Schedule
- Review plan
- Adjust configuration
- Ask about another plan

`Anything else?`/`Ask about another plan` must not navigate to `/`. It keeps the user in `/chat/<threadId>`, appends or focuses a new user-turn opportunity, and allows proposal of another plan in the same conversation. If current data structures still expose one active PlanConfiguration per thread, implementation must support at least the single-active-plan path cleanly and document any remaining multi-plan-thread limitation in the idea log.

### 6. Introduce one shared plan-summary contract

Create a small shared model/helper and component vocabulary for PlanConfiguration summary:

- Plan identity: template name, configured title/intent, status.
- Configuration state: executors, overseers, behavior policies, schedule, selected source groups, seed inputs.
- Runtime state: latest execution, next scheduled run, readiness/cost indicators where available.
- Controls: run, schedule, revise, inspect artifacts, inspect audit.

Chat, lists, canvas/settings, execution detail, artifacts, and audit should use this vocabulary even when each surface has different density.

## Risks / Trade-offs

- [Risk] Multiple plans in one thread may exceed the current `activePlanConfigurationID` assumption. -> Mitigation: implement same-thread continuation for the first active plan now, allow new proposal turns without leaving the thread, and record the multi-plan-thread model if a deeper data model change is required.
- [Risk] Backend-generated recommendations may be thin while the catalog has only one template. -> Mitigation: derive deterministic recommendations from template options and classifier inputs, then improve with richer catalog content later.
- [Risk] Aggregate RSS installations can create duplicates. -> Mitigation: use a stable tenant/template/source-group key or config hash to reuse an existing aggregate installation when selections match.
- [Risk] Final confirmation copy can regress in one locale. -> Mitigation: keep sentence templates localized and test `en` and `pt-BR` resolution.
- [Risk] The overseer issue may be state-machine or transition related. -> Mitigation: include explicit end-to-end verification that all slot bindings lead to an `OVERSEER_STEP` in the same thread, then fix the smallest failing layer.
