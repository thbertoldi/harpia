ALTER TABLE llm_budget_reservations
    RENAME COLUMN subtask_id TO step_id;

ALTER TABLE llm_usage_events
    RENAME COLUMN subtask_id TO step_id;

DROP INDEX IF EXISTS idx_llm_usage_events_task;
CREATE INDEX idx_llm_usage_events_task
    ON llm_usage_events(tenant_id, task_id, step_id, ts DESC);
