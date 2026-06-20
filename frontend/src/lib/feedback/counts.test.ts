import { describe, expect, it, vi, beforeEach } from "vitest";

const feedbackClientMock = vi.hoisted(() => ({
  feedbackClient: {
    listPendingFeedback: vi.fn(),
  },
}));
vi.mock("$lib/rpc", () => feedbackClientMock);

beforeEach(() => {
  feedbackClientMock.feedbackClient.listPendingFeedback.mockReset();
});

describe("loadPendingFeedback", () => {
  it("drains all yielded pages and returns the combined feedbackRequests array", async () => {
    feedbackClientMock.feedbackClient.listPendingFeedback.mockImplementation(
      async function* () {
        yield {
          feedbackRequests: [{ id: "a" }, { id: "b" }],
          nextPageToken: "tok2",
        };
        yield { feedbackRequests: [{ id: "c" }], nextPageToken: "" };
      },
    );

    const { loadPendingFeedback } = await import("./counts");
    const got = await loadPendingFeedback("tenant-1");

    expect(got.map((f) => f.id)).toEqual(["a", "b", "c"]);
    expect(
      feedbackClientMock.feedbackClient.listPendingFeedback,
    ).toHaveBeenCalledTimes(1);
    expect(
      feedbackClientMock.feedbackClient.listPendingFeedback,
    ).toHaveBeenCalledWith({
      tenantId: "tenant-1",
      pageSize: 50,
      pageToken: "",
    });
  });

  it("returns an empty array when the stream yields no items", async () => {
    feedbackClientMock.feedbackClient.listPendingFeedback.mockImplementation(
      async function* () {
        // no yields
      },
    );
    const { loadPendingFeedback } = await import("./counts");
    expect(await loadPendingFeedback("tenant-1")).toEqual([]);
  });
});

describe("countPendingFeedback", () => {
  it("returns the number of pending feedback requests", async () => {
    feedbackClientMock.feedbackClient.listPendingFeedback.mockImplementation(
      async function* () {
        yield {
          feedbackRequests: [{ id: "a" }, { id: "b" }, { id: "c" }],
          nextPageToken: "",
        };
      },
    );
    const { countPendingFeedback } = await import("./counts");
    expect(await countPendingFeedback("tenant-1")).toBe(3);
  });
});
