import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

// Mock the RPC client at the module boundary (same pattern as client.test.ts).
// watchThreadMessages pulls `threadClient` from `$lib/rpc`; we replace
// `watchThreadMessages` with a controllable fake that returns distinct
// async-iterables per call.
const threadClientMock = vi.hoisted(() => ({
  threadClient: {
    watchThreadMessages: vi.fn(),
  },
}));
vi.mock("$lib/rpc", () => threadClientMock);

import { watchThreadMessages, type WatchThreadOptions } from "./watch";
import type { ChatMessage } from "./types";

// watchThreadMessages is declared to return AsyncIterable (the public
// contract), but at runtime it's an async generator function and thus returns
// an AsyncGenerator exposing the .next()/.return() API these tests drive. The
// cast is sound and keeps the production signature unchanged.
type WatchGenerator = AsyncGenerator<ChatMessage[], void, unknown>;
function start(
  tenantId: string,
  threadId: string,
  options?: WatchThreadOptions,
): WatchGenerator {
  return watchThreadMessages(tenantId, threadId, options) as WatchGenerator;
}

// A proto-shaped message with just the fields chatMessageFromProto reads.
// sequenceNumber is a bigint — never coerced to Number.
type FakeProtoMessage = {
  id: string;
  sequenceNumber: bigint;
};

type FakeStreamEvent = { messages: FakeProtoMessage[] };

// Builds an async-iterable that yields `events` in order then ENDS (done).
// This simulates a ConnectRPC server stream that drops after sending.
function finiteStream(
  events: FakeStreamEvent[],
): AsyncIterable<FakeStreamEvent> {
  return {
    [Symbol.asyncIterator]() {
      let i = 0;
      return {
        next: () =>
          Promise.resolve(
            i < events.length
              ? { value: events[i++], done: false as const }
              : { value: undefined, done: true as const },
          ),
      };
    },
  };
}

// Builds an async-iterable that yields `events` then THROWS, simulating a
// stream that dies with a network/proxy error rather than a clean EOF.
function errorStream(
  events: FakeStreamEvent[],
  error: Error,
): AsyncIterable<FakeStreamEvent> {
  return {
    [Symbol.asyncIterator]() {
      let i = 0;
      return {
        async next() {
          if (i < events.length) {
            return { value: events[i++], done: false as const };
          }
          throw error;
        },
      };
    },
  };
}

// Narrows an IteratorResult to its yielded value. vitest's expect().toBe(false)
// doesn't narrow the type, so accessing .value directly leaves `void | T`; this
// helper throws if the generator returned done and yields the batch type-safely.
function batch(result: IteratorResult<ChatMessage[], void>): ChatMessage[] {
  if (result.done) {
    throw new Error(
      "expected a yielded batch, but the generator returned done",
    );
  }
  return result.value;
}

// Wind a generator down: abort the controller and flush the pending pull so
// the generator returns cleanly (no dangling microtasks between tests). Used
// after assertions are done; the pull resolves with { done: true } on abort.
async function teardown(
  gen: WatchGenerator,
  controller: AbortController,
): Promise<void> {
  controller.abort();
  // Flush any in-flight backoff so the abort propagates and the generator
  // returns. 2000ms covers the ±20% jitter on the 800ms backoff.
  await vi.advanceTimersByTimeAsync(2000);
}

beforeEach(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
  threadClientMock.threadClient.watchThreadMessages.mockReset();
});

describe("watchThreadMessages — auto-reconnect", () => {
  it("reconnects after a clean stream end and yields batches from both calls", async () => {
    threadClientMock.threadClient.watchThreadMessages
      .mockReturnValueOnce(
        finiteStream([{ messages: [{ id: "m1", sequenceNumber: 100n }] }]),
      )
      .mockReturnValueOnce(
        finiteStream([{ messages: [{ id: "m2", sequenceNumber: 200n }] }]),
      )
      // Subsequent reconnects idle — keeps the infinite loop from spamming.
      .mockReturnValue(finiteStream([]));

    const controller = new AbortController();
    const gen = start("t", "th", { signal: controller.signal });

    // First batch arrives from the initial stream.
    const r1 = await gen.next();
    expect(r1.done).toBe(false);
    expect(batch(r1)[0].sequenceNumber).toBe(100n);

    // Pull again — the first stream has ended, so the generator enters the
    // reconnect backoff. Advance the (fake) backoff timer to trigger the
    // second stream, which yields the further batch.
    const secondPull = gen.next();
    await vi.advanceTimersByTimeAsync(2000);
    const r2 = await secondPull;
    expect(r2.done).toBe(false);
    expect(batch(r2)[0].sequenceNumber).toBe(200n);

    await teardown(gen, controller);

    // The consumer saw messages from BOTH stream calls.
    expect(
      threadClientMock.threadClient.watchThreadMessages,
    ).toHaveBeenCalledTimes(2);
  });

  it("resumes from the max sequenceNumber of the previously yielded batch", async () => {
    // A batch with two messages — the cursor must end at the MAX seq in the
    // batch, not the last positionally.
    threadClientMock.threadClient.watchThreadMessages
      .mockReturnValueOnce(
        finiteStream([
          {
            messages: [
              { id: "m1", sequenceNumber: 41n },
              { id: "m2", sequenceNumber: 42n },
            ],
          },
        ]),
      )
      .mockReturnValue(finiteStream([]));

    const controller = new AbortController();
    const gen = start("t", "th", { signal: controller.signal });

    await gen.next(); // initial batch (cursor -> 42n)

    // Trigger a reconnect: the empty stream yields nothing, so the generator
    // loops back into backoff. Abort winds it down; the pending pull
    // resolves with done:true once the abort propagates.
    const reconnectPull = gen.next();
    await vi.advanceTimersByTimeAsync(2000);
    controller.abort();
    await reconnectPull;

    // The reconnect call's request carried the max seq from the first batch.
    const reconnectRequest =
      threadClientMock.threadClient.watchThreadMessages.mock.calls[1][0];
    expect(reconnectRequest.sinceSequenceNumber).toBe(42n);
    expect(reconnectRequest.tenantId).toBe("t");
    expect(reconnectRequest.threadId).toBe("th");
  });

  it("honors an explicit sinceSequenceNumber on the initial open", async () => {
    threadClientMock.threadClient.watchThreadMessages.mockReturnValue(
      finiteStream([]),
    );

    const controller = new AbortController();
    const gen = start("t", "th", {
      signal: controller.signal,
      sinceSequenceNumber: 9n,
    });

    // The mock is called synchronously when the generator starts.
    const pull = gen.next();
    const initialRequest =
      threadClientMock.threadClient.watchThreadMessages.mock.calls[0][0];
    expect(initialRequest.sinceSequenceNumber).toBe(9n);

    await teardown(gen, controller);
    await pull;
  });

  it("reconnects after the stream errors (swallows stream errors)", async () => {
    threadClientMock.threadClient.watchThreadMessages
      .mockReturnValueOnce(
        errorStream([], new Error("connection reset by peer")),
      )
      .mockReturnValueOnce(
        finiteStream([{ messages: [{ id: "m1", sequenceNumber: 5n }] }]),
      )
      .mockReturnValue(finiteStream([]));

    const controller = new AbortController();
    const gen = start("t", "th", { signal: controller.signal });

    // First stream throws immediately. The generator swallows the error,
    // backs off, then reconnects and yields the batch from stream 2.
    const recoveryPull = gen.next();
    await vi.advanceTimersByTimeAsync(2000);
    const r1 = await recoveryPull;
    expect(r1.done).toBe(false);
    expect(batch(r1)[0].sequenceNumber).toBe(5n);

    await teardown(gen, controller);

    expect(
      threadClientMock.threadClient.watchThreadMessages,
    ).toHaveBeenCalledTimes(2);
  });

  it("stops reconnecting when the abort signal fires during backoff", async () => {
    threadClientMock.threadClient.watchThreadMessages
      .mockReturnValueOnce(
        finiteStream([{ messages: [{ id: "m1", sequenceNumber: 7n }] }]),
      )
      // A would-be reconnect — must never be reached.
      .mockReturnValue(
        finiteStream([{ messages: [{ id: "x", sequenceNumber: 8n }] }]),
      );

    const controller = new AbortController();
    const gen = start("t", "th", { signal: controller.signal });

    const r1 = await gen.next();
    expect(batch(r1)[0].sequenceNumber).toBe(7n);

    // Pull again — the generator enters the backoff (timer pending).
    const nextPull = gen.next();
    // Abort DURING the backoff. The abort listener resolves the delay
    // immediately; the generator returns without reopening the stream.
    controller.abort();
    const r2 = await nextPull;
    expect(r2.done).toBe(true);

    // Exactly one stream call — no reconnect attempt.
    expect(
      threadClientMock.threadClient.watchThreadMessages,
    ).toHaveBeenCalledTimes(1);
  });

  it("ignores empty heartbeat batches without yielding to the caller", async () => {
    // The server sends empty `messages: []` events as heartbeats to keep
    // the stream alive. The generator must swallow them — never surface an
    // empty batch to the UI — and stay open on the SAME stream for the
    // real batch that follows.
    threadClientMock.threadClient.watchThreadMessages
      .mockReturnValueOnce(
        finiteStream([
          { messages: [] }, // heartbeat — must be skipped
          { messages: [{ id: "m1", sequenceNumber: 10n }] }, // real batch
        ]),
      )
      .mockReturnValue(finiteStream([]));

    const controller = new AbortController();
    const gen = start("t", "th", { signal: controller.signal });

    // First pull: the heartbeat is skipped, so the FIRST yielded value is
    // the real batch (not an empty array). The generator never terminated
    // on the heartbeat.
    const r1 = await gen.next();
    expect(r1.done).toBe(false);
    expect(batch(r1)).toHaveLength(1);
    expect(batch(r1)[0].id).toBe("m1");

    await teardown(gen, controller);

    // Exactly one stream call: the heartbeat did not end the stream and did
    // not trigger a reconnect. The generator stayed alive in-place.
    expect(
      threadClientMock.threadClient.watchThreadMessages,
    ).toHaveBeenCalledTimes(1);
  });

  it("preserves bigint sequence numbers beyond Number.MAX_SAFE_INTEGER", async () => {
    // 2^65 — well past the 2^53 Number precision cliff.
    const huge = 2n ** 65n;
    threadClientMock.threadClient.watchThreadMessages
      .mockReturnValueOnce(
        finiteStream([
          {
            messages: [
              { id: "m1", sequenceNumber: huge - 1n },
              { id: "m2", sequenceNumber: huge },
            ],
          },
        ]),
      )
      .mockReturnValue(finiteStream([]));

    const controller = new AbortController();
    const gen = start("t", "th", { signal: controller.signal });

    const r1 = await gen.next();
    // bigint round-trips exactly through chatMessageFromProto.
    expect(batch(r1).map((m) => m.sequenceNumber)).toEqual([huge - 1n, huge]);

    // Reconnect cursor is the exact bigint, not a precision-lossy Number.
    const reconnectPull = gen.next();
    await vi.advanceTimersByTimeAsync(2000);
    controller.abort();
    await reconnectPull;

    const reconnectRequest =
      threadClientMock.threadClient.watchThreadMessages.mock.calls[1][0];
    expect(reconnectRequest.sinceSequenceNumber).toBe(huge);
    expect(typeof reconnectRequest.sinceSequenceNumber).toBe("bigint");
  });
});
