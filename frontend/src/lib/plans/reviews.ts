import { planClient } from "$lib/rpc";
import {
  ReviewDecisionKind,
  ReviewRequestStatus,
  type ReviewRequest,
} from "$lib/gen/harpia/plans/v1/plans_pb";

export { ReviewDecisionKind, ReviewRequestStatus };
export type { ReviewRequest };

export type ReviewResponse = "accept" | "revise";

export function isReviewActionDisabled(status: ReviewRequestStatus): boolean {
  return status !== ReviewRequestStatus.PENDING;
}

export async function loadReview(
  tenantId: string,
  reviewRequestId: string,
): Promise<ReviewRequest> {
  const response = await planClient.getReviewRequest({
    tenantId,
    reviewRequestId,
  });
  if (!response.reviewRequest) throw new Error("Review request not found");
  return response.reviewRequest;
}

export async function listReviews(
  tenantId: string,
  planExecutionId = "",
): Promise<ReviewRequest[]> {
  const response = await planClient.listReviewRequests({
    tenantId,
    planExecutionId: planExecutionId || undefined,
    pageSize: 100,
    pageToken: "",
  });
  return response.reviewRequests;
}

export async function respondToReview(
  tenantId: string,
  reviewRequestId: string,
  response: ReviewResponse,
  feedback = "",
): Promise<ReviewRequest> {
  const result = await planClient.respondToReviewRequest({
    tenantId,
    reviewRequestId,
    decision: {
      kind:
        response === "accept"
          ? ReviewDecisionKind.ACCEPT
          : ReviewDecisionKind.REVISE,
      feedback: response === "revise" ? feedback : "",
      decidedByUserId: "",
      decidedAt: "",
    },
  });
  if (!result.reviewRequest)
    throw new Error("Missing review request in response");
  return result.reviewRequest;
}
