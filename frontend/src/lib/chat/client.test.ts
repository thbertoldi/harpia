import { describe, expect, it, vi, beforeEach } from "vitest";
import type { ChatMessage } from "./types";

const threadClientMock = vi.hoisted(() => ({
  threadClient: {
    appendThreadMessage: vi.fn(),
    listThreadMessages: vi.fn(),
  },
}));
vi.mock("$lib/rpc", () => threadClientMock);

beforeEach(() => {
  threadClientMock.threadClient.appendThreadMessage.mockReset();
  threadClientMock.threadClient.listThreadMessages.mockReset();
});

describe("appendThreadMessage", () => {
  it("calls threadClient.appendThreadMessage with the proto enum values and returns the persisted message", async () => {
    threadClientMock.threadClient.appendThreadMessage.mockResolvedValueOnce({
      message: {
        id: "msg-1",
        tenantId: "tenant-1",
        threadId: "thread-1",
        role: 1, // OVERSEER
        kind: 1, // USER_TEXT
        text: "remember to update news source",
        payloadJson: "{}",
        sequenceNumber: 42n,
      },
    });

    const { appendThreadMessage } = await import("./client");
    const got = await appendThreadMessage(
      "tenant-1",
      "thread-1",
      "OVERSEER",
      "USER_TEXT",
      "remember to update news source",
    );

    expect(got.id).toBe("msg-1");
    expect(got.text).toBe("remember to update news source");
    expect(got.role).toBe("OVERSEER");
    expect(got.kind).toBe("USER_TEXT");
    expect(
      threadClientMock.threadClient.appendThreadMessage,
    ).toHaveBeenCalledTimes(1);
    expect(
      threadClientMock.threadClient.appendThreadMessage,
    ).toHaveBeenCalledWith({
      tenantId: "tenant-1",
      threadId: "thread-1",
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
    threadClientMock.threadClient.listThreadMessages.mockResolvedValueOnce({
      messages: [
        {
          id: "msg-1",
          tenantId: "tenant-1",
          threadId: "thread-1",
          role: 3, // SYSTEM
          kind: 3, // CONFIGURATION_SAVED
          text: "Configuration saved.",
          payloadJson: "{}",
          sequenceNumber: 1n,
        },
      ],
      nextPageToken: "",
    });
    const { loadThreadMessages } = await import("./client");
    const got = await loadThreadMessages("tenant-1", "thread-1");
    expect(got).toHaveLength(1);
    expect(got[0].kind).toBe("CONFIGURATION_SAVED");
    expect(got[0].role).toBe("SYSTEM");
  });

  it("follows nextPageToken and concatenates pages in ascending sequence order", async () => {
    // Page 1: messages 1..2, token points at seq 2.
    // Page 2: messages 3..4, token points at seq 4.
    // Page 3: message 5, no token (done).
    // Mirrors the backend paging scheme (nextPageToken = last seq; next page
    // returns strictly-greater seqs in ascending order).
    threadClientMock.threadClient.listThreadMessages
      .mockResolvedValueOnce({
        messages: [
          {
            id: "m1",
            tenantId: "tenant-1",
            threadId: "thread-1",
            role: 3,
            kind: 1,
            text: "",
            payloadJson: "{}",
            sequenceNumber: 1n,
          },
          {
            id: "m2",
            tenantId: "tenant-1",
            threadId: "thread-1",
            role: 3,
            kind: 1,
            text: "",
            payloadJson: "{}",
            sequenceNumber: 2n,
          },
        ],
        nextPageToken: "2",
      })
      .mockResolvedValueOnce({
        messages: [
          {
            id: "m3",
            tenantId: "tenant-1",
            threadId: "thread-1",
            role: 3,
            kind: 1,
            text: "",
            payloadJson: "{}",
            sequenceNumber: 3n,
          },
          {
            id: "m4",
            tenantId: "tenant-1",
            threadId: "thread-1",
            role: 3,
            kind: 1,
            text: "",
            payloadJson: "{}",
            sequenceNumber: 4n,
          },
        ],
        nextPageToken: "4",
      })
      .mockResolvedValueOnce({
        messages: [
          {
            id: "m5",
            tenantId: "tenant-1",
            threadId: "thread-1",
            role: 3,
            kind: 1,
            text: "",
            payloadJson: "{}",
            sequenceNumber: 5n,
          },
        ],
        nextPageToken: "",
      });

    const { loadThreadMessages } = await import("./client");
    const got = await loadThreadMessages("tenant-1", "thread-1");

    // Global ascending order preserved by concatenation alone.
    expect(got.map((m) => m.id)).toEqual(["m1", "m2", "m3", "m4", "m5"]);
    expect(got.map((m) => m.sequenceNumber)).toEqual([1n, 2n, 3n, 4n, 5n]);
    expect(
      threadClientMock.threadClient.listThreadMessages,
    ).toHaveBeenCalledTimes(3);
    // Each follow-up request carries the previous page's token.
    expect(
      threadClientMock.threadClient.listThreadMessages,
    ).toHaveBeenNthCalledWith(2, expect.objectContaining({ pageToken: "2" }));
    expect(
      threadClientMock.threadClient.listThreadMessages,
    ).toHaveBeenNthCalledWith(3, expect.objectContaining({ pageToken: "4" }));
  });

  it("returns [] for an empty thread (no messages, no next token)", async () => {
    threadClientMock.threadClient.listThreadMessages.mockResolvedValueOnce({
      messages: [],
      nextPageToken: "",
    });
    const { loadThreadMessages } = await import("./client");
    const got = await loadThreadMessages("tenant-1", "thread-1");
    expect(got).toEqual([]);
    expect(
      threadClientMock.threadClient.listThreadMessages,
    ).toHaveBeenCalledTimes(1);
  });
});

describe("mergeThreadMessages", () => {
  // Build a minimal ChatMessage: the helper only reads id + sequenceNumber.
  function mk(id: string, seq: number | bigint): ChatMessage {
    return {
      id,
      tenantId: "",
      threadId: "",
      executionId: "",
      role: "SYSTEM",
      kind: "USER_TEXT",
      text: "",
      payloadJson: "{}",
      authorUserId: "",
      sequenceNumber: BigInt(seq),
      createdAt: "",
    } as unknown as ChatMessage;
  }

  it("appends incoming messages in ascending sequenceNumber order", async () => {
    const { mergeThreadMessages } = await import("./client");
    const existing = [mk("a", 1n), mk("b", 2n)];
    const incoming = [mk("c", 3n), mk("d", 4n)];
    const merged = mergeThreadMessages(existing, incoming);
    expect(merged.map((m) => m.id)).toEqual(["a", "b", "c", "d"]);
    expect(merged.map((m) => m.sequenceNumber)).toEqual([1n, 2n, 3n, 4n]);
  });

  it("sorts by bigint sequenceNumber when inputs arrive out of order", async () => {
    const { mergeThreadMessages } = await import("./client");
    // Incoming arrives newest-first (e.g. a replay); the merge must reorder.
    const existing = [mk("c", 3n)];
    const incoming = [mk("d", 4n), mk("a", 1n), mk("b", 2n)];
    const merged = mergeThreadMessages(existing, incoming);
    expect(merged.map((m) => m.id)).toEqual(["a", "b", "c", "d"]);
  });

  it("dedupes messages with the same id across existing and incoming", async () => {
    const { mergeThreadMessages } = await import("./client");
    const existing = [mk("a", 1n), mk("b", 2n)];
    const incoming = [mk("b", 2n), mk("c", 3n)];
    const merged = mergeThreadMessages(existing, incoming);
    expect(merged.map((m) => m.id)).toEqual(["a", "b", "c"]);
  });

  it("dedupes messages within the existing list itself", async () => {
    const { mergeThreadMessages } = await import("./client");
    const existing = [mk("a", 1n), mk("a", 1n)];
    const merged = mergeThreadMessages(existing, []);
    expect(merged.map((m) => m.id)).toEqual(["a"]);
  });

  it("handles bigint sequenceNumber values that exceed Number.MAX_SAFE_INTEGER", async () => {
    const { mergeThreadMessages } = await import("./client");
    const huge = 2n ** 60n; // larger than Number.MAX_SAFE_INTEGER
    const existing = [mk("a", huge + 1n)];
    const incoming = [mk("b", huge)];
    const merged = mergeThreadMessages(existing, incoming);
    // b (huge) must sort before a (huge + 1) via bigint comparison.
    expect(merged.map((m) => m.id)).toEqual(["b", "a"]);
  });

  it("returns an empty array when both inputs are empty", async () => {
    const { mergeThreadMessages } = await import("./client");
    expect(mergeThreadMessages([], [])).toEqual([]);
  });
});
