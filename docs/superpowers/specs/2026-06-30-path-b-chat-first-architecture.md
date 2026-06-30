# Path B: Chat-First Architecture

**Date:** 2026-06-30
**Status:** Approved architecture direction; awaiting implementation plan
**Decision:** Hard migration to first-class Threads
**Related spec:** `docs/superpowers/specs/2026-06-29-conversational-plan-workspace-design.md`

## 1. Decision

Harpia will make `Thread` the durable top-level product object.

Today the chat experience is hosted inside a `PlanConfiguration` at
`/plans/configurations/[id]`. That keeps the UI, API, and database organized
around the technical plan object even when the intended experience is "talk to
Aiuna and watch the work happen".

Path B flips that ownership:

```text
Thread
  -> messages
  -> proposed or attached PlanConfiguration
  -> executions
  -> artifacts
  -> approvals and elicitations
```

The user starts with a conversation. The system can then propose, create,
update, run, and inspect a plan inside that conversation.

This is intentionally a hard architecture migration, not a frontend-only alias.
The existing plan routes and plan-thread RPCs may remain briefly as compatibility
surfaces, but new product work should target `ThreadService` and
`/chat/[threadId]`.

## 2. Goals

1. Let a user start from natural language without choosing a template first.
2. Make `/chat/[threadId]` the primary workspace for planning, execution,
   artifacts, errors, approvals, and history.
3. Move chat ownership from `PlanService` to a dedicated `ThreadService`.
4. Keep the execution engine plan-based internally while making the user-facing
   workspace thread-based.
5. Preserve existing plan configurations, executions, messages, and artifact
   provenance through a database migration.
6. Create a clean extension point for the Copilot/router agent that can propose
   or mutate plans from chat.

## 3. Non-Goals

- Replacing Temporal or the plan execution model.
- Removing plan templates, plan configurations, or step DAGs.
- Building a complete autonomous Manus clone in this migration.
- Supporting multiple active plan configurations per thread in v1.
- Rewriting artifact storage or text editing beyond the existing artifact
  versioning direction.
- Making natural language the only configuration path. Inline forms remain
  important for precision.

## 4. Product Model

### 4.1 Thread

`Thread` is the object the user opens, resumes, searches, and shares with the
system.

Required fields:

```text
Thread
  id
  tenant_id
  title
  status
  created_at
  updated_at
  archived_at
  created_by_user_id
  active_plan_configuration_id
```

Initial statuses:

- `open`: usable conversation.
- `running`: attached plan has an active execution.
- `needs_attention`: waiting for approval, elicitation, configuration, or error
  recovery.
- `completed`: latest attached plan execution completed.
- `archived`: hidden from normal history.

`active_plan_configuration_id` is optional. A new thread can exist before a plan
has been proposed or created.

### 4.2 Thread Messages

`ThreadMessage.thread_id` must reference `threads.id`.

The existing `chat_messages` table already stores a generic `thread_id`, but it
currently uses `plan_configuration_id` values. Path B must stop relying on that
convention.

Additional message kinds:

```text
THREAD_MESSAGE_KIND_PLAN_PROPOSED
THREAD_MESSAGE_KIND_PLAN_ATTACHED
THREAD_MESSAGE_KIND_PLAN_UPDATED
THREAD_MESSAGE_KIND_PLAN_RUN_REQUESTED
THREAD_MESSAGE_KIND_ARTIFACT_CREATED
THREAD_MESSAGE_KIND_ARTIFACT_UPDATED
THREAD_MESSAGE_KIND_ERROR_RAISED
THREAD_MESSAGE_KIND_ERROR_RECOVERED
```

`payload_json` can remain the structured payload mechanism for v1, but each new
kind needs a documented payload shape in the proto comments or a companion
reference doc.

### 4.3 Plan Configuration

`PlanConfiguration` remains the execution configuration object.

Add a nullable relationship back to the owning thread:

```text
plan_configurations.thread_id UUID NULL REFERENCES threads(id)
```

Rules:

1. A thread may have zero or one active plan configuration in v1.
2. A plan configuration created from chat must set `thread_id`.
3. Existing plan configurations must be backfilled into one thread each.
4. Future support for alternatives or branches should use a separate
   `thread_plan_configurations` relation instead of overloading v1 fields.

## 5. Backend Architecture

### 5.1 Protobuf

Move thread lifecycle and message RPCs into `harpia.chat.v1`.

Add messages:

```text
Thread
CreateThreadRequest / Response
GetThreadRequest / Response
ListThreadsRequest / Response
ArchiveThreadRequest / Response
ListThreadMessagesRequest / Response
WatchThreadMessagesRequest / Response
AppendThreadMessageRequest / Response
```

Add service:

```text
service ThreadService {
  rpc CreateThread(CreateThreadRequest) returns (CreateThreadResponse);
  rpc GetThread(GetThreadRequest) returns (GetThreadResponse);
  rpc ListThreads(ListThreadsRequest) returns (stream ListThreadsResponse);
  rpc ArchiveThread(ArchiveThreadRequest) returns (ArchiveThreadResponse);

  rpc ListThreadMessages(ListThreadMessagesRequest) returns (ListThreadMessagesResponse);
  rpc WatchThreadMessages(WatchThreadMessagesRequest) returns (stream WatchThreadMessagesResponse);
  rpc AppendThreadMessage(AppendThreadMessageRequest) returns (AppendThreadMessageResponse);
}
```

Deprecate these `PlanService` RPCs:

- `ListPlanThreadMessages`
- `WatchPlanThreadMessages`
- `AppendPlanThreadMessage`

They can remain during the migration window, but they should call the new thread
store only after resolving `plan_configuration_id -> thread_id`.

### 5.2 Database

Add migration `000014_threads.sql` or the next available migration number.

Required schema:

```sql
CREATE TABLE threads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    title TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'open',
    created_by_user_id UUID,
    active_plan_configuration_id UUID,
    archived_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE plan_configurations
    ADD COLUMN thread_id UUID REFERENCES threads(id);
```

After backfill, add the message relationship:

```sql
ALTER TABLE chat_messages
    ALTER COLUMN thread_id TYPE UUID USING thread_id::uuid,
    ADD CONSTRAINT chat_messages_thread_id_fkey
        FOREIGN KEY (thread_id) REFERENCES threads(id);
```

If direct type conversion is too risky for existing environments, use a staged
migration:

1. Add `new_thread_id UUID`.
2. Backfill from plan configuration mapping.
3. Dual-read or one-time rewrite.
4. Rename columns.
5. Add foreign key.

RLS must be enabled for `threads` with the same tenant isolation behavior as
plan configurations and chat messages.

### 5.3 Backfill

For each existing `plan_configurations` row:

1. Create one `threads` row with the same `tenant_id`.
2. Set `threads.active_plan_configuration_id` to the plan configuration id.
3. Set `plan_configurations.thread_id` to the new thread id.
4. Rewrite `chat_messages.thread_id` from the old plan configuration id to the
   new thread id.
5. Preserve per-thread `sequence_number` ordering.

Thread titles can be derived in order:

1. plan template name.
2. parameter value such as theme/topic when available.
3. `Untitled chat`.

### 5.4 Control Plane

Introduce `internal/threads` or equivalent ownership separate from
`internal/plans`.

Responsibilities:

- thread CRUD.
- message append/list/watch.
- title/status updates.
- plan attachment lookup.
- compatibility lookup from legacy plan configuration id.

`internal/chat.Store` should either become the threads message store or move
under the new package. It must no longer describe `thread_id` as
`plan_configuration_id`.

Plan execution code that currently appends messages using configuration ids must
append using the owning thread id. If an execution is started from a legacy
configuration without `thread_id`, the control plane should create or resolve a
thread before emitting messages.

### 5.5 Copilot Agent

Add a system-level Copilot/router agent that operates on threads.

Initial capabilities:

- receive new user messages from a thread.
- search or rank plan templates.
- propose a plan.
- create or attach a `PlanConfiguration`.
- update template input parameters.
- explain missing inputs.
- request execution when the plan is ready.

The agent does not need full autonomy in the first slice. Deterministic tools
and simple intent routing are acceptable as long as the contract is thread-first.

## 6. Frontend Architecture

### 6.1 Routes

Primary routes:

```text
/new
/chat/[threadId]
/chat/[threadId]/executions/[executionId]
/artifacts
/artifacts/[artifactId]
```

`/new` should become a chat composer. On submit:

1. call `ThreadService.CreateThread`.
2. persist the first user message.
3. route to `/chat/[threadId]`.
4. let the Copilot/router propose or attach a plan.

`/plans/configurations/[configurationId]` becomes a compatibility or debug
surface. It should either redirect to the owning thread or expose an explicit
technical details page for platform users.

### 6.2 Chat Workspace

`/chat/[threadId]` owns the full conversational workspace:

- conversation stream.
- plan proposal cards.
- template input cards.
- run button and run status.
- activity timeline.
- artifact rail/detail panel.
- approvals and elicitation prompts.
- friendly errors with recovery actions.
- debug/details drawer for DAG, bindings, raw ids, and execution tables.

The current `ConversationalWorkspace` component can remain the foundation, but
its props should move from `configurationId` to `threadId` plus an optional
attached plan configuration.

### 6.3 Inline Plan Proposal

When the Copilot proposes or attaches a plan, render a structured card inside
the chat:

```text
PlanProposedCard
  template summary
  inferred input parameters
  missing inputs
  schedule summary
  cost summary
  actions: edit inputs, create plan, run
```

This is the replacement for forcing the user to choose a template before
entering the workspace.

### 6.4 Thread History

Add a lightweight thread history surface.

Minimum viable behavior:

- sidebar/new-chat entry point.
- recent threads list.
- status indicators for running, needs attention, completed, archived.
- search can wait until after the migration.

## 7. Compatibility Rules

1. New UI work must call `ThreadService`.
2. Old plan-thread RPCs can remain temporarily but must be marked deprecated.
3. Legacy plan routes must resolve their owning thread before showing chat.
4. Execution, approval, elicitation, and artifact links should prefer thread
   routes when a thread exists.
5. Tests should cover both new thread-first paths and legacy redirects until the
   old routes are removed.

## 8. Implementation Milestones

### A. Schema and proto foundation

- Add `Thread` proto and `ThreadService`.
- Add database `threads` table.
- Add `plan_configurations.thread_id`.
- Backfill existing configurations and chat messages.
- Generate Go and TypeScript clients.

### B. Control plane service

- Implement thread CRUD.
- Implement list/watch/append thread messages.
- Wire legacy plan-thread RPCs through thread lookup.
- Update plan execution message emission to use thread ids.
- Add unit tests for tenant isolation, message sequencing, and compatibility.

### C. Frontend route migration

- Add `/chat/[threadId]` route.
- Refactor chat client from `planClient.*PlanThreadMessages` to
  `threadClient.*ThreadMessages`.
- Move `ConversationalWorkspace` to thread-first props.
- Make `/new` create a thread from the initial prompt.
- Redirect or bridge `/plans/configurations/[id]`.

### D. Plan proposal flow

- Add message kinds and UI for plan proposals and attached plans.
- Add deterministic first-pass router behavior for known templates.
- Let users review/update template inputs inside chat.
- Attach/create the `PlanConfiguration` from the thread.

### E. Execution and artifacts in thread context

- Start plan executions from `/chat/[threadId]`.
- Show execution history in the thread.
- Keep generated artifacts in the thread workspace and global artifact library.
- Make approval and elicitation links route back to the thread context.

## 9. Testing

Backend:

- migration backfills one thread per existing plan configuration.
- chat messages move from old config ids to new thread ids without losing order.
- `ThreadService` list/watch/append respects tenant boundaries.
- legacy plan-thread RPCs resolve configuration id to thread id.
- plan execution appends run and step messages to the thread id.
- creating a plan from a thread sets `plan_configurations.thread_id`.

Frontend:

- `/new` creates a thread and routes to `/chat/[threadId]`.
- `/chat/[threadId]` loads messages through `ThreadService`.
- plan proposal card renders missing and inferred inputs.
- run status, errors, approvals, and artifacts appear in the thread workspace.
- old `/plans/configurations/[id]` route redirects or bridges correctly.

End to end:

1. Start at `/new` with "Create a LinkedIn post about retail in Portuguese".
2. Land in `/chat/[threadId]`.
3. See a proposed plan with inferred theme and language.
4. Fill or adjust missing inputs.
5. Run the plan from the chat.
6. See progress, generated artifacts, approval prompts, and errors in the same
   thread.
7. Reopen the thread from history and inspect prior messages and artifacts.

## 10. Risks

- **Migration blast radius:** plan routes, execution messages, approvals,
  elicitations, and artifacts all reference plan configuration ids today.
  Mitigation: implement compatibility lookup and test old links during the
  migration.
- **Foreign key conversion:** `chat_messages.thread_id` is currently text.
  Mitigation: stage the conversion if direct cast/backfill is risky.
- **Agent ambiguity:** free-form chat can imply many possible plans. Mitigation:
  start with deterministic known-template routing and explicit confirmation.
- **UI scope creep:** Manus inspiration can turn into a broad workspace rewrite.
  Mitigation: first slice is thread ownership, plan proposal, run, artifacts,
  and history only.

## 11. Acceptance Criteria

The hard Path B migration is complete when:

1. `ThreadService` owns thread lifecycle and message streaming.
2. New chats start from `/new` and land on `/chat/[threadId]`.
3. New chat messages persist against `threads.id`, not plan configuration ids.
4. Existing plan configurations and chat messages are backfilled into threads.
5. A thread can propose or attach a plan configuration.
6. A plan can run from the thread workspace.
7. Run progress, failures, approvals, elicitations, and artifacts are visible in
   the thread workspace.
8. Existing plan configuration links do not break; they resolve to the owning
   thread or a clear technical details surface.
9. The artifact library remains usable and artifacts retain plan/thread
   provenance.
10. Tests cover migration, `ThreadService`, frontend routing, and the primary
    chat-first e2e journey.

## 12. Implementation Plan Gate

Do not start coding this migration from isolated file edits. The next step is a
separate implementation plan that sequences schema, API, compatibility, frontend
routes, and e2e verification so trunk remains usable during the migration.
