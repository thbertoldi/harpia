import { beforeEach, describe, expect, it, vi, type Mock } from "vitest";

vi.mock("$lib/rpc", () => ({
  planClient: {
    listReviewRequests: vi.fn(),
    getReviewRequest: vi.fn(),
    respondToReviewRequest: vi.fn(),
  },
}));

import { planClient } from "$lib/rpc";
import {
  ReviewDecisionKind,
  ReviewRequestStatus,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import {
  isReviewActionDisabled,
  listReviews,
  respondToReview,
} from "./reviews";

const listMock = planClient.listReviewRequests as unknown as Mock;
const respondMock = planClient.respondToReviewRequest as unknown as Mock;

beforeEach(() => vi.clearAllMocks());

describe("reviews", () => {
  it("lists review requests scoped to an execution", async () => {
    listMock.mockResolvedValue({ reviewRequests: [{ id: "review-1" }] });
    await expect(listReviews("tenant-1", "execution-1")).resolves.toHaveLength(
      1,
    );
    expect(listMock).toHaveBeenCalledWith({
      tenantId: "tenant-1",
      planExecutionId: "execution-1",
      pageSize: 100,
      pageToken: "",
    });
  });

  it("sends revision feedback as a revise decision", async () => {
    respondMock.mockResolvedValue({ reviewRequest: { id: "review-1" } });
    await respondToReview(
      "tenant-1",
      "review-1",
      "revise",
      "Make the hook clearer",
    );
    expect(respondMock).toHaveBeenCalledWith(
      expect.objectContaining({
        reviewRequestId: "review-1",
        decision: expect.objectContaining({
          kind: ReviewDecisionKind.REVISE,
          feedback: "Make the hook clearer",
        }),
      }),
    );
  });

  it("allows actions only while pending", () => {
    expect(isReviewActionDisabled(ReviewRequestStatus.PENDING)).toBe(false);
    expect(isReviewActionDisabled(ReviewRequestStatus.ACCEPTED)).toBe(true);
  });
});
