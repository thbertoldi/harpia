-- Immutable derivative artifacts are content-addressed per tenant and type.
-- This makes retrying a deterministic carousel PDF build return its original
-- ArtifactRef instead of adding a duplicate document artifact.
CREATE UNIQUE INDEX artifacts_tenant_type_content_hash_unique
    ON artifacts (tenant_id, artifact_type_id, content_hash);
