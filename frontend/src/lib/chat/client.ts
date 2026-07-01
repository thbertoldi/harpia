import { threadClient } from "$lib/rpc";
import {
  type ChatMessage,
  type ChatMessageKind,
  type ChatMessageRole,
  chatKindToProto,
  chatMessageFromProto,
  chatRoleToProto,
} from "./types";

export async function appendThreadMessage(
  tenantId: string,
  threadId: string,
  role: ChatMessageRole,
  kind: ChatMessageKind,
  text: string,
  payloadJson = "{}",
  executionId = "",
): Promise<ChatMessage> {
  const response = await threadClient.appendThreadMessage({
    tenantId,
    threadId,
    role: chatRoleToProto(role),
    kind: chatKindToProto(kind),
    text,
    payloadJson,
    executionId,
  });
  if (!response.message) {
    throw new Error("appendThreadMessage returned no message");
  }
  return chatMessageFromProto(response.message);
}

export async function loadThreadMessages(
  tenantId: string,
  threadId: string,
  pageSize = 100,
): Promise<ChatMessage[]> {
  const response = await threadClient.listThreadMessages({
    tenantId,
    threadId,
    pageSize,
    pageToken: "",
  });
  return response.messages.map(chatMessageFromProto);
}
