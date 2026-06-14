import { describe, expect, it } from "vitest";
import {
  MOCK_AUDIT_EVENTS,
  type AuditEvent,
} from "$lib/mocks/audit-events";
import {
  compareAuditEvents,
  decodePageToken,
  encodePageToken,
  filterAuditEvents,
  getAuditEvents,
  paginateAuditEvents,
  sortAuditEvents,
} from "$lib/audit/audit-store";

describe("filterAuditEvents", () => {
  it("filters by task id substring", () => {
    const result = filterAuditEvents(MOCK_AUDIT_EVENTS, {
      taskId: "a1b2c3d4",
    });
    expect(result.length).toBeGreaterThan(0);
    expect(result.every((e) => e.taskId.includes("a1b2c3d4"))).toBe(true);
  });

  it("filters by human user id", () => {
    const result = filterAuditEvents(MOCK_AUDIT_EVENTS, {
      userId: "dev-overseer",
    });
    expect(result.length).toBeGreaterThan(0);
    expect(
      result.every(
        (e) => e.actor.kind === "human" && e.actor.id === "dev-overseer",
      ),
    ).toBe(true);
  });

  it("filters by agent type", () => {
    const result = filterAuditEvents(MOCK_AUDIT_EVENTS, {
      agentType: "planner",
    });
    expect(result.length).toBeGreaterThan(0);
    expect(
      result.every(
        (e) => e.actor.kind === "agent" && e.actor.agentType === "planner",
      ),
    ).toBe(true);
  });

  it("filters by decision", () => {
    const result = filterAuditEvents(MOCK_AUDIT_EVENTS, {
      decision: "approve",
    });
    expect(result).toHaveLength(1);
    expect(result[0].decision).toBe("approve");
  });

  it("filters by date range", () => {
    const sample = MOCK_AUDIT_EVENTS[0];
    const day = sample.timestamp.slice(0, 10);
    const result = filterAuditEvents(MOCK_AUDIT_EVENTS, {
      dateFrom: day,
      dateTo: day,
    });
    expect(result.length).toBeGreaterThan(0);
    expect(
      result.every((e) => e.timestamp.slice(0, 10) === day),
    ).toBe(true);
  });

  it("combines multiple filters", () => {
    const result = filterAuditEvents(MOCK_AUDIT_EVENTS, {
      taskId: "a1b2c3d4",
      decision: "approve",
    });
    expect(result).toHaveLength(1);
    expect(result[0].eventId).toBe("evt-005-feedback-approved");
  });

  it("returns empty when no match", () => {
    const result = filterAuditEvents(MOCK_AUDIT_EVENTS, {
      taskId: "nonexistent-task",
    });
    expect(result).toHaveLength(0);
  });
});

describe("sortAuditEvents", () => {
  it("orders by timestamp desc then eventId desc", () => {
    const sorted = sortAuditEvents(MOCK_AUDIT_EVENTS);
    for (let i = 1; i < sorted.length; i++) {
      expect(compareAuditEvents(sorted[i - 1], sorted[i])).toBeLessThanOrEqual(
        0,
      );
    }
  });
});

describe("paginateAuditEvents", () => {
  const sorted = sortAuditEvents(MOCK_AUDIT_EVENTS);

  it("returns first page with next token", () => {
    const page = paginateAuditEvents(sorted, null, 5);
    expect(page.events).toHaveLength(5);
    expect(page.nextPageToken).not.toBeNull();
  });

  it("returns subsequent page using keyset token", () => {
    const first = paginateAuditEvents(sorted, null, 5);
    const second = paginateAuditEvents(sorted, first.nextPageToken, 5);
    expect(second.events).toHaveLength(5);
    expect(second.events[0].eventId).not.toBe(first.events[0].eventId);

    const firstIds = new Set(first.events.map((e) => e.eventId));
    for (const event of second.events) {
      expect(firstIds.has(event.eventId)).toBe(false);
    }
  });

  it("returns null next token on last page", () => {
    const all = paginateAuditEvents(sorted, null, MOCK_AUDIT_EVENTS.length);
    expect(all.nextPageToken).toBeNull();
    expect(all.events).toHaveLength(MOCK_AUDIT_EVENTS.length);
  });

  it("encodes and decodes page tokens", () => {
    const event = sorted[3];
    const token = encodePageToken({
      ts: event.timestamp,
      eventId: event.eventId,
    });
    const decoded = decodePageToken(token);
    expect(decoded).toEqual({
      ts: event.timestamp,
      eventId: event.eventId,
    });
  });
});

describe("getAuditEvents", () => {
  it("applies filters and pagination together", async () => {
    const page = await getAuditEvents({ taskId: "a1b2c3d4" }, null, 3);
    expect(page.events.length).toBeLessThanOrEqual(3);
    expect(page.events.every((e) => e.taskId.includes("a1b2c3d4"))).toBe(
      true,
    );

    if (page.nextPageToken) {
      const next = await getAuditEvents(
        { taskId: "a1b2c3d4" },
        page.nextPageToken,
        3,
      );
      const firstIds = new Set(page.events.map((e: AuditEvent) => e.eventId));
      for (const event of next.events) {
        expect(firstIds.has(event.eventId)).toBe(false);
      }
    }
  });
});
