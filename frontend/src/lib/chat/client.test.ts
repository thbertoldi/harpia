import { describe, expect, it, vi, beforeEach } from "vitest";

const planClientMock = vi.hoisted(() => ({
  planClient: {
    appendPlanThreadMessage: vi.fn(),
    listPlanThreadMessages: vi.fn(),
  },
}));
vi.mock("$lib/rpc", () => planClientMock);

beforeEach(() => {
  planClientMock.planClient.appendPlanThreadMessage.mockReset();
  planClientMock.planClient.listPlanThreadMessages.mockReset();
});

describe("appendThreadMessage", () => {
  it("calls planClient.appendPlanThreadMessage with the proto enum values and returns the persisted message", async () => {
    planClientMock.planClient.appendPlanThreadMessage.mockResolvedValueOnce({
      message: {
        id: "msg-1",
        tenantId: "tenant-1",
        threadId: "config-1",
        role: 1,  // OVERSEER
        kind: 1,  // USER_TEXT
        text: "remember to update news source",
        payloadJson: "{}",
        sequenceNumber: 42n,
      },
    });

    const { appendThreadMessage } = await import("./client");
    const got = await appendThreadMessage(
      "tenant-1",
      "config-1",
      "OVERSEER",
      "USER_TEXT",
      "remember to update news source",
    );

    expect(got.id).toBe("msg-1");
    expect(got.text).toBe("remember to update news source");
    expect(got.role).toBe("OVERSEER");
    expect(got.kind).toBe("USER_TEXT");
    expect(
      planClientMock.planClient.appendPlanThreadMessage,
    ).toHaveBeenCalledTimes(1);
    expect(
      planClientMock.planClient.appendPlanThreadMessage,
    ).toHaveBeenCalledWith({
      tenantId: "tenant-1",
      planConfigurationId: "config-1",
      role: 1,
      kind: 1,
      text: "remember to update news source",
      payloadJson: "{}",
      executionId: "",
    });
  });
});

describe("loadThreadMessages", () => {
  it("returns the message array from the RPC response", async () => {
    planClientMock.planClient.listPlanThreadMessages.mockResolvedValueOnce({
      messages: [
        {
          id: "msg-1",
          tenantId: "tenant-1",
          threadId: "config-1",
          role: 3,  // SYSTEM
          kind: 3,  // CONFIGURATION_SAVED
          text: "Configuration saved.",
          payloadJson: "{}",
          sequenceNumber: 1n,
        },
      ],
    });
    const { loadThreadMessages } = await import("./client");
    const got = await loadThreadMessages("tenant-1", "config-1");
    expect(got).toHaveLength(1);
    expect(got[0].kind).toBe("CONFIGURATION_SAVED");
    expect(got[0].role).toBe("SYSTEM");
  });
});
