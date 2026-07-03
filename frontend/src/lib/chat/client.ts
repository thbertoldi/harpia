import { threadClient } from "$lib/rpc";
import type { Thread } from "$lib/gen/harpia/chat/v1/chat_pb";
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

/**
 * Recent chat threads for recall (home-page "recent conversations"). The
 * ListThreads RPC streams pages; we take the first page only.
 */
export async function listRecentThreads(
  tenantId: string,
  pageSize = 8,
): Promise<Thread[]> {
  const threads: Thread[] = [];
  for await (const page of threadClient.listThreads({
    tenantId,
    pageSize,
    pageToken: "",
  })) {
    threads.push(...page.threads);
    break;
  }
  return threads;
}
