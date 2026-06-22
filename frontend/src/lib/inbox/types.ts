import type {
  ApprovalRequest,
  ElicitationRequest,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import type { FeedbackRequest } from "$lib/gen/harpia/feedback/v1/feedback_pb";

export type InboxItemKind = "elicitation" | "approval" | "feedback";

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

export interface InboxFeedbackItem extends InboxItemBase {
  kind: "feedback";
  taskId: string;
  raw: FeedbackRequest;
}

export type InboxItem =
  | InboxElicitationItem
  | InboxApprovalItem
  | InboxFeedbackItem;
