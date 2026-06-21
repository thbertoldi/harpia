import {
  loadInboxApprovals,
  watchApprovalRequests,
} from "$lib/plans/approvals";
import {
  loadInboxElicitations,
  watchElicitations,
} from "$lib/plans/elicitations";
import { loadPendingFeedback } from "$lib/feedback/counts";
import type {
  ApprovalRequest,
  ElicitationRequest,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import type { FeedbackRequest } from "$lib/gen/harpia/feedback/v1/feedback_pb";
import type {
  InboxApprovalItem,
  InboxElicitationItem,
  InboxFeedbackItem,
  InboxItem,
} from "./types";

export interface InboxSources {
  watchElicitations: (tenantId: string) => AsyncIterable<ElicitationRequest[]>;
  watchApprovalRequests: (tenantId: string) => AsyncIterable<ApprovalRequest[]>;
  loadFeedback: (tenantId: string) => Promise<FeedbackRequest[]>;
}

export const DEFAULT_INBOX_SOURCES: InboxSources = {
  watchElicitations: (tenantId) =>
    watchElicitations(tenantId, { addressedToMe: true }),
  watchApprovalRequests: (tenantId) => watchApprovalRequests(tenantId),
  loadFeedback: loadPendingFeedback,
};

export async function* watchInbox(
  tenantId: string,
  sources: InboxSources = DEFAULT_INBOX_SOURCES,
): AsyncIterable<InboxItem[]> {
  // resolveNext is initialised to a noop so its type is `() => void` (no
  // `| null` union). TypeScript narrows `let x: T | null = null` to `null` at
  // sites after callback-only reassignments — the Promise executor below only
  // mutates it from inside a closure, so a strict-mode compiler would treat
  // the finally-block `resolveNext?.()` as a call on `never`. Keeping the
  // variable always-callable sidesteps that quirk and removes the need for
  // optional chaining.
  const NOOP: () => void = () => {};

  let elicitations: ElicitationRequest[] = [];
  let approvals: ApprovalRequest[] = [];
  let feedbacks: FeedbackRequest[] = [];
  let dirty = true;
  let resolveNext: () => void = NOOP;
  let done = false;

  const wake = () => {
    dirty = true;
    const r = resolveNext;
    resolveNext = NOOP;
    r();
  };

  const consume = async <T>(
    iter: AsyncIterable<T[]>,
    sink: (batch: T[]) => void,
  ) => {
    try {
      for await (const batch of iter) {
        if (done) return;
        sink(batch);
        wake();
      }
    } catch {
      // Stream errors are non-fatal; the inbox will keep showing the last
      // known batches from the other sources.
    }
  };

  void consume(sources.watchElicitations(tenantId), (b) => {
    elicitations = b;
  });
  void consume(sources.watchApprovalRequests(tenantId), (b) => {
    approvals = b;
  });
  void sources
    .loadFeedback(tenantId)
    .then((b) => {
      feedbacks = b;
      wake();
    })
    .catch(() => {
      /* swallow; inbox stays empty for feedback */
    });

  try {
    while (!done) {
      if (dirty) {
        dirty = false;
        yield combine(elicitations, approvals, feedbacks);
      }
      await new Promise<void>((resolve) => {
        if (dirty) {
          resolve();
        } else {
          resolveNext = resolve;
        }
      });
    }
  } finally {
    done = true;
    const r = resolveNext;
    resolveNext = NOOP;
    r();
  }
}

export async function countInbox(tenantId: string): Promise<number> {
  const [e, a, f] = await Promise.all([
    loadInboxElicitations(tenantId).catch(() => []),
    loadInboxApprovals(tenantId).catch(() => []),
    loadPendingFeedback(tenantId).catch(() => []),
  ]);
  return e.length + a.length + f.length;
}

function combine(
  elicitations: ElicitationRequest[],
  approvals: ApprovalRequest[],
  feedbacks: FeedbackRequest[],
): InboxItem[] {
  const items: InboxItem[] = [
    ...elicitations.map(toInboxElicitation),
    ...approvals.map(toInboxApproval),
    ...feedbacks.map(toInboxFeedback),
  ];
  items.sort((a, b) => b.createdAt.localeCompare(a.createdAt));
  return items;
}

function toInboxElicitation(req: ElicitationRequest): InboxElicitationItem {
  return {
    id: req.id,
    kind: "elicitation",
    createdAt: req.createdAt,
    planName: "Plan", // refined post-M3 once plan registry is reachable
    taskName: req.planStepKey || req.stepExecutionId || "Task",
    summary: req.prompt,
    planExecutionId: req.planExecutionId,
    stepExecutionId: req.stepExecutionId,
    raw: req,
  };
}

function toInboxApproval(req: ApprovalRequest): InboxApprovalItem {
  return {
    id: req.id,
    kind: "approval",
    createdAt: req.requestedAt,
    planName: "Plan",
    taskName: req.planStepKey || req.stepExecutionId || "Task",
    // Approval summaries are computed by the renderer (InboxRow) via i18n
    // — keeping aggregator output locale-free.
    summary: "",
    planExecutionId: req.planExecutionId,
    stepExecutionId: req.stepExecutionId,
    inputArtifactId: req.inputArtifactId,
    raw: req,
  };
}

function toInboxFeedback(req: FeedbackRequest): InboxFeedbackItem {
  return {
    id: req.id,
    kind: "feedback",
    createdAt: req.createdAt,
    planName: "Plan",
    taskName: req.taskId || "Task",
    summary: req.question,
    taskId: req.taskId,
    raw: req,
  };
}
