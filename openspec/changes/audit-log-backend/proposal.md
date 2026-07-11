## Why

The `/admin/audit` surface currently presents browser-local mock events, so it cannot show the governed operations that actually occurred in a tenant or provide a trustworthy audit trail. The platform constitution makes auditable, tenant-governed operations a product invariant; a persisted, tenant-isolated audit log is needed before the existing surface can serve that role.

## What Changes

- Add a tenant-scoped, RLS-enforced `audit_events` ledger with indexes for the audit browser's filtered, newest-first keyset queries.
- Add `harpia.audit.v1.AuditService.ListAuditEvents`, its generated Connect handlers, and a Go audit repository/handler for listing persisted events.
- Add a shared audit recording adapter: Connect request interception records successful public mutations, while explicit workflow/domain emission records operations that do not originate in an RPC (including execution lifecycle and workflow signals).
- Record the initial audited operation set: successful dev-auth authentication, PlanConfiguration create/update and configuration changes, PlanExecution and StepExecution lifecycle changes, ExecutorInstallation create/update/delete, ApprovalRequest creation/decision, and workflow signals. Status and selected key-field transitions include before/after values; the change does not attempt universal field-level diffs.
- Audit only operations Harpia performs today. Zitadel-managed identity/role changes and a PlanConfiguration deletion operation are outside this change because Harpia exposes neither mutation path; this change MUST NOT add RPCs or handlers solely to manufacture audit coverage.
- Replace the localStorage/mock implementation behind the existing audit UI with generated `AuditService` calls, including server-side filters, keyset paging, and paged retrieval for the existing CSV/JSON exports. Remove mock mutation/reset behavior and its production call sites.
- **BREAKING**: Replace the mock model's legacy task-centric audit subject with a generic typed subject so new contracts use the constitution's PlanConfiguration and PlanExecution terminology rather than introducing new `task` fields.

## Capabilities

### New Capabilities

- `audit-log`: Tenant-isolated recording and querying of governed operation audit events through a ConnectRPC API and the existing audit browser.

### Modified Capabilities

(none)

## Impact

- **Database**: new `000018_audit_events.sql` migration, RLS policy, and keyset/filter indexes under `database/migrations/`.
- **Proto/API**: new `proto/harpia/audit/v1/audit.proto`, Buf-generated Go/TypeScript Connect bindings, and API route registration in `control-plane/cmd/api/main.go`.
- **Control plane**: new audit domain port/repository/handler and recording adapter, plus wiring at the public Connect interceptor chain and workflow/execution mutation sources. The audit adapter remains outside domain packages.
- **Frontend**: `$lib/rpc.ts`, the audit data adapter/tests, and mock-dependent call sites switch to `AuditService`; the existing audit route and CSV/JSON UI remain in place.
- **Verification**: migration/RLS and keyset-query tests, recorder/interceptor and workflow-emitter tests, handler/API tests, generated-contract checks, and frontend audit adapter/UI tests. The related-subject and event-type filter labels introduced by the mock-model cleanup ship as flat `en` and `pt-BR` translation keys.
