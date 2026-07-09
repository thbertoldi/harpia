import type { InboxApprovalItem } from "./types";

type ChatOrRunsPath = "/runs" | `/chat/${string}`;

function appendParams<T extends `/chat/${string}`>(
  path: T,
  params: Record<string, string>,
): T | `${T}?${string}` {
  const search = new URLSearchParams(
    Object.entries(params)
      .map(([key, value]): [string, string] => [key, value.trim()])
      .filter((entry) => entry[1] !== ""),
  );
  const encoded = search.toString();
  return (encoded ? `${path}?${encoded}` : path) as T | `${T}?${string}`;
}

export function chatPlanPath(
  threadId: string,
  configurationId: string,
  executionId = "",
): ChatOrRunsPath {
  const id = threadId.trim();
  if (!id) return "/runs";
  return appendParams(`/chat/${encodeURIComponent(id)}`, {
    plan: configurationId,
    execution: executionId,
  });
}

/**
 * DOM id / URL-hash anchor for an in-thread approval card. Inbox and runs deep
 * links append `#<approvalAnchorId>` so the chat page can scroll to the matching
 * APPROVAL_RAISED card; ApprovalRefCard renders the same id. Approval ids are
 * UUIDs (URL-safe), so the hash and the DOM id are byte-identical.
 */
export function approvalAnchorId(approvalId: string): string {
  return `m-approval-${approvalId.trim()}`;
}

export function inboxApprovalThreadPath(
  item: InboxApprovalItem,
): ChatOrRunsPath {
  const threadId = item.threadId.trim();
  if (!threadId) return "/runs";
  const path = chatPlanPath(
    threadId,
    item.configurationId,
    item.planExecutionId,
  );
  return `${path}#${approvalAnchorId(item.id)}` as ChatOrRunsPath;
}
