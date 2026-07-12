/**
 * Audit log data adapter.
 *
 * Thin adapter over the generated `harpia.audit.v1.AuditService.ListAuditEvents`
 * RPC. It maps proto events/filters to the canonical UI model, resolves the
 * selected tenant, sends every filter to the server, and preserves the
 * server's keyset page order. The browser never filters, sorts, or paginates
 * locally — export follows the server's `next_page_token` chain so every
 * matching event appears exactly once.
 */
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { requireTenantId } from "$lib/auth";
import { auditClient } from "$lib/rpc";
import {
  ActorKind,
  AuditEventType,
  AuditSubjectType,
  BoundedContext,
  FeedbackDecision,
  type AuditActor as ProtoAuditActor,
  type AuditEvent as ProtoAuditEvent,
  type AuditSubject as ProtoAuditSubject,
  type PayloadDiffEntry as ProtoPayloadDiffEntry,
} from "$lib/gen/harpia/audit/v1/audit_pb";

// Re-export the proto enums so pages/tests compare against stable values
// without reaching into the generated package directly.
export { ActorKind, AuditEventType, AuditSubjectType, BoundedContext };

// ---------------------------------------------------------------------------
// Canonical UI model (plan-centric taxonomy — no legacy task vocabulary)
// ---------------------------------------------------------------------------

export type AuditActorKind = "human" | "agent" | "workflow_engine";

export type AuditFeedbackDecision =
  | "approve"
  | "reject"
  | "modify"
  | "escalate";

export type AuditActor = {
  kind: AuditActorKind;
  id: string;
  displayName: string;
  /** Present when actor.kind === "agent". */
  agentType?: string;
};

export type AuditSubject = {
  /** Proto `AuditSubjectType` enum; absent subjects (auth events) are omitted. */
  type: AuditSubjectType;
  id: string;
};

export type PayloadDiffEntry = {
  field: string;
  before: string | null;
  after: string | null;
};

export type AuditEvent = {
  /** Stable event identifier (UUID) used as the keyset tiebreaker. */
  eventId: string;
  tenantId: string;
  eventType: AuditEventType;
  boundedContext: BoundedContext;
  actor: AuditActor;
  /** Optional typed subject; absent for authentication events. */
  subject?: AuditSubject;
  payloadDiff: PayloadDiffEntry[];
  /** ISO-8601, derived from the proto `occurred_at` Timestamp. */
  timestamp: string;
  traceId?: string;
  decision?: AuditFeedbackDecision;
};

export type AuditEventFilters = {
  /** Repeated proto event types; sent verbatim to the server. */
  eventTypes?: AuditEventType[];
  /** Human actor id; becomes `actor.actor_id` with kind HUMAN. */
  actorId?: string;
  /** Agent actor type; becomes `actor.agent_type` with kind AGENT. */
  agentType?: string;
  /** Typed related-subject id; becomes `subject_id`. */
  subjectId?: string;
  decision?: AuditFeedbackDecision;
  /** Inclusive ISO-8601 date bounds, sent verbatim. */
  dateFrom?: string;
  dateTo?: string;
};

export type AuditPageToken = string | null;

export type AuditEventsPage = {
  events: AuditEvent[];
  nextPageToken: string | null;
};

// ---------------------------------------------------------------------------
// Proto → UI mapping
// ---------------------------------------------------------------------------

const ACTOR_KIND_FROM_PROTO: Record<number, AuditActorKind> = {
  [ActorKind.HUMAN]: "human",
  [ActorKind.AGENT]: "agent",
  [ActorKind.WORKFLOW_ENGINE]: "workflow_engine",
};

const ACTOR_KIND_NAME: Record<AuditActorKind, string> = {
  human: "HUMAN",
  agent: "AGENT",
  workflow_engine: "WORKFLOW_ENGINE",
};

const DECISION_FROM_PROTO: Record<number, AuditFeedbackDecision> = {
  [FeedbackDecision.APPROVE]: "approve",
  [FeedbackDecision.REJECT]: "reject",
  [FeedbackDecision.MODIFY]: "modify",
  [FeedbackDecision.ESCALATE]: "escalate",
};

const DECISION_TO_PROTO: Record<AuditFeedbackDecision, FeedbackDecision> = {
  approve: FeedbackDecision.APPROVE,
  reject: FeedbackDecision.REJECT,
  modify: FeedbackDecision.MODIFY,
  escalate: FeedbackDecision.ESCALATE,
};

function mapActor(actor: ProtoAuditActor | undefined): AuditActor {
  const protoKind = actor?.kind ?? ActorKind.UNSPECIFIED;
  return {
    kind: ACTOR_KIND_FROM_PROTO[protoKind] ?? "human",
    id: actor?.actorId ?? "",
    displayName: actor?.displayName ?? "",
    agentType: actor?.agentType,
  };
}

function mapDiff(entries: ProtoPayloadDiffEntry[]): PayloadDiffEntry[] {
  return entries.map((entry) => ({
    field: entry.field,
    before: entry.before ?? null,
    after: entry.after ?? null,
  }));
}

function mapSubject(
  subject: ProtoAuditSubject | undefined,
): AuditSubject | undefined {
  if (!subject || subject.subjectType === AuditSubjectType.UNSPECIFIED) {
    return undefined;
  }
  return { type: subject.subjectType, id: subject.subjectId };
}

function mapAuditEvent(event: ProtoAuditEvent): AuditEvent {
  const decision = event.decision
    ? DECISION_FROM_PROTO[event.decision]
    : undefined;
  return {
    eventId: event.eventId,
    tenantId: event.tenantId,
    eventType: event.eventType,
    boundedContext: event.boundedContext,
    actor: mapActor(event.actor),
    subject: mapSubject(event.subject),
    payloadDiff: mapDiff(event.payloadDiff),
    timestamp: event.occurredAt
      ? timestampDate(event.occurredAt).toISOString()
      : "",
    traceId: event.traceId,
    decision,
  };
}

// ---------------------------------------------------------------------------
// UI filters → proto request
// ---------------------------------------------------------------------------

/**
 * Builds the single server-side actor filter. The proto contract narrows to
 * one actor, so when both an agent type and a human actor id are supplied the
 * agent type wins (it is the more specific select). Human-only and
 * agent-only cases map to their respective actor kinds.
 */
function buildActorFilter(
  filters: AuditEventFilters,
): { kind: ActorKind; actorId: string; agentType?: string } | undefined {
  if (filters.agentType) {
    return {
      kind: ActorKind.AGENT,
      actorId: "",
      agentType: filters.agentType,
    };
  }
  if (filters.actorId) {
    return { kind: ActorKind.HUMAN, actorId: filters.actorId };
  }
  return undefined;
}

type ListRequest = {
  tenantId: string;
  eventTypes: AuditEventType[];
  actor: ReturnType<typeof buildActorFilter>;
  dateFrom?: string;
  dateTo?: string;
  subjectId?: string;
  decision?: FeedbackDecision;
  pageSize: number;
  pageToken?: string;
};

function toListRequest(
  tenantId: string,
  filters: AuditEventFilters,
  pageToken: AuditPageToken,
  pageSize: number,
): ListRequest {
  return {
    tenantId,
    eventTypes: filters.eventTypes ?? [],
    actor: buildActorFilter(filters),
    dateFrom: filters.dateFrom,
    dateTo: filters.dateTo,
    subjectId: filters.subjectId,
    decision: filters.decision
      ? DECISION_TO_PROTO[filters.decision]
      : undefined,
    pageSize,
    pageToken: pageToken ?? undefined,
  };
}

// ---------------------------------------------------------------------------
// Data access — server-authoritative paging, no client filtering/sorting
// ---------------------------------------------------------------------------

const DEFAULT_PAGE_SIZE = 10;
/** Page size used while walking the full filtered set for CSV/JSON export. */
const EXPORT_PAGE_SIZE = 100;

/**
 * Loads one filtered, newest-first keyset page from the server. The returned
 * `nextPageToken` is the opaque server cursor; pass it back as `pageToken`
 * for the next page. No client-side filtering or pagination is performed.
 */
export async function getAuditEvents(
  filters: AuditEventFilters = {},
  pageToken: AuditPageToken = null,
  pageSize = DEFAULT_PAGE_SIZE,
): Promise<AuditEventsPage> {
  const tenantId = requireTenantId();
  const response = await auditClient.listAuditEvents(
    toListRequest(tenantId, filters, pageToken, pageSize),
  );
  return {
    events: response.events.map(mapAuditEvent),
    nextPageToken: response.nextPageToken ?? null,
  };
}

/**
 * Walks every server page for the active filters so CSV/JSON exports contain
 * the complete matching set. Each returned event appears exactly once because
 * the loop terminates only when the server stops returning a next token, and
 * the server guarantees non-overlapping keyset pages.
 */
export async function getAllFilteredAuditEvents(
  filters: AuditEventFilters = {},
): Promise<AuditEvent[]> {
  const tenantId = requireTenantId();
  const all: AuditEvent[] = [];
  let pageToken: AuditPageToken = null;
  do {
    const response = await auditClient.listAuditEvents(
      toListRequest(tenantId, filters, pageToken, EXPORT_PAGE_SIZE),
    );
    for (const event of response.events) {
      all.push(mapAuditEvent(event));
    }
    pageToken = response.nextPageToken ?? null;
  } while (pageToken);
  return all;
}

// ---------------------------------------------------------------------------
// Display helpers — stable proto-enum names used to build translate() keys
// ---------------------------------------------------------------------------

export function auditEventTypeName(type: AuditEventType): string {
  return AuditEventType[type] ?? "UNSPECIFIED";
}

export function auditBoundedContextName(context: BoundedContext): string {
  return BoundedContext[context] ?? "UNSPECIFIED";
}

export function auditSubjectTypeName(type: AuditSubjectType): string {
  return AuditSubjectType[type] ?? "UNSPECIFIED";
}

export function auditActorKindName(kind: AuditActorKind): string {
  return ACTOR_KIND_NAME[kind];
}

// ---------------------------------------------------------------------------
// Exports — CSV / JSON
// ---------------------------------------------------------------------------

export function auditEventsToCsv(events: AuditEvent[]): string {
  const headers = [
    "event_id",
    "timestamp",
    "event_type",
    "bounded_context",
    "actor_kind",
    "actor_id",
    "actor_name",
    "agent_type",
    "subject_type",
    "subject_id",
    "decision",
    "trace_id",
    "payload_diff",
  ];

  const rows = events.map((e) =>
    [
      e.eventId,
      e.timestamp,
      auditEventTypeName(e.eventType),
      auditBoundedContextName(e.boundedContext),
      auditActorKindName(e.actor.kind),
      e.actor.id,
      e.actor.displayName,
      e.actor.agentType ?? "",
      e.subject ? auditSubjectTypeName(e.subject.type) : "",
      e.subject?.id ?? "",
      e.decision ?? "",
      e.traceId ?? "",
      JSON.stringify(e.payloadDiff),
    ]
      .map(csvEscape)
      .join(","),
  );

  return [headers.join(","), ...rows].join("\n");
}

function csvEscape(value: string): string {
  if (value.includes(",") || value.includes('"') || value.includes("\n")) {
    return `"${value.replace(/"/g, '""')}"`;
  }
  return value;
}

export function auditEventsToJson(events: AuditEvent[]): string {
  return JSON.stringify(
    events.map((e) => ({
      event_id: e.eventId,
      tenant_id: e.tenantId,
      timestamp: e.timestamp,
      event_type: auditEventTypeName(e.eventType),
      bounded_context: auditBoundedContextName(e.boundedContext),
      actor: {
        kind: auditActorKindName(e.actor.kind),
        id: e.actor.id,
        display_name: e.actor.displayName,
        agent_type: e.actor.agentType ?? null,
      },
      subject: e.subject
        ? {
            type: auditSubjectTypeName(e.subject.type),
            id: e.subject.id,
          }
        : null,
      payload_diff: e.payloadDiff,
      trace_id: e.traceId ?? null,
      decision: e.decision ?? null,
    })),
    null,
    2,
  );
}

export function downloadTextFile(
  content: string,
  filename: string,
  mimeType: string,
): void {
  const blob = new Blob([content], { type: mimeType });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  anchor.click();
  URL.revokeObjectURL(url);
}

// ---------------------------------------------------------------------------
// Filter option lists (UI dropdowns)
// ---------------------------------------------------------------------------

export const FEEDBACK_DECISIONS: AuditFeedbackDecision[] = [
  "approve",
  "reject",
  "modify",
  "escalate",
];

export const AGENT_TYPES = [
  "planner",
  "supervisor",
  "worker",
  "deep_research",
  "workflow",
] as const;

/** Every audited event type except UNSPECIFIED, for the event-type filter. */
export const AUDIT_EVENT_TYPES: AuditEventType[] = [
  AuditEventType.PLAN_CONFIGURATION_CREATED,
  AuditEventType.PLAN_CONFIGURATION_UPDATED,
  AuditEventType.PLAN_CONFIGURATION_STATUS_CHANGED,
  AuditEventType.PLAN_EXECUTION_CREATED,
  AuditEventType.PLAN_EXECUTION_STARTED,
  AuditEventType.PLAN_EXECUTION_COMPLETED,
  AuditEventType.PLAN_EXECUTION_FAILED,
  AuditEventType.STEP_EXECUTION_STARTED,
  AuditEventType.STEP_EXECUTION_COMPLETED,
  AuditEventType.STEP_EXECUTION_FAILED,
  AuditEventType.STEP_EXECUTION_SKIPPED,
  AuditEventType.APPROVAL_CREATED,
  AuditEventType.APPROVAL_DECIDED,
  AuditEventType.WORKFLOW_SIGNAL_RECEIVED,
  AuditEventType.AUTHENTICATION_DEV_AUTH,
  AuditEventType.EXECUTOR_INSTALLATION_CREATED,
  AuditEventType.EXECUTOR_INSTALLATION_UPDATED,
  AuditEventType.EXECUTOR_INSTALLATION_DELETED,
];
