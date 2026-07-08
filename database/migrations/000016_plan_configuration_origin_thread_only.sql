ALTER TABLE plan_configurations
    DROP CONSTRAINT IF EXISTS plan_configurations_thread_tenant_fk,
    DROP CONSTRAINT IF EXISTS plan_configurations_id_thread_tenant_unique,
    DROP COLUMN IF EXISTS thread_id;

COMMENT ON COLUMN plan_configurations.origin_thread_id IS
    'Canonical owning thread for this plan configuration; runtime and interaction events append here.';
