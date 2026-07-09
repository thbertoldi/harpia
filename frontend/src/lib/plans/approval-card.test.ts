import { describe, expect, it } from "vitest";
import type { ChatMessage } from "$lib/chat/types";
import {
  effectiveApprovalDecision,
  parseApprovalPayloadContext,
  shortApprovalContextId,
  streamedApprovalDecision,
} from "./approval-card";

function msg(
  kind: ChatMessage["kind"],
  id: string,
  payload: Record<string, unknown>,
): ChatMessage {
  return {
    id,
    tenantId: "tenant-1",
    threadId: "thread-1",
    executionId: "exec-1",
    role: "SYSTEM",
    kind,
    text: "",
    payloadJson: JSON.stringify(payload),
    authorUserId: "",
    sequenceNumber: 1n,
    createdAt: "2026-07-08T10:00:00Z",
  };
}

describe("parseApprovalPayloadContext", () => {
  it("reads enriched approval context for thread cards", () => {
    expect(
      parseApprovalPayloadContext(
        JSON.stringify({
          approval_request_id: "approval-1",
          plan_configuration_id: "config-1",
          plan_execution_id: "exec-1",
          step_execution_id: "step-exec-1",
          plan_step_key: "publish-linkedin",
          input_artifact_id: "artifact-linkedin-draft",
        }),
      ),
    ).toEqual({
      approvalRequestId: "approval-1",
      approved: null,
      inputArtifactId: "artifact-linkedin-draft",
      planStepKey: "publish-linkedin",
      planConfigurationId: "config-1",
      planExecutionId: "exec-1",
      stepExecutionId: "step-exec-1",
    });
  });

  it("returns null when the approval request id is missing", () => {
    expect(
      parseApprovalPayloadContext(JSON.stringify({ approved: true })),
    ).toBe(null);
  });
});

describe("approval decision state", () => {
  it("syncs a pending raised card to a later streamed decision", () => {
    const raised = msg("APPROVAL_RAISED", "raised-1", {
      approval_request_id: "approval-1",
    });
    const decided = msg("APPROVAL_DECIDED", "decided-1", {
      approval_request_id: "approval-1",
      approved: false,
    });

    expect(
      streamedApprovalDecision("approval-1", raised.id, [raised, decided]),
    ).toBe("rejected");
    expect(
      effectiveApprovalDecision(
        raised,
        parseApprovalPayloadContext(raised.payloadJson),
        [raised, decided],
        null,
      ),
    ).toBe("rejected");
  });

  it("uses the local optimistic decision until the stream catches up", () => {
    const raised = msg("APPROVAL_RAISED", "raised-1", {
      approval_request_id: "approval-1",
    });

    expect(
      effectiveApprovalDecision(
        raised,
        parseApprovalPayloadContext(raised.payloadJson),
        [raised],
        "approved",
      ),
    ).toBe("approved");
  });

  it("reads terminal state directly from decided messages", () => {
    const decided = msg("APPROVAL_DECIDED", "decided-1", {
      approval_request_id: "approval-1",
      approved: true,
    });

    expect(
      effectiveApprovalDecision(
        decided,
        parseApprovalPayloadContext(decided.payloadJson),
        [decided],
        null,
      ),
    ).toBe("approved");
  });
});

describe("shortApprovalContextId", () => {
  it("keeps short identifiers readable and truncates uuid-like values", () => {
    expect(shortApprovalContextId("exec-1")).toBe("exec-1");
    expect(shortApprovalContextId("12345678-1234-1234-1234-123456789abc")).toBe(
      "12345678",
    );
  });
});
