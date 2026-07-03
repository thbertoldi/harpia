-- ADR-017 Phase 2: first-class plan lifecycle kind and origin conversation.

SET row_security = off;

ALTER TABLE plan_configurations
    ADD COLUMN kind TEXT NOT NULL DEFAULT 'one_shot',
    ADD COLUMN origin_thread_id UUID;

UPDATE plan_configurations
SET origin_thread_id = thread_id
WHERE origin_thread_id IS NULL;

ALTER TABLE plan_configurations
    ALTER COLUMN origin_thread_id SET NOT NULL,
    ADD CONSTRAINT plan_configurations_kind_check CHECK (kind IN ('one_shot', 'recurring')),
    ADD CONSTRAINT plan_configurations_origin_thread_tenant_fk
        FOREIGN KEY (origin_thread_id, tenant_id)
        REFERENCES threads(id, tenant_id);

CREATE INDEX idx_plan_configurations_origin_thread
    ON plan_configurations(tenant_id, origin_thread_id);

COMMENT ON COLUMN plan_configurations.kind IS
    'ADR-017 lifecycle kind: one_shot plans run on demand; recurring plans are schedule-managed.';
COMMENT ON COLUMN plan_configurations.origin_thread_id IS
    'ADR-017 immutable conversation that spawned this plan configuration.';
