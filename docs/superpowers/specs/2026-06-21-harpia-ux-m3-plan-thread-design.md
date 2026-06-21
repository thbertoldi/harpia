# Harpia UX Realignment — M3 Plan Thread — Implementation Design Spec

**Date:** 2026-06-21
**Status:** Approved (brainstorm), awaiting plan
**Originating conversation:** Brainstorming session 2026-06-21 between Thiago and Claude.
**Extends:** `docs/superpowers/specs/2026-06-19-harpia-ux-realignment-design.md` (master design)

This spec resolves M3's implementation-level decisions. The user-facing UX is already settled in the master spec — §4.2 (one thread per Plan), §5.2 (plan thread shape), §6 (route disposition), §9 (v1/v2 split). This document captures the backend service shape, persistence model, frontend rendering decisions, and the route/component decisions needed to write the M3 implementation plan.

---

## 1. Scope

M3 ships the **plan thread** — the chat surface that becomes the home for each configured Plan. Per master spec §4.2, this is a durable per-PlanConfiguration chat thread containing:

- Configuration history (the act of saving the configuration; M5 will add the assistant's configuration conversation here)
- System events for every execution (run-started, step-bound, run-completed)
- Pointer messages for elicitations and approvals raised on this plan (with answered/decided follow-up messages)
- User-typed composer messages (passive in M3 — persisted but generate no assistant response)
- An embedded DAG mini-map of the plan's task graph

The thread surfaces all "Open thread" links from the M2 inbox into a working destination.

**M3 is universal** — like M2, not flag-gated. Once shipped, the new thread is the canonical view of a plan; the legacy `/plans/[templateId]` template detail page survives unchanged.

---

## 2. Resolved decisions

### 2.1 Persistence model — Option A (everything is a chat_message)

A single `chat_messages` table is the source of truth for thread history. Every event is a row:

- `CONFIGURATION_SAVED` — written when a PlanConfiguration is created/updated (plan-scope, `execution_id` NULL)
- `RUN_STARTED` / `RUN_COMPLETED` / `RUN_FAILED` — written by Temporal workflow activities (execution-scope)
- `STEP_BOUND` — written when a step's executor produces output (execution-scope)
- `ELICITATION_RAISED` / `ELICITATION_ANSWERED` — pointer rows referencing `plan_elicitations.id` (execution-scope)
- `APPROVAL_RAISED` / `APPROVAL_DECIDED` — pointer rows referencing `plan_approval_requests.id` (execution-scope)
- `USER_TEXT` — composer-typed messages from the Overseer (plan-scope)
- `ASSISTANT_TEXT` — reserved for M5 (not emitted in M3)

Live state (currently-pending elicitations/approvals) continues to come from the M2 inbox aggregator, scoped to this plan. The thread renders by merging:
- A complete history stream of `chat_messages` for this configuration_id
- A live filter of `watchInbox(tenantId)` output restricted to items belonging to this configuration_id

No projection logic on top of `plan_executions` / `step_executions` / `plan_elicitations` / `plan_approval_requests` — those tables are the operational state; chat_messages is the narrative log.

**Trade-off accepted:** every step transition adds one chat_message INSERT. For the v1 GTM target (Ana, single-seat, weekly cadence), volume is negligible. Document this as a scale consideration if tenant cardinality changes.

### 2.2 Service shape — Option B (RPCs on PlanService, but extraction-friendly)

The chat RPCs live on the existing `PlanService` for v1, but the surrounding architecture is structured so that a future `ChatService` extraction is mechanical:

- **Proto:** chat-message types (`ThreadMessage`, `ThreadMessageKind`, `ThreadMessageRole`) live in a NEW package `proto/harpia/chat/v1/chat.proto`. They are imported by `harpia/plans/v1/plans.proto`. New RPCs (`WatchPlanThreadMessages`, `AppendPlanThreadMessage`, `ListPlanThreadMessages`) are added to `PlanService`. The existing `ThreadMessage` type inside `plans.proto` (currently embedded in `ElicitationRequest.thread`) is migrated to import from the new chat package — same fields, new home.
- **DB:** `chat_messages` table keyed by a generic `thread_id TEXT NOT NULL` column (= `plan_configuration_id` for v1). Forward-compat: when extracted, `thread_id` becomes a foreign key to a `chat_threads` table. No separate `threads` table in M3.
- **Go:** new `control-plane/internal/chat/` package with a generic store interface (`AppendMessage`, `ListMessages`, `WatchMessages`). `control-plane/internal/plans/` calls into it — no chat business logic in the plan handler. Extraction = move RPCs from `PlanHandler` to a new `ChatHandler` calling the same store.

### 2.3 Routing — Option C (`/plans/configurations/[configurationId]`)

The thread lives at `/plans/configurations/[configurationId]`. Spec §6 calls for `/plans/[id]` ultimately, but routing collides with the existing `/plans/[templateId]` template detail page. The new `/plans/configurations/...` namespace mirrors the existing `/plans/executions/[id]` precedent — clean, no migrations, no broken bookmarks. The eventual rename to `/plans/[id]` is deferred to whichever milestone moves `/plans → /discover`.

Inbox "Open thread" links rewire to this path with a hash anchor (§2.7 below).

### 2.4 Composer — Option A (active-but-passive)

The composer accepts user text and persists each submission as a `USER_TEXT` chat_message. No backend assistant response in M3 — that's M5.

After the user submits their first message in a thread, a one-time hint banner renders: *"The assistant doesn't reply yet — coming in a later milestone."* The hint is per-thread and dismissable (state held client-side in localStorage; key `harpia.thread.<configId>.composerHintDismissed`).

Plumbing exercised: composer → `AppendPlanThreadMessage` RPC → `chat.AppendMessage` store call → DB row → live stream pushes new row → thread re-renders.

### 2.5 Thread initial state — Option i (CONFIGURATION_SAVED first)

When a PlanConfiguration is saved (via the existing wizard, which survives until M5), the handler that creates the configuration row also writes a `CONFIGURATION_SAVED` chat_message. The thread page renders this as the first message. The DAG mini-map (§2.8) is embedded immediately after.

Subsequent system events (executions, elicitations) accumulate in chronological order below.

### 2.6 Stream architecture — match existing polling pattern

`WatchPlanThreadMessages` follows the `WatchElicitations` Go server pattern: a 2-second `time.NewTicker` polling loop comparing the latest known message_id (sequence number) to the database, pushing batches of new messages on the stream. No LISTEN/NOTIFY, no pubsub. Consistent with codebase, sufficient for v1 throughput.

The request includes a `since_sequence_number` so a reconnecting client can resume from the last received message rather than re-streaming the full history.

### 2.7 AbortSignal — backfill the M2 debt as part of M3

The new `watchPlanThreadMessages` wrapper accepts `signal?: AbortSignal`. As part of the same task, the THREE existing watch* wrappers are backfilled with identical signal support:
- `watchElicitations` (`lib/plans/elicitations.ts`)
- `watchApprovalRequests` (`lib/plans/approvals.ts`)
- `watchInbox` (`lib/inbox/aggregator.ts`) — cascades `signal` to the two underlying watchers it merges

The final reviewer of M2 flagged this as a leak; M3 is the right time to clear it because M3 stacks a fourth long-lived stream on the inbox/thread pages.

Frontend consumers update to pass `signal` from an `AbortController` created in their `$effect`, calling `controller.abort()` in the effect's cleanup function. The legacy `let active = true; ... return () => active = false` pattern is replaced by the AbortController pattern; both still work, but new code uses signals.

### 2.8 Per-execution rendering

Each chat_message row carries a nullable `execution_id` column. The thread page groups consecutive same-`execution_id` messages into a collapsible section titled "Run <N> · <status> · <relative-time>" (e.g., "Run 4 · completed · 2 hours ago"). Plan-scope messages (`execution_id` NULL) render inline between execution groups.

Collapsible default state:
- Most-recent execution: expanded
- Older executions: collapsed
- Plan-scope messages: always rendered

Execution group counter (`Run N`) is derived from the chronological order of distinct `execution_id` values in the thread, not from a database column. Frontend computes it.

### 2.9 DAG mini-map — new component, shared helpers

A new component `frontend/src/lib/components/PlanDagMiniMap.svelte` (~50 lines) reuses the existing `frontend/src/lib/plans/artifact-flow.ts` helpers (`buildLinearDagEdges`, `orderPlanStepsLinear`). Renders a single-row strip of step name boxes connected by arrows. Compact — no per-step input/output artifact chips, no per-step description. Sized for embedding inside a chat-message card (~48px tall).

The existing `PlanDagDiagram.svelte` (used in the legacy template detail page) remains unchanged. Two components, shared data layer.

### 2.10 Inbox deep-link anchor

`InboxElicitationActions.svelte` and `InboxApprovalActions` (inside `InboxApprovalEntry.svelte`'s navigation, if any) compute their "Open thread" hrefs as `/plans/configurations/<configId>#m-<messageId>` where:
- `configId` is the PlanConfiguration the elicitation/approval's plan_execution belongs to
- `messageId` is the `chat_messages.id` of the `ELICITATION_RAISED` / `APPROVAL_RAISED` pointer row

The thread page reads `window.location.hash` on mount, scrolls to the matching message element, and applies a CSS class (`.harpia-pulse-anchor`) that briefly pulses a `ring-talon-gold` for 1.5s before settling.

The inbox aggregator (`frontend/src/lib/inbox/aggregator.ts`) currently does not expose `configurationId` on `InboxItem` — M3 adds it for elicitation and approval items. The aggregator joins via existing `plan_execution.plan_configuration_id` (already in the proto).

**Feedback deep-link is intentionally NOT changed in M3.** `InboxFeedbackActions.svelte` keeps its M2 behavior (links to `/oversee`, which still serves the legacy `FeedbackPanel`). Reason: `FeedbackRequest` has no direct configuration_id — only `task_id` and `agent_instance_id`. Deriving the configuration_id requires a multi-hop join (task → step_execution → plan_execution → plan_configuration) that doesn't exist in the inbox aggregator today. Building it is non-trivial work that's not strictly needed for M3's surface, AND M5's chat-driven configuration may change how feedback maps to threads. Feedback "Open thread" stays as-is; M5 (or its own follow-up) handles the repointing.

### 2.11 Sidebar "your plans" list — DEFERRED

Spec §5.2 mentions a per-plan thread list in the sidebar. M3 does NOT ship this. Reasons:
- Ana reaches threads via inbox "Open thread" or direct URL — sufficient for v1 dogfooding
- Building the sidebar list adds design surface (how many shown, sort order, active vs. recent) that wasn't covered in this brainstorm
- The thread page works independently without it

Tracked as a follow-up for a later milestone or its own small PR.

---

## 3. Architecture sketch

### 3.1 Backend

```
proto/harpia/chat/v1/chat.proto          [NEW]
  message ThreadMessage { id, thread_id, role, kind, text, payload_json, execution_id, sequence_number, created_at, author_user_id }
  enum ThreadMessageRole { UNSPECIFIED, OVERSEER, AGENT, SYSTEM }
    // Note: AGENT role is reserved for M5 (assistant responses).
    // In M3, only OVERSEER (composer-typed) and SYSTEM (events) are written.
    // SYSTEM messages have author_user_id NULL.
  enum ThreadMessageKind { UNSPECIFIED, USER_TEXT, ASSISTANT_TEXT, CONFIGURATION_SAVED, RUN_STARTED, RUN_COMPLETED, RUN_FAILED, STEP_BOUND, ELICITATION_RAISED, ELICITATION_ANSWERED, APPROVAL_RAISED, APPROVAL_DECIDED }
    // Note: ASSISTANT_TEXT is reserved for M5 — never written in M3.

proto/harpia/plans/v1/plans.proto         [EXTEND]
  // The existing ThreadMessage type in plans.proto (embedded inside
  // ElicitationRequest.thread for agent↔overseer dialog within a single
  // elicitation) is LEFT UNCHANGED. The new chat.v1.ThreadMessage is a
  // separate type for plan-thread history. Coexistence keeps M3's wire
  // surface independent of the elicitation thread; a later milestone may
  // unify them when the semantics demand it.
  // Add to PlanService:
  rpc WatchPlanThreadMessages(WatchPlanThreadMessagesRequest)
      returns (stream WatchPlanThreadMessagesResponse);
  rpc AppendPlanThreadMessage(AppendPlanThreadMessageRequest)
      returns (AppendPlanThreadMessageResponse);
  rpc ListPlanThreadMessages(ListPlanThreadMessagesRequest)
      returns (stream ListPlanThreadMessagesResponse);

database/migrations/000012_chat_messages.sql              [NEW]
  CREATE TABLE chat_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    thread_id TEXT NOT NULL,                  -- = plan_configuration_id for v1
    execution_id UUID NULL,                   -- nullable for plan-scope messages
    role  TEXT NOT NULL,                       -- OVERSEER | AGENT | SYSTEM
    kind  TEXT NOT NULL,
    text  TEXT NOT NULL DEFAULT '',
    payload_json JSONB NOT NULL DEFAULT '{}', -- per-kind structured payload
    author_user_id UUID NULL,
    sequence_number BIGINT NOT NULL,           -- per-thread monotonic
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
  );
  CREATE INDEX idx_chat_messages_thread_seq ON chat_messages (thread_id, sequence_number);
  CREATE INDEX idx_chat_messages_thread_created ON chat_messages (thread_id, created_at);
  -- RLS policy enforces tenant_id = current_tenant_id()

control-plane/internal/chat/                               [NEW PACKAGE]
  store.go   — generic Store interface + pgxpool impl
  messages.go — MessageBuilder helpers for each Kind (typed payloads)

control-plane/internal/plans/                              [EXTEND]
  thread_handler.go [NEW] — implements the three PlanService chat RPCs
                            by delegating to chat.Store
  configuration_handler.go — CreatePlanConfiguration / UpdatePlanConfiguration
                              also write CONFIGURATION_SAVED message
  // Workflow activities in internal/workflow/plans.go:
  //   StartPlanExecutionActivity        → appends RUN_STARTED
  //   CompleteStepExecutionActivity     → appends STEP_BOUND
  //   CompletePlanExecutionActivity     → appends RUN_COMPLETED or RUN_FAILED
  //                                       (branch on execution.status at completion)
  //   AwaitElicitationStepExecutionActivity → appends ELICITATION_RAISED
  //   The elicitation-response handler in elicitations_handler.go
  //     (RespondToElicitation) → appends ELICITATION_ANSWERED
  //   The approval-raise path is the same code that inserts a
  //     plan_approval_requests row — locate via `grep -rn
  //     "INSERT INTO plan_approval_requests" control-plane/`. Append
  //     APPROVAL_RAISED in the same transaction as that INSERT.
  //   The approval-response handler in approvals_handler.go
  //     (RespondToApprovalRequest) → appends APPROVAL_DECIDED
```

### 3.2 Frontend

```
frontend/src/lib/chat/                                     [NEW]
  types.ts        — TS shape mirroring proto (ThreadMessage, kinds, roles)
  client.ts       — appendMessage(threadId, ...) wrapping the RPC
  watch.ts        — async generator wrapping WatchPlanThreadMessages stream

frontend/src/lib/plans/                                    [EXTEND]
  elicitations.ts — add `signal?: AbortSignal` to WatchElicitationsOptions
  approvals.ts    — add `signal?: AbortSignal` to WatchApprovalRequestsOptions

frontend/src/lib/inbox/aggregator.ts                       [EXTEND]
  // Expose configurationId on InboxItem (look up via plan_execution).
  // Add AbortSignal support to watchInbox (cascade from M3's pattern).

frontend/src/lib/plans/                                    [EXTEND]
  thread.ts       — buildExecutionGroups(messages): groups consecutive
                    same-execution_id messages, computes "Run N" labels

frontend/src/lib/components/                               [NEW]
  PlanDagMiniMap.svelte — compact horizontal mini-map (~50 LOC)

frontend/src/lib/components/thread/                        [NEW DIRECTORY]
  ThreadMessage.svelte           — chat bubble shell (avatar, role, text/payload)
  SystemEventCard.svelte         — kind-discriminated render for system events
  ElicitationRefCard.svelte      — pointer-message render, links into the inline elicit form
  ApprovalRefCard.svelte         — same shape, for approvals
  ExecutionSection.svelte        — collapsible wrapper around an execution_id group
  ThreadComposer.svelte          — textarea + send button, persists USER_TEXT
  HintBanner.svelte              — dismissable "assistant doesn't reply yet" banner

frontend/src/routes/plans/configurations/[configurationId]/  [NEW ROUTE]
  +page.svelte    — the plan thread page
  +page.ts        — load: fetches PlanConfiguration + PlanTemplate (for DAG)
                    via existing RPCs

frontend/src/lib/components/inbox/                         [MODIFY]
  InboxElicitationActions.svelte — rewire href to /plans/configurations/<id>#m-<msgId>
  InboxFeedbackActions.svelte    — rewire (still imperfect — feedback doesn't
                                    have a config_id directly; defer or graceful-fallback)

frontend/src/lib/i18n/{en,pt-BR}.json                      [EXTEND]
  thread.* keys for: composer placeholder, hint banner, system event labels,
  empty state, "Run N" template, role labels, etc. Lockstep parity enforced.
```

---

## 4. v1→v2 hinges (what M3 must not foreclose)

- **Assistant brain (M5).** `ASSISTANT_TEXT` kind reserved. `AppendPlanThreadMessage` RPC accepts any role/kind a future call site might use. The chat.Store generic API has no assumption about who writes messages.
- **Generic ChatService extraction (post-M3).** Documented in §2.2 — proto types in `harpia/chat/v1`, generic `thread_id` column, generic Go package.
- **Multi-overseer / agent-as-overseer (post-M3).** `ThreadMessage.author_user_id` is nullable and a UUID — supports any actor.
- **Recently-completed items in "Earlier today" (a deferred M2 piece).** The `selectEarlierTodayItems` helper from M2 buckets/inbox can be wired here as a tab/filter when needed; messages have `created_at` so the data is present.
- **Per-execution canvas (M4).** Each `ExecutionSection` in the thread has a known `execution_id` — M4's canvas can deep-link from a section header to `/plans/configurations/<id>/canvas?run=<execution_id>`.

---

## 5. Out of scope

- Active configuration assistant — M5.
- Expanded canvas — M4.
- Persona/admin split — M6.
- Sidebar per-plan listing (see §2.11).
- Real-time chat features (typing indicators, read receipts, message edit/delete).
- ASSISTANT_TEXT message emission (reserved but never written in M3).
- New translations beyond what M3 itself adds.
- Multi-tenant chat semantics changes (current RLS policy applies as-is).
- Backend changes to elicitation/approval data models (the existing tables stay untouched; M3 only adds pointer messages referencing them).

---

## 6. Implementation milestones (sketch — refined in the plan)

Approximate decomposition for the plan to fill in. Each item is one task or a small cluster.

### Backend
1. `harpia/chat/v1` proto package + ThreadMessage / Kind / Role enums + generate Go + TS
2. `proto/harpia/plans/v1/plans.proto`: migrate `ElicitationRequest.thread` to use new package; add the three new RPCs to `PlanService`
3. `database/migrations/000012_chat_messages.sql`
4. `internal/chat/` package: Store interface + pgxpool impl + tests
5. `internal/plans/thread_handler.go`: implements the three RPCs
6. `internal/plans/configuration_handler.go`: write CONFIGURATION_SAVED on save
7. Workflow activity hooks: RUN_STARTED, RUN_COMPLETED, RUN_FAILED, STEP_BOUND, ELICITATION_RAISED/ANSWERED, APPROVAL_RAISED/DECIDED

### Frontend lib
8. `lib/chat/`: types + client.ts + watch.ts
9. AbortSignal backfill across `lib/plans/elicitations.ts`, `lib/plans/approvals.ts`, AND `lib/inbox/aggregator.ts` (cascade)
10. `lib/inbox/aggregator.ts`: expose `configurationId` on InboxItem (elicitations + approvals; not feedback — see §2.10)
11. `lib/plans/thread.ts`: execution grouping helper + tests

### Frontend components
12. `lib/components/PlanDagMiniMap.svelte`
13. `lib/components/thread/` directory: shell + kind-discriminated cards + composer + hint banner

### Page + integration
14. `routes/plans/configurations/[configurationId]/+page.svelte` + `+page.ts`
15. Inbox deep-link wiring: rewire `InboxElicitationActions` and `InboxFeedbackActions`, update aggregator output
16. i18n keys (lockstep en + pt-BR)

### Verification + review
17. End-to-end smoke + verification log
18. Final whole-branch review

---

## 7. Open questions

None. All Phase 1 brainstorm decisions resolved. Items deferred by explicit decision (sidebar list, configuration assistant, etc.) are documented in §5.

---

## 8. Closes / references

- Master design spec §4.2, §5.2, §6, §9 — UX shape and route disposition
- M2 final-review Important #5 (AbortSignal leak) — addressed in §2.7
- Spec §10 M3 — milestone scope confirmed
- GitHub issues: none directly closed (the open `#121` E3.6 Chat-assisted configuration flow is M5, not M3 — M3 ships the surface, M5 ships the assistant)
