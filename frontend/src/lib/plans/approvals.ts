import { planClient } from "$lib/rpc";
import {
  ApprovalRequestStatus,
  type ApprovalRequest,
} from "$lib/gen/harpia/plans/v1/plans_pb";

export { ApprovalRequestStatus };
export type { ApprovalRequest };

export type WatchApprovalRequestsOptions = {
  stepExecutionId?: string;
  planExecutionId?: string;
};

export function isTerminalApprovalStatus(
  status: ApprovalRequestStatus,
): boolean {
  return (
    status === ApprovalRequestStatus.APPROVED ||
    status === ApprovalRequestStatus.REJECTED
  );
}

export function isApprovalActionDisabled(
  status: ApprovalRequestStatus,
): boolean {
  return status !== ApprovalRequestStatus.PENDING;
}

export function approvalStatusLabelKey(status: ApprovalRequestStatus): string {
  switch (status) {
    case ApprovalRequestStatus.PENDING:
      return "approvals.status.pending";
    case ApprovalRequestStatus.APPROVED:
      return "approvals.status.approved";
    case ApprovalRequestStatus.REJECTED:
      return "approvals.status.rejected";
    default:
      return "approvals.status.unknown";
  }
}

export async function loadInboxApprovals(
  tenantId: string,
): Promise<ApprovalRequest[]> {
  const response = await planClient.listApprovalRequests({
    tenantId,
    status: ApprovalRequestStatus.PENDING,
    pageSize: 100,
    pageToken: "",
  });
  return response.approvalRequests;
}

export async function countPendingApprovals(tenantId: string): Promise<number> {
  const approvals = await loadInboxApprovals(tenantId);
  return approvals.length;
}

export async function loadApproval(
  tenantId: string,
  approvalRequestId: string,
): Promise<ApprovalRequest> {
  const response = await planClient.getApprovalRequest({
    tenantId,
    approvalRequestId,
  });
  if (!response.approvalRequest) {
    throw new Error("Approval request not found");
  }
  return response.approvalRequest;
}

export async function respondToApprovalRequest(
  tenantId: string,
  approvalRequestId: string,
  approved: boolean,
  reason = "",
): Promise<ApprovalRequest> {
  const response = await planClient.respondToApprovalRequest({
    tenantId,
    approvalRequestId,
    approved,
    reason,
  });
  if (!response.approvalRequest) {
    throw new Error("Missing approval request in response");
  }
  return response.approvalRequest;
}

export async function* watchApprovalRequests(
  tenantId: string,
  options: WatchApprovalRequestsOptions = {},
): AsyncIterable<ApprovalRequest[]> {
  for await (const event of planClient.watchApprovalRequests({
    tenantId,
    stepExecutionId: options.stepExecutionId,
    planExecutionId: options.planExecutionId,
  })) {
    yield event.approvalRequests;
  }
}
