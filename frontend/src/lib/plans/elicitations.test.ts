import { beforeEach, describe, expect, it, vi, type Mock } from "vitest";

vi.mock("$lib/rpc", () => ({
  planClient: {
    listElicitations: vi.fn(),
    getElicitation: vi.fn(),
    respondToElicitation: vi.fn(),
    watchElicitations: vi.fn(),
  },
}));

import { planClient } from "$lib/rpc";
import { ElicitationStatus } from "$lib/gen/harpia/plans/v1/plans_pb";
import {
  buildPayloadJson,
  formatCountdown,
  isExpired,
  isFormDisabled,
  isTerminalStatus,
  loadInboxElicitations,
  parseSchemaFields,
  respondToElicitation,
  timeRemainingMs,
} from "./elicitations";

const listMock = planClient.listElicitations as unknown as Mock;
const respondMock = planClient.respondToElicitation as unknown as Mock;

beforeEach(() => {
  vi.clearAllMocks();
});

describe("inbox loading", () => {
  it("returns pending items addressed to the current overseer", async () => {
    listMock.mockResolvedValue({
      elicitations: [
        { id: "e1", status: ElicitationStatus.PENDING },
        { id: "e2", status: ElicitationStatus.PENDING },
      ],
    });

    const items = await loadInboxElicitations("tenant-1");

    expect(items).toHaveLength(2);
    expect(listMock).toHaveBeenCalledWith(
      expect.objectContaining({
        tenantId: "tenant-1",
        addressedToMe: true,
        status: ElicitationStatus.PENDING,
      }),
    );
  });
});

describe("response submission", () => {
  it("forwards payload and text to the client and returns the updated elicitation", async () => {
    respondMock.mockResolvedValue({
      elicitation: { id: "e1", status: ElicitationStatus.ANSWERED },
    });

    const result = await respondToElicitation("tenant-1", "e1", {
      payloadJson: '{"answer":"yes"}',
      responseText: "looks good",
    });

    expect(respondMock).toHaveBeenCalledWith({
      tenantId: "tenant-1",
      elicitationId: "e1",
      payloadJson: '{"answer":"yes"}',
      responseText: "looks good",
    });
    expect(result.status).toBe(ElicitationStatus.ANSWERED);
  });

  it("throws when the server omits the elicitation", async () => {
    respondMock.mockResolvedValue({});
    await expect(
      respondToElicitation("tenant-1", "e1", { responseText: "hi" }),
    ).rejects.toThrow();
  });
});

describe("terminal status gating", () => {
  it("disables the form for any non-pending status", () => {
    expect(isFormDisabled(ElicitationStatus.PENDING)).toBe(false);
    expect(isFormDisabled(ElicitationStatus.ANSWERED)).toBe(true);
    expect(isFormDisabled(ElicitationStatus.TIMED_OUT)).toBe(true);
    expect(isFormDisabled(ElicitationStatus.CANCELLED)).toBe(true);
  });

  it("treats answered/timed-out/cancelled as terminal", () => {
    expect(isTerminalStatus(ElicitationStatus.PENDING)).toBe(false);
    expect(isTerminalStatus(ElicitationStatus.ANSWERED)).toBe(true);
    expect(isTerminalStatus(ElicitationStatus.TIMED_OUT)).toBe(true);
  });
});

describe("schema-driven form parsing", () => {
  it("returns an empty list when there is no schema", () => {
    expect(parseSchemaFields("")).toEqual([]);
    expect(parseSchemaFields("{}")).toEqual([]);
    expect(parseSchemaFields("not json")).toEqual([]);
  });

  it("maps basic property types to form fields", () => {
    const schema = JSON.stringify({
      properties: {
        tone: { type: "string", enum: ["formal", "casual"], title: "Tone" },
        count: { type: "integer" },
        notes: { type: "string", maxLength: 500 },
        name: { type: "string" },
      },
      required: ["tone"],
    });

    const fields = parseSchemaFields(schema);

    expect(fields).toHaveLength(4);
    const byName = Object.fromEntries(fields.map((f) => [f.name, f]));
    expect(byName.tone).toMatchObject({
      type: "select",
      label: "Tone",
      required: true,
      options: ["formal", "casual"],
    });
    expect(byName.count.type).toBe("number");
    expect(byName.notes.type).toBe("textarea");
    expect(byName.name).toMatchObject({ type: "string", required: false });
  });
});

describe("payload building", () => {
  it("drops empty values and serializes to JSON", () => {
    expect(buildPayloadJson({ a: "1", b: "", c: "x" })).toBe(
      '{"a":"1","c":"x"}',
    );
  });
});

describe("timeout helpers", () => {
  const now = Date.parse("2026-01-01T00:00:00Z");

  it("computes remaining time and expiry", () => {
    const future = "2026-01-01T02:30:00Z";
    expect(timeRemainingMs(future, now)).toBe(9_000_000);
    expect(isExpired(future, now)).toBe(false);
    expect(isExpired("2025-12-31T23:00:00Z", now)).toBe(true);
  });

  it("returns null countdown when there is no deadline", () => {
    expect(timeRemainingMs("", now)).toBeNull();
    expect(formatCountdown("", now)).toBe("");
  });

  it("formats hours and minutes", () => {
    expect(formatCountdown("2026-01-01T02:30:00Z", now)).toBe("2h 30m");
    expect(formatCountdown("2026-01-01T00:08:00Z", now)).toBe("8m");
    expect(formatCountdown("2026-01-01T00:00:30Z", now)).toBe("<1m");
    expect(formatCountdown("2025-12-31T23:00:00Z", now)).toBe("0m");
  });
});
