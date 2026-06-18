-- Tenant LLM provider configurations (issue #33)
-- Stores per-tenant BYO provider credentials encrypted with a two-layer
-- envelope (per-row DEK + active KEK). The DB never sees plaintext keys or
-- KEK material; only ciphertext + the KEK version that wrapped the DEK.
--
-- Design reference: docs/notes/2026-06-16-tenant-llm-config-design.md (§3, §4).

CREATE TABLE tenant_llm_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    -- Identifier of the KEK version that wrapped the row's DEK at write time.
    -- Resolver picks the matching KEK by this value, so KEK rotation does not
    -- require re-encrypting historical rows.
    kek_version TEXT NOT NULL,
    -- Wrapped data-encryption key: nonce || ciphertext || tag (AES-256-GCM).
    encrypted_dek BYTEA NOT NULL,
    -- Provider API key sealed under the per-row DEK: nonce || ciphertext || tag.
    encrypted_api_key BYTEA NOT NULL,
    default_model TEXT NOT NULL DEFAULT '',
    allowed_models TEXT[] NOT NULL DEFAULT '{}',
    managed_by TEXT NOT NULL DEFAULT 'tenant_self'
        CHECK (managed_by IN ('tenant_self', 'platform', 'proxy_virtual')),
    last_rotated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, provider)
);

CREATE INDEX idx_tenant_llm_configs_tenant
    ON tenant_llm_configs(tenant_id);
CREATE INDEX idx_tenant_llm_configs_tenant_provider
    ON tenant_llm_configs(tenant_id, provider);

CREATE OR REPLACE FUNCTION tenant_llm_configs_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tenant_llm_configs_updated_at
    BEFORE UPDATE ON tenant_llm_configs
    FOR EACH ROW EXECUTE FUNCTION tenant_llm_configs_set_updated_at();

ALTER TABLE tenant_llm_configs ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenant_llm_configs FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_tenant_llm_configs ON tenant_llm_configs
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());
