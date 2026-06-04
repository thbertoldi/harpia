-- Harpia Initial Schema
-- Multi-tenant: all tenant-scoped tables include tenant_id with RLS

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================================
-- Tenants
-- ============================================================
CREATE TABLE tenants (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    slug TEXT UNIQUE NOT NULL,
    settings JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================
-- Users
-- ============================================================
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    external_id TEXT NOT NULL,           -- Authentik user ID
    email TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    role TEXT NOT NULL DEFAULT 'member', -- owner, admin, member
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(tenant_id, external_id)
);

-- ============================================================
-- Agent Types (global registry, visible to all tenants)
-- ============================================================
CREATE TABLE agent_types (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT UNIQUE NOT NULL,
    description TEXT NOT NULL,
    input_schema JSONB NOT NULL DEFAULT '{}',
    output_schema JSONB NOT NULL DEFAULT '{}',
    mcp_servers JSONB NOT NULL DEFAULT '[]',
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================
-- Tasks
-- ============================================================
CREATE TABLE tasks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    -- pending, planning, queued, running, waiting_human, completed, failed, cancelled
    priority INTEGER NOT NULL DEFAULT 0,
    agent_type_id UUID REFERENCES agent_types(id),
    context JSONB NOT NULL DEFAULT '{}',
    result JSONB,
    error TEXT,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

-- ============================================================
-- Subtasks (from planner decomposition)
-- ============================================================
CREATE TABLE subtasks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    parent_subtask_id UUID REFERENCES subtasks(id),
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    agent_type_id UUID REFERENCES agent_types(id),
    context JSONB NOT NULL DEFAULT '{}',
    result JSONB,
    error TEXT,
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

-- ============================================================
-- Agent Instances (runtime records)
-- ============================================================
CREATE TABLE agent_instances (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    task_id UUID REFERENCES tasks(id),
    subtask_id UUID REFERENCES subtasks(id),
    agent_type_id UUID NOT NULL REFERENCES agent_types(id),
    status TEXT NOT NULL DEFAULT 'provisioning',
    -- provisioning, running, waiting_human, completed, failed
    temporal_workflow_id TEXT,
    pod_name TEXT,
    logs JSONB NOT NULL DEFAULT '{}',
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

-- ============================================================
-- Human Feedback
-- ============================================================
CREATE TABLE human_feedback (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_instance_id UUID NOT NULL REFERENCES agent_instances(id),
    request TEXT NOT NULL,
    response TEXT,
    requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    responded_at TIMESTAMPTZ
);

-- ============================================================
-- Indexes
-- ============================================================
CREATE INDEX idx_users_tenant ON users(tenant_id);
CREATE INDEX idx_tasks_tenant_status ON tasks(tenant_id, status);
CREATE INDEX idx_tasks_created_by ON tasks(created_by);
CREATE INDEX idx_subtasks_task ON subtasks(task_id);
CREATE INDEX idx_agent_instances_tenant ON agent_instances(tenant_id);
CREATE INDEX idx_agent_instances_task ON agent_instances(task_id);

-- ============================================================
-- Row-Level Security (multi-tenant isolation)
-- ============================================================

-- Enable RLS on tenant-scoped tables
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE tasks ENABLE ROW LEVEL SECURITY;
ALTER TABLE subtasks ENABLE ROW LEVEL SECURITY;
ALTER TABLE agent_instances ENABLE ROW LEVEL SECURITY;
ALTER TABLE human_feedback ENABLE ROW LEVEL SECURITY;

-- Helper: current tenant from JWT or request context
-- Application sets "harpia.tenant_id" via SET LOCAL before queries
CREATE FUNCTION current_tenant_id() RETURNS UUID AS $$
BEGIN
    RETURN NULLIF(current_setting('harpia.tenant_id', true), '')::UUID;
END;
$$ LANGUAGE plpgsql STABLE;

-- Users: filter by tenant
CREATE POLICY tenant_isolation_users ON users
    FOR ALL USING (tenant_id = current_tenant_id());

-- Tasks: filter by tenant
CREATE POLICY tenant_isolation_tasks ON tasks
    FOR ALL USING (tenant_id = current_tenant_id());

-- Subtasks: filter by tenant via join to tasks
CREATE POLICY tenant_isolation_subtasks ON subtasks
    FOR ALL USING (
        task_id IN (SELECT id FROM tasks WHERE tenant_id = current_tenant_id())
    );

-- Agent instances: filter by tenant
CREATE POLICY tenant_isolation_agent_instances ON agent_instances
    FOR ALL USING (tenant_id = current_tenant_id());

-- Human feedback: filter by tenant via join to agent_instances
CREATE POLICY tenant_isolation_human_feedback ON human_feedback
    FOR ALL USING (
        agent_instance_id IN (
            SELECT id FROM agent_instances WHERE tenant_id = current_tenant_id()
        )
    );
