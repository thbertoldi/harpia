# Issue 32 Investigation: Postgres RLS for Tenant-Scoped Tables

## Hand-off Brief

Issue #32 is valid: Harpia already had partial RLS migrations, but the runtime tenant setting was not applied in the transaction that ran repository queries. The pre-change database helper acquired a pooled connection and executed `SET LOCAL`, then repository methods issued later statements without an explicit transaction, so PostgreSQL discarded the local setting after the `SET LOCAL` statement. A complete fix needs schema hardening and an infrastructure-level transaction helper, not handler-level tenant plumbing.

## Case Info

- Input: GitHub issue #32, "feat(identity): Postgres RLS policies for all tenant-scoped tables"
- Worktree: `/home/thbertoldi/harpia/.worktrees/issue-32-postgres-rls`
- Bounded context: Identity & Tenants, with repository/infrastructure enforcement for other tenant-scoped contexts

## Evidence

- Confirmed: ADR-003 requires PostgreSQL RLS for tenant isolation and states tenant queries should be filtered at the database level via a tenant setting. See `docs/adr/ADR-003-data-architecture.md:13`.
- Confirmed: ADR-006 separates Identity & Tenants from Task Management, Agent Orchestration, and Human Interaction, so tenant plumbing belongs at integration boundaries rather than scattered in domain handlers. See `docs/adr/ADR-006-domain-driven-design.md:43`.
- Confirmed: Existing migrations enable RLS on `users`, `tasks`, `subtasks`, `agent_instances`, and `human_feedback`, and define `current_tenant_id()` from `harpia.tenant_id`. See `database/migrations/000001_initial_schema.sql:136`.
- Confirmed: `agent_types` is currently enabled for RLS but has a `SELECT USING (true)` policy and no `tenant_id`, despite issue #32 treating it as tenant-bound. See `database/migrations/000002_schema_sync.sql:80`.
- Confirmed: `subtasks` currently lacks a direct `tenant_id`; its RLS policy infers tenant through `tasks`. See `database/migrations/000001_initial_schema.sql:72`.
- Confirmed: The fixed implementation now centralizes tenant database access through `database.WithTenant`, which starts a transaction before setting `harpia.tenant_id`. See `control-plane/internal/database/postgres.go:41`.
- Confirmed: Task repositories already centralize tenant database access through the database helper seam, so the infrastructure helper is the right place to fix runtime RLS behavior. See `control-plane/internal/tasks/repository.go:45`.

## Conclusion

Confidence: High.

The required implementation path is:

1. Add a migration that gives present tenant-scoped tables direct tenant ownership where missing (`subtasks`, `agent_types`), replaces permissive policies with tenant isolation policies, and forces RLS on tenant-scoped tables so the application owner cannot bypass policies.
2. Replace the connection-level helper with a transaction-scoped helper that calls `set_config('harpia.tenant_id', ..., true)` and executes repository work inside that transaction.
3. Update agent repository methods to accept tenant context because `agent_types` becomes tenant-owned.
4. Add an integration-style database test that proves tenant A cannot read tenant B rows through the helper.

## Missing Evidence

- `feedback_events`, `tool_definitions`, and `audit_log` are named in issue #32 but do not exist in the current schema. This implementation should not create unused domain tables only to attach RLS; the migration should cover the current schema and the final report should call out the absent tables explicitly.
