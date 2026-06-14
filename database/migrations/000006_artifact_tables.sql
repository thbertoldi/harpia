-- Harpia Artifact Storage (E1.2, issue #109)
-- Global artifact type registry and tenant-scoped artifact metadata.

-- ============================================================
-- Artifact Types (global registry)
-- ============================================================
CREATE TABLE artifact_types (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    key TEXT UNIQUE NOT NULL,
    schema_ref TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    description TEXT NOT NULL DEFAULT ''
);

-- ============================================================
-- Artifacts (tenant-scoped metadata; payloads in Garage)
-- ============================================================
CREATE TABLE artifacts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    artifact_type_id UUID NOT NULL REFERENCES artifact_types(id),
    storage_uri TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_artifacts_tenant ON artifacts(tenant_id);
CREATE INDEX idx_artifacts_type ON artifacts(artifact_type_id);
CREATE INDEX idx_artifacts_tenant_created ON artifacts(tenant_id, created_at DESC);

-- ============================================================
-- Row-Level Security
-- ============================================================
ALTER TABLE artifacts ENABLE ROW LEVEL SECURITY;
ALTER TABLE artifacts FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_artifacts ON artifacts
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());

-- ============================================================
-- Seed: artifact types matching proto message keys
-- ============================================================
INSERT INTO artifact_types (id, key, schema_ref, version, description)
VALUES
    (
        'b1000000-0000-4000-8000-000000000001',
        'harpia.artifacts.v1.DateRange',
        'harpia.artifacts.v1/DateRange',
        1,
        'Inclusive date range for plan seed inputs.'
    ),
    (
        'b1000000-0000-4000-8000-000000000002',
        'harpia.artifacts.v1.NewsList',
        'harpia.artifacts.v1/NewsList',
        1,
        'Curated news articles collected for a plan step.'
    ),
    (
        'b1000000-0000-4000-8000-000000000003',
        'harpia.artifacts.v1.TextDraft',
        'harpia.artifacts.v1/TextDraft',
        1,
        'Platform-neutral text draft.'
    ),
    (
        'b1000000-0000-4000-8000-000000000004',
        'harpia.artifacts.v1.LinkedInPostDraft',
        'harpia.artifacts.v1/LinkedInPostDraft',
        1,
        'LinkedIn-specific post draft.'
    ),
    (
        'b1000000-0000-4000-8000-000000000005',
        'harpia.artifacts.v1.PublishConfirmation',
        'harpia.artifacts.v1/PublishConfirmation',
        1,
        'Confirmation payload after publishing to an external platform.'
    )
ON CONFLICT (key) DO NOTHING;
