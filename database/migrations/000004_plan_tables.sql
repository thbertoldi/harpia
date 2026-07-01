-- Harpia Plan-Centric Model (ADR-012, E2.1)
-- Plan templates (global catalog), tenant configurations, and execution records.

-- ============================================================
-- Plan Templates (global catalog)
-- ============================================================
CREATE TABLE plan_templates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    key TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    vertical TEXT NOT NULL DEFAULT '',
    version INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE plan_template_steps (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    plan_template_id UUID NOT NULL REFERENCES plan_templates(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    input_artifact_type_id TEXT NOT NULL,
    output_artifact_type_id TEXT NOT NULL,
    executor_requirement JSONB NOT NULL DEFAULT '{}',
    default_executor_sku_key TEXT,
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(plan_template_id, key)
);

CREATE TABLE plan_template_step_dependencies (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    plan_template_id UUID NOT NULL REFERENCES plan_templates(id) ON DELETE CASCADE,
    from_step_key TEXT NOT NULL,
    to_step_key TEXT NOT NULL,
    UNIQUE(plan_template_id, from_step_key, to_step_key)
);

-- ============================================================
-- Plan Configurations (tenant-scoped)
-- ============================================================
CREATE TABLE plan_configurations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    workspace_id UUID REFERENCES workspaces(id),
    plan_template_id UUID NOT NULL REFERENCES plan_templates(id),
    plan_template_version INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    seed_artifacts JSONB NOT NULL DEFAULT '[]',
    slot_bindings JSONB NOT NULL DEFAULT '[]',
    overseer_bindings JSONB NOT NULL DEFAULT '[]',
    behavior_policies JSONB NOT NULL DEFAULT '{}',
    schedule JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================
-- Plan Executions (tenant-scoped)
-- ============================================================
CREATE TABLE plan_executions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    plan_configuration_id UUID NOT NULL REFERENCES plan_configurations(id),
    plan_configuration_snapshot JSONB NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'pending',
    triggered_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE step_executions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    plan_execution_id UUID NOT NULL REFERENCES plan_executions(id) ON DELETE CASCADE,
    plan_step_key TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    input_artifact_id TEXT,
    output_artifact_id TEXT,
    executor_installation_snapshot JSONB NOT NULL DEFAULT '{}',
    attempt INTEGER NOT NULL DEFAULT 1,
    elicitation_thread_id TEXT,
    approval_request_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================
-- Indexes
-- ============================================================
CREATE INDEX idx_plan_template_steps_template ON plan_template_steps(plan_template_id);
CREATE INDEX idx_plan_template_deps_template ON plan_template_step_dependencies(plan_template_id);
CREATE INDEX idx_plan_configurations_tenant ON plan_configurations(tenant_id);
CREATE INDEX idx_plan_configurations_template ON plan_configurations(plan_template_id);
CREATE INDEX idx_plan_configurations_status ON plan_configurations(tenant_id, status);
CREATE INDEX idx_plan_executions_tenant ON plan_executions(tenant_id);
CREATE INDEX idx_plan_executions_configuration ON plan_executions(plan_configuration_id);
CREATE INDEX idx_step_executions_execution ON step_executions(plan_execution_id);
CREATE INDEX idx_step_executions_tenant ON step_executions(tenant_id);

-- ============================================================
-- Row-Level Security
-- ============================================================
ALTER TABLE plan_configurations ENABLE ROW LEVEL SECURITY;
ALTER TABLE plan_configurations FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_plan_configurations ON plan_configurations
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());

ALTER TABLE plan_executions ENABLE ROW LEVEL SECURITY;
ALTER TABLE plan_executions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_plan_executions ON plan_executions
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());

ALTER TABLE step_executions ENABLE ROW LEVEL SECURITY;
ALTER TABLE step_executions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_step_executions ON step_executions
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());
