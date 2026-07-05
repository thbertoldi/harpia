import type { InboxItem } from "./types";

/**
 * The subset of inbox items both the shell badge and the `/inbox` list treat
 * as "pending / needs you": pending approvals + elicitations.
 *
 * `watchInbox` feeds other consumers too and the raw batch length is not
 * guaranteed to equal the number of rows the inbox page actually renders.
 * Routing both surfaces through this single predicate is what keeps the
 * badge count and the rendered list in lockstep — change what counts as
 * "pending" here and both follow.
 */
export function selectPendingItems(items: InboxItem[]): InboxItem[] {
  return items.filter(
    (it) => it.kind === "elicitation" || it.kind === "approval",
  );
}

/** Number of items the badge should show / the inbox list should render. */
export function countPendingInbox(items: InboxItem[]): number {
  return selectPendingItems(items).length;
}
