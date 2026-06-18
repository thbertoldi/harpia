import { beforeEach, describe, expect, it, vi, type Mock } from "vitest";

vi.mock("$lib/rpc", () => ({
  planClient: {
    listApprovalRequests: vi.fn(),
    getApprovalRequest: vi.fn(),
    respondToApprovalRequest: vi.fn(),
    watchApprovalRequests: vi.fn(),
  },
}));

import { planClient } from "$lib/rpc";
import { ApprovalRequestStatus } from "$lib/gen/harpia/plans/v1/plans_pb";
import {
  isApprovalActionDisabled,
  isTerminalApprovalStatus,
  loadInboxApprovals,
  respondToApprovalRequest,
} from "./approvals";

const listMock = planClient.listApprovalRequests as unknown as Mock;
const respondMock = planClient.respondToApprovalRequest as unknown as Mock;

beforeEach(() => {
  vi.clearAllMocks();
});

describe("approval inbox", () => {
  it("loads pending approval requests", async () => {
    listMock.mockResolvedValue({
      approvalRequests: [
        { id: "a1", status: ApprovalRequestStatus.PENDING },
      ],
    });
    const items = await loadInboxApprovals("tenant-1");
    expect(items).toHaveLength(1);
    expect(listMock).toHaveBeenCalledWith({
      tenantId: "tenant-1",
      status: ApprovalRequestStatus.PENDING,
      pageSize: 100,
      pageToken: "",
    });
  });
});

describe("approval helpers", () => {
  it("marks pending as actionable", () => {
    expect(isApprovalActionDisabled(ApprovalRequestStatus.PENDING)).toBe(false);
    expect(isTerminalApprovalStatus(ApprovalRequestStatus.APPROVED)).toBe(true);
  });
});

describe("respondToApprovalRequest", () => {
  it("returns updated approval from API", async () => {
    respondMock.mockResolvedValue({
      approvalRequest: {
        id: "a1",
        status: ApprovalRequestStatus.REJECTED,
      },
    });
    const updated = await respondToApprovalRequest(
      "tenant-1",
      "a1",
      false,
      "tone mismatch",
    );
    expect(updated.status).toBe(ApprovalRequestStatus.REJECTED);
    expect(respondMock).toHaveBeenCalledWith({
      tenantId: "tenant-1",
      approvalRequestId: "a1",
      approved: false,
      reason: "tone mismatch",
    });
  });
});
