-- Plan approval requests (E4.2, issue #123)
-- Durable records for publish approval gates created by PlanWorkflow.

CREATE TABLE plan_approval_requests (
    id TEXT PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    plan_execution_id UUID NOT NULL REFERENCES plan_executions(id) ON DELETE CASCADE,
    step_execution_id UUID NOT NULL REFERENCES step_executions(id) ON DELETE CASCADE,
    plan_step_key TEXT NOT NULL,
    input_artifact_id TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    decision_reason TEXT NOT NULL DEFAULT '',
    requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    decided_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, step_execution_id)
);

CREATE INDEX idx_plan_approval_requests_tenant_status
    ON plan_approval_requests(tenant_id, status, requested_at DESC);
CREATE INDEX idx_plan_approval_requests_execution
    ON plan_approval_requests(plan_execution_id);
CREATE INDEX idx_plan_approval_requests_step
    ON plan_approval_requests(step_execution_id);

ALTER TABLE plan_approval_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE plan_approval_requests FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_plan_approval_requests ON plan_approval_requests
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());
