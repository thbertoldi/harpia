## Context

`/admin/audit` is currently backed entirely by `frontend/src/lib/mocks/audit-events.ts` and a localStorage store. Its model also preserves superseded task/subtask vocabulary, so it cannot represent the constitution's PlanConfiguration and PlanExecution operations consistently or prove what occurred in a tenant. There is no persisted audit table, RPC contract, or control-plane recording path.

This is a cross-cutting governance capability. It must use the request identity/RLS boundary described in the Platform Constitution rather than create a seventh business bounded context or let individual domains write infrastructure records directly. The control plane already composes public Connect interceptors in `cmd/api/main.go`; workflow and Temporal operations do not pass through that chain.

## Goals / Non-Goals

**Goals:**

- Persist an append-only, tenant-scoped audit event ledger and expose filtered, newest-first keyset pages to the authenticated tenant.
- Preserve the mock model's useful information (event ID/type, actor snapshot, bounded context, diff entries, timestamp, trace ID, and optional human decision) while replacing its legacy `taskId` with a typed, optional subject.
- Cover successful public mutations and asynchronous workflow/execution operations through one audit adapter without putting database or Connect dependencies in domain packages.
- Let the existing audit browser and its CSV/JSON exports use the real API without adding streaming, a new UI surface, or new export formats.

**Non-Goals:**

- Real-time audit-event streaming, polling policy, retention/archiving, external SIEM delivery, or a write RPC for browser clients.
- A universal before/after diff engine, audit replay/event sourcing, or audit coverage for every RPC and internal operation.
- Zitadel-managed identity/role/permission changes and PlanConfiguration deletion. Harpia has no mutation path for either today, and this change does not add RPCs or handlers solely to create audit events.
- Persisting credentials, bearer tokens, OAuth configuration, API keys, raw authorization headers, or arbitrary request bodies in event payloads.
- Changing the authorization model; RLS remains the tenant data boundary and existing role checks continue to authorize each operation.

## Decisions

### 1. Use a tenant-scoped append-only ledger with forced RLS and tuple keysets

Migration `000018_audit_events.sql` will create `audit_events` with UUID `id`, `tenant_id`, `event_type`, `bounded_context`, actor snapshot fields (`actor_kind`, `actor_id`, `actor_display_name`, nullable `actor_agent_type`), nullable typed subject fields (`subject_type`, `subject_id`), `payload_diff JSONB NOT NULL DEFAULT '[]'`, nullable `decision`, nullable `trace_id`, and `occurred_at TIMESTAMPTZ NOT NULL DEFAULT now()`. Diff entries retain the mock shape `{field, before, after}` where values are strings or null. The new subject accommodates PlanConfiguration, PlanExecution, StepExecution, ExecutorInstallation, ApprovalRequest, and authentication events without reintroducing task terminology; authentication events legitimately have no subject.

The migration will enable and force RLS, with the same `tenant_id = current_tenant_id()` `USING` and `WITH CHECK` policy used by other tenant-owned tables. The audit repository must set the tenant database context through the existing tenant-safe database boundary before every append or list operation; a caller-supplied tenant ID is never sufficient to bypass RLS.

Indexes will cover `(tenant_id, occurred_at DESC, id DESC)` for the canonical query, plus tenant-prefixed event-type and actor indexes. The repository will order by `(occurred_at DESC, id DESC)` and decode an opaque cursor containing that tuple; the next page uses the strict tuple predicate `(occurred_at, id) < ($cursor_occurred_at, $cursor_id)`. Date bounds and other filters are applied before this predicate. This avoids offset drift and duplicate/omitted records when newer events arrive while a user pages.

`event_type` remains a validated application/proto vocabulary rather than a PostgreSQL enum, so the ledger does not require a database type migration for every future operation. The initial vocabulary maps the mock's feedback, agent, and workflow semantics to current names (`plan_configuration.*`, `plan_execution.*`, `step_execution.*`, `approval.*`, `workflow.signal_received`) and adds `authentication.dev_auth` and `executor_installation.*` operation types. `bounded_context` uses the Constitution's `plan_management`, `human_interaction`, `identity_tenants`, and `workflow_engine` names, plus `executor_catalog` for ExecutorInstallation changes.

**Alternatives considered:** an offset cursor is simpler but is unstable under concurrent inserts; a timestamp-only cursor is ambiguous for events sharing a timestamp; a PostgreSQL enum makes ad-hoc data invalid but makes the cross-cutting event vocabulary unnecessarily expensive to extend. A generic JSON blob for the whole event would prevent efficient actor/event filters and make the contract opaque.

### 2. Add a read-only `harpia.audit.v1.AuditService`

`proto/harpia/audit/v1/audit.proto` will define `AuditService.ListAuditEvents`. The request includes `tenant_id`, repeated event types, an optional actor filter (kind, actor ID, and agent type), optional inclusive ISO-8601 date bounds, optional subject ID, optional decision, `page_size`, and an opaque `page_token`. The response contains repeated structured `AuditEvent` messages and `next_page_token`. Enums model actor kind, bounded context, subject type, feedback decision, and the initial event vocabulary; `PayloadDiffEntry` keeps nullable before/after values via proto optional fields.

The public handler uses `identity.RequireTenant` before listing, limits page size to a documented maximum, rejects malformed/inconsistent cursors with `InvalidArgument`, and returns no events outside the selected tenant. The generated handler is registered in the same public Connect interceptor chain as the existing services. There is intentionally no client-callable record/create RPC.

**Alternatives considered:** adding audit methods to PlanService would couple a governance read model to Plan Management and exclude identity/executor events. A server stream is deferred because the requested browse/load-more behavior does not need a persistent connection.

### 3. Use a hybrid interceptor plus explicit non-RPC emitter, backed by one audit adapter

Create `control-plane/internal/audit` as an application/infrastructure adapter with a small `Recorder` port, event draft model, PostgreSQL repository, background writer, and public handler. Domain packages depend only on a narrow recording interface or supply semantic metadata through application-layer wiring; they do not import Connect, pgx, or the background worker.

An `AuditInterceptor` is inserted after `identity.NewRequestContextInterceptor` in the public interceptor chain. It owns tenant/actor/trace capture, the successful-response boundary, method allowlist, and enqueueing. A registered mapping converts the targeted public RPC methods to audit event types. Where a mutation needs selected before/after values or a more precise subject than the request alone can provide, the application handler enriches a request-scoped audit draft; the interceptor emits exactly one event after the RPC succeeds. This avoids scattered database writes while preserving status, binding, overseer, and behavior-policy changes.

The mapping covers successful dev-token authentication resolution (the only Harpia-managed authentication path), PlanConfiguration create/update/status and configuration changes, PlanExecution creation, ExecutorInstallation create/update/delete, and approval decisions. There is no standalone login endpoint: a successful request authenticated by the enabled dev token records `authentication.dev_auth`. PlanConfiguration deletion and Zitadel-managed identity/role changes are intentionally absent because no Harpia mutation path exists, and this change does not add one.

PlanExecution and StepExecution start/complete/fail/skip transitions, ApprovalRequest creation, and workflow-signal receipt are not RPC-derived. Temporal workflow code remains deterministic: on these transitions it schedules an audit activity, and that activity calls the same `Recorder` with the workflow-engine actor, typed subject, and trace ID when available. Expected execution failure is itself an auditable lifecycle event, while a rejected/failed public mutation must not produce a successful mutation event. The recorder redacts/omits secret-bearing fields before queueing.

**Alternatives considered:** an interceptor alone cannot observe Temporal signals or reliably calculate before values; recorder calls scattered in every Connect handler duplicate tenant/actor/trace logic and are easy to omit. The hybrid keeps the public transport boundary centralized and makes the exceptional asynchronous sources explicit.

### 4. Use a bounded asynchronous writer, not synchronous inserts on request latency

`Recorder.Record` copies and validates an event draft into a bounded in-process channel. A lifecycle-managed background writer batches inserts through the tenant-safe repository; request completion does not wait for PostgreSQL latency. It flushes/drains queued events during orderly shutdown. A full queue or failed batch produces a structured error and operational metric with no secret payload; it does not block or fail the user's already-authorized operation. Failed batches are retried with bounded backoff before being reported as lost.

This deliberately trades crash-window/queue-saturation durability for the explicit non-blocking request requirement and avoids adding a broker or outbox in this change. The queue capacity, batch size, retry count, and flush interval are configuration values with conservative defaults and testable shutdown behavior.

**Alternatives considered:** synchronous inserts provide stronger per-request durability but add database latency and availability to every mutation. A transactional outbox is more durable but requires a dispatcher and changes to each source transaction; a message broker adds infrastructure not justified by this MVP scope.

### 5. Swap the frontend data adapter while preserving the audit surface

Add `auditClient` to `frontend/src/lib/rpc.ts` from generated `AuditService` bindings. Rewrite `frontend/src/lib/audit/audit-store.ts` as a thin API adapter that maps proto events and UI filters to `ListAuditEvents`; it no longer reads/writes localStorage, seeds locale-specific events, encodes local cursors, or appends/resets mock events. `getAllFilteredAuditEvents` follows server `next_page_token` values so existing CSV/JSON export buttons keep working.

The adapter and route will use the selected tenant ID, send filters to the server, and preserve the server's keyset order. It will render the typed related subject instead of the mock-only task ID and expose the API's event-type filter; add the corresponding flat translation keys to both `frontend/src/lib/i18n/en.json` and `pt-BR.json`. Remove `frontend/src/lib/mocks/audit-events.ts` and the mock append call in `agent-catalog.ts`; actual executor-installation changes are audited at their backend mutation boundary.

**Alternatives considered:** retaining mock data as a fallback obscures backend failures and violates the requirement that the audit log be real. Filtering and paging a fetched full history in the browser defeats the database indexes and cannot scale.

## Risks / Trade-offs

- **[Asynchronous events can be lost during a process crash or sustained database outage]** → Bound the queue, retry failed batches, drain on graceful shutdown, and emit visible structured errors/metrics for drops; a durable outbox remains a future hardening path.
- **[An allowlist can miss newly introduced mutating operations]** → Make the mapping a table-driven unit-tested registry and add a review checklist/test for every targeted existing RPC family; non-RPC workflow sources have explicit tests. New CRUD/identity operations require their owning change to add coverage.
- **[An audit diff could leak credentials or sensitive configuration]** → Build diffs from an allowlist of safe fields, redact known secret names defensively, and test that auth/OAuth/API-key values never reach the stored event.
- **[RLS is accidentally bypassed by a background goroutine]** → Require all audit reads/writes to run through the same tenant-scoped database transaction helper and add cross-tenant integration tests for both paths.
- **[The legacy mock's task labels drift from the plan-centric taxonomy]** → Treat this as the planned pre-v1 breaking cleanup: map legacy UI display to typed related subjects and ship the replacement labels in both locales.

## Migration Plan

1. Add and apply migration `000018_audit_events.sql`, including forced RLS, policies, and indexes before deploying code that writes events.
2. Add the proto contract and regenerate Buf/Connect Go and TypeScript bindings; add the audit repository, writer, handler, interceptor, and workflow emitters behind the new table.
3. Register the service and recorder during API/worker startup, then deploy the backend. Existing operations continue if the audit writer is unavailable, with the failure observable through logs/metrics.
4. Deploy the frontend adapter and bilingual related-subject copy; remove mock-only production sources and update their tests.
5. Roll back application/frontend code by removing the service/interceptor wiring if necessary. The new table is additive and remains safely unused; do not drop audit history as part of rollback.

## Open Questions

None for this scoped change. The bounded in-process writer's durability trade-off is intentional; retention, streaming, and a durable outbox are explicitly deferred non-goals.
