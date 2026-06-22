import { describe, expect, it } from "vitest";
import { buildThreadSections } from "./thread";
import type { ChatMessage } from "$lib/chat/types";

function msg(
  id: string,
  executionId: string,
  kind: ChatMessage["kind"],
  sequenceNumber: number,
): ChatMessage {
  return {
    id,
    tenantId: "tenant-1",
    threadId: "config-1",
    executionId,
    role: kind === "USER_TEXT" ? "OVERSEER" : "SYSTEM",
    kind,
    text: id,
    payloadJson: "{}",
    authorUserId: "",
    sequenceNumber: BigInt(sequenceNumber),
    createdAt: `2026-06-21T10:0${sequenceNumber}:00Z`,
  };
}

describe("buildThreadSections", () => {
  it("returns an empty array for no messages", () => {
    expect(buildThreadSections([])).toEqual([]);
  });

  it("renders plan-scope messages as plan-scope sections in chronological order", () => {
    const messages = [
      msg("m1", "", "CONFIGURATION_SAVED", 1),
      msg("m2", "", "USER_TEXT", 2),
    ];
    const sections = buildThreadSections(messages);
    expect(sections).toHaveLength(2);
    expect(sections[0].kind).toBe("plan-scope");
    expect(sections[1].kind).toBe("plan-scope");
  });

  it("groups consecutive same-executionId messages into one ExecutionGroup", () => {
    const messages = [
      msg("m1", "", "CONFIGURATION_SAVED", 1),
      msg("m2", "exec-A", "RUN_STARTED", 2),
      msg("m3", "exec-A", "STEP_BOUND", 3),
      msg("m4", "exec-A", "RUN_COMPLETED", 4),
    ];
    const sections = buildThreadSections(messages);
    expect(sections).toHaveLength(2);
    expect(sections[0].kind).toBe("plan-scope");
    expect(sections[1].kind).toBe("execution");
    if (sections[1].kind === "execution") {
      expect(sections[1].group.executionId).toBe("exec-A");
      expect(sections[1].group.runNumber).toBe(1);
      expect(sections[1].group.messages).toHaveLength(3);
      expect(sections[1].group.status).toBe("completed");
    }
  });

  it("numbers runs in chronological appearance order", () => {
    const messages = [
      msg("m1", "exec-A", "RUN_STARTED", 1),
      msg("m2", "exec-A", "RUN_COMPLETED", 2),
      msg("m3", "exec-B", "RUN_STARTED", 3),
      msg("m4", "exec-B", "RUN_FAILED", 4),
    ];
    const sections = buildThreadSections(messages);
    expect(sections).toHaveLength(2);
    if (sections[0].kind === "execution") {
      expect(sections[0].group.runNumber).toBe(1);
      expect(sections[0].group.status).toBe("completed");
    }
    if (sections[1].kind === "execution") {
      expect(sections[1].group.runNumber).toBe(2);
      expect(sections[1].group.status).toBe("failed");
    }
  });

  it("derives status from the latest terminal message in the group; otherwise 'running'", () => {
    const running = [
      msg("m1", "exec-A", "RUN_STARTED", 1),
      msg("m2", "exec-A", "STEP_BOUND", 2),
    ];
    const sections = buildThreadSections(running);
    if (sections[0].kind === "execution") {
      expect(sections[0].group.status).toBe("running");
    }
  });

  it("re-groups when execution_id changes back (e.g. inter-execution plan-scope message)", () => {
    const messages = [
      msg("m1", "exec-A", "RUN_STARTED", 1),
      msg("m2", "exec-A", "RUN_COMPLETED", 2),
      msg("m3", "", "USER_TEXT", 3),
      msg("m4", "exec-B", "RUN_STARTED", 4),
    ];
    const sections = buildThreadSections(messages);
    expect(sections).toHaveLength(3);
    expect(sections[0].kind).toBe("execution");
    expect(sections[1].kind).toBe("plan-scope");
    expect(sections[2].kind).toBe("execution");
  });
});
