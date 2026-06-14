/**
 * Typed mock audit events aligned with ADR-006 Human Interaction
 * event-sourced domain model. Swap audit-store.ts to real backend later.
 */

/** Bounded contexts from ADR-006 §2 */
export type BoundedContext =
  | "task_management"
  | "agent_orchestration"
  | "human_interaction"
  | "identity_tenants"
  | "workflow_engine";

export type AuditActorKind = "human" | "agent";

export type AuditActor = {
  kind: AuditActorKind;
  id: string;
  displayName: string;
  /** Present when actor.kind === "agent" */
  agentType?: string;
};

/** Human feedback decisions per ADR-006 ubiquitous language */
export type FeedbackDecision = "approve" | "reject" | "modify" | "escalate";

export type PayloadDiffEntry = {
  field: string;
  before: string | null;
  after: string | null;
};

/** Domain event types spanning core bounded contexts */
export type AuditEventType =
  | "task.created"
  | "task.status_changed"
  | "task.completed"
  | "feedback.requested"
  | "feedback.decision_submitted"
  | "agent.execution_started"
  | "agent.execution_completed"
  | "agent.execution_failed"
  | "workflow.signal_received";

export type AuditEvent = {
  /** Stable event identifier for keyset pagination */
  eventId: string;
  tenantId: string;
  eventType: AuditEventType;
  actor: AuditActor;
  boundedContext: BoundedContext;
  taskId: string;
  payloadDiff: PayloadDiffEntry[];
  /** ISO-8601 timestamp */
  timestamp: string;
  /** OpenTelemetry trace id (ADR OTEL #20) */
  traceId: string;
  /** Set on feedback.decision_submitted events */
  decision?: FeedbackDecision;
};

export type AuditEventFilters = {
  taskId?: string;
  userId?: string;
  agentType?: string;
  decision?: FeedbackDecision;
  dateFrom?: string;
  dateTo?: string;
};

export type AuditEventsPage = {
  events: AuditEvent[];
  nextPageToken: string | null;
};

/** Keyset cursor encoded as base64 JSON { ts, eventId } */
export type AuditPageToken = string | null;

const TENANT = "tenant-aiuna";

function iso(daysAgo: number, hours = 12, minutes = 0): string {
  const d = new Date();
  d.setUTCDate(d.getUTCDate() - daysAgo);
  d.setUTCHours(hours, minutes, 0, 0);
  return d.toISOString();
}

/** Mock trail: task submit → planner → overseer approval → worker execution */
export const MOCK_AUDIT_EVENTS: AuditEvent[] = [
  {
    eventId: "evt-001-task-created",
    tenantId: TENANT,
    eventType: "task.created",
    actor: {
      kind: "human",
      id: "dev-leader",
      displayName: "Lena Leader",
    },
    boundedContext: "task_management",
    taskId: "task-a1b2c3d4",
    payloadDiff: [
      { field: "title", before: null, after: "Deploy Q2 pricing update" },
      { field: "status", before: null, after: "pending" },
    ],
    timestamp: iso(5, 9, 15),
    traceId: "4bf92f3577b34da6a3ce929d0e0e4736",
  },
  {
    eventId: "evt-002-planning-started",
    tenantId: TENANT,
    eventType: "agent.execution_started",
    actor: {
      kind: "agent",
      id: "agent-inst-planner-01",
      displayName: "Planner",
      agentType: "planner",
    },
    boundedContext: "agent_orchestration",
    taskId: "task-a1b2c3d4",
    payloadDiff: [
      { field: "agent_type", before: null, after: "planner" },
      { field: "phase", before: null, after: "decomposition" },
    ],
    timestamp: iso(5, 9, 18),
    traceId: "4bf92f3577b34da6a3ce929d0e0e4736",
  },
  {
    eventId: "evt-003-task-decomposed",
    tenantId: TENANT,
    eventType: "task.status_changed",
    actor: {
      kind: "agent",
      id: "agent-inst-planner-01",
      displayName: "Planner",
      agentType: "planner",
    },
    boundedContext: "task_management",
    taskId: "task-a1b2c3d4",
    payloadDiff: [
      { field: "status", before: "pending", after: "planning" },
      { field: "subtask_count", before: "0", after: "3" },
    ],
    timestamp: iso(5, 9, 22),
    traceId: "4bf92f3577b34da6a3ce929d0e0e4736",
  },
  {
    eventId: "evt-004-feedback-requested",
    tenantId: TENANT,
    eventType: "feedback.requested",
    actor: {
      kind: "agent",
      id: "agent-inst-supervisor-01",
      displayName: "Supervisor",
      agentType: "supervisor",
    },
    boundedContext: "human_interaction",
    taskId: "task-a1b2c3d4",
    payloadDiff: [
      {
        field: "question",
        before: null,
        after: "Approve deployment plan for staging?",
      },
      { field: "channel", before: null, after: "ui" },
    ],
    timestamp: iso(5, 9, 25),
    traceId: "00f067aa0ba902b7c91958a0a0a0a0a0",
  },
  {
    eventId: "evt-005-feedback-approved",
    tenantId: TENANT,
    eventType: "feedback.decision_submitted",
    actor: {
      kind: "human",
      id: "dev-overseer",
      displayName: "Owen Overseer",
    },
    boundedContext: "human_interaction",
    taskId: "task-a1b2c3d4",
    decision: "approve",
    payloadDiff: [
      { field: "decision", before: "pending", after: "approve" },
      { field: "comment", before: null, after: "Looks good for staging." },
    ],
    timestamp: iso(5, 10, 5),
    traceId: "00f067aa0ba902b7c91958a0a0a0a0a0",
  },
  {
    eventId: "evt-006-worker-started",
    tenantId: TENANT,
    eventType: "agent.execution_started",
    actor: {
      kind: "agent",
      id: "agent-inst-worker-01",
      displayName: "Worker",
      agentType: "worker",
    },
    boundedContext: "agent_orchestration",
    taskId: "task-a1b2c3d4",
    payloadDiff: [
      { field: "agent_type", before: null, after: "worker" },
      { field: "subtask_id", before: null, after: "subtask-001" },
    ],
    timestamp: iso(5, 10, 8),
    traceId: "00f067aa0ba902b7c91958a0a0a0a0a0",
  },
  {
    eventId: "evt-007-worker-completed",
    tenantId: TENANT,
    eventType: "agent.execution_completed",
    actor: {
      kind: "agent",
      id: "agent-inst-worker-01",
      displayName: "Worker",
      agentType: "worker",
    },
    boundedContext: "agent_orchestration",
    taskId: "task-a1b2c3d4",
    payloadDiff: [
      { field: "subtask_id", before: "subtask-001", after: "subtask-001" },
      { field: "status", before: "running", after: "completed" },
    ],
    timestamp: iso(5, 10, 45),
    traceId: "00f067aa0ba902b7c91958a0a0a0a0a0",
  },
  {
    eventId: "evt-008-task-completed",
    tenantId: TENANT,
    eventType: "task.completed",
    actor: {
      kind: "agent",
      id: "agent-inst-supervisor-01",
      displayName: "Supervisor",
      agentType: "supervisor",
    },
    boundedContext: "task_management",
    taskId: "task-a1b2c3d4",
    payloadDiff: [
      { field: "status", before: "in_progress", after: "completed" },
    ],
    timestamp: iso(5, 11, 0),
    traceId: "00f067aa0ba902b7c91958a0a0a0a0a0",
  },
  {
    eventId: "evt-009-task-created-b",
    tenantId: TENANT,
    eventType: "task.created",
    actor: {
      kind: "human",
      id: "dev-leader",
      displayName: "Lena Leader",
    },
    boundedContext: "task_management",
    taskId: "task-e5f6g7h8",
    payloadDiff: [
      { field: "title", before: null, after: "Investigate latency spike" },
      { field: "status", before: null, after: "pending" },
    ],
    timestamp: iso(3, 14, 0),
    traceId: "a1b2c3d4e5f6789012345678901234ab",
  },
  {
    eventId: "evt-010-feedback-rejected",
    tenantId: TENANT,
    eventType: "feedback.decision_submitted",
    actor: {
      kind: "human",
      id: "dev-overseer",
      displayName: "Owen Overseer",
    },
    boundedContext: "human_interaction",
    taskId: "task-e5f6g7h8",
    decision: "reject",
    payloadDiff: [
      { field: "decision", before: "pending", after: "reject" },
      {
        field: "comment",
        before: null,
        after: "Root cause analysis incomplete.",
      },
    ],
    timestamp: iso(3, 15, 30),
    traceId: "a1b2c3d4e5f6789012345678901234ab",
  },
  {
    eventId: "evt-011-research-started",
    tenantId: TENANT,
    eventType: "agent.execution_started",
    actor: {
      kind: "agent",
      id: "agent-inst-research-01",
      displayName: "Deep Research",
      agentType: "deep_research",
    },
    boundedContext: "agent_orchestration",
    taskId: "task-e5f6g7h8",
    payloadDiff: [
      { field: "agent_type", before: null, after: "deep_research" },
    ],
    timestamp: iso(2, 8, 0),
    traceId: "b2c3d4e5f6789012345678901234abcd",
  },
  {
    eventId: "evt-012-feedback-modified",
    tenantId: TENANT,
    eventType: "feedback.decision_submitted",
    actor: {
      kind: "human",
      id: "dev-engineer",
      displayName: "Eli Engineer",
    },
    boundedContext: "human_interaction",
    taskId: "task-i9j0k1l2",
    decision: "modify",
    payloadDiff: [
      { field: "decision", before: "pending", after: "modify" },
      {
        field: "comment",
        before: null,
        after: "Add canary deployment step before prod.",
      },
    ],
    timestamp: iso(1, 16, 45),
    traceId: "c3d4e5f6789012345678901234abcdef",
  },
  {
    eventId: "evt-013-feedback-escalated",
    tenantId: TENANT,
    eventType: "feedback.decision_submitted",
    actor: {
      kind: "human",
      id: "dev-overseer",
      displayName: "Owen Overseer",
    },
    boundedContext: "human_interaction",
    taskId: "task-m3n4o5p6",
    decision: "escalate",
    payloadDiff: [
      { field: "decision", before: "pending", after: "escalate" },
      {
        field: "comment",
        before: null,
        after: "Requires security team review.",
      },
    ],
    timestamp: iso(0, 11, 20),
    traceId: "d4e5f6789012345678901234abcdef01",
  },
  {
    eventId: "evt-014-workflow-signal",
    tenantId: TENANT,
    eventType: "workflow.signal_received",
    actor: {
      kind: "agent",
      id: "temporal-worker-01",
      displayName: "Workflow Engine",
      agentType: "workflow",
    },
    boundedContext: "workflow_engine",
    taskId: "task-m3n4o5p6",
    payloadDiff: [
      { field: "signal", before: null, after: "FeedbackReceived" },
      { field: "workflow_id", before: null, after: "wf-m3n4o5p6" },
    ],
    timestamp: iso(0, 11, 21),
    traceId: "d4e5f6789012345678901234abcdef01",
  },
  {
    eventId: "evt-015-execution-failed",
    tenantId: TENANT,
    eventType: "agent.execution_failed",
    actor: {
      kind: "agent",
      id: "agent-inst-worker-02",
      displayName: "Worker",
      agentType: "worker",
    },
    boundedContext: "agent_orchestration",
    taskId: "task-q7r8s9t0",
    payloadDiff: [
      { field: "status", before: "running", after: "failed" },
      { field: "error", before: null, after: "Tool timeout: kubectl apply" },
    ],
    timestamp: iso(0, 9, 0),
    traceId: "e5f6789012345678901234abcdef0123",
  },
];
