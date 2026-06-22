import { planClient } from "$lib/rpc";
import { type ChatMessage, chatMessageFromProto } from "./types";

export interface WatchThreadOptions {
  sinceSequenceNumber?: bigint;
  signal?: AbortSignal;
}

export async function* watchThreadMessages(
  tenantId: string,
  configurationId: string,
  options: WatchThreadOptions = {},
): AsyncIterable<ChatMessage[]> {
  const sinceSeq = options.sinceSequenceNumber ?? 0n;
  for await (const event of planClient.watchPlanThreadMessages(
    {
      tenantId,
      planConfigurationId: configurationId,
      sinceSequenceNumber: sinceSeq,
    },
    { signal: options.signal },
  )) {
    yield event.messages.map(chatMessageFromProto);
  }
}
