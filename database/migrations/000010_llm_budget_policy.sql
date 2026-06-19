-- LLM cost tracking and per-tenant quotas (issue #34).
-- Budget Policy is a capability inside Agent Orchestration (ADR-010).

CREATE TABLE tenant_quotas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    -- Empty provider means tenant-wide quota across all LLM providers.
    provider TEXT NOT NULL DEFAULT '',
    period TEXT NOT NULL CHECK (period IN ('daily', 'monthly')),
    budget_usd NUMERIC(20, 8) NOT NULL CHECK (budget_usd >= 0),
    hard_cap BOOLEAN NOT NULL DEFAULT true,
    warning_threshold NUMERIC(6, 5) NOT NULL DEFAULT 0.8
        CHECK (warning_threshold > 0 AND warning_threshold <= 1),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, provider, period)
);

CREATE INDEX idx_tenant_quotas_tenant_provider
    ON tenant_quotas(tenant_id, provider);

CREATE OR REPLACE FUNCTION tenant_quotas_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tenant_quotas_updated_at
    BEFORE UPDATE ON tenant_quotas
    FOR EACH ROW EXECUTE FUNCTION tenant_quotas_set_updated_at();

CREATE TABLE tenant_usage_summary (
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    -- Empty provider means aggregate usage across all LLM providers.
    provider TEXT NOT NULL DEFAULT '',
    period TEXT NOT NULL CHECK (period IN ('daily', 'monthly')),
    period_start DATE NOT NULL,
    input_tokens BIGINT NOT NULL DEFAULT 0 CHECK (input_tokens >= 0),
    output_tokens BIGINT NOT NULL DEFAULT 0 CHECK (output_tokens >= 0),
    cost_usd NUMERIC(20, 8) NOT NULL DEFAULT 0 CHECK (cost_usd >= 0),
    event_count BIGINT NOT NULL DEFAULT 0 CHECK (event_count >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, provider, period, period_start)
);

CREATE INDEX idx_tenant_usage_summary_tenant_provider_period
    ON tenant_usage_summary(tenant_id, provider, period, period_start DESC);

CREATE TABLE llm_budget_reservations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    provider TEXT NOT NULL DEFAULT '',
    task_id UUID REFERENCES tasks(id) ON DELETE SET NULL,
    subtask_id UUID REFERENCES subtasks(id) ON DELETE SET NULL,
    plan_execution_id UUID REFERENCES plan_executions(id) ON DELETE SET NULL,
    step_execution_id UUID REFERENCES step_executions(id) ON DELETE SET NULL,
    agent_type TEXT NOT NULL DEFAULT '',
    estimated_cost_usd NUMERIC(20, 8) NOT NULL CHECK (estimated_cost_usd >= 0),
    status TEXT NOT NULL DEFAULT 'reserved'
        CHECK (status IN ('reserved', 'consumed', 'released', 'expired')),
    idempotency_key TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    released_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX idx_llm_budget_reservations_idempotency
    ON llm_budget_reservations(tenant_id, idempotency_key)
    WHERE idempotency_key <> '';
CREATE INDEX idx_llm_budget_reservations_active
    ON llm_budget_reservations(tenant_id, provider, expires_at)
    WHERE status = 'reserved';

CREATE TABLE llm_usage_event_keys (
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    idempotency_key TEXT NOT NULL,
    usage_event_id UUID NOT NULL,
    usage_event_ts TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, idempotency_key)
);

CREATE TABLE llm_usage_events (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    reservation_id UUID REFERENCES llm_budget_reservations(id) ON DELETE SET NULL,
    task_id UUID REFERENCES tasks(id) ON DELETE SET NULL,
    subtask_id UUID REFERENCES subtasks(id) ON DELETE SET NULL,
    plan_execution_id UUID REFERENCES plan_executions(id) ON DELETE SET NULL,
    step_execution_id UUID REFERENCES step_executions(id) ON DELETE SET NULL,
    agent_type TEXT NOT NULL DEFAULT '',
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    input_tokens BIGINT NOT NULL CHECK (input_tokens >= 0),
    output_tokens BIGINT NOT NULL CHECK (output_tokens >= 0),
    cost_usd NUMERIC(20, 8) NOT NULL CHECK (cost_usd >= 0),
    idempotency_key TEXT NOT NULL DEFAULT '',
    ts TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (id, ts)
) PARTITION BY RANGE (ts);

CREATE INDEX idx_llm_usage_events_tenant_ts
    ON llm_usage_events(tenant_id, ts DESC);
CREATE INDEX idx_llm_usage_events_tenant_provider_ts
    ON llm_usage_events(tenant_id, provider, ts DESC);
CREATE INDEX idx_llm_usage_events_task
    ON llm_usage_events(tenant_id, task_id, subtask_id, ts DESC);

CREATE OR REPLACE FUNCTION ensure_llm_usage_events_partition(event_ts TIMESTAMPTZ)
RETURNS VOID AS $$
DECLARE
    start_date DATE := date_trunc('month', event_ts)::DATE;
    end_date DATE := (date_trunc('month', event_ts) + interval '1 month')::DATE;
    partition_name TEXT := 'llm_usage_events_' || to_char(date_trunc('month', event_ts), 'YYYY_MM');
BEGIN
    EXECUTE format(
        'CREATE TABLE IF NOT EXISTS %I PARTITION OF llm_usage_events FOR VALUES FROM (%L) TO (%L)',
        partition_name,
        start_date,
        end_date
    );
END;
$$ LANGUAGE plpgsql;

ALTER TABLE tenant_quotas ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenant_quotas FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_tenant_quotas ON tenant_quotas
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());

ALTER TABLE tenant_usage_summary ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenant_usage_summary FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_tenant_usage_summary ON tenant_usage_summary
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());

ALTER TABLE llm_budget_reservations ENABLE ROW LEVEL SECURITY;
ALTER TABLE llm_budget_reservations FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_llm_budget_reservations ON llm_budget_reservations
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());

ALTER TABLE llm_usage_event_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE llm_usage_event_keys FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_llm_usage_event_keys ON llm_usage_event_keys
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());

ALTER TABLE llm_usage_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE llm_usage_events FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_llm_usage_events ON llm_usage_events
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());
