-- Path B thread foundation.

SET row_security = off;

CREATE TABLE threads (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    title TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'THREAD_STATUS_OPEN',
    active_plan_configuration_id UUID,
    created_by_user_id UUID,
    archived_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT threads_status_check CHECK (status IN (
        'THREAD_STATUS_UNSPECIFIED',
        'THREAD_STATUS_OPEN',
        'THREAD_STATUS_RUNNING',
        'THREAD_STATUS_NEEDS_ATTENTION',
        'THREAD_STATUS_COMPLETED',
        'THREAD_STATUS_ARCHIVED'
    )),
    UNIQUE (id, tenant_id)
);

ALTER TABLE plan_configurations
    ADD COLUMN thread_id UUID;

INSERT INTO threads (
    tenant_id,
    title,
    status,
    active_plan_configuration_id,
    archived_at,
    created_at,
    updated_at
)
SELECT
    pc.tenant_id,
    COALESCE(NULLIF(pt.name, ''), 'Untitled chat'),
    CASE
        WHEN pc.status = 'scheduled' THEN 'THREAD_STATUS_OPEN'
        WHEN pc.status = 'archived' THEN 'THREAD_STATUS_ARCHIVED'
        ELSE 'THREAD_STATUS_OPEN'
    END,
    pc.id,
    CASE
        WHEN pc.status = 'archived' THEN pc.updated_at
        ELSE NULL
    END,
    pc.created_at,
    pc.updated_at
FROM plan_configurations pc
JOIN plan_templates pt ON pt.id = pc.plan_template_id;

UPDATE plan_configurations pc
SET thread_id = t.id
FROM threads t
WHERE t.active_plan_configuration_id = pc.id
  AND t.tenant_id = pc.tenant_id;

DO $$
DECLARE
    invalid_count BIGINT;
BEGIN
    SELECT COUNT(*)
    INTO invalid_count
    FROM plan_configurations
    WHERE thread_id IS NULL;

    IF invalid_count > 0 THEN
        RAISE EXCEPTION 'threads backfill failed: plan_configurations.thread_id contains % NULL values', invalid_count;
    END IF;

    SELECT COUNT(*)
    INTO invalid_count
    FROM chat_messages
    WHERE thread_id !~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$';

    IF invalid_count > 0 THEN
        RAISE EXCEPTION 'threads backfill failed: chat_messages.thread_id contains % non-UUID values', invalid_count;
    END IF;

    SELECT COUNT(*)
    INTO invalid_count
    FROM chat_messages cm
    WHERE NOT EXISTS (
        SELECT 1
        FROM plan_configurations pc
        WHERE pc.tenant_id = cm.tenant_id
          AND pc.id::text = cm.thread_id
    );

    IF invalid_count > 0 THEN
        RAISE EXCEPTION 'threads backfill failed: % chat_messages.thread_id values do not match a same-tenant plan_configurations.id', invalid_count;
    END IF;
END $$;

ALTER TABLE plan_configurations
    ALTER COLUMN thread_id SET NOT NULL,
    ADD CONSTRAINT plan_configurations_id_thread_tenant_unique UNIQUE (id, thread_id, tenant_id);

ALTER TABLE threads
    ADD CONSTRAINT threads_active_plan_configuration_thread_tenant_fk
    FOREIGN KEY (active_plan_configuration_id, id, tenant_id)
    REFERENCES plan_configurations(id, thread_id, tenant_id);

ALTER TABLE chat_messages
    ADD COLUMN new_thread_id UUID;

UPDATE chat_messages cm
SET new_thread_id = pc.thread_id
FROM plan_configurations pc
WHERE cm.tenant_id = pc.tenant_id
  AND cm.thread_id = pc.id::text;

DO $$
DECLARE
    invalid_count BIGINT;
BEGIN
    SELECT COUNT(*)
    INTO invalid_count
    FROM chat_messages
    WHERE new_thread_id IS NULL;

    IF invalid_count > 0 THEN
        RAISE EXCEPTION 'threads backfill failed: chat_messages.new_thread_id contains % NULL values after backfill', invalid_count;
    END IF;
END $$;

DROP INDEX IF EXISTS idx_chat_messages_thread_seq;
DROP INDEX IF EXISTS idx_chat_messages_thread_created;

ALTER TABLE chat_messages
    DROP CONSTRAINT chat_messages_thread_seq_unique,
    DROP COLUMN thread_id;

ALTER TABLE chat_messages
    RENAME COLUMN new_thread_id TO thread_id;

ALTER TABLE chat_messages
    ALTER COLUMN thread_id SET NOT NULL;

ALTER TABLE chat_messages
    ADD CONSTRAINT chat_messages_thread_seq_unique UNIQUE (thread_id, sequence_number);

ALTER TABLE plan_configurations
    ADD CONSTRAINT plan_configurations_thread_tenant_fk
    FOREIGN KEY (thread_id, tenant_id)
    REFERENCES threads(id, tenant_id);

ALTER TABLE chat_messages
    ADD CONSTRAINT chat_messages_thread_tenant_fk
    FOREIGN KEY (thread_id, tenant_id)
    REFERENCES threads(id, tenant_id);

CREATE INDEX idx_threads_tenant_updated ON threads(tenant_id, updated_at DESC);
CREATE INDEX idx_threads_tenant_status ON threads(tenant_id, status, updated_at DESC);
CREATE INDEX idx_threads_active_plan_configuration ON threads(tenant_id, active_plan_configuration_id);
CREATE INDEX idx_plan_configurations_thread ON plan_configurations(tenant_id, thread_id);
CREATE INDEX idx_chat_messages_thread_seq ON chat_messages (thread_id, sequence_number);
CREATE INDEX idx_chat_messages_thread_created ON chat_messages (thread_id, created_at);

ALTER TABLE threads ENABLE ROW LEVEL SECURITY;
ALTER TABLE threads FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_threads ON threads
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());

COMMENT ON TABLE threads IS
    'Top-level chat workspace. Path B uses threads.id as chat_messages.thread_id.';
COMMENT ON COLUMN plan_configurations.thread_id IS
    'Owning Thread for chat-first workspaces.';
COMMENT ON COLUMN chat_messages.thread_id IS
    'Foreign key to threads.id.';
