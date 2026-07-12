-- Harpia Audit Event Ledger (audit-log-backend §1)
-- Tenant-scoped, append-only audit trail for governed operations.
-- Design: openspec/changes/audit-log-backend/design.md (Decision 1).

-- ============================================================
-- audit_events
-- ============================================================
-- Append-only ledger of audited operations. Each row is one
-- transition of a governed operation (PlanConfiguration /
-- PlanExecution / StepExecution / ExecutorInstallation /
-- ApprovalRequest / authentication). RLS is the tenant data
-- boundary; the append-only guarantee is enforced by a trigger
-- (see audit_events_immutable) that is independent of the
-- connecting role, so a missed REVOKE cannot weaken it.
--
-- Dedupe / idempotency (senior architectural amendment):
-- Temporal activities are at-least-once, and Wave 3 records
-- "exactly one event per transition." A natural-key
-- `dedupe_key` (nullable) plus a partial UNIQUE index lets the
-- recorder use INSERT ... ON CONFLICT (tenant_id, dedupe_key)
-- DO NOTHING so replays never duplicate a row. Events with no
-- natural idempotency key leave dedupe_key NULL and are always
-- inserted. This prevents *duplication*; it is NOT an
-- exactly-once ledger — the in-process writer makes delivery
-- best-effort (a crashed process can lose queued events). That
-- tradeoff is the accepted design (design.md:58-64,74-79).
CREATE TABLE audit_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    -- Nullable: events with no natural idempotency key are NULL
    -- and exempt from the partial unique index below.
    dedupe_key TEXT,
    event_type TEXT NOT NULL,
    bounded_context TEXT NOT NULL,
    actor_kind TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    actor_display_name TEXT NOT NULL DEFAULT '',
    -- Present only for agent/workflow actors.
    actor_agent_type TEXT,
    -- Optional typed subject; authentication events legitimately
    -- have no subject (both columns NULL).
    subject_type TEXT,
    subject_id TEXT,
    payload_diff JSONB NOT NULL DEFAULT '[]',
    decision TEXT,
    trace_id TEXT,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================
-- Indexes
-- ============================================================
-- Canonical newest-first keyset query.
CREATE INDEX idx_audit_events_tenant_occurred
    ON audit_events(tenant_id, occurred_at DESC, id DESC);

-- Event-type filter within a tenant.
CREATE INDEX idx_audit_events_tenant_type_occurred
    ON audit_events(tenant_id, event_type, occurred_at DESC);

-- Actor filter within a tenant.
CREATE INDEX idx_audit_events_tenant_actor_occurred
    ON audit_events(tenant_id, actor_kind, actor_id, occurred_at DESC);

-- Idempotent replay dedupe (amendment). Partial: NULL keys are
-- exempt so non-idempotent events always insert.
CREATE UNIQUE INDEX idx_audit_events_tenant_dedupe
    ON audit_events(tenant_id, dedupe_key)
    WHERE dedupe_key IS NOT NULL;

-- ============================================================
-- Row-Level Security (forced)
-- ============================================================
-- Forced so a missed SET LOCAL cannot leak cross-tenant rows;
-- matches the pattern in 000003_tenant_rls_hardening.sql.
ALTER TABLE audit_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit_events FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation_audit_events ON audit_events;
CREATE POLICY tenant_isolation_audit_events ON audit_events
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());

-- ============================================================
-- Append-only enforcement (DB-level)
-- ============================================================
-- A trigger blocks UPDATE and DELETE regardless of the
-- connecting role, so the ledger is immutable even if a future
-- app role is granted broad privileges. Schema changes
-- (ALTER TABLE) do not fire row triggers, so future migrations
-- can still evolve the table; an operator that must correct
-- history can temporarily DISABLE TRIGGER.
CREATE OR REPLACE FUNCTION audit_events_reject_mutation() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit_events is append-only (operation % not permitted on %)',
        TG_OP, TG_TABLE_NAME;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS audit_events_block_update ON audit_events;
CREATE TRIGGER audit_events_block_update
    BEFORE UPDATE ON audit_events
    FOR EACH ROW
    EXECUTE FUNCTION audit_events_reject_mutation();

DROP TRIGGER IF EXISTS audit_events_block_delete ON audit_events;
CREATE TRIGGER audit_events_block_delete
    BEFORE DELETE ON audit_events
    FOR EACH ROW
    EXECUTE FUNCTION audit_events_reject_mutation();
