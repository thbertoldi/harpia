-- Harpia Tenant RLS Hardening
-- Defense in depth for tenant-scoped tables (issue #32).

-- ============================================================
-- Tenant ownership columns
-- ============================================================

ALTER TABLE subtasks
    ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id);

UPDATE subtasks st
    SET tenant_id = t.tenant_id
    FROM tasks t
    WHERE st.task_id = t.id
      AND st.tenant_id IS NULL;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM subtasks WHERE tenant_id IS NULL) THEN
        RAISE EXCEPTION 'cannot enforce subtasks.tenant_id: existing rows could not be backfilled';
    END IF;
END;
$$;

ALTER TABLE subtasks
    ALTER COLUMN tenant_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_subtasks_tenant ON subtasks(tenant_id);

ALTER TABLE agent_types
    ADD COLUMN IF NOT EXISTS tenant_id UUID REFERENCES tenants(id);

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM (
            SELECT agent_type_id, tenant_id FROM tasks WHERE agent_type_id IS NOT NULL
            UNION ALL
            SELECT agent_type_id, tenant_id FROM subtasks WHERE agent_type_id IS NOT NULL
            UNION ALL
            SELECT agent_type_id, tenant_id FROM agent_instances WHERE agent_type_id IS NOT NULL
        ) refs
        GROUP BY agent_type_id
        HAVING count(DISTINCT tenant_id) > 1
    ) THEN
        RAISE EXCEPTION 'cannot enforce agent_types.tenant_id: an existing agent type is referenced by multiple tenants';
    END IF;
END;
$$;

UPDATE agent_types agt
    SET tenant_id = refs.tenant_id
    FROM (
        SELECT agent_type_id, (array_agg(DISTINCT tenant_id))[1] AS tenant_id
        FROM (
            SELECT agent_type_id, tenant_id FROM tasks WHERE agent_type_id IS NOT NULL
            UNION ALL
            SELECT agent_type_id, tenant_id FROM subtasks WHERE agent_type_id IS NOT NULL
            UNION ALL
            SELECT agent_type_id, tenant_id FROM agent_instances WHERE agent_type_id IS NOT NULL
        ) referenced_agent_types
        GROUP BY agent_type_id
    ) refs
    WHERE agt.id = refs.agent_type_id
      AND agt.tenant_id IS NULL;

UPDATE agent_types
    SET tenant_id = (SELECT id FROM tenants ORDER BY created_at ASC, id ASC LIMIT 1)
    WHERE tenant_id IS NULL;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM agent_types WHERE tenant_id IS NULL) THEN
        RAISE EXCEPTION 'cannot enforce agent_types.tenant_id: existing rows could not be assigned to a tenant';
    END IF;
END;
$$;

ALTER TABLE agent_types
    ALTER COLUMN tenant_id SET NOT NULL;

ALTER TABLE agent_types
    DROP CONSTRAINT IF EXISTS agent_types_name_key;

CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_types_tenant_name ON agent_types(tenant_id, name);
CREATE INDEX IF NOT EXISTS idx_agent_types_tenant_enabled ON agent_types(tenant_id, enabled);

-- ============================================================
-- RLS helpers
-- ============================================================

CREATE OR REPLACE FUNCTION current_tenant_id() RETURNS UUID AS $$
BEGIN
    RETURN NULLIF(current_setting('harpia.tenant_id', true), '')::UUID;
END;
$$ LANGUAGE plpgsql STABLE;

CREATE OR REPLACE FUNCTION current_harpia_user_external_id() RETURNS TEXT AS $$
BEGIN
    RETURN NULLIF(current_setting('harpia.user_external_id', true), '');
END;
$$ LANGUAGE plpgsql STABLE;

-- ============================================================
-- Forced RLS policies
-- ============================================================

ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE users FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation_users ON users;
CREATE POLICY tenant_isolation_users ON users
    FOR ALL
    USING (
        tenant_id = current_tenant_id()
        OR external_id = current_harpia_user_external_id()
    )
    WITH CHECK (
        tenant_id = current_tenant_id()
        OR external_id = current_harpia_user_external_id()
    );

ALTER TABLE tasks ENABLE ROW LEVEL SECURITY;
ALTER TABLE tasks FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation_tasks ON tasks;
CREATE POLICY tenant_isolation_tasks ON tasks
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());

ALTER TABLE subtasks ENABLE ROW LEVEL SECURITY;
ALTER TABLE subtasks FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation_subtasks ON subtasks;
CREATE POLICY tenant_isolation_subtasks ON subtasks
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());

ALTER TABLE agent_instances ENABLE ROW LEVEL SECURITY;
ALTER TABLE agent_instances FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation_agent_instances ON agent_instances;
CREATE POLICY tenant_isolation_agent_instances ON agent_instances
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());

ALTER TABLE human_feedback ENABLE ROW LEVEL SECURITY;
ALTER TABLE human_feedback FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation_human_feedback ON human_feedback;
CREATE POLICY tenant_isolation_human_feedback ON human_feedback
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());

ALTER TABLE workspaces ENABLE ROW LEVEL SECURITY;
ALTER TABLE workspaces FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation_workspaces ON workspaces;
CREATE POLICY tenant_isolation_workspaces ON workspaces
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());

ALTER TABLE agent_types ENABLE ROW LEVEL SECURITY;
ALTER TABLE agent_types FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS agent_types_read_all ON agent_types;
DROP POLICY IF EXISTS tenant_isolation_agent_types ON agent_types;
CREATE POLICY tenant_isolation_agent_types ON agent_types
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());
