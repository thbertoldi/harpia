import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  ActorKind,
  AuditEventType,
  AuditSubjectType,
  BoundedContext,
  FeedbackDecision,
} from "$lib/gen/harpia/audit/v1/audit_pb";

const { listAuditEvents, requireTenantId } = vi.hoisted(() => ({
  listAuditEvents: vi.fn(),
  requireTenantId: vi.fn(() => "tenant-1"),
}));

vi.mock("$lib/rpc", () => ({
  auditClient: {
    listAuditEvents,
  },
}));

vi.mock("$lib/auth", () => ({
  requireTenantId,
}));

beforeEach(() => {
  listAuditEvents.mockReset();
  requireTenantId.mockReturnValue("tenant-1");
});

/** Minimal proto-shaped event for mapping assertions. */
function protoEvent(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    eventId: "evt-001",
    tenantId: "tenant-1",
    eventType: AuditEventType.PLAN_CONFIGURATION_CREATED,
    boundedContext: BoundedContext.PLAN_MANAGEMENT,
    actor: {
      kind: ActorKind.HUMAN,
      actorId: "dev-leader",
      displayName: "Lena Leader",
    },
    subject: {
      subjectType: AuditSubjectType.PLAN_CONFIGURATION,
      subjectId: "plan-cfg-001",
    },
    payloadDiff: [
      { field: "status", before: "DRAFT", after: "RUNNABLE" },
      { field: "title", before: undefined, after: "Deploy pricing" },
    ],
    occurredAt: { seconds: 1_717_000_000n, nanos: 0 },
    traceId: "trace-abc",
    decision: FeedbackDecision.APPROVE,
    ...overrides,
  };
}

describe("getAuditEvents", () => {
  it("maps proto events to the canonical UI model and preserves the server next token", async () => {
    listAuditEvents.mockResolvedValueOnce({
      events: [protoEvent()],
      nextPageToken: "opaque-cursor",
    });

    const { getAuditEvents } = await import("./audit-store");
    const page = await getAuditEvents({}, null, 10);

    expect(page.nextPageToken).toBe("opaque-cursor");
    expect(page.events).toHaveLength(1);
    const event = page.events[0];
    expect(event?.eventId).toBe("evt-001");
    expect(event?.eventType).toBe(AuditEventType.PLAN_CONFIGURATION_CREATED);
    expect(event?.boundedContext).toBe(BoundedContext.PLAN_MANAGEMENT);
    expect(event?.actor).toEqual({
      kind: "human",
      id: "dev-leader",
      displayName: "Lena Leader",
      agentType: undefined,
    });
    expect(event?.subject).toEqual({
      type: AuditSubjectType.PLAN_CONFIGURATION,
      id: "plan-cfg-001",
    });
    // occurredAt { seconds: 1717000000 } → ISO string for that instant.
    expect(event?.timestamp).toBe(new Date(1_717_000_000_000).toISOString());
    expect(event?.traceId).toBe("trace-abc");
    expect(event?.decision).toBe("approve");
    // proto optional `before` (undefined) maps to null.
    expect(event?.payloadDiff[1]?.before).toBeNull();
    expect(event?.payloadDiff[1]?.after).toBe("Deploy pricing");
  });

  it("sends the selected tenant id, server-side filters, and the page token", async () => {
    listAuditEvents.mockResolvedValueOnce({ events: [], nextPageToken: "" });

    const { getAuditEvents } = await import("./audit-store");
    await getAuditEvents(
      {
        eventTypes: [AuditEventType.APPROVAL_DECIDED],
        agentType: "planner",
        subjectId: "plan-cfg-9",
        decision: "reject",
        dateFrom: "2026-01-01",
        dateTo: "2026-01-31",
      },
      "cursor-2",
      25,
    );

    expect(listAuditEvents).toHaveBeenCalledTimes(1);
    expect(listAuditEvents).toHaveBeenCalledWith({
      tenantId: "tenant-1",
      eventTypes: [AuditEventType.APPROVAL_DECIDED],
      actor: { kind: ActorKind.AGENT, actorId: "", agentType: "planner" },
      dateFrom: "2026-01-01",
      dateTo: "2026-01-31",
      subjectId: "plan-cfg-9",
      decision: FeedbackDecision.REJECT,
      pageSize: 25,
      pageToken: "cursor-2",
    });
  });

  it("maps a human actor id to an HUMAN actor filter", async () => {
    listAuditEvents.mockResolvedValueOnce({ events: [], nextPageToken: "" });

    const { getAuditEvents } = await import("./audit-store");
    await getAuditEvents({ actorId: "dev-overseer" }, null, 10);

    expect(listAuditEvents).toHaveBeenCalledWith(
      expect.objectContaining({
        actor: { kind: ActorKind.HUMAN, actorId: "dev-overseer" },
      }),
    );
  });

  it("omits the actor filter when neither actor id nor agent type is set", async () => {
    listAuditEvents.mockResolvedValueOnce({ events: [], nextPageToken: "" });

    const { getAuditEvents } = await import("./audit-store");
    await getAuditEvents({}, null, 10);

    expect(listAuditEvents).toHaveBeenCalledWith(
      expect.objectContaining({ actor: undefined }),
    );
  });

  it("normalizes an absent next token to null", async () => {
    listAuditEvents.mockResolvedValueOnce({
      events: [],
      nextPageToken: undefined,
    });

    const { getAuditEvents } = await import("./audit-store");
    const page = await getAuditEvents();
    expect(page.nextPageToken).toBeNull();
  });

  it("maps agent and workflow-engine actor kinds and omits an unspecified subject", async () => {
    listAuditEvents.mockResolvedValueOnce({
      events: [
        protoEvent({
          eventId: "agent-evt",
          actor: {
            kind: ActorKind.AGENT,
            actorId: "agent-inst-1",
            displayName: "Planner",
            agentType: "planner",
          },
          subject: { subjectType: AuditSubjectType.UNSPECIFIED, subjectId: "" },
        }),
        protoEvent({
          eventId: "wf-evt",
          actor: {
            kind: ActorKind.WORKFLOW_ENGINE,
            actorId: "temporal-worker-1",
            displayName: "Workflow Engine",
          },
        }),
      ],
      nextPageToken: "",
    });

    const { getAuditEvents } = await import("./audit-store");
    const page = await getAuditEvents();
    expect(page.events[0]?.actor.kind).toBe("agent");
    expect(page.events[0]?.actor.agentType).toBe("planner");
    expect(page.events[0]?.subject).toBeUndefined();
    expect(page.events[1]?.actor.kind).toBe("workflow_engine");
  });
});

describe("getAllFilteredAuditEvents", () => {
  it("follows the server next_page_token chain and concatenates non-overlapping pages", async () => {
    listAuditEvents
      .mockResolvedValueOnce({
        events: [protoEvent({ eventId: "e1" }), protoEvent({ eventId: "e2" })],
        nextPageToken: "page-2",
      })
      .mockResolvedValueOnce({
        events: [protoEvent({ eventId: "e3" })],
        nextPageToken: "page-3",
      })
      .mockResolvedValueOnce({
        events: [protoEvent({ eventId: "e4" })],
        nextPageToken: "",
      });

    const { getAllFilteredAuditEvents } = await import("./audit-store");
    const all = await getAllFilteredAuditEvents({ subjectId: "plan-cfg-1" });

    expect(all.map((e) => e.eventId)).toEqual(["e1", "e2", "e3", "e4"]);
    expect(listAuditEvents).toHaveBeenCalledTimes(3);
    // Each follow-up carries the previous page's opaque server token.
    expect(listAuditEvents).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({ pageToken: "page-2" }),
    );
    expect(listAuditEvents).toHaveBeenNthCalledWith(
      3,
      expect.objectContaining({ pageToken: "page-3" }),
    );
  });

  it("uses the export page size and sends the active filters on every call", async () => {
    listAuditEvents.mockResolvedValueOnce({ events: [], nextPageToken: "" });

    const { getAllFilteredAuditEvents } = await import("./audit-store");
    await getAllFilteredAuditEvents({ decision: "approve" });

    expect(listAuditEvents).toHaveBeenCalledWith(
      expect.objectContaining({
        pageSize: 100,
        decision: FeedbackDecision.APPROVE,
        tenantId: "tenant-1",
      }),
    );
  });

  it("returns an empty array when the server has no matching events", async () => {
    listAuditEvents.mockResolvedValueOnce({ events: [], nextPageToken: "" });

    const { getAllFilteredAuditEvents } = await import("./audit-store");
    const all = await getAllFilteredAuditEvents();
    expect(all).toEqual([]);
    expect(listAuditEvents).toHaveBeenCalledTimes(1);
  });
});

describe("auditEventsToCsv", () => {
  it("emits the typed subject columns and stable proto-enum names", async () => {
    listAuditEvents.mockResolvedValueOnce({
      events: [protoEvent()],
      nextPageToken: "",
    });
    const { getAuditEvents, auditEventsToCsv } = await import("./audit-store");
    const { events } = await getAuditEvents();
    const csv = auditEventsToCsv(events);

    const header = csv.split("\n")[0];
    expect(header).toContain("subject_type");
    expect(header).toContain("subject_id");
    expect(header).not.toContain("task_id");

    const row = csv.split("\n")[1];
    expect(row).toContain("PLAN_CONFIGURATION_CREATED");
    expect(row).toContain("PLAN_MANAGEMENT");
    expect(row).toContain("HUMAN");
    expect(row).toContain("plan-cfg-001");
    expect(row).toContain("approve");
  });

  it("leaves subject columns empty for subject-less (auth) events", async () => {
    listAuditEvents.mockResolvedValueOnce({
      events: [
        protoEvent({
          eventId: "auth-evt",
          eventType: AuditEventType.AUTHENTICATION_DEV_AUTH,
          boundedContext: BoundedContext.IDENTITY_TENANTS,
          subject: undefined,
          decision: undefined,
          actor: {
            kind: ActorKind.HUMAN,
            actorId: "dev@harpia",
            displayName: "Dev User",
          },
        }),
      ],
      nextPageToken: "",
    });
    const { getAuditEvents, auditEventsToCsv } = await import("./audit-store");
    const { events } = await getAuditEvents();
    const csv = auditEventsToCsv(events);
    const row = csv.split("\n")[1];
    expect(row).toContain("AUTHENTICATION_DEV_AUTH");
    expect(row).toContain("dev@harpia");
  });
});

describe("auditEventsToJson", () => {
  it("serializes enum names and a null subject for auth events", async () => {
    listAuditEvents.mockResolvedValueOnce({
      events: [
        protoEvent({
          subject: undefined,
          decision: undefined,
        }),
      ],
      nextPageToken: "",
    });
    const { getAuditEvents, auditEventsToJson } = await import("./audit-store");
    const { events } = await getAuditEvents();
    const parsed = JSON.parse(auditEventsToJson(events));
    expect(parsed[0].event_type).toBe("PLAN_CONFIGURATION_CREATED");
    expect(parsed[0].bounded_context).toBe("PLAN_MANAGEMENT");
    expect(parsed[0].subject).toBeNull();
    expect(parsed[0].decision).toBeNull();
    expect(parsed[0].actor.kind).toBe("HUMAN");
  });
});

describe("display name helpers", () => {
  it("resolve proto enums to their stable name strings", async () => {
    const {
      auditEventTypeName,
      auditBoundedContextName,
      auditSubjectTypeName,
    } = await import("./audit-store");
    expect(auditEventTypeName(AuditEventType.WORKFLOW_SIGNAL_RECEIVED)).toBe(
      "WORKFLOW_SIGNAL_RECEIVED",
    );
    expect(auditBoundedContextName(BoundedContext.EXECUTOR_CATALOG)).toBe(
      "EXECUTOR_CATALOG",
    );
    expect(auditSubjectTypeName(AuditSubjectType.APPROVAL_REQUEST)).toBe(
      "APPROVAL_REQUEST",
    );
  });
});
