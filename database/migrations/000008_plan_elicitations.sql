-- Plan elicitations (E5.1, issue #130)
-- Durable in-app elicitation threads: an agent StepExecution raises a question
-- (ElicitationRequest) that the assigned overseer answers, resuming the
-- PlanWorkflow via the `plan-elicitation-response` Temporal signal.

CREATE TABLE plan_elicitations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    plan_execution_id UUID NOT NULL REFERENCES plan_executions(id) ON DELETE CASCADE,
    step_execution_id UUID NOT NULL REFERENCES step_executions(id) ON DELETE CASCADE,
    plan_step_key TEXT NOT NULL DEFAULT '',
    elicitation_thread_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    prompt TEXT NOT NULL DEFAULT '',
    schema_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    response_json JSONB,
    response_text TEXT NOT NULL DEFAULT '',
    timeout_behavior TEXT NOT NULL DEFAULT '',
    overseer_user_id UUID,
    responded_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ,
    responded_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, step_execution_id, elicitation_thread_id)
);

CREATE INDEX idx_plan_elicitations_tenant_status
    ON plan_elicitations(tenant_id, status, created_at DESC);
CREATE INDEX idx_plan_elicitations_step
    ON plan_elicitations(step_execution_id);
CREATE INDEX idx_plan_elicitations_execution
    ON plan_elicitations(plan_execution_id);
CREATE INDEX idx_plan_elicitations_overseer
    ON plan_elicitations(tenant_id, overseer_user_id, status);

ALTER TABLE plan_elicitations ENABLE ROW LEVEL SECURITY;
ALTER TABLE plan_elicitations FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_plan_elicitations ON plan_elicitations
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());
