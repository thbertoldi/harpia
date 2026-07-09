import { describe, expect, it } from "vitest";
import type { InboxApprovalItem } from "./types";
import {
  approvalAnchorId,
  chatPlanPath,
  executionAnchorId,
  inboxApprovalThreadPath,
  runExecutionThreadPath,
} from "./links";

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
  approvalRequestId: "approval-1",
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

  it("derives the approval anchor id the chat card renders", () => {
    // The deep-link hash and the ApprovalRefCard element id must agree so the
    // chat page can scroll to the right card.
    expect(approvalAnchorId("approval-1")).toBe("m-approval-approval-1");
    expect(inboxApprovalThreadPath(item)).toContain(
      `#${approvalAnchorId(item.approvalRequestId)}`,
    );
  });

  it("falls back to runs when an approval has no canonical thread", () => {
    expect(inboxApprovalThreadPath({ ...item, threadId: "" })).toBe("/runs");
  });

  it("anchors runs execution links to the matching execution card", () => {
    expect(runExecutionThreadPath("thread-1", "config-1", "exec-1")).toBe(
      "/chat/thread-1?plan=config-1&execution=exec-1#m-execution-exec-1",
    );
    expect(runExecutionThreadPath("thread-1", "config-1", "exec-1")).toContain(
      `#${executionAnchorId("exec-1")}`,
    );
  });

  it("omits the execution anchor when there is no execution id", () => {
    expect(runExecutionThreadPath("thread-1", "config-1", "")).toBe(
      "/chat/thread-1?plan=config-1",
    );
  });

  it("falls back to runs when an execution link has no thread", () => {
    expect(runExecutionThreadPath("", "config-1", "exec-1")).toBe("/runs");
  });
});
