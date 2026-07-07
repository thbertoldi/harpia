import type { InboxApprovalItem } from "./types";

function appendParams(path: string, params: Record<string, string>): string {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    const trimmed = value.trim();
    if (trimmed) search.set(key, trimmed);
  }
  const encoded = search.toString();
  return encoded ? `${path}?${encoded}` : path;
}

export function chatPlanPath(
  threadId: string,
  configurationId: string,
  executionId = "",
): string {
  const id = threadId.trim();
  if (!id) return "/runs";
  return appendParams(`/chat/${encodeURIComponent(id)}`, {
    plan: configurationId,
    execution: executionId,
  });
}

export function inboxApprovalThreadPath(item: InboxApprovalItem): string {
  const threadId = item.threadId || item.configurationId;
  if (!threadId) return "/runs";
  const path = chatPlanPath(
    threadId,
    item.configurationId,
    item.planExecutionId,
  );
  return `${path}#m-approval-${encodeURIComponent(item.id)}`;
}
