-- M3 (plan thread): per-PlanConfiguration chat messages.
-- See docs/superpowers/specs/2026-06-21-harpia-ux-m3-plan-thread-design.md §2.1.
--
-- Schema is generic (thread_id is a TEXT key) so a future ChatService can
-- reuse the table for non-Plan chat surfaces. In M3 thread_id always equals
-- a plan_configuration_id (UUID stringified).

CREATE TABLE chat_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    thread_id TEXT NOT NULL,
    execution_id UUID,
    role TEXT NOT NULL,
    kind TEXT NOT NULL,
    text TEXT NOT NULL DEFAULT '',
    payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    author_user_id UUID,
    sequence_number BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chat_messages_thread_seq_unique UNIQUE (thread_id, sequence_number)
);

CREATE INDEX idx_chat_messages_thread_seq ON chat_messages (thread_id, sequence_number);
CREATE INDEX idx_chat_messages_thread_created ON chat_messages (thread_id, created_at);
CREATE INDEX idx_chat_messages_tenant ON chat_messages (tenant_id);

ALTER TABLE chat_messages ENABLE ROW LEVEL SECURITY;
ALTER TABLE chat_messages FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_chat_messages ON chat_messages
    FOR ALL
    USING (tenant_id = current_tenant_id())
    WITH CHECK (tenant_id = current_tenant_id());

COMMENT ON TABLE chat_messages IS
    'Per-thread durable chat records. M3 uses plan_configuration_id as thread_id.';
COMMENT ON COLUMN chat_messages.execution_id IS
    'Nullable. NULL for plan-scope messages (CONFIGURATION_SAVED, USER_TEXT). Set for execution-scope events.';
COMMENT ON COLUMN chat_messages.sequence_number IS
    'Per-thread monotonic; used by WatchPlanThreadMessages for resume.';
