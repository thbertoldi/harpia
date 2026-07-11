## ADDED Requirements

### Requirement: Tenant-isolated persisted audit ledger
The system SHALL persist audit events in an append-only `audit_events` table scoped by `tenant_id`. Each event MUST retain a stable ID, event type, bounded context, actor kind/ID/display-name snapshot and optional agent type, optional typed subject, ordered payload diff entries, timestamp, optional trace ID, and optional feedback decision. The table MUST enable and force Row-Level Security with a policy requiring `tenant_id = current_tenant_id()` for reads and writes.

#### Scenario: Same-tenant event is persisted and visible
- **WHEN** a recorded event is written while the database tenant context is tenant A
- **THEN** the event is stored with tenant A, its safe actor/subject/diff/trace fields are preserved, and a tenant A query can return it

#### Scenario: Cross-tenant event is not visible
- **WHEN** an event belongs to tenant A and a request or background write executes with the database tenant context for tenant B
- **THEN** RLS prevents tenant B from reading or writing that tenant A event

#### Scenario: Secret-bearing data is excluded from an event
- **WHEN** an audited operation includes an authorization header, API key, OAuth credential, or secret configuration field
- **THEN** the stored event contains no secret value in its actor, subject, payload diff, or trace fields

### Requirement: Stable filtered keyset audit listing
The system SHALL provide `harpia.audit.v1.AuditService.ListAuditEvents` for authenticated tenant-scoped listing. The request MUST support tenant, event-type, actor, inclusive date-range, typed-subject, and feedback-decision filters plus an opaque keyset page token; the response MUST return events ordered by `(occurred_at DESC, event_id DESC)` and an opaque next-page token when another matching page exists. The handler MUST authorize the requested tenant through the request context, cap page size, and reject malformed or inconsistent page tokens with `InvalidArgument`.

#### Scenario: Filter by event type, actor, and date range
- **WHEN** a tenant requests audit events for one or more event types, a matching actor filter, and inclusive date bounds
- **THEN** the response contains only that tenant's events satisfying every supplied filter in newest-first order

#### Scenario: Keyset page remains non-overlapping while new events arrive
- **WHEN** a client requests a first audit page, new matching events are recorded, and the client requests the next page with the first page's token
- **THEN** the next page contains only events strictly older than the cursor tuple and contains no event returned by the first page

#### Scenario: Unauthorized tenant cannot be listed
- **WHEN** a caller requests a tenant that is not available in its authenticated request context
- **THEN** the service returns `PermissionDenied` and no audit events

### Requirement: Audited existing public operations
The system SHALL record a single successful audit event for each targeted existing public operation through the audit interceptor, using the request identity for human actors and the active trace ID when available. The initial interceptor coverage MUST include successful enabled dev-token authentication resolution, PlanConfiguration create/update/status changes, slot-binding/OverseerBinding/PlanBehaviorPolicies changes, PlanExecution creation, ExecutorInstallation create/update/delete, and approval decisions. Status and selected safe key-field transitions MUST include before/after diff entries. The system MUST NOT add a PlanConfiguration deletion or identity/role/permission mutation RPC or handler solely for audit coverage.

#### Scenario: PlanConfiguration status change is audited
- **WHEN** an authorized user changes a PlanConfiguration from `DRAFT` to `RUNNABLE`
- **THEN** exactly one `plan_configuration.status_changed` event is queued with the human actor, `plan_management` bounded context, PlanConfiguration subject, and a `status` diff from `DRAFT` to `RUNNABLE`

#### Scenario: Executor installation update is audited without credentials
- **WHEN** an authorized user changes an ExecutorInstallation's safe configuration or connection state
- **THEN** exactly one `executor_installation.updated` event is queued with the ExecutorInstallation subject and safe changed fields, without recording credentials or tokens

#### Scenario: Dev authentication is audited without a login RPC
- **WHEN** `allowDevAuth` is enabled and a successful public request is authenticated with the dev token
- **THEN** exactly one `authentication.dev_auth` event is queued with the resolved human actor and no credential value

#### Scenario: Rejected mutation does not create a successful audit event
- **WHEN** a targeted mutating RPC is rejected or fails before its domain mutation completes
- **THEN** the interceptor does not record a successful mutation audit event for that request

### Requirement: Workflow and execution events use the shared recorder
The system SHALL record PlanExecution and StepExecution start/complete/fail/skip transitions, ApprovalRequest creation, and workflow-signal receipt through the same audit recorder even when they originate outside a public Connect RPC. Temporal workflow code MUST schedule an audit activity rather than perform a direct database write. Such events MUST identify the workflow-engine actor, bounded context, typed subject, and trace ID when available.

#### Scenario: Failed PlanExecution is recorded from workflow code
- **WHEN** a PlanExecution fails in the workflow worker without a browser RPC in progress
- **THEN** the shared recorder queues a `plan_execution.failed` event with the PlanExecution subject and its safe failure/status diff

#### Scenario: Workflow signal is recorded once
- **WHEN** the workflow engine receives an eligible signal for a PlanExecution or StepExecution
- **THEN** the workflow path queues one `workflow.signal_received` event without relying on the public Connect interceptor

### Requirement: Asynchronous audit writer does not add request insert latency
The system SHALL enqueue validated audit events to a bounded background writer and SHALL NOT wait for a PostgreSQL insert before completing an otherwise successful public request. The writer MUST batch and retry failed inserts with bounded backoff, drain queued entries during orderly shutdown, and emit structured operational failure evidence without event payload secrets when its queue is full or a batch is exhausted.

#### Scenario: Successful mutation returns before audit batch flush
- **WHEN** a targeted public mutation succeeds while the audit writer has not yet flushed its batch
- **THEN** the mutation response completes after its event is accepted by the recorder without waiting for the audit insert to finish

#### Scenario: Writer outage is observable without blocking the user
- **WHEN** an audit batch repeatedly fails or the bounded queue is full
- **THEN** the user operation is not blocked by the audit write and the system emits a structured error/metric identifying the recording failure without exposing payload data

### Requirement: Audit browser uses the real audit API
The frontend SHALL replace localStorage and hardcoded mock audit events with generated `AuditService.ListAuditEvents` calls. It MUST send the selected tenant and active filters to the service, use server page tokens for next/previous navigation, and fetch all server pages matching the active filters for the existing CSV and JSON exports. It MUST not append, reset, seed, or display mock audit data in production.

#### Scenario: Audit page applies a server-side filter
- **WHEN** a user applies an actor, event type, related-subject, decision, or date filter in the audit browser
- **THEN** the frontend requests the filtered page from `AuditService` and renders only events returned by the service

#### Scenario: Export includes all matching backend pages
- **WHEN** a user exports filtered audit events that span more than one API page
- **THEN** the frontend follows each `next_page_token` and exports every returned matching event in the existing CSV or JSON format

#### Scenario: Related-subject copy is localized
- **WHEN** the audit browser renders the replacement for the mock task-ID label in either supported locale
- **THEN** it uses flat `translate()` keys present in both `en` and `pt-BR` locale files
