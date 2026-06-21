# Harpia UX Realignment — M2 Unified Inbox — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the three legacy oversight routes (`/oversee`, `/elicitations`, `/approvals`) with a single `/inbox` aggregator that combines pending elicitations, approvals, and feedback into one cross-plan "Needs you" surface, with inline approve/reject for binary approvals.

**Architecture:** A new `lib/inbox/` module owns a watch function that merges the existing per-type streams (`watchElicitations`, `watchApprovalRequests`) plus a one-shot feedback load (`listPendingFeedback`) into a single sorted `InboxItem[]` AsyncGenerator. The `/inbox` page subscribes to this aggregator. Approval rows expand inline to render the input artifact and call the existing `respondToApprovalRequest` RPC without leaving the inbox. Elicitation and feedback rows offer an "Open thread" affordance that gracefully degrades to the existing detail-page routes (M3 will replace those targets with plan-thread anchors). Legacy list routes become 302 redirects to `/inbox`; their nav entries collapse into a single "Needs you" entry that reuses the existing M1 `nav.needsYou` key. The legacy badge components are deleted.

**Tech Stack:** SvelteKit 2.16 + Svelte 5 (runes), TypeScript 5.7, Connect RPC (@connectrpc/connect), Tailwind v4 with Harpia design tokens, Vitest.

## Global Constraints

- **M2 changes are NOT flag-gated.** Unlike M1's foundation scaffolding, M2 ships the real /inbox to everyone — both flag-on and flag-off users — and the legacy list routes are deleted for everyone. The M1 flag (`PUBLIC_FEATURE_UX_REALIGNMENT_M1`) still governs the persona-aware sidebar, but the unified inbox itself is universal.
- **Vocabulary canonical per spec §2** (`docs/superpowers/specs/2026-06-19-harpia-ux-realignment-design.md`). User-facing copy uses *Task / Plan / Agent / Integration / Executor / Overseer / Elicitation / Approval / Feedback / Artifact*. Subtype filter chips use "Elicitations", "Approvals", "Feedback" (plural nouns); detail copy uses singular ("Elicitation", "Approval", "Feedback").
- **The "Earlier today" boundary is since-midnight-local in the user's timezone, capped at 20 items**, sorted by most-recent first.
- **Inline approval preview** is an expand-in-place that renders the input artifact below the row (Option A from M2 brainstorm). No modal, no navigation.
- **"Open thread" gracefully degrades to legacy detail routes** in M2: elicitations link to `/plans/executions/[executionId]/elicitations/[elicitationId]`, approvals link to `/plans/executions/[executionId]/approvals/[approvalId]`. M3 will repoint these to plan-thread anchors.
- **Detail-page back-links must point to `/inbox`**, not the deleted list routes.
- **Tests live next to source** as `<name>.test.ts`. Run with `npm run test` from `frontend/`.
- **Lockstep i18n.** Every key added/removed must change both `en.json` and `pt-BR.json` in the same commit. The parity test from M1 (`hardcoded-copy.test.ts`) enforces this.
- **Commit per task.** Conventional Commits with `feat(ux-m2):` / `chore(ux-m2):` / `test(ux-m2):` scope.
- **No Claude co-author trailer** on any commit (per project convention).
- **Design tokens locked.** Use only existing tokens (`bg-obsidian`, `bg-obsidian-light`, `text-cream`, `text-crown-ash`, `text-talon-gold`, `border-plumage`, `font-heading`, `font-mono`, etc.). Mockup reference: `.superpowers/brainstorm/3640967-*/content/unified-inbox.html`.

---

## File map (all changes in M2)

**Create:**
- `frontend/src/lib/inbox/types.ts` — discriminated union `InboxItem` (elicitation | approval | feedback)
- `frontend/src/lib/inbox/aggregator.ts` — `watchInbox(tenantId, sources?)`, `countInbox(tenantId)`
- `frontend/src/lib/inbox/aggregator.test.ts`
- `frontend/src/lib/inbox/buckets.ts` — `selectEarlierTodayItems(items, now, tz)` helper
- `frontend/src/lib/inbox/buckets.test.ts`
- `frontend/src/lib/feedback/counts.ts` — `countPendingFeedback(tenantId)`, `loadPendingFeedback(tenantId)` (matches existing per-type lib shape)
- `frontend/src/lib/feedback/counts.test.ts`
- `frontend/src/lib/components/InboxBadge.svelte`
- `frontend/src/lib/components/inbox/InboxRow.svelte` — shared row shell (subtype pill, source breadcrumb, age, slot for actions)
- `frontend/src/lib/components/inbox/InboxElicitationActions.svelte` — "Open thread" button
- `frontend/src/lib/components/inbox/InboxFeedbackActions.svelte` — "Open thread" button
- `frontend/src/lib/components/inbox/InboxApprovalEntry.svelte` — wraps the full approval row + footer expansion; owns local state (expanded, submitting, decision, rejectReason)
- `frontend/src/lib/components/inbox/InboxApprovalPreview.svelte` — renders the input artifact inline when expanded
- `frontend/src/routes/oversee/+page.ts` — 302 redirect loader
- `frontend/src/routes/elicitations/+page.ts` — 302 redirect loader
- `frontend/src/routes/approvals/+page.ts` — 302 redirect loader

**Modify (rewrite):**
- `frontend/src/routes/inbox/+page.svelte` — replace the M1 MilestoneStub with the real inbox UI

**Modify:**
- `frontend/src/lib/nav/sections.ts` — collapse three entries into a single "Needs you" entry
- `frontend/src/lib/nav/nav.test.ts` — update expectations
- `frontend/src/routes/+layout.svelte` — replace the three `{#if section.href === "/oversee|/elicitations|/approvals"}` badge blocks with one `{#if section.href === "/inbox"}<InboxBadge />{/if}` block
- `frontend/src/routes/plans/executions/[executionId]/elicitations/[elicitationId]/+page.svelte` — back-link from `/elicitations` → `/inbox`
- `frontend/src/routes/plans/executions/[executionId]/approvals/[approvalId]/+page.svelte` — back-link from `/approvals` → `/inbox`
- `frontend/src/lib/i18n/en.json` — remove `nav.oversee`, `nav.elicitations`, `nav.approvals`; add `inbox.*` keys
- `frontend/src/lib/i18n/pt-BR.json` — same lockstep
- `frontend/src/lib/i18n/hardcoded-copy.test.ts` — update any path allowlists that reference deleted files

**Delete:**
- `frontend/src/routes/oversee/+page.svelte`
- `frontend/src/routes/elicitations/+page.svelte`
- `frontend/src/routes/approvals/+page.svelte`
- `frontend/src/lib/components/FeedbackBadge.svelte` (no longer used after Task 3)
- `frontend/src/lib/components/ElicitationBadge.svelte`
- `frontend/src/lib/components/ApprovalBadge.svelte`

---

## Task 1: Inbox types + feedback count helper

**Files:**
- Create: `frontend/src/lib/inbox/types.ts`
- Create: `frontend/src/lib/feedback/counts.ts`
- Test: `frontend/src/lib/feedback/counts.test.ts`

**Interfaces:**
- Produces:
  - `type InboxItemKind = 'elicitation' | 'approval' | 'feedback'`
  - `type InboxItem = InboxElicitationItem | InboxApprovalItem | InboxFeedbackItem` (discriminated by `kind`)
  - Each variant carries: `id: string`, `kind: InboxItemKind`, `createdAt: string` (ISO), `planName: string`, `taskName: string`, `summary: string`, `raw: ElicitationRequest | ApprovalRequest | FeedbackRequest`
  - Elicitation variant adds: `planExecutionId: string`, `stepExecutionId: string`
  - Approval variant adds: `planExecutionId: string`, `stepExecutionId: string`, `inputArtifactId: string`
  - Feedback variant adds: `taskId: string`
  - `function loadPendingFeedback(tenantId: string): Promise<FeedbackRequest[]>`
  - `function countPendingFeedback(tenantId: string): Promise<number>`

- [ ] **Step 1: Write the failing test for counts**

> **IMPORTANT** — `feedbackClient.listPendingFeedback` is declared `methodKind: "server_streaming"` in the generated client (`frontend/src/lib/gen/harpia/feedback/v1/feedback_pb.ts`), which means it returns `AsyncIterable<ListPendingFeedbackResponse>` (NOT a Promise of one paginated response). Reference usage: `frontend/src/routes/oversee/+page.svelte` consumes it with `for await (const res of client.listPendingFeedback(...))`. The tests and implementation below match that streaming contract.

Create `frontend/src/lib/feedback/counts.test.ts`:

```ts
import { describe, expect, it, vi, beforeEach } from "vitest";

const feedbackClientMock = vi.hoisted(() => ({
  feedbackClient: {
    listPendingFeedback: vi.fn(),
  },
}));
vi.mock("$lib/rpc", () => feedbackClientMock);

beforeEach(() => {
  feedbackClientMock.feedbackClient.listPendingFeedback.mockReset();
});

describe("loadPendingFeedback", () => {
  it("drains all streamed responses and combines their feedbackRequests arrays", async () => {
    feedbackClientMock.feedbackClient.listPendingFeedback.mockImplementation(
      async function* () {
        yield { feedbackRequests: [{ id: "a" }, { id: "b" }], nextPageToken: "tok2" };
        yield { feedbackRequests: [{ id: "c" }], nextPageToken: "" };
      },
    );

    const { loadPendingFeedback } = await import("./counts");
    const got = await loadPendingFeedback("tenant-1");

    expect(got.map((f) => f.id)).toEqual(["a", "b", "c"]);
    expect(
      feedbackClientMock.feedbackClient.listPendingFeedback,
    ).toHaveBeenCalledTimes(1);
    expect(
      feedbackClientMock.feedbackClient.listPendingFeedback,
    ).toHaveBeenCalledWith({
      tenantId: "tenant-1",
      pageSize: 50,
      pageToken: "",
    });
  });

  it("returns an empty array when the stream yields no items", async () => {
    feedbackClientMock.feedbackClient.listPendingFeedback.mockImplementation(
      async function* () {
        // no yields
      },
    );
    const { loadPendingFeedback } = await import("./counts");
    expect(await loadPendingFeedback("tenant-1")).toEqual([]);
  });
});

describe("countPendingFeedback", () => {
  it("returns the number of pending feedback requests", async () => {
    feedbackClientMock.feedbackClient.listPendingFeedback.mockImplementation(
      async function* () {
        yield {
          feedbackRequests: [{ id: "a" }, { id: "b" }, { id: "c" }],
          nextPageToken: "",
        };
      },
    );
    const { countPendingFeedback } = await import("./counts");
    expect(await countPendingFeedback("tenant-1")).toBe(3);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd frontend && npx vitest run src/lib/feedback/counts.test.ts
```

Expected: FAIL — `Cannot find module './counts'`.

- [ ] **Step 3: Write minimal implementation of counts**

Create `frontend/src/lib/feedback/counts.ts`:

```ts
import { feedbackClient } from "$lib/rpc";
import type { FeedbackRequest } from "$lib/gen/harpia/feedback/v1/feedback_pb";

export async function loadPendingFeedback(
  tenantId: string,
): Promise<FeedbackRequest[]> {
  const out: FeedbackRequest[] = [];
  for await (const response of feedbackClient.listPendingFeedback({
    tenantId,
    pageSize: 50,
    pageToken: "",
  })) {
    out.push(...response.feedbackRequests);
  }
  return out;
}

export async function countPendingFeedback(
  tenantId: string,
): Promise<number> {
  const feedback = await loadPendingFeedback(tenantId);
  return feedback.length;
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd frontend && npx vitest run src/lib/feedback/counts.test.ts
```

Expected: 3 tests pass.

- [ ] **Step 5: Create the inbox item types**

Create `frontend/src/lib/inbox/types.ts`:

```ts
import type {
  ApprovalRequest,
  ElicitationRequest,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import type { FeedbackRequest } from "$lib/gen/harpia/feedback/v1/feedback_pb";

export type InboxItemKind = "elicitation" | "approval" | "feedback";

interface InboxItemBase {
  id: string;
  kind: InboxItemKind;
  /** ISO timestamp; used for sorting (most recent first) and bucketing. */
  createdAt: string;
  /** Resolved plan display name (falls back to "Plan" when unknown). */
  planName: string;
  /** Resolved task/step display name (falls back to "Task" when unknown). */
  taskName: string;
  /** One-line user-facing summary of the ask. */
  summary: string;
}

export interface InboxElicitationItem extends InboxItemBase {
  kind: "elicitation";
  planExecutionId: string;
  stepExecutionId: string;
  raw: ElicitationRequest;
}

export interface InboxApprovalItem extends InboxItemBase {
  kind: "approval";
  planExecutionId: string;
  stepExecutionId: string;
  inputArtifactId: string;
  raw: ApprovalRequest;
}

export interface InboxFeedbackItem extends InboxItemBase {
  kind: "feedback";
  taskId: string;
  raw: FeedbackRequest;
}

export type InboxItem =
  | InboxElicitationItem
  | InboxApprovalItem
  | InboxFeedbackItem;
```

- [ ] **Step 6: Commit**

```bash
cd /home/thbertoldi/harpia && git add frontend/src/lib/feedback frontend/src/lib/inbox/types.ts
git commit -m "feat(ux-m2): add InboxItem types and feedback count helper"
```

---

## Task 2: Inbox aggregator

**Files:**
- Create: `frontend/src/lib/inbox/aggregator.ts`
- Test: `frontend/src/lib/inbox/aggregator.test.ts`
- Create: `frontend/src/lib/inbox/buckets.ts`
- Test: `frontend/src/lib/inbox/buckets.test.ts`

**Interfaces:**
- Consumes:
  - `watchElicitations`, `loadInboxElicitations` from `$lib/plans/elicitations`
  - `watchApprovalRequests`, `loadInboxApprovals` from `$lib/plans/approvals`
  - `loadPendingFeedback` from `$lib/feedback/counts` (Task 1)
  - `InboxItem`, `InboxElicitationItem`, `InboxApprovalItem`, `InboxFeedbackItem` from `./types`
- Produces:
  - `interface InboxSources { watchElicitations(t): AsyncIterable<ElicitationRequest[]>; watchApprovalRequests(t): AsyncIterable<ApprovalRequest[]>; loadFeedback(t): Promise<FeedbackRequest[]> }`
  - `const DEFAULT_INBOX_SOURCES: InboxSources` wiring the real per-type lib functions
  - `function watchInbox(tenantId: string, sources?: InboxSources): AsyncIterable<InboxItem[]>` — merges all three sources, yields the combined sorted list every time any source updates
  - `function countInbox(tenantId: string): Promise<number>` — sums all three pending counts via one-shot loads (used by the badge)
  - `function selectEarlierTodayItems(items: InboxItem[], now: Date, tz: string): InboxItem[]` — filters to today's local calendar day, sorts most-recent-first, caps at 20

- [ ] **Step 1: Write the failing test for buckets**

Create `frontend/src/lib/inbox/buckets.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { selectEarlierTodayItems } from "./buckets";
import type { InboxItem } from "./types";

function elicit(id: string, createdAt: string): InboxItem {
  return {
    id,
    kind: "elicitation",
    createdAt,
    planName: "Plan",
    taskName: "Task",
    summary: "Q?",
    planExecutionId: "pe",
    stepExecutionId: "se",
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    raw: {} as any,
  };
}

describe("selectEarlierTodayItems", () => {
  const now = new Date("2026-06-20T15:30:00Z"); // 12:30 BRT

  it("returns only items whose createdAt falls on today's local calendar day, sorted most-recent-first", () => {
    const items: InboxItem[] = [
      elicit("today-late", "2026-06-20T18:00:00Z"),  // 15:00 BRT June 20
      elicit("today-early", "2026-06-20T11:00:00Z"), // 08:00 BRT June 20
      elicit("yesterday", "2026-06-20T01:00:00Z"),   // 22:00 BRT June 19
    ];
    const result = selectEarlierTodayItems(items, now, "America/Sao_Paulo");
    expect(result.map((i) => i.id)).toEqual(["today-late", "today-early"]);
  });

  it("caps at 20 items, keeping the 20 most recent", () => {
    const items: InboxItem[] = Array.from({ length: 25 }, (_, i) =>
      elicit(`x${i}`, `2026-06-20T${String(i).padStart(2, "0")}:00:00Z`),
    );
    const result = selectEarlierTodayItems(items, now, "America/Sao_Paulo");
    expect(result).toHaveLength(20);
    expect(result[0].id).toBe("x24"); // most recent of the day
    expect(result[19].id).toBe("x5"); // 20th most recent
  });

  it("returns an empty array when no items belong to today's local day", () => {
    const items: InboxItem[] = [
      elicit("yesterday", "2026-06-19T20:00:00Z"),
      elicit("yesterday-late", "2026-06-20T01:00:00Z"), // 22:00 BRT June 19
    ];
    const result = selectEarlierTodayItems(items, now, "America/Sao_Paulo");
    expect(result).toEqual([]);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd frontend && npx vitest run src/lib/inbox/buckets.test.ts
```

Expected: FAIL — `Cannot find module './buckets'`.

- [ ] **Step 3: Write buckets implementation**

Create `frontend/src/lib/inbox/buckets.ts`:

```ts
import type { InboxItem } from "./types";

const EARLIER_TODAY_CAP = 20;

/**
 * Filter items down to those whose createdAt falls on today's local
 * calendar day, sorted most-recent-first, capped at 20.
 *
 * In M2 the aggregator only emits currently-pending items, so the inbox
 * page passes an empty list here and the result is always []. M3 will
 * start feeding recently-completed items so the "Earlier today" section
 * actually populates. The helper exists in M2 so the page is wired
 * end-to-end and M3 only has to swap the data source.
 */
export function selectEarlierTodayItems(
  items: InboxItem[],
  now: Date,
  tz: string,
): InboxItem[] {
  const todayKey = localDayKey(now, tz);
  return items
    .filter((item) => localDayKey(new Date(item.createdAt), tz) === todayKey)
    .sort((a, b) => b.createdAt.localeCompare(a.createdAt))
    .slice(0, EARLIER_TODAY_CAP);
}

function localDayKey(date: Date, tz: string): string {
  return new Intl.DateTimeFormat("en-CA", {
    timeZone: tz,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(date);
}
```

- [ ] **Step 4: Run buckets tests to verify they pass**

```bash
cd frontend && npx vitest run src/lib/inbox/buckets.test.ts
```

Expected: 3 tests pass.

- [ ] **Step 5: Write the failing aggregator test**

Create `frontend/src/lib/inbox/aggregator.test.ts`:

```ts
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
```

- [ ] **Step 6: Run aggregator test to verify it fails**

```bash
cd frontend && npx vitest run src/lib/inbox/aggregator.test.ts
```

Expected: FAIL — module not found.

- [ ] **Step 7: Write the aggregator**

Create `frontend/src/lib/inbox/aggregator.ts`:

```ts
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
  watchElicitations: (
    tenantId: string,
  ) => AsyncIterable<ElicitationRequest[]>;
  watchApprovalRequests: (
    tenantId: string,
  ) => AsyncIterable<ApprovalRequest[]>;
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

  const consume = async <T,>(
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
    summary: "Approve to publish",
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
```

- [ ] **Step 8: Run all inbox tests to verify they pass**

```bash
cd frontend && npx vitest run src/lib/inbox/
```

Expected: 6 tests across the two suites pass.

- [ ] **Step 9: Commit**

```bash
cd /home/thbertoldi/harpia && git add frontend/src/lib/inbox/
git commit -m "feat(ux-m2): add inbox aggregator and earlier-today bucketing"
```

---

## Task 3: InboxBadge component + sidebar swap (delete legacy badges)

**Files:**
- Create: `frontend/src/lib/components/InboxBadge.svelte`
- Modify: `frontend/src/routes/+layout.svelte` (the badge iteration block around the section render)
- Delete: `frontend/src/lib/components/FeedbackBadge.svelte`
- Delete: `frontend/src/lib/components/ElicitationBadge.svelte`
- Delete: `frontend/src/lib/components/ApprovalBadge.svelte`

**Interfaces:**
- Consumes: `countInbox` from `$lib/inbox/aggregator` (Task 2); `getTenant` from `$lib/auth`
- Produces: an `InboxBadge` Svelte component that renders a badge styled like the legacy ones, but the count comes from `countInbox(tenantId)`. Polls every 30 seconds (the legacy badges loaded once on mount and never refreshed; the new badge represents unified cross-plan state and needs to stay live).

- [ ] **Step 1: Confirm visual styling matches the legacy badges**

```bash
cd /home/thbertoldi/harpia && cat frontend/src/lib/components/ElicitationBadge.svelte
```

Use the same Tailwind classes verbatim (`ml-auto inline-flex animate-pulse items-center justify-center rounded-full bg-talon-gold px-1.5 py-0.5 font-mono text-[10px] leading-none font-bold text-obsidian`) and the same null/zero guard (`{#if count > 0}`). The legacy badges only loaded once on mount; the new InboxBadge below adds a 30-second polling refresh because it represents live cross-plan state.

- [ ] **Step 2: Create the InboxBadge component**

Create `frontend/src/lib/components/InboxBadge.svelte`:

```svelte
<script lang="ts">
  import { onDestroy } from "svelte";
  import { getTenant } from "$lib/auth";
  import { countInbox } from "$lib/inbox/aggregator";

  let count = $state(0);
  let timer: ReturnType<typeof setInterval> | undefined;

  async function refresh() {
    const tenant = getTenant();
    if (!tenant?.id) return;
    try {
      count = await countInbox(tenant.id);
    } catch {
      // Keep the last-known count if the load fails.
    }
  }

  $effect(() => {
    void refresh();
    timer = setInterval(refresh, 30_000);
    return () => {
      if (timer) clearInterval(timer);
    };
  });

  onDestroy(() => {
    if (timer) clearInterval(timer);
  });
</script>

{#if count > 0}
  <span
    class="ml-auto inline-flex animate-pulse items-center justify-center rounded-full bg-talon-gold px-1.5 py-0.5 font-mono text-[10px] leading-none font-bold text-obsidian"
  >
    {count}
  </span>
{/if}
```

- [ ] **Step 3: Update +layout.svelte to use InboxBadge**

In `frontend/src/routes/+layout.svelte`, find the existing block:

```svelte
{#if section.href === "/oversee"}
  <FeedbackBadge />
{/if}
{#if section.href === "/elicitations"}
  <ElicitationBadge />
{/if}
{#if section.href === "/approvals"}
  <ApprovalBadge />
{/if}
```

REPLACE that entire block with:

```svelte
{#if section.href === "/inbox"}
  <InboxBadge />
{/if}
```

Also REMOVE the now-unused imports at the top of the script block:

```ts
import FeedbackBadge from "$lib/components/FeedbackBadge.svelte";
import ElicitationBadge from "$lib/components/ElicitationBadge.svelte";
import ApprovalBadge from "$lib/components/ApprovalBadge.svelte";
```

And ADD the new import:

```ts
import InboxBadge from "$lib/components/InboxBadge.svelte";
```

- [ ] **Step 4: Delete the legacy badge components**

```bash
cd /home/thbertoldi/harpia
rm frontend/src/lib/components/FeedbackBadge.svelte
rm frontend/src/lib/components/ElicitationBadge.svelte
rm frontend/src/lib/components/ApprovalBadge.svelte
```

- [ ] **Step 5: Verify no orphan references**

```bash
cd /home/thbertoldi/harpia && grep -rn "FeedbackBadge\|ElicitationBadge\|ApprovalBadge" frontend/src 2>/dev/null
```

Expected: zero matches.

- [ ] **Step 6: Run type-check and tests**

```bash
cd frontend && npx svelte-kit sync && npm run check && npm run test
```

Expected: zero NEW type errors; all tests pass.

- [ ] **Step 7: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/lib/components/InboxBadge.svelte frontend/src/routes/+layout.svelte
git add -u frontend/src/lib/components
git commit -m "feat(ux-m2): replace three legacy badges with unified InboxBadge"
```

---

## Task 4: Inbox page — base render, filters, source breadcrumbs, "Open thread"

**Files:**
- Rewrite: `frontend/src/routes/inbox/+page.svelte` (the M1 MilestoneStub becomes the real inbox)
- Create: `frontend/src/lib/components/inbox/InboxRow.svelte`
- Create: `frontend/src/lib/components/inbox/InboxElicitationActions.svelte`
- Create: `frontend/src/lib/components/inbox/InboxFeedbackActions.svelte`
- Modify: `frontend/src/lib/i18n/en.json` (add inbox.* keys)
- Modify: `frontend/src/lib/i18n/pt-BR.json` (lockstep)

**Interfaces:**
- Consumes:
  - `watchInbox`, `selectEarlierTodayItems` from `$lib/inbox/{aggregator,buckets}` (Task 2)
  - `getTenant`, `translate`, `locale` from `$lib`
- Produces:
  - A renderable `/inbox` page with: header ("Needs you · N pending"), filter chips (All / Elicitations / Approvals / Feedback), active rows (sorted), "Earlier today" section, empty state
  - Each row uses `InboxRow.svelte` as the shell; subtype-specific actions slot in via `InboxElicitationActions` / `InboxFeedbackActions` (approval actions in Task 5)

- [ ] **Step 1: Add the inbox i18n keys (en + pt-BR)**

In `frontend/src/lib/i18n/en.json`, after the existing `"nav.planCanvas"` entry, INSERT:

```json
  "inbox.title": "Needs you",
  "inbox.pendingCount": "{count} pending",
  "inbox.empty": "All caught up — your plans are running smoothly.",
  "inbox.filters.all": "All",
  "inbox.filters.elicitations": "Elicitations",
  "inbox.filters.approvals": "Approvals",
  "inbox.filters.feedback": "Feedback",
  "inbox.subtype.elicitation": "Elicit",
  "inbox.subtype.approval": "Approve",
  "inbox.subtype.feedback": "Feedback",
  "inbox.actions.openThread": "Open thread",
  "inbox.actions.approve": "Approve",
  "inbox.actions.reject": "Reject",
  "inbox.actions.preview": "Preview",
  "inbox.earlierToday": "Earlier today",
  "inbox.error": "Could not load the inbox.",
  "inbox.row.source": "{plan} · {task}",
```

In `frontend/src/lib/i18n/pt-BR.json`, INSERT the same keys in the same position with Portuguese values:

```json
  "inbox.title": "Sua atenção",
  "inbox.pendingCount": "{count} pendente(s)",
  "inbox.empty": "Tudo em dia — seus planos estão funcionando.",
  "inbox.filters.all": "Tudo",
  "inbox.filters.elicitations": "Elicitações",
  "inbox.filters.approvals": "Aprovações",
  "inbox.filters.feedback": "Feedback",
  "inbox.subtype.elicitation": "Elicit",
  "inbox.subtype.approval": "Aprovar",
  "inbox.subtype.feedback": "Feedback",
  "inbox.actions.openThread": "Abrir conversa",
  "inbox.actions.approve": "Aprovar",
  "inbox.actions.reject": "Recusar",
  "inbox.actions.preview": "Visualizar",
  "inbox.earlierToday": "Mais cedo hoje",
  "inbox.error": "Não foi possível carregar a inbox.",
  "inbox.row.source": "{plan} · {task}",
```

- [ ] **Step 2: Verify i18n parity test still passes**

```bash
cd frontend && npx vitest run src/lib/i18n/
```

Expected: 13/13 pass.

- [ ] **Step 3a: Add the `formatRelativeTime` i18n helper (does not exist yet)**

`frontend/src/lib/i18n/format.ts` currently only exports `formatLocaleDate` and `formatLocaleDateTime`. Add `formatRelativeTime` as a new export at the end of that file:

```ts
const RELATIVE_UNITS: Array<{
  limitSeconds: number;
  divisorSeconds: number;
  unit: Intl.RelativeTimeFormatUnit;
}> = [
  { limitSeconds: 60, divisorSeconds: 1, unit: "second" },
  { limitSeconds: 3600, divisorSeconds: 60, unit: "minute" },
  { limitSeconds: 86_400, divisorSeconds: 3600, unit: "hour" },
  { limitSeconds: 604_800, divisorSeconds: 86_400, unit: "day" },
];

/**
 * Returns a short relative-time string (e.g. "2 min ago", "1 hr ago",
 * "ontem", "há 3 dias") for an ISO-8601 instant in the past, localised
 * via Intl.RelativeTimeFormat. Falls back to `formatLocaleDate` for ages
 * beyond a week.
 */
export function formatRelativeTime(iso: string, locale: string): string {
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "";
  const deltaSeconds = Math.max(0, Math.floor((Date.now() - then) / 1000));
  for (const { limitSeconds, divisorSeconds, unit } of RELATIVE_UNITS) {
    if (deltaSeconds < limitSeconds) {
      const value = Math.max(1, Math.floor(deltaSeconds / divisorSeconds));
      return new Intl.RelativeTimeFormat(locale, { numeric: "auto" }).format(
        -value,
        unit,
      );
    }
  }
  return formatLocaleDate(iso, locale);
}
```

This pure helper takes an ISO instant and a locale and returns a localised relative-time string. No additional state, no test required for this step (covered indirectly by the inbox page's smoke check).

- [ ] **Step 3: Create the shared InboxRow shell**

Create `frontend/src/lib/components/inbox/InboxRow.svelte`. Note the optional `footer` snippet — Task 5's approval entry uses it to render the inline preview and reject-reason textarea inside the same card, below the 3-column row.

```svelte
<script lang="ts">
  import type { Snippet } from "svelte";
  import { locale, translate } from "$lib/i18n";
  import { formatRelativeTime } from "$lib/i18n/format";
  import type { InboxItem } from "$lib/inbox/types";

  interface Props {
    item: InboxItem;
    urgent?: boolean;
    actions: Snippet;
    footer?: Snippet;
  }

  let { item, urgent = false, actions, footer }: Props = $props();

  const subtypeKey = $derived(`inbox.subtype.${item.kind}`);
  const subtypePillClass = $derived(
    item.kind === "elicitation"
      ? "border border-talon-gold/40 bg-talon-gold/10 text-talon-gold"
      : item.kind === "approval"
        ? "border border-talon-gold bg-talon-gold/15 text-cream"
        : "border border-plumage bg-plumage text-crown-ash",
  );

  const sourceLabel = $derived(
    translate("inbox.row.source", $locale)
      .replace("{plan}", item.planName)
      .replace("{task}", item.taskName),
  );
</script>

<div
  class="rounded-lg border border-plumage bg-obsidian-light px-4 py-3 transition-colors hover:border-talon-gold"
  class:border-l-2={urgent}
  class:border-l-talon-gold={urgent}
>
  <div class="grid grid-cols-[auto_1fr_auto] items-center gap-4">
    <span
      class={`min-w-[64px] rounded text-center text-[9px] font-bold tracking-wider uppercase px-2 py-1 ${subtypePillClass}`}
    >
      {translate(subtypeKey, $locale)}
    </span>
    <div class="min-w-0">
      <div class="mb-1 font-mono text-[11px] text-crown-ash">
        {sourceLabel}
      </div>
      <div class="mb-1 text-[13px] leading-snug text-cream">{item.summary}</div>
      <div class="flex gap-3 text-[11px] text-crown-ash">
        <span>{formatRelativeTime(item.createdAt, $locale)}</span>
      </div>
    </div>
    <div class="flex gap-1.5">
      {@render actions()}
    </div>
  </div>
  {#if footer}
    <div class="mt-3 border-t border-plumage pt-3">
      {@render footer()}
    </div>
  {/if}
</div>
```

Note: `formatRelativeTime` is the helper added in Step 3a above. Import path: `$lib/i18n/format`.

- [ ] **Step 4: Create the elicitation actions component**

Create `frontend/src/lib/components/inbox/InboxElicitationActions.svelte`:

```svelte
<script lang="ts">
  import { locale, translate } from "$lib/i18n";
  import type { InboxElicitationItem } from "$lib/inbox/types";

  interface Props {
    item: InboxElicitationItem;
  }
  let { item }: Props = $props();

  const href = $derived(
    `/plans/executions/${item.planExecutionId}/elicitations/${item.id}`,
  );
</script>

<a
  {href}
  class="rounded border border-talon-gold bg-talon-gold px-3 py-1.5 text-[11px] font-semibold text-obsidian hover:opacity-90"
>
  {translate("inbox.actions.openThread", $locale)}
</a>
```

- [ ] **Step 5: Create the feedback actions component**

Create `frontend/src/lib/components/inbox/InboxFeedbackActions.svelte`:

```svelte
<script lang="ts">
  import { locale, translate } from "$lib/i18n";
  import type { InboxFeedbackItem } from "$lib/inbox/types";

  interface Props {
    item: InboxFeedbackItem;
  }
  let { item }: Props = $props();

  // Feedback detail still lives under /oversee in M2; M3 will repoint.
  const href = $derived(`/oversee#${item.id}`);
</script>

<a
  {href}
  class="rounded border border-plumage bg-transparent px-3 py-1.5 text-[11px] font-medium text-crown-ash hover:border-talon-gold hover:text-talon-gold"
>
  {translate("inbox.actions.openThread", $locale)}
</a>
```

Note: The `/oversee` route is being deleted in Task 6 and replaced by a 302 to `/inbox`. The link above intentionally points to `/oversee#<id>` which will redirect — until M3 re-anchors feedback to a per-plan thread, opening a feedback thread from the inbox bounces the user back to the inbox. This is the documented "graceful degrade" behavior for M2 (spec §5.1: "free-form answers belong in chat, not a list row"). Acceptable trade-off — feedback is not yet in scope for inline editing.

- [ ] **Step 6: Rewrite the inbox page**

Overwrite `frontend/src/routes/inbox/+page.svelte` (currently the M1 stub) with the real implementation:

```svelte
<script lang="ts">
  import { getTenant } from "$lib/auth";
  import { locale, translate } from "$lib/i18n";
  import { watchInbox } from "$lib/inbox/aggregator";
  import type { InboxItem, InboxItemKind } from "$lib/inbox/types";
  import InboxRow from "$lib/components/inbox/InboxRow.svelte";
  import InboxElicitationActions from "$lib/components/inbox/InboxElicitationActions.svelte";
  import InboxFeedbackActions from "$lib/components/inbox/InboxFeedbackActions.svelte";

  type Filter = "all" | InboxItemKind;

  let items = $state<InboxItem[]>([]);
  let filter = $state<Filter>("all");
  let loadError = $state(false);

  $effect(() => {
    const tenant = getTenant();
    if (!tenant?.id) return;
    let active = true;
    (async () => {
      try {
        for await (const batch of watchInbox(tenant.id)) {
          if (!active) return;
          items = batch;
        }
      } catch {
        if (active) loadError = true;
      }
    })();
    return () => {
      active = false;
    };
  });

  const visible = $derived(
    filter === "all" ? items : items.filter((it) => it.kind === filter),
  );

  const counts = $derived({
    all: items.length,
    elicitation: items.filter((it) => it.kind === "elicitation").length,
    approval: items.filter((it) => it.kind === "approval").length,
    feedback: items.filter((it) => it.kind === "feedback").length,
  });

  // M2 ships only the live "needs you" list. The selectEarlierTodayItems
  // helper exists in lib/inbox/buckets but is not wired here yet — M3 will
  // feed it recently-completed items from a separate query and render the
  // "Earlier today" section below this one.
</script>

<svelte:head>
  <title>{translate("inbox.title", $locale)} · Harpia</title>
</svelte:head>

<div class="mx-auto max-w-5xl px-6 py-6">
  <header class="mb-6 flex items-baseline justify-between">
    <h1 class="font-heading text-2xl text-cream">
      {translate("inbox.title", $locale)}
      <span class="ml-2 text-xs font-normal text-crown-ash">
        {translate("inbox.pendingCount", $locale).replace(
          "{count}",
          String(counts.all),
        )}
      </span>
    </h1>
  </header>

  <div class="mb-5 flex flex-wrap gap-2">
    {#each [
      { key: "all" as Filter, label: "inbox.filters.all", count: counts.all },
      { key: "elicitation" as Filter, label: "inbox.filters.elicitations", count: counts.elicitation },
      { key: "approval" as Filter, label: "inbox.filters.approvals", count: counts.approval },
      { key: "feedback" as Filter, label: "inbox.filters.feedback", count: counts.feedback },
    ] as chip (chip.key)}
      <button
        type="button"
        onclick={() => (filter = chip.key)}
        class={`flex items-center gap-1.5 rounded-full border px-3 py-1.5 text-[12px] transition ${
          filter === chip.key
            ? "border-talon-gold bg-talon-gold/10 text-talon-gold"
            : "border-plumage text-crown-ash hover:border-talon-gold hover:text-talon-gold"
        }`}
      >
        {translate(chip.label, $locale)}
        <span
          class={`rounded-full px-1.5 py-0.5 text-[10px] font-semibold ${
            filter === chip.key
              ? "bg-talon-gold text-obsidian"
              : "bg-plumage text-cream"
          }`}
        >
          {chip.count}
        </span>
      </button>
    {/each}
  </div>

  {#if loadError}
    <p class="rounded border border-plumage bg-obsidian-light px-4 py-3 text-sm text-crown-ash">
      {translate("inbox.error", $locale)}
    </p>
  {:else if visible.length === 0}
    <p class="rounded border border-plumage bg-obsidian-light px-4 py-3 text-sm text-crown-ash">
      {translate("inbox.empty", $locale)}
    </p>
  {:else}
    <ul class="flex flex-col gap-2">
      {#each visible as item (item.id)}
        <li>
          <InboxRow {item}>
            {#snippet actions()}
              {#if item.kind === "elicitation"}
                <InboxElicitationActions {item} />
              {:else if item.kind === "feedback"}
                <InboxFeedbackActions {item} />
              {:else}
                <!-- approval actions arrive in Task 5 -->
                <span class="text-[11px] text-crown-ash">…</span>
              {/if}
            {/snippet}
          </InboxRow>
        </li>
      {/each}
    </ul>
  {/if}
</div>
```

Note: the approval row's actions slot intentionally renders a placeholder in Task 4; Task 5 wires in `InboxApprovalActions`.

- [ ] **Step 7: Run type-check and tests**

```bash
cd frontend && npx svelte-kit sync && npm run check && npm run test
```

Expected: zero NEW type errors; tests pass (parity test now covers the new keys).

- [ ] **Step 8: Run lint and auto-format**

```bash
cd frontend && npm run format && npm run lint
```

Expected: clean.

- [ ] **Step 9: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/lib/i18n/ frontend/src/routes/inbox/ frontend/src/lib/components/inbox/
git commit -m "feat(ux-m2): render the inbox page with filters and graceful-degrade open-thread"
```

---

## Task 5: Approval entry — inline preview expand + Approve/Reject

**Files:**
- Create: `frontend/src/lib/components/inbox/InboxApprovalEntry.svelte` — wrapper that owns the per-approval state (expanded, submitting, decision, rejectReason) and renders `InboxRow` with action buttons in the `actions` snippet and the preview/textarea in the `footer` snippet.
- Modify: `frontend/src/routes/inbox/+page.svelte` (render `InboxApprovalEntry` for approval items instead of `InboxRow + actions` directly).
- Modify: `frontend/src/lib/i18n/en.json` (add approval preview keys)
- Modify: `frontend/src/lib/i18n/pt-BR.json` (lockstep)

**Interfaces:**
- Consumes:
  - `respondToApprovalRequest` from `$lib/plans/approvals`
  - `getTenant` from `$lib/auth`
  - `toUserMessage` from `$lib/connect-errors` (existing helper used by the legacy detail page)
  - `InboxApprovalItem` from `$lib/inbox/types`
  - `InboxRow` from `./InboxRow.svelte` (Task 4 — note `footer` snippet support added in Task 4 specifically to host this expansion)
  - **`ArtifactPreview` from `$lib/components/ArtifactPreview.svelte`** — already exists in the codebase, props `{ tenantId: string; artifactId: string }`, handles its own load/error/empty states via `$lib/artifacts/preview`. Use it directly in the footer; do NOT create an inbox-specific preview component or extract any new loader.
- Produces: `InboxApprovalEntry` component covering four states:
  - Default: row shows `Preview / Reject / Approve` buttons; no footer.
  - Expanded: footer renders `<ArtifactPreview ... />` inline; button label changes to `Hide`.
  - Submitting: buttons disabled.
  - Decided: row replaces buttons with a `Approved` or `Rejected` label.

- [ ] **Step 1: Add the approval-flow i18n keys**

In `frontend/src/lib/i18n/en.json`, insert after `inbox.actions.preview`:

```json
  "inbox.actions.hide": "Hide",
  "inbox.actions.submitting": "Sending…",
  "inbox.decision.approved": "Approved",
  "inbox.decision.rejected": "Rejected",
  "inbox.rejectReason.placeholder": "Reason (required to reject)",
```

In `frontend/src/lib/i18n/pt-BR.json`:

```json
  "inbox.actions.hide": "Ocultar",
  "inbox.actions.submitting": "Enviando…",
  "inbox.decision.approved": "Aprovado",
  "inbox.decision.rejected": "Recusado",
  "inbox.rejectReason.placeholder": "Motivo (obrigatório para recusar)",
```

(`ArtifactPreview.svelte` ships its own load/error/empty copy via `artifactPreview.*` keys — do NOT add `inbox.preview.*` keys.)

- [ ] **Step 2: Confirm the existing ArtifactPreview API**

```bash
cd /home/thbertoldi/harpia && head -25 frontend/src/lib/components/ArtifactPreview.svelte
```

You should see props `{ tenantId: string; artifactId: string; preview?, loading?, error? }` (the last three are `$bindable` and optional — leave them unbound in the inbox use). Confirmed: just pass `tenantId` and `artifactId`.

- [ ] **Step 4: Create the approval entry wrapper component**

Create `frontend/src/lib/components/inbox/InboxApprovalEntry.svelte`. This component is the full row+expansion for one approval; it owns the local state and uses `InboxRow`'s `actions` and `footer` snippets to place content correctly. The page no longer needs to know about expansion state for approvals.

```svelte
<script lang="ts">
  import { locale, translate } from "$lib/i18n";
  import { getTenant } from "$lib/auth";
  import { respondToApprovalRequest } from "$lib/plans/approvals";
  import { toUserMessage } from "$lib/connect-errors";
  import type { InboxApprovalItem } from "$lib/inbox/types";
  import InboxRow from "./InboxRow.svelte";
  import ArtifactPreview from "$lib/components/ArtifactPreview.svelte";

  interface Props {
    item: InboxApprovalItem;
  }
  let { item }: Props = $props();

  let expanded = $state(false);
  let submitting = $state(false);
  let decision = $state<"approved" | "rejected" | null>(null);
  let rejectReason = $state("");
  let rejectMode = $state(false);
  let errorMessage = $state<string | null>(null);

  const tenantId = $derived(getTenant()?.id ?? "");

  async function submit(approved: boolean) {
    const tenant = getTenant();
    if (!tenant?.id || submitting) return;
    if (!approved && rejectReason.trim().length === 0) {
      rejectMode = true;
      return;
    }
    submitting = true;
    errorMessage = null;
    try {
      await respondToApprovalRequest(
        tenant.id,
        item.id,
        approved,
        approved ? "" : rejectReason.trim(),
      );
      decision = approved ? "approved" : "rejected";
      expanded = false;
      rejectMode = false;
    } catch (e) {
      errorMessage = toUserMessage(e);
    } finally {
      submitting = false;
    }
  }

  const hasFooter = $derived(expanded || rejectMode || errorMessage !== null);
</script>

<InboxRow {item}>
  {#snippet actions()}
    {#if decision}
      <span class="text-[11px] font-semibold text-talon-gold">
        {translate(
          decision === "approved"
            ? "inbox.decision.approved"
            : "inbox.decision.rejected",
          $locale,
        )}
      </span>
    {:else}
      <button
        type="button"
        onclick={() => (expanded = !expanded)}
        disabled={submitting}
        class="rounded border border-plumage bg-transparent px-3 py-1.5 text-[11px] font-medium text-crown-ash hover:border-talon-gold hover:text-talon-gold disabled:opacity-50"
      >
        {translate(
          expanded ? "inbox.actions.hide" : "inbox.actions.preview",
          $locale,
        )}
      </button>
      <button
        type="button"
        onclick={() => submit(false)}
        disabled={submitting}
        class="rounded border border-plumage bg-transparent px-3 py-1.5 text-[11px] font-medium text-crown-ash hover:border-red-400 hover:text-red-400 disabled:opacity-50"
      >
        {translate("inbox.actions.reject", $locale)}
      </button>
      <button
        type="button"
        onclick={() => submit(true)}
        disabled={submitting}
        class="rounded border border-talon-gold bg-talon-gold px-3 py-1.5 text-[11px] font-semibold text-obsidian hover:opacity-90 disabled:opacity-50"
      >
        {translate(
          submitting ? "inbox.actions.submitting" : "inbox.actions.approve",
          $locale,
        )}
      </button>
    {/if}
  {/snippet}

  {#snippet footer()}
    {#if hasFooter}
      {#if expanded && tenantId && item.inputArtifactId}
        <div class="mt-3">
          <ArtifactPreview {tenantId} artifactId={item.inputArtifactId} />
        </div>
      {/if}
      {#if rejectMode}
        <textarea
          class="mt-3 w-full rounded border border-plumage bg-obsidian px-3 py-2 text-[12px] text-cream focus:border-talon-gold focus:outline-none"
          rows="2"
          placeholder={translate("inbox.rejectReason.placeholder", $locale)}
          bind:value={rejectReason}
        ></textarea>
      {/if}
      {#if errorMessage}
        <p class="mt-2 text-[11px] text-red-400">{errorMessage}</p>
      {/if}
    {/if}
  {/snippet}
</InboxRow>
```

Note: the `footer` snippet is passed regardless; Svelte renders it only when invoked. The `{#if hasFooter}` guard inside the snippet keeps the rendered output empty until any of the three reasons fires, so `InboxRow`'s footer divider doesn't appear in the default state. If you want the divider to also disappear when the footer is empty, change `InboxRow.svelte`'s `{#if footer}` to `{#if footer && hasFooter}` — but `hasFooter` lives on the entry, not the row, so prefer the pattern as written (footer-snippet-with-internal-guard) over plumbing more props through.

- [ ] **Step 5: Wire approval entries into the inbox page**

In `frontend/src/routes/inbox/+page.svelte`, REPLACE the entire approval branch of the actions snippet — the current `{:else}` arm AND its surrounding markup — so the approval case bypasses `InboxRow` and renders `InboxApprovalEntry` directly:

```svelte
{#each visible as item (item.id)}
  <li>
    {#if item.kind === "approval"}
      <InboxApprovalEntry {item} />
    {:else}
      <InboxRow {item}>
        {#snippet actions()}
          {#if item.kind === "elicitation"}
            <InboxElicitationActions {item} />
          {:else if item.kind === "feedback"}
            <InboxFeedbackActions {item} />
          {/if}
        {/snippet}
      </InboxRow>
    {/if}
  </li>
{/each}
```

And ADD the import:

```ts
import InboxApprovalEntry from "$lib/components/inbox/InboxApprovalEntry.svelte";
```

- [ ] **Step 6: Run type-check, tests, and format**

```bash
cd frontend && npx svelte-kit sync && npm run check && npm run test && npm run format && npm run lint
```

Expected: zero NEW type errors; tests pass; lint clean.

- [ ] **Step 7: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/lib/i18n/ frontend/src/lib/components/inbox/ frontend/src/routes/inbox/+page.svelte
git commit -m "feat(ux-m2): inline approval preview and approve/reject in the inbox"
```

---

## Task 6: Replace legacy oversight routes with /inbox redirects (delete list pages, update sections.ts, update tests, remove i18n keys)

**Files:**
- Create: `frontend/src/routes/oversee/+page.ts` — 302 redirect to `/inbox`
- Create: `frontend/src/routes/elicitations/+page.ts` — 302 redirect to `/inbox`
- Create: `frontend/src/routes/approvals/+page.ts` — 302 redirect to `/inbox`
- Delete: `frontend/src/routes/oversee/+page.svelte`
- Delete: `frontend/src/routes/elicitations/+page.svelte`
- Delete: `frontend/src/routes/approvals/+page.svelte`
- Modify: `frontend/src/lib/nav/sections.ts` — remove the three oversight entries, add one inbox entry
- Modify: `frontend/src/lib/nav/nav.test.ts` — update expected lists
- Modify: `frontend/src/lib/i18n/en.json` — remove `nav.oversee`, `nav.elicitations`, `nav.approvals`
- Modify: `frontend/src/lib/i18n/pt-BR.json` — lockstep
- Modify: `frontend/src/lib/i18n/hardcoded-copy.test.ts` — remove any references to deleted files

**Interfaces:**
- Produces: `/oversee`, `/elicitations`, `/approvals` 302-redirect to `/inbox`. The legacy nav exposes one `/inbox` entry using the existing `nav.needsYou` i18n key.

- [ ] **Step 1: Create the three redirect loaders**

Create `frontend/src/routes/oversee/+page.ts`:

```ts
import { redirect } from "@sveltejs/kit";
import type { PageLoad } from "./$types";

export const load: PageLoad = () => {
  throw redirect(302, "/inbox");
};
```

Create `frontend/src/routes/elicitations/+page.ts`:

```ts
import { redirect } from "@sveltejs/kit";
import type { PageLoad } from "./$types";

export const load: PageLoad = () => {
  throw redirect(302, "/inbox");
};
```

Create `frontend/src/routes/approvals/+page.ts`:

```ts
import { redirect } from "@sveltejs/kit";
import type { PageLoad } from "./$types";

export const load: PageLoad = () => {
  throw redirect(302, "/inbox");
};
```

- [ ] **Step 2: Delete the legacy list pages**

```bash
cd /home/thbertoldi/harpia
rm frontend/src/routes/oversee/+page.svelte
rm frontend/src/routes/elicitations/+page.svelte
rm frontend/src/routes/approvals/+page.svelte
```

- [ ] **Step 3: Update the legacy nav module**

In `frontend/src/lib/nav/sections.ts`, REMOVE the three entries:

```ts
{
  i18nKey: "nav.oversee",
  href: "/oversee",
  icon: Eye,
  visibleTo: "all",
},
{
  i18nKey: "nav.elicitations",
  href: "/elicitations",
  icon: MessagesSquare,
  visibleTo: "all",
},
{
  i18nKey: "nav.approvals",
  href: "/approvals",
  icon: ShieldCheck,
  visibleTo: "all",
},
```

REPLACE them (same position in the array) with one entry:

```ts
{
  i18nKey: "nav.needsYou",
  href: "/inbox",
  icon: Inbox,
  visibleTo: "all",
},
```

And UPDATE the imports at the top of the file — REMOVE `Eye`, `MessagesSquare`, `ShieldCheck` (verify they're not used elsewhere in the file first); ADD `Inbox`. The full new import line should be (verify against the existing imports):

```ts
import {
  Activity,
  BookOpen,
  Bot,
  Inbox,
  LayoutDashboard,
  Plug,
  ScrollText,
  Settings,
} from "lucide-svelte";
```

- [ ] **Step 4: Update the nav tests**

In `frontend/src/lib/nav/nav.test.ts`, the existing tests reference `/oversee`, `/elicitations`, `/approvals` as expected hrefs. Update each assertion to use `/inbox` once.

Specifically, the test at lines 100-114 (`describe("navSectionDefs")`) asserts:

```ts
expect(navSectionDefs.map((d) => d.href)).toEqual([
  "/",
  "/tasks/ongoing",
  "/oversee",
  "/elicitations",
  "/approvals",
  "/plans",
  "/integrations",
  "/audit",
  "/agents",
  "/settings",
]);
```

REPLACE that assertion with:

```ts
expect(navSectionDefs.map((d) => d.href)).toEqual([
  "/",
  "/tasks/ongoing",
  "/inbox",
  "/plans",
  "/integrations",
  "/audit",
  "/agents",
  "/settings",
]);
```

Update other tests in the same file that reference the deleted hrefs:
- The `"includes base sections for all roles"` test currently expects 10 sections for Leader and 6 for undefined. With three entries collapsing into one, those numbers become 8 and 4. Update them.
- The `"resolves labels via translate callback"` test enumerates the i18n keys. Remove the three deleted keys and add `"t:nav.needsYou"` in their position.

- [ ] **Step 5: Remove the deleted i18n keys**

In `frontend/src/lib/i18n/en.json`, DELETE the lines:

```json
  "nav.oversee": "Oversee",
  "nav.elicitations": "Elicitations",
  "nav.approvals": "Approvals",
```

In `frontend/src/lib/i18n/pt-BR.json`, DELETE the lines:

```json
  "nav.oversee": "Supervisão",
  "nav.elicitations": "Elicitações",
  "nav.approvals": "Aprovações",
```

The parity test will catch any drift.

- [ ] **Step 6: Update hardcoded-copy test if needed**

```bash
cd /home/thbertoldi/harpia && grep -n "oversee\|elicitations\|approvals" frontend/src/lib/i18n/hardcoded-copy.test.ts | head
```

If any deleted file is referenced in an allowlist or path list, remove the reference. If none, no changes needed.

- [ ] **Step 7: Verify no orphan references**

```bash
cd /home/thbertoldi/harpia && grep -rn "nav.oversee\|nav.elicitations\|nav.approvals" frontend/src 2>/dev/null
```

Expected: zero matches (i18n keys and nav-section i18nKeys are all gone).

```bash
grep -rn "from .\$lib/components/FeedbackBadge\|from .\$lib/components/ElicitationBadge\|from .\$lib/components/ApprovalBadge" frontend/src 2>/dev/null
```

Expected: zero matches (Task 3 already deleted these).

- [ ] **Step 8: Run full test suite, type-check, lint**

```bash
cd frontend && npx svelte-kit sync && npm run check && npm run test && npm run format && npm run lint
```

Expected: zero NEW type errors; all tests pass; lint clean.

- [ ] **Step 9: Commit**

```bash
cd /home/thbertoldi/harpia
git add -A frontend/src/routes/oversee frontend/src/routes/elicitations frontend/src/routes/approvals
git add frontend/src/lib/nav/ frontend/src/lib/i18n/
git commit -m "feat(ux-m2): redirect legacy oversight routes to /inbox; collapse nav"
```

---

## Task 7: Update detail-page back-links

**Files:**
- Modify: `frontend/src/routes/plans/executions/[executionId]/elicitations/[elicitationId]/+page.svelte` (back-link around line 194)
- Modify: `frontend/src/routes/plans/executions/[executionId]/approvals/[approvalId]/+page.svelte` (back-link around line 125)

**Interfaces:**
- Behavior change only: the back-link href changes from `/elicitations` / `/approvals` to `/inbox`. No new exports.

- [ ] **Step 1: Locate the elicitation detail back-link**

```bash
cd /home/thbertoldi/harpia && grep -n 'href=.*elicitations\|href=.*"/elicitations"' frontend/src/routes/plans/executions/\[executionId\]/elicitations/\[elicitationId\]/+page.svelte
```

Find the exact line that links back to `/elicitations`. It will look like `<a href="/elicitations" ...>` or a `goto("/elicitations")` call or a translated label rendered next to a left arrow icon.

- [ ] **Step 2: Update the elicitation detail back-link**

Edit `frontend/src/routes/plans/executions/[executionId]/elicitations/[elicitationId]/+page.svelte`:
- Change the `href` value from `/elicitations` to `/inbox`.
- If the rendered label uses the i18n key `nav.elicitations`, switch it to `nav.needsYou` (the i18n key is also gone after Task 6).
- If the label is hardcoded as a translated "Back to elicitations" string, switch it to `translate("inbox.title", $locale)` or another suitable existing key.

- [ ] **Step 3: Locate the approval detail back-link**

```bash
cd /home/thbertoldi/harpia && grep -n 'href=.*approvals\|href=.*"/approvals"' frontend/src/routes/plans/executions/\[executionId\]/approvals/\[approvalId\]/+page.svelte
```

- [ ] **Step 4: Update the approval detail back-link**

Same as Step 2 but for approvals: change `href` from `/approvals` to `/inbox`. Update label accordingly.

- [ ] **Step 5: Verify all detail pages compile and render**

```bash
cd frontend && npx svelte-kit sync && npm run check
```

Expected: zero NEW type errors.

- [ ] **Step 6: Run full lint and tests**

```bash
cd frontend && npm run format && npm run lint && npm run test
```

Expected: clean / passing.

- [ ] **Step 7: Commit**

```bash
cd /home/thbertoldi/harpia
git add frontend/src/routes/plans/executions/\[executionId\]/elicitations/\[elicitationId\]/+page.svelte
git add frontend/src/routes/plans/executions/\[executionId\]/approvals/\[approvalId\]/+page.svelte
git commit -m "feat(ux-m2): point elicitation+approval detail back-links to /inbox"
```

---

## Task 8: End-to-end verification

**Files:** None modified — manual + automated verification. Deliverable is a verification log committed at the end.

- [ ] **Step 1: Run the full automated suite**

```bash
cd frontend && npm run test 2>&1 | tail -5
cd frontend && npm run check 2>&1 | tail -15
cd frontend && npm run lint 2>&1 | tail -5
```

Expected: all tests pass; type-check at trunk baseline (no NEW errors beyond the 10 known pre-existing ones from M1's baseline); lint clean.

- [ ] **Step 2: Smoke-test the dev server (flag off)**

```bash
cd frontend && npm run dev -- --port 5179 > /tmp/harpia-dev-flag-off.log 2>&1 &
DEV=$!
sleep 8
echo "--- legacy redirects ---"
curl -s -o /dev/null -w "/oversee → HTTP %{http_code}\n" -L --max-redirs 0 http://localhost:5179/oversee
curl -s -o /dev/null -w "/elicitations → HTTP %{http_code}\n" -L --max-redirs 0 http://localhost:5179/elicitations
curl -s -o /dev/null -w "/approvals → HTTP %{http_code}\n" -L --max-redirs 0 http://localhost:5179/approvals
echo "--- inbox ---"
curl -s -o /dev/null -w "/inbox → HTTP %{http_code}\n" -L --max-redirs 0 http://localhost:5179/inbox
tail -3 /tmp/harpia-dev-flag-off.log
kill $DEV 2>/dev/null; wait $DEV 2>/dev/null
```

Expected: all routes return 302 (auth redirect to /login is fine — confirms the routes resolve through the SvelteKit router; the legacy-route 302 to /inbox composes with auth 302 — either way you get a 302 with no 500 in the dev log).

- [ ] **Step 3: Smoke-test the dev server (flag on)**

```bash
cd frontend && PUBLIC_FEATURE_UX_REALIGNMENT_M1=true npm run dev -- --port 5179 > /tmp/harpia-dev-flag-on.log 2>&1 &
DEV=$!
sleep 8
curl -s -o /dev/null -w "/inbox → HTTP %{http_code}\n" -L --max-redirs 0 http://localhost:5179/inbox
tail -3 /tmp/harpia-dev-flag-on.log
kill $DEV 2>/dev/null; wait $DEV 2>/dev/null
```

Expected: 302 to /login (auth path).

- [ ] **Step 4: Write the verification log**

Create `docs/superpowers/plans/2026-06-20-harpia-ux-realignment-m2-unified-inbox.verification.md`:

```markdown
# M2 Verification — <YYYY-MM-DD>

Branch: `feat/ux-realignment-m2-unified-inbox`
Plan: `docs/superpowers/plans/2026-06-20-harpia-ux-realignment-m2-unified-inbox.md`

## Automated checks (executed)

| Check | Result | Notes |
|---|---|---|
| `npm run test` | ✅ <N/N> passing | New tests: feedback counts, inbox aggregator, buckets |
| `npm run check` | ✅ trunk baseline, 0 new | |
| `npm run lint` | ✅ clean | |
| Dev server boot, flag off | ✅ | Vite ready, legacy routes → 302 → /inbox → 302 → /login |
| Dev server boot, flag on | ✅ | |

## Browser-driven verification (manual — required before merge)

Run `cd frontend && PUBLIC_FEATURE_UX_REALIGNMENT_M1=true npm run dev` and walk:

| Step | Expected |
|---|---|
| Log in as Leader (dev) and visit `/inbox` | Page renders header "Needs you · N pending", filter chips (All / Elicitations / Approvals / Feedback), and rows for pending items. |
| Visit `/oversee` directly | 302-redirects to `/inbox`. |
| Visit `/elicitations` directly | 302-redirects to `/inbox`. |
| Visit `/approvals` directly | 302-redirects to `/inbox`. |
| With pending approvals, click "Preview" on a row | Row expands inline to show the input artifact body; button label changes to "Hide". |
| Click "Approve" | RPC fires, row marks as "Approved" (greyed). |
| Click "Reject" without text | A textarea appears below the row asking for a reason. |
| Click "Reject" again with a reason | RPC fires, row marks as "Rejected". |
| Click "Open thread" on an elicitation row | Navigates to the existing detail page (graceful degrade — M3 will repoint). |
| Click the "Back" link on a detail page | Returns to `/inbox`, not the deleted list route. |
| Sidebar | Shows a single "Needs you" entry with a badge equal to total pending. The legacy persona sees the same single entry (no Oversee / Elicitations / Approvals separately). |

## Commits

(list git log range)
```

- [ ] **Step 5: Commit the verification log**

```bash
cd /home/thbertoldi/harpia
git add docs/superpowers/plans/2026-06-20-harpia-ux-realignment-m2-unified-inbox.verification.md
git commit -m "chore(ux-m2): record M2 verification notes"
```

- [ ] **Step 6: Confirm clean state**

```bash
cd frontend && npm run test && npm run check && npm run lint
```

Expected: all pass.

---

## Done criteria

M2 is complete when all of the following hold:

1. `/inbox` renders pending elicitations, approvals, and feedback in one combined sorted list with filter chips.
2. Approvals can be approved or rejected inline (with reason for reject) without leaving `/inbox`.
3. Approval rows can preview the input artifact inline (expand/hide).
4. Elicitation and Feedback rows offer "Open thread" that navigates to the existing detail pages (M3 will repoint).
5. `/oversee`, `/elicitations`, `/approvals` 302-redirect to `/inbox`.
6. The sidebar shows a single "Needs you" entry (legacy nav and M1 nav both).
7. The three legacy badge components are deleted; one new `InboxBadge` shows the combined count.
8. Detail-page back-links point to `/inbox`.
9. All tests pass (`npm run test`), type-check at trunk baseline (`npm run check`), lint passes (`npm run lint`).
10. The three legacy `nav.*` i18n keys (oversee/elicitations/approvals) are deleted; new `inbox.*` keys are added with en/pt parity.

After M2 ships, M3 (plan thread) can begin — the "Open thread" affordances on inbox rows are the natural integration point for the per-plan chat surface.
