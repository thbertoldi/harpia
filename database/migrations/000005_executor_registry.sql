-- Executor commercial packaging registry (ADR-012 section 8)

CREATE TABLE executor_skus (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    key TEXT UNIQUE NOT NULL,
    display_name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    kind TEXT NOT NULL CHECK (kind IN ('integration', 'agent')),
    price_cents BIGINT NOT NULL DEFAULT 0,
    currency TEXT NOT NULL DEFAULT 'USD',
    compatibility JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE executor_entitlements (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    executor_sku_id UUID NOT NULL REFERENCES executor_skus(id),
    granted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    granted_by UUID REFERENCES users(id),
    UNIQUE (tenant_id, executor_sku_id)
);

CREATE TABLE executor_installations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    executor_sku_id UUID NOT NULL REFERENCES executor_skus(id),
    kind TEXT NOT NULL CHECK (kind IN ('integration', 'agent')),
    display_name TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    connection_status TEXT CHECK (
        connection_status IS NULL OR connection_status IN (
            'disconnected', 'connecting', 'connected', 'error'
        )
    ),
    config_json JSONB NOT NULL DEFAULT '{}',
    manifest_id TEXT,
    manifest_version TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (
        (kind = 'integration' AND connection_status IS NOT NULL)
        OR (kind = 'agent' AND manifest_id IS NOT NULL AND manifest_version IS NOT NULL)
    )
);

CREATE INDEX idx_executor_skus_kind ON executor_skus(kind);
CREATE INDEX idx_executor_entitlements_tenant ON executor_entitlements(tenant_id);
CREATE INDEX idx_executor_entitlements_sku ON executor_entitlements(executor_sku_id);
CREATE INDEX idx_executor_installations_tenant ON executor_installations(tenant_id);
CREATE INDEX idx_executor_installations_sku ON executor_installations(executor_sku_id);
CREATE INDEX idx_executor_installations_tenant_kind ON executor_installations(tenant_id, kind);

ALTER TABLE executor_entitlements ENABLE ROW LEVEL SECURITY;
ALTER TABLE executor_installations ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_executor_entitlements ON executor_entitlements
    FOR ALL USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());

CREATE POLICY tenant_isolation_executor_installations ON executor_installations
    FOR ALL USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());
