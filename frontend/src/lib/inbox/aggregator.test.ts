import { describe, expect, it } from "vitest";
import { watchInbox, type InboxSources } from "./aggregator";
import type { InboxItem } from "./types";
import type {
  ApprovalRequest,
  ElicitationRequest,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import type { FeedbackRequest } from "$lib/gen/harpia/feedback/v1/feedback_pb";

function makeElicit(over: Partial<ElicitationRequest>): ElicitationRequest {
  return {
    id: "e1",
    prompt: "What now?",
    planExecutionId: "pe-1",
    planStepKey: "Write Draft",
    stepExecutionId: "se-1",
    createdAt: "2026-06-20T10:00:00Z",
    expiresAt: "",
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    ...(over as any),
  } as ElicitationRequest;
}

function makeApproval(over: Partial<ApprovalRequest>): ApprovalRequest {
  return {
    id: "a1",
    planExecutionId: "pe-2",
    planStepKey: "Publish",
    stepExecutionId: "se-2",
    inputArtifactId: "art-1",
    requestedAt: "2026-06-20T11:00:00Z",
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    ...(over as any),
  } as ApprovalRequest;
}

function makeFeedback(over: Partial<FeedbackRequest>): FeedbackRequest {
  return {
    id: "f1",
    question: "Rate this?",
    taskId: "task-1",
    agentInstanceId: "agent-1",
    createdAt: "2026-06-20T09:00:00Z",
    options: [],
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    ...(over as any),
  } as FeedbackRequest;
}

async function* yieldOnce<T>(value: T): AsyncIterable<T> {
  yield value;
}

function singleEmitSources(over: Partial<InboxSources> = {}): InboxSources {
  return {
    watchElicitations: () => yieldOnce([makeElicit({})]),
    watchApprovalRequests: () => yieldOnce([makeApproval({})]),
    loadFeedback: async () => [makeFeedback({})],
    ...over,
  };
}

describe("watchInbox", () => {
  it("yields a combined sorted list (most recent first) after all sources emit", async () => {
    const sources = singleEmitSources();
    const iter = watchInbox("tenant-1", sources)[Symbol.asyncIterator]();
    // Drain until we have all three kinds present.
    let last: InboxItem[] = [];
    for (let i = 0; i < 5; i++) {
      const { value, done } = await iter.next();
      if (done) break;
      last = value;
      if (last.length === 3) break;
    }
    expect(last).toHaveLength(3);
    expect(last.map((it) => it.kind)).toEqual([
      "approval", // 11:00
      "elicitation", // 10:00
      "feedback", // 09:00
    ]);
    expect(last.map((it) => it.createdAt)).toEqual([
      "2026-06-20T11:00:00Z",
      "2026-06-20T10:00:00Z",
      "2026-06-20T09:00:00Z",
    ]);
  });

  it("derives summary/planName/taskName from the underlying request fields", async () => {
    // watchInbox yields an initial empty list before any source emits (it's a
    // loading-state marker for the page); loop until e2 actually arrives.
    const sources = singleEmitSources({
      watchElicitations: () =>
        yieldOnce([
          makeElicit({
            id: "e2",
            prompt: "Avoid which topic?",
            planStepKey: "Write Draft",
          }),
        ]),
      watchApprovalRequests: () => yieldOnce([]),
      loadFeedback: async () => [],
    });
    const iter = watchInbox("tenant-1", sources)[Symbol.asyncIterator]();
    let item: InboxItem | undefined;
    for (let i = 0; i < 5; i++) {
      const { value, done } = await iter.next();
      if (done) break;
      item = value!.find((it) => it.id === "e2");
      if (item) break;
    }
    expect(item?.kind).toBe("elicitation");
    expect(item?.summary).toBe("Avoid which topic?");
    expect(item?.taskName).toBe("Write Draft");
    // planName falls back to a generic label when we don't have a registry lookup
    expect(item?.planName).toBeTruthy();
  });

  it("surfaces pending publish approvals with configuration and input artifact pointers", async () => {
    const sources = singleEmitSources({
      watchElicitations: () => yieldOnce([]),
      watchApprovalRequests: () =>
        yieldOnce([
          makeApproval({
            id: "approval-publish",
            planConfigurationId: "config-linkedin",
            planExecutionId: "exec-linkedin",
            planStepKey: "publish-linkedin",
            stepExecutionId: "step-publish",
            inputArtifactId: "artifact-linkedin-draft",
            requestedAt: "2026-06-26T15:00:00Z",
          }),
        ]),
      loadFeedback: async () => [],
    });
    const iter = watchInbox("tenant-1", sources)[Symbol.asyncIterator]();
    let item: InboxItem | undefined;
    for (let i = 0; i < 5; i++) {
      const { value, done } = await iter.next();
      if (done) break;
      item = value!.find((entry) => entry.id === "approval-publish");
      if (item) break;
    }

    expect(item?.kind).toBe("approval");
    if (!item || item.kind !== "approval") {
      throw new Error("expected approval inbox item");
    }
    expect(item.configurationId).toBe("config-linkedin");
    expect(item.planExecutionId).toBe("exec-linkedin");
    expect(item.stepExecutionId).toBe("step-publish");
    expect(item.inputArtifactId).toBe("artifact-linkedin-draft");
  });

  it("re-emits when a stream produces a new batch", async () => {
    async function* twoBatches(): AsyncIterable<ElicitationRequest[]> {
      yield [makeElicit({ id: "e-first" })];
      yield [makeElicit({ id: "e-first" }), makeElicit({ id: "e-second" })];
    }
    const sources: InboxSources = {
      watchElicitations: () => twoBatches(),
      watchApprovalRequests: () => yieldOnce([]),
      loadFeedback: async () => [],
    };
    const iter = watchInbox("tenant-1", sources)[Symbol.asyncIterator]();
    const first = await iter.next();
    const second = await iter.next();
    const third = await iter.next();
    // At least one yielded value should contain both elicitations.
    const all = [first.value, second.value, third.value].filter(Boolean);
    const found = all.some((batch) => batch!.length === 2);
    expect(found).toBe(true);
  });
});
