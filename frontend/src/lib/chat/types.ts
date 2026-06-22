import {
  ThreadMessageKind as ProtoThreadMessageKind,
  ThreadMessageRole as ProtoThreadMessageRole,
} from "$lib/gen/harpia/chat/v1/chat_pb";
import type { ThreadMessage as ProtoThreadMessage } from "$lib/rpc";

export type ChatMessageRole = "OVERSEER" | "AGENT" | "SYSTEM";
export type ChatMessageKind =
  | "USER_TEXT"
  | "ASSISTANT_TEXT"
  | "CONFIGURATION_SAVED"
  | "RUN_STARTED"
  | "RUN_COMPLETED"
  | "RUN_FAILED"
  | "STEP_BOUND"
  | "ELICITATION_RAISED"
  | "ELICITATION_ANSWERED"
  | "APPROVAL_RAISED"
  | "APPROVAL_DECIDED";

export interface ChatMessage {
  id: string;
  tenantId: string;
  threadId: string;
  executionId: string;            // empty string when plan-scope
  role: ChatMessageRole;
  kind: ChatMessageKind;
  text: string;
  payloadJson: string;            // JSON-encoded per-kind payload; parse at use site
  authorUserId: string;           // empty for SYSTEM
  sequenceNumber: bigint;
  createdAt: string;              // ISO-8601 (from proto Timestamp)
}

const ROLE_FROM_PROTO: Record<number, ChatMessageRole> = {
  [ProtoThreadMessageRole.OVERSEER]: "OVERSEER",
  [ProtoThreadMessageRole.AGENT]: "AGENT",
  [ProtoThreadMessageRole.SYSTEM]: "SYSTEM",
};

const KIND_FROM_PROTO: Record<number, ChatMessageKind> = {
  [ProtoThreadMessageKind.USER_TEXT]: "USER_TEXT",
  [ProtoThreadMessageKind.ASSISTANT_TEXT]: "ASSISTANT_TEXT",
  [ProtoThreadMessageKind.CONFIGURATION_SAVED]: "CONFIGURATION_SAVED",
  [ProtoThreadMessageKind.RUN_STARTED]: "RUN_STARTED",
  [ProtoThreadMessageKind.RUN_COMPLETED]: "RUN_COMPLETED",
  [ProtoThreadMessageKind.RUN_FAILED]: "RUN_FAILED",
  [ProtoThreadMessageKind.STEP_BOUND]: "STEP_BOUND",
  [ProtoThreadMessageKind.ELICITATION_RAISED]: "ELICITATION_RAISED",
  [ProtoThreadMessageKind.ELICITATION_ANSWERED]: "ELICITATION_ANSWERED",
  [ProtoThreadMessageKind.APPROVAL_RAISED]: "APPROVAL_RAISED",
  [ProtoThreadMessageKind.APPROVAL_DECIDED]: "APPROVAL_DECIDED",
};

const ROLE_TO_PROTO: Record<ChatMessageRole, ProtoThreadMessageRole> = {
  OVERSEER: ProtoThreadMessageRole.OVERSEER,
  AGENT: ProtoThreadMessageRole.AGENT,
  SYSTEM: ProtoThreadMessageRole.SYSTEM,
};

const KIND_TO_PROTO: Record<ChatMessageKind, ProtoThreadMessageKind> = {
  USER_TEXT: ProtoThreadMessageKind.USER_TEXT,
  ASSISTANT_TEXT: ProtoThreadMessageKind.ASSISTANT_TEXT,
  CONFIGURATION_SAVED: ProtoThreadMessageKind.CONFIGURATION_SAVED,
  RUN_STARTED: ProtoThreadMessageKind.RUN_STARTED,
  RUN_COMPLETED: ProtoThreadMessageKind.RUN_COMPLETED,
  RUN_FAILED: ProtoThreadMessageKind.RUN_FAILED,
  STEP_BOUND: ProtoThreadMessageKind.STEP_BOUND,
  ELICITATION_RAISED: ProtoThreadMessageKind.ELICITATION_RAISED,
  ELICITATION_ANSWERED: ProtoThreadMessageKind.ELICITATION_ANSWERED,
  APPROVAL_RAISED: ProtoThreadMessageKind.APPROVAL_RAISED,
  APPROVAL_DECIDED: ProtoThreadMessageKind.APPROVAL_DECIDED,
};

export function chatMessageFromProto(p: ProtoThreadMessage): ChatMessage {
  return {
    id: p.id,
    tenantId: p.tenantId,
    threadId: p.threadId,
    executionId: p.executionId,
    role: ROLE_FROM_PROTO[p.role] ?? "SYSTEM",
    kind: KIND_FROM_PROTO[p.kind] ?? "USER_TEXT",
    text: p.text,
    payloadJson: p.payloadJson,
    authorUserId: p.authorUserId,
    sequenceNumber: p.sequenceNumber,
    createdAt: p.createdAt ? p.createdAt.toDate().toISOString() : "",
  };
}

export function chatRoleToProto(role: ChatMessageRole): ProtoThreadMessageRole {
  return ROLE_TO_PROTO[role];
}

export function chatKindToProto(kind: ChatMessageKind): ProtoThreadMessageKind {
  return KIND_TO_PROTO[kind];
}
