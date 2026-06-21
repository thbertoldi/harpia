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
      elicit("today-late", "2026-06-20T18:00:00Z"), // 15:00 BRT June 20
      elicit("today-early", "2026-06-20T11:00:00Z"), // 08:00 BRT June 20
      elicit("yesterday", "2026-06-20T01:00:00Z"), // 22:00 BRT June 19
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
