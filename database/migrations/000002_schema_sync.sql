-- Harpia Schema Sync
-- pgvector extension for agent capability matching (ADR-003)
-- Workspaces table for tenant-scoped project organization (ADR-006)
-- Restructured human_feedback as standalone bounded context (ADR-006)
-- Task extensions for operator assignment and cost budgeting
-- Zitadel identity alignment (ADR-004)

CREATE EXTENSION IF NOT EXISTS vector;

-- ============================================================
-- Fix comment: Authentik -> Zitadel (ADR-004)
-- ============================================================
COMMENT ON COLUMN users.external_id IS 'Zitadel user ID';

-- ============================================================
-- Agent Types: add capability embedding (ADR-002)
-- ============================================================
ALTER TABLE agent_types
    ADD COLUMN capabilities_embedding vector(1536);

-- ============================================================
-- Tasks: add operator assignment and cost budget
-- ============================================================
ALTER TABLE tasks
    ADD COLUMN assigned_operator_id UUID REFERENCES users(id),
    ADD COLUMN cost_budget NUMERIC;

-- ============================================================
-- Workspaces (tenant-scoped project organization)
-- ============================================================
CREATE TABLE workspaces (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, slug)
);

-- ============================================================
-- Human Feedback: restructure as standalone bounded context
-- ============================================================
ALTER TABLE human_feedback
    ALTER COLUMN agent_instance_id DROP NOT NULL,
    ADD COLUMN task_id UUID REFERENCES tasks(id),
    ADD COLUMN subtask_id UUID REFERENCES subtasks(id),
    ADD COLUMN tenant_id UUID REFERENCES tenants(id),
    ADD COLUMN channel TEXT NOT NULL DEFAULT 'ui',
    ADD COLUMN external_ref TEXT,
    ADD COLUMN status TEXT NOT NULL DEFAULT 'pending',
    ADD COLUMN decision TEXT,
    ADD COLUMN comment TEXT;

-- Backfill tenant_id for existing rows from agent_instances join
UPDATE human_feedback hf
    SET tenant_id = ai.tenant_id
    FROM agent_instances ai
    WHERE hf.agent_instance_id IS NOT NULL
      AND hf.agent_instance_id = ai.id;

ALTER TABLE human_feedback
    ALTER COLUMN tenant_id SET NOT NULL;

-- ============================================================
-- Indexes
-- ============================================================
CREATE INDEX idx_tasks_assigned_operator ON tasks(assigned_operator_id);
CREATE INDEX idx_tasks_cost_budget ON tasks(cost_budget) WHERE cost_budget IS NOT NULL;
CREATE INDEX idx_human_feedback_tenant ON human_feedback(tenant_id);
CREATE INDEX idx_human_feedback_task ON human_feedback(task_id);
CREATE INDEX idx_human_feedback_status ON human_feedback(tenant_id, status);
CREATE INDEX idx_workspaces_tenant ON workspaces(tenant_id);
CREATE INDEX idx_workspaces_slug ON workspaces(tenant_id, slug);

-- ============================================================
-- Row-Level Security
-- ============================================================

ALTER TABLE workspaces ENABLE ROW LEVEL SECURITY;
ALTER TABLE agent_types ENABLE ROW LEVEL SECURITY;

-- Workspaces: filter by tenant
CREATE POLICY tenant_isolation_workspaces ON workspaces
    FOR ALL USING (tenant_id = current_tenant_id());

-- Agent types: global read, admin mutate
CREATE POLICY agent_types_read_all ON agent_types
    FOR SELECT USING (true);

-- Human feedback: update to use tenant_id directly (standalone bounded context)
DROP POLICY IF EXISTS tenant_isolation_human_feedback ON human_feedback;

CREATE POLICY tenant_isolation_human_feedback ON human_feedback
    FOR ALL USING (tenant_id = current_tenant_id());
