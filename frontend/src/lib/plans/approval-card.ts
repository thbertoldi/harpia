import type { ChatMessage } from "$lib/chat/types";

export type ApprovalDecision = "approved" | "rejected" | null;

export interface ApprovalPayloadContext {
  approvalRequestId: string;
  approved: boolean | null;
  inputArtifactId: string;
  planStepKey: string;
  planConfigurationId: string;
  planExecutionId: string;
  stepExecutionId: string;
}

function stringField(raw: Record<string, unknown>, key: string): string {
  const value = raw[key];
  return typeof value === "string" ? value : "";
}

export function parseApprovalPayloadContext(
  payloadJson: string,
): ApprovalPayloadContext | null {
  if (!payloadJson) return null;
  try {
    const parsed = JSON.parse(payloadJson) as unknown;
    if (!parsed || typeof parsed !== "object") return null;
    const raw = parsed as Record<string, unknown>;
    const approvalRequestId = stringField(raw, "approval_request_id");
    if (!approvalRequestId) return null;
    return {
      approvalRequestId,
      approved: typeof raw.approved === "boolean" ? raw.approved : null,
      inputArtifactId: stringField(raw, "input_artifact_id"),
      planStepKey: stringField(raw, "plan_step_key"),
      planConfigurationId: stringField(raw, "plan_configuration_id"),
      planExecutionId: stringField(raw, "plan_execution_id"),
      stepExecutionId: stringField(raw, "step_execution_id"),
    };
  } catch {
    return null;
  }
}

export function decisionFromApprovedFlag(
  approved: boolean | null | undefined,
): ApprovalDecision {
  if (approved === true) return "approved";
  if (approved === false) return "rejected";
  return null;
}

export function streamedApprovalDecision(
  approvalRequestId: string,
  currentMessageId: string,
  messages: ChatMessage[],
): ApprovalDecision {
  if (!approvalRequestId) return null;
  for (const candidate of messages) {
    if (candidate.kind !== "APPROVAL_DECIDED") continue;
    if (candidate.id === currentMessageId) continue;
    const parsed = parseApprovalPayloadContext(candidate.payloadJson);
    if (parsed?.approvalRequestId === approvalRequestId) {
      return decisionFromApprovedFlag(parsed.approved);
    }
  }
  return null;
}

export function effectiveApprovalDecision(
  message: ChatMessage,
  parsed: ApprovalPayloadContext | null,
  messages: ChatMessage[],
  localDecision: ApprovalDecision,
): ApprovalDecision {
  if (message.kind === "APPROVAL_DECIDED") {
    return decisionFromApprovedFlag(parsed?.approved);
  }
  return (
    streamedApprovalDecision(
      parsed?.approvalRequestId ?? "",
      message.id,
      messages,
    ) ?? localDecision
  );
}

export function shortApprovalContextId(value: string): string {
  const trimmed = value.trim();
  return trimmed.length > 12 ? trimmed.slice(0, 8) : trimmed;
}
