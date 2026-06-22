import { describe, expect, it } from "vitest";
import { buildCanvasState } from "./canvas-state";
import type { ChatMessage } from "$lib/chat/types";
import type { PlanStep, SlotBinding } from "$lib/gen/harpia/plans/v1/plans_pb";

function msg(
  id: string,
  kind: ChatMessage["kind"],
  payload: Record<string, unknown>,
  sequenceNumber: number,
): ChatMessage {
  return {
    id,
    tenantId: "tenant-1",
    threadId: "config-1",
    executionId: "exec-1",
    role: "SYSTEM",
    kind,
    text: id,
    payloadJson: JSON.stringify(payload),
    authorUserId: "",
    sequenceNumber: BigInt(sequenceNumber),
    createdAt: `2026-06-22T10:0${sequenceNumber}:00Z`,
  };
}

function step(key: string): PlanStep {
  return { key } as PlanStep;
}

function binding(stepKey: string, installationId: string): SlotBinding {
  return { stepKey, executorInstallationId: installationId } as SlotBinding;
}

describe("buildCanvasState", () => {
  it("seeds 'unbound' for every step when no messages and no slot bindings exist", () => {
    const state = buildCanvasState([], [step("a"), step("b"), step("c")]);
    expect(state.a.status).toBe("unbound");
    expect(state.b.status).toBe("unbound");
    expect(state.c.status).toBe("unbound");
  });

  it("seeds 'bound' for steps with an executor installation in slot bindings", () => {
    const state = buildCanvasState(
      [],
      [step("a"), step("b")],
      [binding("a", "inst-a")],
    );
    expect(state.a.status).toBe("bound");
    expect(state.b.status).toBe("unbound");
  });

  it("returns 'running' for a step with STEP_STARTED and no STEP_BOUND", () => {
    const messages = [
      msg(
        "m1",
        "STEP_STARTED",
        { step_key: "a", step_execution_id: "se-a" },
        1,
      ),
    ];
    const state = buildCanvasState(messages, [step("a"), step("b")]);
    expect(state.a.status).toBe("running");
    expect(state.a.stepExecutionId).toBe("se-a");
    expect(state.a.startedAt).toBe("2026-06-22T10:01:00Z");
    // b has no execution event and no slot binding passed in → unbound.
    expect(state.b.status).toBe("unbound");
  });

  it("returns 'done' for a step with STEP_BOUND, carrying outputArtifactId", () => {
    const messages = [
      msg(
        "m1",
        "STEP_STARTED",
        { step_key: "a", step_execution_id: "se-a" },
        1,
      ),
      msg(
        "m2",
        "STEP_BOUND",
        { step_key: "a", output_artifact_id: "art-x" },
        2,
      ),
    ];
    const state = buildCanvasState(messages, [step("a")]);
    expect(state.a.status).toBe("done");
    expect(state.a.outputArtifactId).toBe("art-x");
    expect(state.a.completedAt).toBe("2026-06-22T10:02:00Z");
  });

  it("returns 'awaiting_elicitation' when ELICITATION_RAISED is the latest event for a step", () => {
    const messages = [
      msg(
        "m1",
        "STEP_STARTED",
        { step_key: "a", step_execution_id: "se-a" },
        1,
      ),
      msg("m2", "ELICITATION_RAISED", { elicitation_id: "el-1" }, 2),
    ];
    const state = buildCanvasState(messages, [step("a")]);
    expect(state.a.status).toBe("awaiting_elicitation");
    expect(state.a.pendingElicitationId).toBe("el-1");
  });

  it("clears the elicitation pointer when ELICITATION_ANSWERED arrives", () => {
    const messages = [
      msg(
        "m1",
        "STEP_STARTED",
        { step_key: "a", step_execution_id: "se-a" },
        1,
      ),
      msg("m2", "ELICITATION_RAISED", { elicitation_id: "el-1" }, 2),
      msg("m3", "ELICITATION_ANSWERED", { elicitation_id: "el-1" }, 3),
    ];
    const state = buildCanvasState(messages, [step("a")]);
    expect(state.a.status).toBe("running");
    expect(state.a.pendingElicitationId).toBeUndefined();
  });

  it("returns 'awaiting_approval' when APPROVAL_RAISED is pending for a step", () => {
    const messages = [
      msg(
        "m1",
        "STEP_STARTED",
        { step_key: "publish", step_execution_id: "se-p" },
        1,
      ),
      msg("m2", "APPROVAL_RAISED", { approval_request_id: "ap-1" }, 2),
    ];
    const state = buildCanvasState(messages, [step("publish")]);
    expect(state.publish.status).toBe("awaiting_approval");
    expect(state.publish.pendingApprovalId).toBe("ap-1");
  });

  it("returns 'failed' when RUN_FAILED is the run-level event and the step was running", () => {
    const messages = [
      msg(
        "m1",
        "STEP_STARTED",
        { step_key: "a", step_execution_id: "se-a" },
        1,
      ),
      msg("m2", "RUN_FAILED", { error: "step a failed: timeout" }, 2),
    ];
    const state = buildCanvasState(messages, [step("a")]);
    expect(state.a.status).toBe("failed");
    expect(state.a.errorMessage).toBe("step a failed: timeout");
  });

  it("payload pointers (elicitation/approval) require the step to be running first", () => {
    const messages = [
      msg("m1", "ELICITATION_RAISED", { elicitation_id: "el-orphan" }, 1),
    ];
    // Orphan pointer with no STEP_STARTED leaves the step at its seed status
    // (unbound here since no slot bindings were passed).
    const state = buildCanvasState(messages, [step("a")]);
    expect(state.a.status).toBe("unbound");
  });

  it("duplicate STEP_STARTED does not wipe awaiting_elicitation pointer", () => {
    const messages = [
      msg(
        "m1",
        "STEP_STARTED",
        { step_key: "a", step_execution_id: "se-a" },
        1,
      ),
      msg("m2", "ELICITATION_RAISED", { elicitation_id: "el-1" }, 2),
      msg(
        "m3",
        "STEP_STARTED",
        { step_key: "a", step_execution_id: "se-a" },
        3,
      ),
    ];
    const state = buildCanvasState(messages, [step("a")]);
    expect(state.a.status).toBe("awaiting_elicitation");
    expect(state.a.pendingElicitationId).toBe("el-1");
  });
});
