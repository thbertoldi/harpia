import { describe, expect, it } from "vitest";
import type { InboxApprovalItem } from "./types";
import { chatPlanPath, inboxApprovalThreadPath } from "./links";

const item = {
  id: "approval-1",
  kind: "approval",
  createdAt: "2026-07-07T10:00:00Z",
  planName: "Plan",
  taskName: "publish-linkedin",
  summary: "",
  planExecutionId: "exec-1",
  stepExecutionId: "step-1",
  inputArtifactId: "artifact-1",
  configurationId: "config-1",
  threadId: "thread-1",
  raw: {},
} as InboxApprovalItem;

describe("inbox links", () => {
  it("builds chat plan links with execution context", () => {
    expect(chatPlanPath("thread-1", "config-1", "exec-1")).toBe(
      "/chat/thread-1?plan=config-1&execution=exec-1",
    );
  });

  it("anchors approval links to the matching chat approval card", () => {
    expect(inboxApprovalThreadPath(item)).toBe(
      "/chat/thread-1?plan=config-1&execution=exec-1#m-approval-approval-1",
    );
  });

  it("falls back to configuration id for older approval responses", () => {
    expect(inboxApprovalThreadPath({ ...item, threadId: "" })).toBe(
      "/chat/config-1?plan=config-1&execution=exec-1#m-approval-approval-1",
    );
  });
});
