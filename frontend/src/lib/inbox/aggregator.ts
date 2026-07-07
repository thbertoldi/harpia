import {
  loadInboxApprovals,
  watchApprovalRequests,
} from "$lib/plans/approvals";
import {
  loadInboxElicitations,
  watchElicitations,
} from "$lib/plans/elicitations";
import type {
  ApprovalRequest,
  ElicitationRequest,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import type {
  InboxApprovalItem,
  InboxElicitationItem,
  InboxItem,
} from "./types";

export interface InboxSources {
  watchElicitations: (
    tenantId: string,
    signal?: AbortSignal,
  ) => AsyncIterable<ElicitationRequest[]>;
  watchApprovalRequests: (
    tenantId: string,
    signal?: AbortSignal,
  ) => AsyncIterable<ApprovalRequest[]>;
}

export const DEFAULT_INBOX_SOURCES: InboxSources = {
  watchElicitations: (tenantId, signal) =>
    watchElicitations(tenantId, { addressedToMe: true, signal }),
  watchApprovalRequests: (tenantId, signal) =>
    watchApprovalRequests(tenantId, { signal }),
};

export interface WatchInboxOptions {
  signal?: AbortSignal;
}

export async function* watchInbox(
  tenantId: string,
  sources: InboxSources = DEFAULT_INBOX_SOURCES,
  options: WatchInboxOptions = {},
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

  void consume(sources.watchElicitations(tenantId, options.signal), (b) => {
    elicitations = b;
  });
  void consume(sources.watchApprovalRequests(tenantId, options.signal), (b) => {
    approvals = b;
  });
  try {
    while (!done) {
      if (dirty) {
        dirty = false;
        yield combine(elicitations, approvals);
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
  const [e, a] = await Promise.all([
    loadInboxElicitations(tenantId).catch(() => []),
    loadInboxApprovals(tenantId).catch(() => []),
  ]);
  return e.length + a.length;
}

function combine(
  elicitations: ElicitationRequest[],
  approvals: ApprovalRequest[],
): InboxItem[] {
  const items: InboxItem[] = [
    ...elicitations.map(toInboxElicitation),
    ...approvals.map(toInboxApproval),
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
    configurationId: req.planConfigurationId,
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
    configurationId: req.planConfigurationId,
    threadId: req.threadId,
    raw: req,
  };
}
