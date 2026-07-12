-- Composable LinkedIn execution foundation: version-addressable step outputs,
-- version-pinned approvals, and durable Review/Revision checkpoints.
--
-- Review requests are created by Temporal activities, which are at-least-once.
-- The natural key below makes replay creation idempotent: repository inserts
-- use INSERT ... ON CONFLICT (tenant_id, step_execution_id,
-- subject_artifact_version_id) DO NOTHING and return the existing request.

-- ============================================================
-- Version-aware StepExecution and ApprovalRequest references
-- ============================================================

ALTER TABLE step_executions
    ADD COLUMN input_artifact_version_id UUID,
    ADD COLUMN input_artifact_type_key TEXT,
    ADD COLUMN input_content_hash TEXT,
    ADD COLUMN output_artifact_version_id UUID,
    ADD COLUMN output_artifact_type_key TEXT,
    ADD COLUMN output_content_hash TEXT;

ALTER TABLE plan_approval_requests
    ADD COLUMN subject_artifact_id UUID,
    ADD COLUMN subject_artifact_version_id UUID,
    ADD COLUMN subject_artifact_type_key TEXT,
    ADD COLUMN subject_content_hash TEXT,
    ADD CONSTRAINT plan_approval_requests_subject_artifact_version_tenant_fk
        FOREIGN KEY (subject_artifact_version_id, subject_artifact_id, tenant_id)
        REFERENCES artifact_versions(id, artifact_id, tenant_id);

CREATE INDEX idx_plan_approval_requests_subject_version
    ON plan_approval_requests(tenant_id, subject_artifact_version_id)
    WHERE subject_artifact_version_id IS NOT NULL;

-- ============================================================
-- plan_review_requests
-- ============================================================

CREATE TABLE plan_review_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    plan_execution_id UUID NOT NULL REFERENCES plan_executions(id) ON DELETE CASCADE,
    step_execution_id UUID NOT NULL REFERENCES step_executions(id) ON DELETE CASCADE,
    plan_step_key TEXT NOT NULL,
    subject_artifact_id UUID NOT NULL,
    subject_artifact_version_id UUID NOT NULL,
    subject_artifact_type_key TEXT NOT NULL,
    subject_content_hash TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    revision_feedback TEXT NOT NULL DEFAULT '',
    decision TEXT,
    overseer_user_id UUID,
    decided_by_user_id UUID,
    requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    decided_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT plan_review_requests_status_check CHECK (status IN (
        'pending',
        'accepted',
        'revision_requested',
        'cancelled'
    )),
    CONSTRAINT plan_review_requests_decision_check CHECK (
        decision IS NULL OR decision IN ('accept', 'revise')
    ),
    CONSTRAINT plan_review_requests_subject_artifact_version_tenant_fk
        FOREIGN KEY (subject_artifact_version_id, subject_artifact_id, tenant_id)
        REFERENCES artifact_versions(id, artifact_id, tenant_id),
    CONSTRAINT plan_review_requests_tenant_step_version_unique
        UNIQUE (tenant_id, step_execution_id, subject_artifact_version_id)
);

CREATE INDEX idx_plan_review_requests_tenant_status_requested
    ON plan_review_requests(tenant_id, status, requested_at DESC);
CREATE INDEX idx_plan_review_requests_execution
    ON plan_review_requests(plan_execution_id);
CREATE INDEX idx_plan_review_requests_step
    ON plan_review_requests(step_execution_id);
CREATE INDEX idx_plan_review_requests_overseer
    ON plan_review_requests(tenant_id, overseer_user_id, status);

-- ============================================================
-- Row-Level Security (forced)
-- ============================================================

ALTER TABLE plan_review_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE plan_review_requests FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation_plan_review_requests ON plan_review_requests;
CREATE POLICY tenant_isolation_plan_review_requests ON plan_review_requests
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());
