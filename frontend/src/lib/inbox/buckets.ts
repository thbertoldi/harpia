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
