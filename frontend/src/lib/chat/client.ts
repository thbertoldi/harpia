import { planClient } from "$lib/rpc";
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
  configurationId: string,
  role: ChatMessageRole,
  kind: ChatMessageKind,
  text: string,
  payloadJson = "{}",
  executionId = "",
): Promise<ChatMessage> {
  const response = await planClient.appendPlanThreadMessage({
    tenantId,
    planConfigurationId: configurationId,
    role: chatRoleToProto(role),
    kind: chatKindToProto(kind),
    text,
    payloadJson,
    executionId,
  });
  if (!response.message) {
    throw new Error("appendPlanThreadMessage returned no message");
  }
  return chatMessageFromProto(response.message);
}

export async function loadThreadMessages(
  tenantId: string,
  configurationId: string,
  pageSize = 100,
): Promise<ChatMessage[]> {
  const response = await planClient.listPlanThreadMessages({
    tenantId,
    planConfigurationId: configurationId,
    pageSize,
    pageToken: "",
  });
  return response.messages.map(chatMessageFromProto);
}
