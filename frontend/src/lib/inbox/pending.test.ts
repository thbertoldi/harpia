import { describe, expect, it } from "vitest";
import { countPendingInbox, selectPendingItems } from "./pending";
import type { InboxItem } from "./types";

function elicitation(id: string): InboxItem {
  return {
    kind: "elicitation",
    id,
    createdAt: "2026-07-05T00:00:00Z",
    planName: "Plan",
    taskName: "Task",
    summary: "",
    planExecutionId: "pe",
    stepExecutionId: "se",
    configurationId: "cfg",
    raw: {} as never,
  };
}

function approval(id: string): InboxItem {
  return {
    kind: "approval",
    id,
    createdAt: "2026-07-05T00:00:00Z",
    planName: "Plan",
    taskName: "Task",
    summary: "",
    planExecutionId: "pe",
    stepExecutionId: "se",
    inputArtifactId: "art",
    configurationId: "cfg",
    raw: {} as never,
  };
}

describe("countPendingInbox / selectPendingItems", () => {
  it("counts every elicitation and approval in the batch", () => {
    const items = [elicitation("e1"), approval("a1"), elicitation("e2")];
    expect(countPendingInbox(items)).toBe(3);
    expect(selectPendingItems(items)).toHaveLength(3);
  });

  it("returns 0 for an empty inbox", () => {
    expect(countPendingInbox([])).toBe(0);
    expect(selectPendingItems([])).toEqual([]);
  });

  it("excludes kinds the inbox page does not render", () => {
    // InboxItem is today constrained to elicitation|approval, but the
    // predicate is the canonical funnel for when watchInbox broadens —
    // simulate a foreign kind to prove it is dropped from the count the
    // badge and the list would both show.
    const mixed = [
      approval("a1"),
      { kind: "notification", id: "n1" },
    ] as unknown as InboxItem[];
    expect(countPendingInbox(mixed)).toBe(1);
    expect(selectPendingItems(mixed).map((it) => it.id)).toEqual(["a1"]);
  });
});
