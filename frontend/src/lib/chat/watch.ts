import { threadClient } from "$lib/rpc";
import { type ChatMessage, chatMessageFromProto } from "./types";

export interface WatchThreadOptions {
  sinceSequenceNumber?: bigint;
  signal?: AbortSignal;
}

// Backoff between stream reconnects. A dropped stream (idle/proxy timeout,
// network blip, server restart) is expected on long-lived watches, so we
// retry indefinitely until the caller aborts. The ±20% jitter keeps a fleet
// of clients from reconnecting in lockstep.
const RECONNECT_BACKOFF_MS = 800;

/**
 * Watch a thread for new messages, reconnecting automatically when the
 * underlying ConnectRPC stream ends.
 *
 * The server keeps the stream open and polls the DB; when the stream is cut
 * (idle/proxy timeout, network blip, dev server restart) the raw `for await`
 * simply exits and — without this wrapper — the caller stops receiving
 * updates until a full page reload. This generator wraps that stream in an
 * outer reconnect loop that:
 *
 *   1. Tracks the highest `sequenceNumber` seen (bigint only — never
 *      `Number()`, which would lose precision past 2^53).
 *   2. On any stream end (clean completion OR error), waits a short abortable
 *      backoff and reopens the stream with `sinceSequenceNumber` set to the
 *      cursor, so messages produced during the gap are not skipped.
 *   3. Stops as soon as the caller's `signal` aborts — the abort is never
 *      swallowed, and a backoff in progress is resolved early.
 *
 * Reconnect-time errors are swallowed (they are expected on a long-lived
 * watch). Replay dupes on reconnect are deduped by the caller via
 * `mergeThreadMessages` (id-keyed), so the cursor only guards against gaps,
 * not duplicates.
 */
export async function* watchThreadMessages(
  tenantId: string,
  threadId: string,
  options: WatchThreadOptions = {},
): AsyncIterable<ChatMessage[]> {
  const signal = options.signal;
  // Resume cursor: the highest sequenceNumber seen across all batches.
  let cursor = options.sinceSequenceNumber ?? 0n;

  // Loop forever, opening a fresh stream each iteration, until the caller
  // aborts. A clean stream completion and a stream error are both expected
  // on a long-lived watch and both trigger a reconnect.
  while (!signal?.aborted) {
    try {
      for await (const event of threadClient.watchThreadMessages(
        {
          tenantId,
          threadId,
          sinceSequenceNumber: cursor,
        },
        { signal },
      )) {
        const batch = event.messages.map(chatMessageFromProto);
        // Advance the resume cursor to the highest sequenceNumber in the
        // batch so the next reconnect asks only for newer messages.
        for (const message of batch) {
          if (message.sequenceNumber > cursor) {
            cursor = message.sequenceNumber;
          }
        }
        // Skip empty batches: the server sends them as heartbeats to keep
        // the stream alive. Yielding an empty array would needlessly
        // reassign `messages` and retrigger derived UI state without
        // surfacing anything new. The generator stays open on the same
        // stream and waits for the next (non-empty) batch.
        if (batch.length === 0) continue;
        yield batch;
      }
    } catch {
      // Stream errored (network drop, server restart, proxy reset, or an
      // abort propagated by ConnectRPC). Swallow and reconnect — unless the
      // caller aborted, which the loop guard + abortableDelay honor.
      if (signal?.aborted) return;
    }
    if (signal?.aborted) return;
    // Stream ended (cleanly or with error). Wait a short, abortable backoff
    // before reopening so a hard-down server isn't hammered.
    await abortableDelay(RECONNECT_BACKOFF_MS, signal);
  }
}

/**
 * Resolves after `ms` (±20% jitter), or as soon as `signal` aborts —
 * whichever is first. Used for reconnect backoff: the wait must never block
 * past an abort.
 */
function abortableDelay(ms: number, signal?: AbortSignal): Promise<void> {
  return new Promise<void>((resolve) => {
    if (signal?.aborted) {
      resolve();
      return;
    }
    const jitter = ms * (0.8 + Math.random() * 0.4);
    const timer = setTimeout(() => {
      signal?.removeEventListener("abort", onAbort);
      resolve();
    }, jitter);
    function onAbort(): void {
      clearTimeout(timer);
      resolve();
    }
    if (signal) {
      signal.addEventListener("abort", onAbort, { once: true });
    }
  });
}
