# Harpia UX Realignment — M5 Chat-as-Configuration — Implementation Design Spec

**Date:** 2026-06-22
**Status:** Approved (brainstorm), awaiting plan
**Originating conversation:** Brainstorming session 2026-06-22 between Thiago and Claude.
**Extends:** `docs/superpowers/specs/2026-06-19-harpia-ux-realignment-design.md` (master design)

This spec resolves M5's implementation-level decisions. The user-facing UX shape is settled in the master spec — §4.1 (chat as primary surface), §4.5 (Overseer per-Task), §4.6 (cost pill), §5.2 (plan thread), §5.5 (`/new`), §6 (route disposition), §10 (M5 line item). This document captures the assistant's state-machine architecture, message protocol, entry-flow wiring, schedule + cost + sidebar additions, and the route deletion plan needed to write the M5 implementation plan.

---

## 1. Scope

M5 ships, behind no flag (same posture as M2/M3/M4 — universal):

1. **Configuration assistant** — a deterministic state machine inside a new `control-plane/internal/planassistant/` package. No LLM. Drives Ana through `greet → pick template → bind each Task → set Overseer per Task → set policies → confirm → save`. Composer accepts text; primary interaction is quick-reply chips emitted under each assistant prompt.
2. **`/new`** — empty-state launcher (replaces the M1 stub). Greeting + template gallery chips + composer. On template commit, POST `CreatePlanConfiguration(template_id, status=DRAFT)`; redirect to `/plans/configurations/<id>`. `/new` itself holds no assistant state.
3. **Schedule dialog** — wires the M4-stubbed canvas top-bar `Schedule` button. Cron/recurrence config persisted on the PlanConfiguration. The proto already carries `PlanConfiguration.schedule` (`PlanSchedule { cron_expression, timezone }`) and the DB already has a `schedule JSONB` column — `UpdatePlanConfiguration` accepts it today. M5 adds no new RPC and no migration for this; only the dialog UI and the wiring that calls `UpdatePlanConfiguration` with the schedule field. Cron-to-Temporal **execution wiring is explicitly deferred**; M5 ships the configuration surface only.
4. **Cost pill** — live running sum in the plan-thread top bar and the canvas top bar. Updates as bindings change.
5. **Sidebar "Your plans" list** — persistent left-rail entries under `+ New plan`, sourced from `ListPlanConfigurations`. Closes the M3 §2.11 deferral.
6. **Wizard deletion** — the four `/plans/[templateId]/configure/*` routes are removed. Transitional 302 redirects to `/new?template=<id>` for one release; 404 in M6.

**Out of scope** (deferred by explicit decision):
- LLM-driven intent recognition or free-text dialog (see §2.1). `/new`'s free-text matcher is client-side keyword.
- Cron-to-Temporal wiring (the actual scheduler trigger) — its own backend milestone.
- Agent-as-Overseer (v2 per master §4.5).
- Per-tenant currency / multi-currency cost display — BRL only for v1.
- A dedicated `My plans` route (sidebar list "See all" link points to `/discover` for v1).
- M6 work: persona/admin split, persona toggle, dev-login persona collapse.
- **Seed-artifact collection.** Templates can declare `SeedArtifactBinding` (e.g., an initial RSS feed URL for a newsletter plan). The existing wizard does not collect seed artifacts either — the data path exists but the UX surface is empty. M5 inherits that gap: the assistant walks bindings/overseer/policies/schedule only and saves with `seed_artifacts: []`. A follow-up milestone introduces a `BIND_SEED_ARTIFACT` state once templates that need seeds are documented. Out-of-scope is consistent with current v1 capability, not a regression.

---

## 2. Resolved decisions

### 2.1 Brain — deterministic state machine, no LLM

A new `control-plane/internal/planassistant/` Go package owns the configuration flow. No LLM calls. The state machine is **derived**, not stored: given the PlanConfiguration's current state (template + bindings + overseer + policies + schedule + status) and the chat_messages history, a pure function returns the next assistant turn.

Rejected alternatives (recorded):
- **LLM-augmented intent + slot-fill.** Two narrow LLM calls (free-text at `/new` → template pick; free-text inside slot binding → executor pick) over the existing agent-runtime provider abstraction + tenant BYO keys. Rejected for v1: deterministic flow ships faster, costs zero per plan, has no fallback complexity, and the chip-driven UX is sufficient given v1 templates are a small catalog. The grammar is preserved — when v2 enables free-form composition, the brain can be swapped behind the same chat protocol.
- **Fully conversational LLM agent in agent-runtime** with tool calls (`SuggestTemplate`, `BindExecutor`, etc.). Rejected: highest complexity, real token cost per plan, hallucination risk on bindings. The assistant's job is constrained enough that a state machine fully covers it.

### 2.2 States and transitions

Linear walk over the template's steps, with two side-trips (template re-pick and rebind). Each state corresponds to one assistant turn.

**`AWAITING_TEMPLATE`** is documented for completeness but **unreachable in v1** — the entry flow (§2.4) always commits a template before calling `CreatePlanConfiguration`, so a configuration with empty `plan_template_id` does not exist in v1. The state is present in the derivation function as a defensive fallback that surfaces a `TemplatePickerCard` if reached (e.g., via a future "blank plan" flow). v1 derivation should never land here.

```
AWAITING_TEMPLATE                 (unreachable in v1 — see above)
       │  (USER_SELECTION: chosen template)
       ▼
BINDING_STEP(step_key)        ─┐
       │  (USER_SELECTION:    │  Iterate steps in order.
       │   chosen executor)   │  After last step, advance to overseer.
       ▼                      │
SET_OVERSEER(step_key)        │  Same step_key; default "You".
       │  (USER_SELECTION)    │
       ▼                      │
   (next step?)  ─── yes ─────┘
       │  no
       ▼
SET_POLICIES                      timeout + approval-mode (same fields as
       │  (USER_SELECTION)        the existing PlanPoliciesForm)
       ▼
CONFIRM                           summary card; chips: Save / Edit X
       │  (USER_SELECTION: Save)
       ▼
SAVED                             promotes status DRAFT → RUNNABLE
                                  if validation passes (else stays DRAFT
                                  with inline notice — same semantics as
                                  the wizard's promoteToRunnable).
```

Side trips:
- **Edit pencil** on a prior `ASSISTANT_PROMPT` (any kind) → opens the chip group again with current selection highlighted. Picking a new option writes `STEP_REBOUND` + calls `UpdatePlanConfiguration`. Does NOT rewind the state machine; the linear walk continues at its current cursor.
- **Re-pick template** is **not supported in v1**. Once a template is bound (which happens at `CreatePlanConfiguration` time in v1), it cannot be swapped on the same PlanConfiguration. Switching templates means abandoning this configuration and starting a new one from `/new`.

The state derivation function (`planassistant.DeriveState(configuration, messages)`) reads:
- `configuration.template_id` → if empty, state is `AWAITING_TEMPLATE`.
- iterate `template.steps` in order:
  - find first step with no `slot_bindings` entry → state is `BINDING_STEP(step_key)`.
  - if all steps bound, find first step with no overseer set → state is `SET_OVERSEER(step_key)`.
- if policies unset → state is `SET_POLICIES`.
- if all the above complete and no `SAVED` system message exists for the current cursor → state is `CONFIRM`.
- else `SAVED`.

The status (DRAFT / RUNNABLE) is **orthogonal** to the assistant state: the assistant emits a `CONFIRM` prompt whenever derivation lands there, regardless of whether status is DRAFT or RUNNABLE. After a STEP_REBOUND on a RUNNABLE configuration, derivation lands at `CONFIRM` again (Ana re-confirms via the chip; if she doesn't, status stays RUNNABLE — the unsent CONFIRM is informational).

### 2.3 Chat protocol — new message kinds

Additive to the M3/M4 set in `harpia/chat/v1/chat.proto`:

| Kind | Author | When emitted | `payload_json` shape (sketch) |
|---|---|---|---|
| `CONFIGURATION_STARTED` | SYSTEM | Once, on configuration insert | `{ "template_id": "..." }` |
| `ASSISTANT_PROMPT` | AGENT (assistant) | Whenever `NextTurn` advances state | `{ "state": "BINDING_STEP", "step_key": "...", "options": [{ "id", "label", "sublabel", "value", "price_brl" }] }` |
| `USER_SELECTION` | OVERSEER | When Ana clicks a chip | `{ "in_response_to_message_id": "...", "option_id": "...", "value": "..." }` |
| `STEP_REBOUND` | SYSTEM | On edit-pencil rebind | `{ "step_key": "...", "previous_executor_installation_id": "...", "new_executor_installation_id": "..." }` |
| `SCHEDULE_SET` | SYSTEM | On schedule dialog save | `{ "schedule_cron": "...", "timezone": "..." }` |

Existing kinds (M3/M4) are unchanged. `ASSISTANT_TEXT` (M3 reserved) is used for assistant turns that don't carry chips — narrative-only interjections (e.g., "Saved. Open the canvas any time to watch a run."). In M5, the only ASSISTANT_TEXT we emit is the post-Save acknowledgment.

`USER_TEXT` (M3) stays passive: composer text writes a USER_TEXT row, no assistant response. The M3 hint banner ("the assistant doesn't reply to typed messages — use the chips") survives unchanged in M5; copy may be tightened.

### 2.4 Entry flow

`/new` (formerly an M1 stub):

- `+page.svelte` renders: greeting heading, template gallery (chip cards), composer with placeholder "Or describe what you want to automate…".
- `+page.ts` loads `ListPlanTemplates` (existing RPC) for the gallery.
- Gallery card click → calls a thin wrapper that POSTs `CreatePlanConfiguration(template_id, status=DRAFT, slot_bindings=[])`. The server-side handler (in `configuration_handler.go`) now does, in one transaction:
  1. INSERT into `plan_configurations`.
  2. Append `CONFIGURATION_STARTED` chat message.
  3. Call `planassistant.SeedThread(configurationId)` which appends the first `ASSISTANT_PROMPT` for `BINDING_STEP(steps[0])`.
- Client receives the configuration_id and navigates to `/plans/configurations/<id>`.
- Composer text on `/new` runs a client-side keyword match against template name + description (lowercased substring). If a single match emerges, the chip for that template highlights and pressing Enter commits it. If multiple or none, a small hint renders ("No template matched '<text>'. Pick one below.") and no auto-commit happens.
- `/new` is a transient launcher: it never holds assistant state. Reloading `/new` always returns to the empty greeting.

### 2.5 Assistant rendering inside the thread

The existing `/plans/configurations/[configurationId]/+page.svelte` is extended (no replacement). The thread's `ThreadMessage` dispatcher gains a branch for the new kinds:

- `ASSISTANT_PROMPT` → new component `lib/components/thread/AssistantPromptCard.svelte`. Props: `{ message, configurationId, isLatest }`. Renders the assistant's prose (from `message.text`) plus a chip group derived from `payload_json.options`. The card has an inline `Edit` pencil if the prompt has already been answered (a downstream `USER_SELECTION` exists with `in_response_to_message_id` pointing to this message AND the state is past this step). The latest unanswered prompt is the "live" prompt and is visually distinct (e.g., highlighted border).
- `USER_SELECTION` → rendered as a small bubble: "→ <option label>" (the value Ana picked). No interactivity.
- `STEP_REBOUND` / `SCHEDULE_SET` / `CONFIGURATION_STARTED` → re-uses M3's `SystemEventCard.svelte` with kind-specific copy keys.

For the `CONFIRM` state's prompt, a specialized card `lib/components/thread/ConfirmCard.svelte` is rendered instead of the generic chip card. It shows: template name, ordered list of (Task → Executor + Overseer), policies summary, total cost per run. Chips: `Save` (primary, promotes to RUNNABLE) and `Edit <step>` (one per step — clicking jumps to that prompt's edit pencil).

### 2.6 Trigger model — when the assistant emits

Two entry points trigger `planassistant.NextTurn`:

- **Configuration create.** `CreatePlanConfiguration` in `configuration_handler.go` calls `planassistant.SeedThread` after inserting. Emits `CONFIGURATION_STARTED` + first `ASSISTANT_PROMPT`. Single transaction.
- **USER_SELECTION append.** `AppendPlanThreadMessage` in `thread_handler.go` already accepts arbitrary kinds. When `request.kind == USER_SELECTION`, after writing the message the handler calls `planassistant.NextTurn(configurationId)`. NextTurn:
  1. Re-derives state from the post-USER_SELECTION configuration + messages.
  2. Applies the side effect of the user's selection: e.g., `BINDING_STEP` selection → `UpdatePlanConfiguration` with new slot_binding entry; `SET_OVERSEER` selection → update overseer field; `SET_POLICIES` selection → update policy fields; `CONFIRM` Save → set status RUNNABLE if validation passes.
  3. If new state has a next prompt (i.e., not yet `SAVED`), appends the next `ASSISTANT_PROMPT`.
  4. If state is `SAVED`, appends one `ASSISTANT_TEXT` narrative message ("Saved. Run it manually from the canvas or set a schedule.") and ends.
  All in one transaction. Idempotent: re-calling `NextTurn` on the same state is a no-op (already-emitted prompts are detected by sequence_number lookup before appending).

For the edit pencil, the frontend calls `lib/plans/assistant.ts → editBinding(...)` which:
1. Writes a `STEP_REBOUND` chat message (kind=STEP_REBOUND, role=SYSTEM, written via `AppendPlanThreadMessage`).
2. Calls `UpdatePlanConfiguration` with the swapped slot_binding.
Both calls execute client-side in sequence; the server does not invoke `NextTurn` on `STEP_REBOUND` (intentionally — rebind doesn't advance the state machine).

### 2.7 Schedule dialog

`SchedulesDialog.svelte` lives in `lib/components/canvas/` (next to `SettingsDrawer.svelte`). Modal, not drawer — focused single-purpose interaction per master §5.3.

Fields:
- `Cadence` chip row: `Manual only`, `Daily`, `Weekly`, `Monthly`, `Custom cron`.
- Conditional second row:
  - `Daily` / `Weekly` / `Monthly` → time picker (`HH:MM`) + (for `Weekly`) day-of-week chips, (for `Monthly`) day-of-month input.
  - `Custom cron` → free-text textbox plus a small "Cron syntax" tooltip.
- `Timezone`: read-only display defaulting to tenant timezone (existing setting if present, else `UTC`). v1 does NOT let Ana change timezone — out of scope.
- Cancel / Save buttons.

Save:
- The component constructs the cron expression from the cadence + time fields (or passes the custom string through verbatim).
- Calls `UpdatePlanConfiguration(configurationId, { schedule: { cron_expression, timezone } })` — the existing RPC already accepts `PlanSchedule`; no new RPC is added.
- The `configuration_handler.go` `UpdatePlanConfiguration` method gains a small post-write side effect: if the incoming `schedule` differs from the previous value, validate the cron (`github.com/robfig/cron/v3` parser — add to `go.mod` if not present) and append a `SCHEDULE_SET` chat message. Validation failure returns the error to the client; nothing persists.
- If the schedule is set to non-empty cron AND the configuration is `RUNNABLE`, status promotes to `SCHEDULED` (an existing enum value — `PLAN_CONFIGURATION_STATUS_SCHEDULED`). Clearing the cron back to manual-only on a `SCHEDULED` configuration demotes it to `RUNNABLE`. DRAFT configurations stay DRAFT regardless of schedule.

Entry points: the canvas top bar `Schedule` chip (M4 stub → real) and a `Schedule` chip next to the cost pill in the plan-thread top bar.

**Out of scope:** the cron does NOT yet trigger executions. The persisted cron is the configuration's stated intent; a follow-up backend milestone wires it to Temporal Schedules or equivalent. M5's `SCHEDULE_SET` chat message + persisted column close the UX loop; the runtime wiring is an internal-engineering follow-up that's invisible to Ana.

### 2.8 Cost pill

Pure-function compute over current bindings.

`lib/plans/cost.ts`:

```ts
export type CostBreakdownEntry = {
  stepKey: string;
  stepTitle: string;
  executorInstallationId: string | null;  // null = unbound
  executorName: string | null;
  pricePerRunBrl: number | null;
};
export type RunCost = {
  totalPerRunBrl: number;
  currency: "BRL";
  unboundStepCount: number;
  breakdown: CostBreakdownEntry[];
};
export function computeRunCost(
  template: PlanTemplate,
  configuration: PlanConfiguration,
  executorCatalog: ExecutorCatalog,
): RunCost;
```

The function walks `template.steps` in order, for each step finds the bound executor from `configuration.slot_bindings`, looks up its price in `executorCatalog` (existing data), and sums. Unbound steps contribute 0 and increment `unboundStepCount`. Pure; unit-tested without browser.

`PlanCostPill.svelte` props: `{ template, configuration, executorCatalog }`. Renders compactly: `R$ 0.42 / run` when fully bound; `R$ 0.18 / run · 2 steps pending` when partial. Hover/tap reveals the breakdown list.

Mounted in:
- The plan-thread top strip (added in this milestone — currently the M3 page has no top strip; M5 adds a one-row top strip with plan name + status badge + cost pill + schedule chip).
- `CanvasTopBar.svelte` — extended; the existing top bar already has slots for buttons; the pill sits next to `Run history`.

### 2.9 Sidebar "Your plans" list

`lib/components/sidebar/YourPlansList.svelte`. Mounted inside the existing sidebar component (locate during plan-writing — likely `Sidebar.svelte` or the layout file that owns the left nav).

Data: subscribe to `ListPlanConfigurations(tenant_id)` (existing RPC). The current `ListPlanConfigurations` is a streaming RPC; the sidebar component reuses the same polling/streaming pattern as M2's inbox aggregator (2-second ticker; AbortController + `$effect`).

Rendering:
- Header: "Your plans" (i18n key `sidebar.yourPlans`).
- Up to 10 rows. Each row: configuration name (truncated) + status badge (`DRAFT` / `RUNNABLE` — small text pill) + relative time since last execution (computed client-side from `last_execution_at` if present, else "never run").
- Sort: most-recently-active first. Recency = `updated_at` from `PlanConfiguration` (the proto has no `last_execution_at` field). The configuration's `updated_at` already advances on each `STEP_BOUND` / `RUN_*` activity in M3's path because those go through `UpdatePlanConfiguration` snapshots, so it's a sufficient proxy for v1. If precision becomes a concern later, the M3 chat-stream's `RUN_COMPLETED` event can be joined client-side.
- Click row → navigate to `/plans/configurations/<id>`. Active row highlighted (compare path).
- Empty state: "No plans yet. Start one with **+ New plan**." (the existing `+ New plan` button stays above the list).
- Capped at 10 visible; "See all" footer link → `/discover`. (A dedicated `My plans` route is deferred — `/discover` accepts a `?filter=mine` query as a non-blocking follow-up, but for M5 the link goes to the gallery and Ana finds her own there via the same data.)

The sidebar entry shows only for Ana (operator persona). Persona detection in M5 reuses whatever the current sidebar already does to gate items (probably nothing yet — M6 will formalize it). If no detection exists, the list shows for all sidebar users; M6 hides it for Platform Engineers.

### 2.10 Wizard deletion

The four wizard routes under `/plans/[templateId]/configure/` are deleted:

- `/plans/[templateId]/configure/+page.svelte` (468 LOC slot-binding page)
- `/plans/[templateId]/configure/overseer/+page.svelte`
- `/plans/[templateId]/configure/policies/+page.svelte`
- `/plans/[templateId]/configure/summary/+page.svelte`
- `/plans/[templateId]/configure/+layout.server.ts`

Each route is replaced by a tiny `+page.ts` that 302-redirects to `/new?template=<templateId>`. The `?template=` param tells `/new` to auto-commit that template on mount (treat as if Ana clicked its chip immediately). One release of redirects; M6 deletes them entirely (404).

The slot-binding **helpers** (`lib/plans/slot-binding.ts`) survive — `planassistant` and the cost pill both reuse them (`validateSlotBindings`, `getCompatibleInstallationsForStep`, `selectionsToSlotBindings`). The wizard page is deleted; its data layer is repurposed.

`/plans/[templateId]/+page.svelte` (the template detail page) **stays unchanged** — it's the legacy template detail still served from `/discover`. The "Use this template" CTA on that page is rewired to redirect to `/new?template=<templateId>` instead of `/plans/[templateId]/configure`.

### 2.11 Status transition semantics

The wizard had a draft → runnable promotion gesture (`promoteToRunnable`). M5 preserves the same semantics inside the assistant:

- Configuration created via `/new` starts as `DRAFT` with no bindings.
- Each `USER_SELECTION` against a `BINDING_STEP` / `SET_OVERSEER` / `SET_POLICIES` mutates the configuration **but does not change status** — it stays DRAFT throughout the walk.
- The `CONFIRM → SAVED` Save chip promotes status DRAFT → RUNNABLE. Server-side `validateSlotBindings` runs; on failure, status stays DRAFT, the response carries the validation messages, the assistant emits a follow-up `ASSISTANT_PROMPT` ("Some bindings need attention: [...]. Fix them and try again.") with `Open binding X` chips.
- If, at the moment of Save, the configuration's `schedule.cron_expression` is non-empty, the promotion target is `SCHEDULED` instead of `RUNNABLE` (per §2.7). The Save chip's behavior is therefore: validate → promote to RUNNABLE → if cron is set, immediately promote to SCHEDULED. Atomic from the user's perspective.
- Subsequent edits via the pencil on a RUNNABLE or SCHEDULED configuration:
  - If new state still passes validation → status preserved.
  - If new state fails validation → status flips to DRAFT, `STEP_REBOUND` is still written, and the assistant emits a fix-prompt as above. Returning to a valid state via a follow-up edit does **not** auto-promote back — Ana must re-confirm via a (now visible again) `CONFIRM` prompt.

### 2.12 AbortSignal continuity

M5 components subscribing to `watchPlanThreadMessages`, `ListPlanConfigurations` (for sidebar), and any new watches reuse the M3-established `AbortController + $effect` pattern. Nothing new on the wire; just consistent usage.

---

## 3. Architecture sketch

### 3.1 Backend

```
proto/harpia/chat/v1/chat.proto                            [EXTEND]
  enum ThreadMessageKind {
    ... existing M3/M4 kinds ...
    THREAD_MESSAGE_KIND_CONFIGURATION_STARTED = 13;
    THREAD_MESSAGE_KIND_ASSISTANT_PROMPT      = 14;
    THREAD_MESSAGE_KIND_USER_SELECTION        = 15;
    THREAD_MESSAGE_KIND_STEP_REBOUND          = 16;
    THREAD_MESSAGE_KIND_SCHEDULE_SET          = 17;
  }
  // Exact numeric values determined at proto-edit time based on the
  // next free number after M4's STEP_STARTED.

proto/harpia/plans/v1/plans.proto                          [UNCHANGED]
  // PlanConfiguration.schedule (PlanSchedule { cron_expression, timezone })
  // is already defined. UpdatePlanConfiguration already accepts it.
  // No new RPC or proto field needed for schedule.

(no new migration)
  // plan_configurations.schedule is already a JSONB column from migration
  // 000004. No schema change needed for M5.

control-plane/internal/planassistant/                      [NEW PACKAGE]
  state.go         AssistantState type + DeriveState(config, messages)
                   pure function returning current state.
  controller.go    SeedThread(ctx, configId) — emits CONFIGURATION_STARTED
                                                + first ASSISTANT_PROMPT.
                   NextTurn(ctx, configId)   — applies last USER_SELECTION,
                                                emits next ASSISTANT_PROMPT
                                                or final ASSISTANT_TEXT.
                                                Idempotent. Single tx.
  prompts.go       BuildAssistantPrompt(state, configCtx) — builds the
                   text + payload_json for each state kind. Loads candidate
                   executors via the existing slot-binding helpers for
                   BINDING_STEP options.
  state_test.go, controller_test.go, prompts_test.go

control-plane/internal/plans/                              [EXTEND]
  configuration_handler.go
    CreatePlanConfiguration  — after insert, calls planassistant.SeedThread.
    UpdatePlanConfiguration  — when the incoming schedule field differs
                                from the prior persisted schedule:
                                  - validate cron via robfig/cron/v3 (add to
                                    go.mod if not present),
                                  - append a SCHEDULE_SET chat message,
                                  - reconcile status (RUNNABLE ↔ SCHEDULED)
                                    per §2.7.
                                Existing per-field update semantics are
                                otherwise preserved.

  thread_handler.go
    AppendPlanThreadMessage  — if kind == USER_SELECTION, after the insert
                                calls planassistant.NextTurn(configId).
                                USER_TEXT path unchanged.
```

### 3.2 Frontend

```
frontend/src/lib/chat/types.ts                             [EXTEND]
  Union + KIND_FROM_PROTO + KIND_TO_PROTO gain:
    ASSISTANT_PROMPT, USER_SELECTION, STEP_REBOUND,
    SCHEDULE_SET, CONFIGURATION_STARTED.

frontend/src/lib/plans/cost.ts                             [NEW]
  computeRunCost(template, configuration, executorCatalog) → RunCost
  + unit tests.

frontend/src/lib/plans/assistant.ts                        [NEW]
  selectChip(tenantId, configurationId, promptMessageId, optionId)
    — wraps AppendPlanThreadMessage with USER_SELECTION payload.
  editBinding(tenantId, configurationId, stepKey, newInstallationId)
    — writes STEP_REBOUND chat message + calls UpdatePlanConfiguration.
  + unit tests.

frontend/src/lib/components/thread/                        [NEW FILES]
  AssistantPromptCard.svelte     chip-group renderer with edit pencil.
  TemplatePickerCard.svelte      AWAITING_TEMPLATE variant (used in /new
                                  and as the inline first card if seeded
                                  without template_id; for v1 the latter
                                  is unused — /new always commits a
                                  template before redirect).
  ConfirmCard.svelte             CONFIRM-state summary + Save + Edit chips.

frontend/src/lib/components/                               [NEW FILES]
  PlanCostPill.svelte
  PlanThreadTopBar.svelte        mounts in the thread page (name + status
                                  badge + cost pill + Schedule chip).
  sidebar/YourPlansList.svelte

frontend/src/lib/components/canvas/                        [EXTEND]
  ScheduleDialog.svelte          [NEW] modal dialog for cron config.
  CanvasTopBar.svelte            [MODIFY] wire Schedule button to the new
                                  dialog; mount PlanCostPill.

frontend/src/routes/new/                                   [REPLACE STUB]
  +page.svelte                   greeting + gallery + composer.
  +page.ts                       loads ListPlanTemplates.
  ?template=<id>                 query handler: auto-commit on mount.

frontend/src/routes/plans/configurations/[configurationId]/  [EXTEND]
  +page.svelte                   add PlanThreadTopBar; render
                                  AssistantPromptCard / ConfirmCard for
                                  the new kinds in the section/message
                                  dispatcher.
  +page.ts                       load executorCatalog (existing RPC) for
                                  cost computation.

frontend/src/routes/plans/[templateId]/configure/           [DELETE]
  +page.svelte, +layout.server.ts, overseer/, policies/, summary/
  Replaced by a tiny +page.ts redirect → /new?template=<templateId>.
  All four route trees emit the same redirect.

frontend/src/routes/plans/[templateId]/+page.svelte         [MODIFY]
  "Use this template" CTA href → /new?template=<templateId>
  (was /plans/[templateId]/configure).

frontend/src/lib/i18n/{en,pt-BR}.json                       [EXTEND]
  Lockstep additions:
    assistant.*  prompt copy templates per state kind.
    confirm.*    confirm-card labels.
    cost.*       pill + breakdown labels.
    schedule.*   dialog labels, cadence chips, timezone label.
    sidebar.yourPlans, sidebar.yourPlans.empty, sidebar.seeAll.
    new.*        greeting, composer placeholder, no-match hint.
```

---

## 4. v1 → v2 hinges (what M5 must not foreclose)

- **LLM assistant swap-in.** The chat protocol is the boundary: chip-driven `USER_SELECTION` is one shape of input; future free-text `USER_TEXT` could trigger an LLM call inside `NextTurn`. The protocol's `payload_json` extensibility plus the `state` field already in `ASSISTANT_PROMPT` make this additive — no breaking change to existing rows.
- **Branching DAGs and drag-to-compose.** `planassistant` walks `template.steps` in order; when v2 introduces non-linear templates or freeform composition, the walk function becomes topological-order or composition-event-driven. The protocol and renderer stay; only `state.go`'s iterator changes.
- **Agent-as-Overseer.** `SET_OVERSEER` state's options list is built by `prompts.go`. For v1 the only option besides "You" is a stub; for v2 it queries the agent catalog and includes agent options. No protocol change.
- **Schedule → execution wiring.** The persisted `schedule_cron` column is the seam. A follow-up backend milestone reads it and registers a Temporal Schedule. No UX changes required.
- **Per-tenant currency.** `computeRunCost` returns `currency: "BRL"` hard-coded. To go multi-currency: extend `ExecutorCatalog` with per-SKU currency + a tenant default; the pill renderer formats accordingly. No protocol change.
- **Persona-aware sidebar (M6).** `YourPlansList` mounts unconditionally for v1; M6 wraps the mount in a persona check. The component itself doesn't need to know.

---

## 5. Implementation milestones (sketch — refined in the plan)

Approximate task decomposition. Order roughly mirrors dependency.

### Backend
1. Proto: extend `ThreadMessageKind` with 5 new variants (CONFIGURATION_STARTED, ASSISTANT_PROMPT, USER_SELECTION, STEP_REBOUND, SCHEDULE_SET). Regen Go + TS. No `PlanConfiguration` field changes or new RPCs.
2. `internal/planassistant/` package: `state.go` (DeriveState) + tests.
3. `internal/planassistant/`: `prompts.go` (per-state prompt builders) + tests.
4. `internal/planassistant/`: `controller.go` (SeedThread, NextTurn) + tests.
5. `configuration_handler.go`: wire `SeedThread` into `CreatePlanConfiguration`. Extend `UpdatePlanConfiguration` to detect schedule changes → validate cron, emit `SCHEDULE_SET`, reconcile RUNNABLE ↔ SCHEDULED.
6. `thread_handler.go`: branch on USER_SELECTION → `NextTurn`.

### Frontend lib
7. `lib/chat/types.ts`: extend union + maps with the five new kinds.
8. `lib/plans/cost.ts`: `computeRunCost` + tests.
9. `lib/plans/assistant.ts`: `selectChip` + `editBinding` + tests.

### Frontend components
10. `lib/components/thread/AssistantPromptCard.svelte` (chip group + edit pencil).
11. `lib/components/thread/ConfirmCard.svelte`.
12. `lib/components/thread/TemplatePickerCard.svelte` (used by `/new`).
13. `lib/components/PlanCostPill.svelte`.
14. `lib/components/PlanThreadTopBar.svelte` (mounts pill + schedule chip).
15. `lib/components/canvas/ScheduleDialog.svelte`.
16. `lib/components/canvas/CanvasTopBar.svelte`: wire Schedule + mount cost pill.
17. `lib/components/sidebar/YourPlansList.svelte`.

### Page + integration
18. `/new` page: replace M1 stub with greeting + gallery + composer + `?template=` auto-commit.
19. `/plans/configurations/[configurationId]/+page.svelte`: extend dispatcher for new kinds; mount top bar; load executor catalog in `+page.ts`.
20. Sidebar: mount `YourPlansList` under `+ New plan`.
21. Delete `/plans/[templateId]/configure/*` (four routes + layout); replace each with a redirect `+page.ts`.
22. `/plans/[templateId]/+page.svelte`: rewire "Use this template" CTA href to `/new?template=<id>`.
23. i18n keys (lockstep en + pt-BR) for `assistant.*`, `confirm.*`, `cost.*`, `schedule.*`, `sidebar.yourPlans`, `new.*`.

### Verification + review
24. End-to-end verification log: empty state → /new → pick template → walk every state → save → edit a binding → schedule → run from canvas. Includes the redirect path from each deleted wizard URL.
25. Final whole-branch review.

~25 tasks. Larger than M3 (19) and M4 (21) — the assistant state machine plus the schedule/cost/sidebar additions all live together. The backend addition is smaller than M4's because the schedule plumbing already exists in the proto + DB.

---

## 6. Out of scope

- LLM-driven dialog of any kind (see §2.1).
- Cron → Temporal execution-trigger wiring. `schedule_cron` is persisted; runtime activation is a follow-up backend milestone.
- Agent-as-Overseer picker entries.
- Multi-currency cost display.
- Per-Plan or per-Configuration timezone editing in the dialog (read-only from tenant setting in v1).
- A dedicated `My plans` route — sidebar "See all" → `/discover` for v1.
- M6 work: persona/admin split, persona toggle in user menu, dev-login persona collapse, removal of transitional redirects, final IA polish.
- Mobile-specific layouts beyond ensuring chip groups + cost pill reduce to single column gracefully.
- Onboarding/welcome polish for `/new` (the greeting is functional, not curated).
- Telemetry events for assistant interactions.

---

## 7. Open questions

None blocking. Items deferred by explicit decision are listed in §6.

---

## 8. Closes / references

- Master design spec §4.1, §4.5, §4.6, §5.2, §5.5, §6, §9, §10 — UX shape, route disposition, v1/v2 split, M5 line item.
- M3 design spec §2.4, §2.7 — composer/passive model; AbortSignal pattern reused.
- M4 design spec §2.4 — Schedule button stub explicitly deferred here.
- M3 §2.11 — sidebar "Your plans" deferral closed in this milestone.
- GitHub issue #121 (E3.6 Chat-assisted configuration flow) — closed by this spec.
