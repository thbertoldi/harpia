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
 * Merges two message lists by `id` (deduping) and orders the result ascending
 * by `sequenceNumber`. `sequenceNumber` is a bigint, so it is compared with
 * `</>` rather than arithmetic.
 *
 * Used by the chat route to combine an existing in-memory stream with an
 * incoming batch (watch loop or a fetch-on-flip refetch), so stream replay
 * dupes and a fresh listThreadMessages round-trip can't introduce duplicates
 * or reorder history.
 */
export function mergeThreadMessages(
  existing: ReadonlyArray<ChatMessage>,
  incoming: ReadonlyArray<ChatMessage>,
): ChatMessage[] {
  const seen = new Set<string>();
  const merged: ChatMessage[] = [];
  for (const message of existing) {
    if (seen.has(message.id)) continue;
    seen.add(message.id);
    merged.push(message);
  }
  for (const message of incoming) {
    if (seen.has(message.id)) continue;
    seen.add(message.id);
    merged.push(message);
  }
  merged.sort((a, b) => {
    if (a.sequenceNumber < b.sequenceNumber) return -1;
    if (a.sequenceNumber > b.sequenceNumber) return 1;
    return 0;
  });
  return merged;
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
