-- Template inputs and artifact versions.

ALTER TABLE plan_templates
    ADD COLUMN input_parameters JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE plan_configurations
    ADD COLUMN parameter_values JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE artifacts
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN current_version_id UUID,
    ADD COLUMN plan_configuration_id UUID REFERENCES plan_configurations(id),
    ADD COLUMN plan_execution_id UUID REFERENCES plan_executions(id),
    ADD COLUMN step_execution_id UUID REFERENCES step_executions(id),
    ADD COLUMN status TEXT NOT NULL DEFAULT 'ARTIFACT_STATUS_GENERATED',
    ADD CONSTRAINT artifacts_status_check CHECK (status IN (
        'ARTIFACT_STATUS_UNSPECIFIED',
        'ARTIFACT_STATUS_GENERATED',
        'ARTIFACT_STATUS_EDITED',
        'ARTIFACT_STATUS_APPROVED',
        'ARTIFACT_STATUS_REJECTED',
        'ARTIFACT_STATUS_SUPERSEDED',
        'ARTIFACT_STATUS_FAILED'
    ));

CREATE TABLE artifact_versions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    artifact_id UUID NOT NULL REFERENCES artifacts(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    version_number INTEGER NOT NULL,
    storage_uri TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    source_plan_execution_id UUID REFERENCES plan_executions(id),
    source_step_execution_id UUID REFERENCES step_executions(id),
    source_version_id UUID REFERENCES artifact_versions(id),
    created_by_user_id UUID,
    created_by_kind TEXT NOT NULL DEFAULT 'ARTIFACT_VERSION_CREATED_BY_KIND_SYSTEM',
    edit_summary TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT artifact_versions_created_by_kind_check CHECK (created_by_kind IN (
        'ARTIFACT_VERSION_CREATED_BY_KIND_UNSPECIFIED',
        'ARTIFACT_VERSION_CREATED_BY_KIND_SYSTEM',
        'ARTIFACT_VERSION_CREATED_BY_KIND_AGENT',
        'ARTIFACT_VERSION_CREATED_BY_KIND_INTEGRATION',
        'ARTIFACT_VERSION_CREATED_BY_KIND_USER'
    )),
    UNIQUE (artifact_id, version_number)
);

INSERT INTO artifact_versions (
    artifact_id,
    tenant_id,
    version_number,
    storage_uri,
    content_hash,
    source_plan_execution_id,
    source_step_execution_id,
    created_by_kind,
    edit_summary,
    created_at
)
SELECT
    id,
    tenant_id,
    1,
    storage_uri,
    content_hash,
    plan_execution_id,
    step_execution_id,
    'ARTIFACT_VERSION_CREATED_BY_KIND_SYSTEM',
    'Initial artifact payload',
    created_at
FROM artifacts;

UPDATE artifacts a
SET current_version_id = v.id,
    updated_at = a.created_at
FROM artifact_versions v
WHERE v.artifact_id = a.id
  AND v.version_number = 1;

ALTER TABLE artifacts
    ADD CONSTRAINT artifacts_current_version_fk
    FOREIGN KEY (current_version_id)
    REFERENCES artifact_versions(id);

CREATE INDEX idx_artifacts_tenant_updated ON artifacts(tenant_id, updated_at DESC);
CREATE INDEX idx_artifacts_plan_configuration ON artifacts(tenant_id, plan_configuration_id, updated_at DESC);
CREATE INDEX idx_artifacts_plan_execution ON artifacts(tenant_id, plan_execution_id, updated_at DESC);
CREATE INDEX idx_artifacts_step_execution ON artifacts(tenant_id, step_execution_id);
CREATE INDEX idx_artifacts_status ON artifacts(tenant_id, status);

CREATE INDEX idx_artifact_versions_artifact ON artifact_versions(artifact_id, version_number DESC);
CREATE INDEX idx_artifact_versions_tenant ON artifact_versions(tenant_id, created_at DESC);

ALTER TABLE artifact_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE artifact_versions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_artifact_versions ON artifact_versions
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());

UPDATE plan_templates
SET input_parameters = jsonb_build_array(
    jsonb_build_object(
        'key', 'theme',
        'label', 'Theme',
        'description', 'Main subject for the content.',
        'type', 'TEMPLATE_INPUT_PARAMETER_TYPE_TEXT',
        'required', true,
        'defaultValueJson', '"sports"',
        'runtimeMappings', jsonb_build_array(
            jsonb_build_object(
                'target', 'TEMPLATE_INPUT_RUNTIME_TARGET_SEED_ARTIFACT',
                'stepKey', 'write-draft',
                'inputName', 'harpia.internal.ContentPreferences',
                'jsonPath', '$.topic'
            )
        )
    ),
    jsonb_build_object(
        'key', 'language',
        'label', 'Language',
        'description', 'Language for generated drafts.',
        'type', 'TEMPLATE_INPUT_PARAMETER_TYPE_LANGUAGE',
        'required', true,
        'defaultValueJson', '"pt-BR"',
        'optionsJson', '[{"value":"pt-BR","label":"Portuguese"},{"value":"en-US","label":"English"},{"value":"es","label":"Spanish"}]',
        'runtimeMappings', jsonb_build_array(
            jsonb_build_object(
                'target', 'TEMPLATE_INPUT_RUNTIME_TARGET_SEED_ARTIFACT',
                'stepKey', 'write-draft',
                'inputName', 'harpia.internal.ContentPreferences',
                'jsonPath', '$.language'
            )
        )
    ),
    jsonb_build_object(
        'key', 'tone',
        'label', 'Tone',
        'description', 'Voice used by the writer.',
        'type', 'TEMPLATE_INPUT_PARAMETER_TYPE_SELECT',
        'required', true,
        'defaultValueJson', '"analytical, concise, and practical"',
        'optionsJson', '[{"value":"analytical, concise, and practical","label":"Analytical"},{"value":"friendly and clear","label":"Friendly"},{"value":"executive and direct","label":"Executive"}]',
        'runtimeMappings', jsonb_build_array(
            jsonb_build_object(
                'target', 'TEMPLATE_INPUT_RUNTIME_TARGET_SEED_ARTIFACT',
                'stepKey', 'write-draft',
                'inputName', 'harpia.internal.ContentPreferences',
                'jsonPath', '$.tone'
            )
        )
    ),
    jsonb_build_object(
        'key', 'audience',
        'label', 'Audience',
        'description', 'Who the post should speak to.',
        'type', 'TEMPLATE_INPUT_PARAMETER_TYPE_TEXT',
        'required', false,
        'defaultValueJson', '""',
        'runtimeMappings', jsonb_build_array(
            jsonb_build_object(
                'target', 'TEMPLATE_INPUT_RUNTIME_TARGET_SEED_ARTIFACT',
                'stepKey', 'write-draft',
                'inputName', 'harpia.internal.ContentPreferences',
                'jsonPath', '$.audience'
            )
        )
    ),
    jsonb_build_object(
        'key', 'topics_to_avoid',
        'label', 'Topics to avoid',
        'description', 'Comma-separated topics the writer should avoid.',
        'type', 'TEMPLATE_INPUT_PARAMETER_TYPE_TEXTAREA',
        'required', false,
        'defaultValueJson', '""',
        'runtimeMappings', jsonb_build_array(
            jsonb_build_object(
                'target', 'TEMPLATE_INPUT_RUNTIME_TARGET_SEED_ARTIFACT',
                'stepKey', 'write-draft',
                'inputName', 'harpia.internal.ContentPreferences',
                'jsonPath', '$.topics_to_avoid'
            )
        )
    ),
    jsonb_build_object(
        'key', 'source_group',
        'label', 'Source group',
        'description', 'RSS feed group to use for source articles.',
        'type', 'TEMPLATE_INPUT_PARAMETER_TYPE_INTEGRATION_SELECTOR',
        'required', true,
        'defaultValueJson', '""',
        'runtimeMappings', jsonb_build_array(
            jsonb_build_object(
                'target', 'TEMPLATE_INPUT_RUNTIME_TARGET_SLOT_BINDING',
                'stepKey', 'fetch-news'
            )
        )
    ),
    jsonb_build_object(
        'key', 'date_range',
        'label', 'Date range',
        'description', 'Article publication window.',
        'type', 'TEMPLATE_INPUT_PARAMETER_TYPE_DATE_RANGE',
        'required', true,
        'defaultValueJson', '{"preset":"last_7_days"}',
        'runtimeMappings', jsonb_build_array(
            jsonb_build_object(
                'target', 'TEMPLATE_INPUT_RUNTIME_TARGET_SEED_ARTIFACT',
                'stepKey', 'fetch-news',
                'inputName', 'date_range'
            )
        )
    ),
    jsonb_build_object(
        'key', 'approval_mode',
        'label', 'Approval mode',
        'description', 'Whether publishing requires approval.',
        'type', 'TEMPLATE_INPUT_PARAMETER_TYPE_SELECT',
        'required', true,
        'defaultValueJson', '"require_approval"',
        'optionsJson', '[{"value":"require_approval","label":"Require approval"},{"value":"auto_publish","label":"Auto publish"}]',
        'runtimeMappings', jsonb_build_array(
            jsonb_build_object(
                'target', 'TEMPLATE_INPUT_RUNTIME_TARGET_BEHAVIOR_POLICY',
                'policyKey', 'publish_approval_mode'
            )
        )
    )
)
WHERE key = 'weekly-newsletter-linkedin';
