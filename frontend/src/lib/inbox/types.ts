import type {
  ApprovalRequest,
  ElicitationRequest,
} from "$lib/gen/harpia/plans/v1/plans_pb";

export type InboxItemKind = "elicitation" | "approval";

interface InboxItemBase {
  id: string;
  kind: InboxItemKind;
  /** ISO timestamp; used for sorting (most recent first) and bucketing. */
  createdAt: string;
  /** Resolved plan display name (falls back to "Plan" when unknown). */
  planName: string;
  /** Resolved task/step display name (falls back to "Task" when unknown). */
  taskName: string;
  /** One-line user-facing summary of the ask. */
  summary: string;
}

export interface InboxElicitationItem extends InboxItemBase {
  kind: "elicitation";
  planExecutionId: string;
  stepExecutionId: string;
  configurationId: string;
  raw: ElicitationRequest;
}

export interface InboxApprovalItem extends InboxItemBase {
  kind: "approval";
  planExecutionId: string;
  stepExecutionId: string;
  inputArtifactId: string;
  configurationId: string;
  raw: ApprovalRequest;
}

export type InboxItem = InboxElicitationItem | InboxApprovalItem;
