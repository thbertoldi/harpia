import { feedbackClient } from "$lib/rpc";
import type { FeedbackRequest } from "$lib/gen/harpia/feedback/v1/feedback_pb";

export async function loadPendingFeedback(
  tenantId: string,
): Promise<FeedbackRequest[]> {
  const out: FeedbackRequest[] = [];
  for await (const response of feedbackClient.listPendingFeedback({
    tenantId,
    pageSize: 50,
    pageToken: "",
  })) {
    out.push(...response.feedbackRequests);
  }
  return out;
}

export async function countPendingFeedback(tenantId: string): Promise<number> {
  const feedback = await loadPendingFeedback(tenantId);
  return feedback.length;
}
