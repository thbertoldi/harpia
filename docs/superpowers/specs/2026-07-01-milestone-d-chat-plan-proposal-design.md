# Milestone D: Chat-Driven Plan Proposal

**Date:** 2026-07-01
**Status:** Approved design; awaiting implementation plan
**Related specs:**
- `docs/superpowers/specs/2026-06-30-path-b-chat-first-architecture.md` (§8.D)
- `docs/superpowers/specs/2026-06-29-conversational-plan-workspace-design.md`

**Builds on:** Path B chat thread foundation
(`docs/superpowers/plans/2026-06-30-path-b-chat-thread-foundation.md`, shipped).

## 1. Decision

A user in a thread can go from a natural-language message to an **attached,
ready-to-configure plan** without choosing a template first. An LLM-backed
router classifies the message against the plan template catalog, proposes a
plan with inferred inputs, lets the user review/correct those inputs inline, and
on confirmation creates the `PlanConfiguration` — after which the existing
deterministic `planassistant` binding-matrix flow takes over unchanged.

This is the milestone that closes the foundation's headline gap: today
`/chat/[threadId]` cannot originate a plan (the empty-state has no composer and
`/new` still requires manual template selection).

## 2. Goals

1. Let a thread turn a free-text request into a proposed plan.
2. Use an LLM to pick the template and extract template inputs.
3. Return the proposal synchronously so it feels like chat.
4. Let the user review and correct inferred inputs before the plan is created.
5. Hand off cleanly to the existing binding-matrix configuration flow on attach.
6. Make `/new` prompt-first while keeping template browsing as a fallback.
7. Keep the router behind a port so its implementation (LLM today, something
   else later) can change without touching the thread/proposal flow.

## 3. Non-Goals

- Running a plan from chat. That is Milestone E (`PLAN_RUN_REQUESTED`,
  execution/artifacts/approvals in thread context).
- Post-attach plan edits via chat, or `PLAN_UPDATED` semantics.
- `ARTIFACT_*` and `ERROR_*` message-kind UX (Milestone E).
- Replacing the LinkedIn-specific input form used inside `BindingMatrixCard`.
  The new generic form is introduced for the proposal card only in D.
- A conversational multi-turn clarification loop. Low-confidence handling is a
  shortlist, not a back-and-forth dialogue.
- Any change to `planassistant` (the deterministic state machine).

## 4. Scope

Delivered message kinds:

- `THREAD_MESSAGE_KIND_PLAN_PROPOSED` (18) — rich proposal card.
- `THREAD_MESSAGE_KIND_PLAN_ATTACHED` (19) — system event announcing a plan was
  created from the thread.

The remaining unused kinds (`PLAN_UPDATED` 20, `PLAN_RUN_REQUESTED` 21,
`ARTIFACT_CREATED` 22, `ARTIFACT_UPDATED` 23, `ERROR_RAISED` 24,
`ERROR_RECOVERED` 25) receive frontend enum mappings (cheap, avoids gaps) but no
bespoke rendering — they fall through to `SystemEventCard`. Their product UX is
out of scope for D.

## 5. Backend Architecture

### 5.1 ProposePlan RPC

Add to `harpia.chat.v1.ThreadService`:

```proto
rpc ProposePlan(ProposePlanRequest) returns (ProposePlanResponse);

message ProposePlanRequest {
  string tenant_id = 1;
  string thread_id = 2;
}

message ProposePlanResponse {
  ThreadMessage message = 1; // the appended PLAN_PROPOSED message
}
```

Behavior:

1. Resolve tenant + thread (tenant-scoped, RLS as usual).
2. Read the latest `USER_TEXT` message in the thread. If none, return
   `FailedPrecondition`.
3. Load the template catalog for the tenant.
4. Call the `PlanClassifier` port with (tenant, text, templates).
5. Append a `PLAN_PROPOSED` message whose payload carries the ranked candidates
   (possibly empty), and return it.

The RPC lives on `ThreadService` because it operates on a thread and produces a
thread message. The handler depends on a template-catalog reader and the
classifier port — not on `PlanService` internals.

### 5.2 PlanClassifier port

A Go interface in the control plane:

```go
type PlanClassifier interface {
    Classify(ctx context.Context, in ClassifyInput) ([]Candidate, error)
}

type ClassifyInput struct {
    TenantID  uuid.UUID
    Text      string
    Templates []TemplateSummary // id, key, name, description, input_parameters
}

type Candidate struct {
    TemplateID       uuid.UUID
    Confidence       float64 // 0..1
    InputValuesJSON  string  // extracted values keyed by input parameter key
}
```

Handler tests inject a fake classifier returning fixed candidates
(single / shortlist / empty), keeping the RPC deterministic under test.

### 5.3 LLM adapter

The production `PlanClassifier` implementation:

- Resolves the tenant's provider/model/API key through the existing
  `llm_config` resolver + keyring (roster: DeepSeek/Qwen/etc.; Claude excluded
  per the account-conflict constraint).
- Sends **one** JSON-mode chat completion. The prompt supplies the catalog
  (key, name, description, input parameter keys/types) and the user text, and
  asks for a strict JSON object: a ranked list of candidates with a confidence
  and an `inputs` object keyed by the chosen template's parameter keys.
- Validates every returned template key against the catalog (drops unknown
  keys), and coerces `inputs` onto each template's declared `input_parameters`
  (drops unknown keys, leaves missing ones empty).
- **Graceful degradation:** if no LLM is configured for the tenant, or the call
  fails/times out, or the response fails to parse, it returns an **empty**
  candidate list. `ProposePlan` still succeeds and emits a `PLAN_PROPOSED` with
  no candidates; the card renders the "no match — browse templates" state. The
  chat path never hard-fails because of the model.

### 5.4 Confidence handling

- If exactly one candidate clears the confidence threshold → the card renders
  single-proposal mode (that template + its inputs form).
- Otherwise (multiple plausible, or all below threshold but non-empty) → the
  card renders a ranked shortlist (top 2–3) for the user to choose.
- Empty candidates → "no match" state.

The threshold is a single documented constant; tuning it is not a schema change.

### 5.5 Attach (PLAN_ATTACHED)

Confirmation reuses the existing `PlanService.CreatePlanConfiguration`
(already thread-aware from the foundation: it requires `thread_id` and sets
`threads.active_plan_configuration_id`). The only addition: when
`CreatePlanConfiguration` is called with a `thread_id`, it appends a
`PLAN_ATTACHED` system message to that thread (best-effort, alongside the
existing link).

No change to `planassistant`: its `SeedThread` already fires on configuration
creation, emitting `CONFIGURATION_STARTED` and the binding-matrix
`ASSISTANT_PROMPT`. After attach, the proven matrix → save flow resumes.

## 6. Frontend Architecture

### 6.1 Message-kind mapping

Extend `ChatMessageKind` (`frontend/src/lib/chat/types.ts`) and the proto
enum maps with all eight new kinds. Only `PLAN_PROPOSED` gets a dedicated card;
`PLAN_ATTACHED` and the rest render via `SystemEventCard`.

### 6.2 PlanProposalCard

New `frontend/src/lib/components/thread/PlanProposalCard.svelte`, dispatched
from `ThreadMessage.svelte` on `PLAN_PROPOSED`. Reads its `payloadJson`:

- **Single confident candidate:** template header + `TemplateInputsForm`
  pre-filled with the extracted values, and a Confirm action.
- **Shortlist:** candidate chips; selecting one reveals that template's
  `TemplateInputsForm`.
- **Empty:** a short "couldn't match your request — browse templates" message
  linking to the gallery.

Confirm calls the existing `createPlanConfiguration` (via the shared
`plan-configuration` helper) with `thread_id`, the chosen `template_id`, and
`parameter_values_json` serialized from the form. The new `PLAN_ATTACHED` and
subsequent `CONFIGURATION_STARTED` / matrix `ASSISTANT_PROMPT` arrive through
the existing watch stream.

### 6.3 TemplateInputsForm

New `frontend/src/lib/components/thread/TemplateInputsForm.svelte`, driven by a
template's `input_parameters`. Renders a control per parameter `type`:
`TEXT`, `TEXTAREA`, `SELECT` (from `options_json`), `LANGUAGE`, `DATE_RANGE`,
`INTEGRATION_SELECTOR`. Emits a values object serialized to
`parameter_values_json`. This is the reusable generic form; it is used by the
proposal card in D. (Generalizing `BindingMatrixCard` to use it is a later
cleanup, explicitly out of scope here.)

### 6.4 Routes and composer

- `/chat/[threadId]`: always render the composer (the foundation hid it when
  config-less). On load, if there is no active configuration and the trailing
  message is an unanswered `USER_TEXT` (no `PLAN_PROPOSED` after it), call
  `ProposePlan` and show a pending/typing indicator. In-thread sends while
  config-less follow the same append-`USER_TEXT`-then-`ProposePlan` path.
- `/new`: prompt-first. Submit creates a thread with the initial message and
  navigates to `/chat/[threadId]` (which auto-proposes). The template gallery
  remains below as a fallback that creates a thread with the template
  pre-attached (current behavior, now routing through `/chat`).

## 7. Data Flow (happy path)

1. `/new`: user types "Create a LinkedIn post about retail in Portuguese" →
   `createThread(initialMessageText)` → navigate to `/chat/[threadId]`.
2. Chat page: no config + unanswered `USER_TEXT` → `ProposePlan(thread_id)`.
3. Control plane: latest user text → catalog → `PlanClassifier` (LLM) → ranked
   candidates → append `PLAN_PROPOSED`.
4. Watch stream delivers `PLAN_PROPOSED` → `PlanProposalCard` shows the LinkedIn
   template with theme="retail", language="pt-BR" pre-filled.
5. User adjusts inputs → Confirm → `createPlanConfiguration(thread_id, ...)`.
6. Control plane: creates config, links thread, appends `PLAN_ATTACHED`;
   `planassistant.SeedThread` appends `CONFIGURATION_STARTED` + matrix prompt.
7. Existing binding-matrix flow resumes in the same thread.

## 8. Error Handling

- No `USER_TEXT` in thread → `ProposePlan` returns `FailedPrecondition`; the
  client only calls it when a trailing user message exists, so this is a guard.
- LLM unavailable / failing / unparseable → empty candidates, `PLAN_PROPOSED`
  still emitted, card shows "no match" state. Never blocks the thread.
- Unknown template keys or input keys from the model → dropped during
  validation; a candidate with an unknown template is discarded.
- Duplicate proposals → client checks for an existing `PLAN_PROPOSED` after the
  latest user message before calling `ProposePlan`.

## 9. Testing

Backend:

- `ProposePlan` with a fake classifier: single confident, shortlist, and empty
  results each produce the expected `PLAN_PROPOSED` payload.
- `ProposePlan` with no user message returns `FailedPrecondition`.
- LLM adapter unit test: prompt is built from the catalog; a stubbed model
  response is parsed and validated (unknown template/input keys dropped);
  failure/timeout yields empty candidates.
- `CreatePlanConfiguration` with a `thread_id` emits `PLAN_ATTACHED`.
- Tenant isolation on `ProposePlan`.

Frontend:

- `PlanProposalCard` renders single / shortlist / empty modes from payload.
- `TemplateInputsForm` renders and serializes each parameter type.
- Confirm calls `createPlanConfiguration` with the edited values + `thread_id`.
- `/new` prompt-first submit routes to `/chat/[threadId]`.
- Chat page auto-proposes only when config-less with an unanswered user message.

End to end (smoke):

1. `/new` with "Create a LinkedIn post about retail in Portuguese".
2. Land in `/chat/[threadId]`; a proposal appears with inferred theme+language.
3. Adjust an input, Confirm.
4. `PLAN_ATTACHED` + binding matrix appear in the same thread.

## 10. Risks

- **Model quality / cost.** Off-catalog or messy requests may mis-route.
  Mitigation: confidence threshold → shortlist; empty → gallery; one call per
  proposal; small/cheap roster model.
- **Non-deterministic tests.** Mitigation: classifier is a port; all handler
  and UI tests use fakes/fixtures. Only the thin adapter touches a real model,
  tested against a stub.
- **New LLM-call capability in control-plane.** Mitigation: encapsulated behind
  the adapter using the existing `llm_config` resolver/keyring; no new secret
  handling.
- **Scope creep toward a full copilot.** Mitigation: D stops at propose →
  review → attach; running, updating, and artifact/error UX are later
  milestones.

## 11. Acceptance Criteria

Milestone D is complete when:

1. `ThreadService.ProposePlan` classifies a thread's latest user message and
   emits a `PLAN_PROPOSED` message.
2. Routing runs through a `PlanClassifier` port with an LLM adapter using the
   tenant's configured provider; an unconfigured/failing model degrades to an
   empty proposal, not an error.
3. `PLAN_PROPOSED` renders as a card with single-proposal, shortlist, and
   no-match modes.
4. The card shows an editable, generic inputs form pre-filled with the model's
   extracted values.
5. Confirming creates a `PlanConfiguration` bound to the thread and emits
   `PLAN_ATTACHED`, after which the existing binding-matrix flow appears.
6. `/chat/[threadId]` shows a composer even without a plan and auto-proposes for
   an unanswered opening message.
7. `/new` is prompt-first and routes to `/chat/[threadId]`, with the gallery
   retained as a fallback.
8. Backend, frontend, and a chat-first proposal e2e smoke are covered by tests
   using fakes for the model.

## 12. Implementation Plan Gate

Do not implement from isolated edits. The next step is a separate
implementation plan (via the writing-plans skill) sequencing: proto +
regeneration, classifier port + LLM adapter, `ProposePlan` handler, attach
`PLAN_ATTACHED`, frontend enum mappings, `TemplateInputsForm`,
`PlanProposalCard`, chat composer/auto-propose, `/new` prompt-first, and e2e
smoke — each task with its own verification, executor-cold.
