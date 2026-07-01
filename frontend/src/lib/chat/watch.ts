import { threadClient } from "$lib/rpc";
import { type ChatMessage, chatMessageFromProto } from "./types";

export interface WatchThreadOptions {
  sinceSequenceNumber?: bigint;
  signal?: AbortSignal;
}

export async function* watchThreadMessages(
  tenantId: string,
  threadId: string,
  options: WatchThreadOptions = {},
): AsyncIterable<ChatMessage[]> {
  const sinceSeq = options.sinceSequenceNumber ?? 0n;
  for await (const event of threadClient.watchThreadMessages(
    {
      tenantId,
      threadId,
      sinceSequenceNumber: sinceSeq,
    },
    { signal: options.signal },
  )) {
    yield event.messages.map(chatMessageFromProto);
  }
}
