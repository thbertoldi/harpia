import { describe, expect, it } from "vitest";
import type { ChatMessage } from "$lib/chat/types";
import {
  buildExecutionViewModel,
  parseApprovalPayload,
  parseStepKey,
  parseOutputArtifactId,
} from "$lib/plans/execution-view";
import type { OrderedStep } from "$lib/plans/execution-view";

const STEPS: OrderedStep[] = [
  { key: "fetch", title: "Fetch" },
  { key: "draft", title: "Draft" },
  { key: "publish", title: "Publish" },
];

let seq = 0;
function msg(
  kind: ChatMessage["kind"],
  opts: { exec?: string; payload?: string; seq?: number } = {},
): ChatMessage {
  const s = opts.seq ?? ++seq;
  return {
    id: `m-${s}`,
    tenantId: "t",
    threadId: "th",
    executionId: opts.exec ?? "exec-1",
    role: "SYSTEM",
    kind,
    text: "",
    payloadJson: opts.payload ?? "",
    authorUserId: "",
    sequenceNumber: BigInt(s),
    createdAt: "",
  };
}
const started = (key: string, s?: number) =>
  msg("STEP_STARTED", {
    payload: JSON.stringify({ step_key: key, step_execution_id: `${key}-se` }),
    seq: s,
  });
const bound = (key: string, s?: number) =>
  msg("STEP_BOUND", {
    payload: JSON.stringify({ step_key: key, output_artifact_id: `${key}-a` }),
    seq: s,
  });

function reset() {
  seq = 0;
}

describe("parseStepKey", () => {
  it("reads step_key from a well-formed payload", () => {
    expect(parseStepKey('{"step_key":"draft","step_execution_id":"x"}')).toBe(
      "draft",
    );
  });
  it("returns null for malformed json", () => {
    expect(parseStepKey("{not json")).toBeNull();
  });
  it("returns null when step_key is absent", () => {
    expect(parseStepKey('{"foo":"bar"}')).toBeNull();
  });
  it("returns null for empty input", () => {
    expect(parseStepKey("")).toBeNull();
  });
});

describe("parseOutputArtifactId", () => {
  it("reads output_artifact_id from a well-formed STEP_BOUND payload", () => {
    expect(
      parseOutputArtifactId(
        '{"step_key":"draft","output_artifact_id":"art-1"}',
      ),
    ).toBe("art-1");
  });
  it("returns null for malformed json", () => {
    expect(parseOutputArtifactId("{not json")).toBeNull();
  });
  it("returns null when output_artifact_id is absent", () => {
    expect(parseOutputArtifactId('{"step_key":"draft"}')).toBeNull();
  });
  it("returns null when output_artifact_id is empty", () => {
    expect(parseOutputArtifactId('{"output_artifact_id":""}')).toBeNull();
  });
  it("returns null for empty input", () => {
    expect(parseOutputArtifactId("")).toBeNull();
  });
});

describe("parseApprovalPayload", () => {
  it("reads approval context from an enriched APPROVAL_RAISED payload", () => {
    expect(
      parseApprovalPayload(
        JSON.stringify({
          approval_request_id: "approval-1",
          input_artifact_id: "artifact-linkedin-draft",
          plan_step_key: "publish-linkedin",
        }),
      ),
    ).toEqual({
      approvalRequestId: "approval-1",
      approved: null,
      inputArtifactId: "artifact-linkedin-draft",
      planStepKey: "publish-linkedin",
    });
  });

  it("reads approval decisions", () => {
    expect(
      parseApprovalPayload(
        JSON.stringify({ approval_request_id: "approval-1", approved: true }),
      )?.approved,
    ).toBe(true);
  });
});

describe("buildExecutionViewModel", () => {
  it("is idle with all steps pending before any step event", () => {
    reset();
    const vm = buildExecutionViewModel(
      { executionId: "exec-1", runNumber: 1, messages: [] },
      STEPS,
    );
    expect(vm.state).toBe("idle");
    expect(vm.steps.map((s) => s.status)).toEqual([
      "pending",
      "pending",
      "pending",
    ]);
    expect(vm.doneCount).toBe(0);
    expect(vm.progress).toBe(0);
    expect(vm.runningStep).toBeNull();
    expect(vm.degraded).toBe(false);
  });

  it("marks a step running on STEP_STARTED and counts it as half progress", () => {
    reset();
    const vm = buildExecutionViewModel(
      {
        executionId: "exec-1",
        runNumber: 1,
        messages: [msg("RUN_STARTED"), started("draft")],
      },
      STEPS,
    );
    expect(vm.state).toBe("running");
    expect(vm.steps.map((s) => s.status)).toEqual([
      "pending",
      "running",
      "pending",
    ]);
    expect(vm.runningStep?.key).toBe("draft");
    expect(vm.doneCount).toBe(0);
    // (0 done + 0.5 running) / 3
    expect(vm.progress).toBeCloseTo(1 / 6, 5);
  });

  it("progresses pending -> running -> done across sequential steps", () => {
    reset();
    const vm = buildExecutionViewModel(
      {
        executionId: "exec-1",
        runNumber: 1,
        messages: [
          msg("RUN_STARTED"),
          started("fetch"),
          bound("fetch"),
          started("draft"),
          bound("draft"),
        ],
      },
      STEPS,
    );
    expect(vm.steps.map((s) => s.status)).toEqual(["done", "done", "pending"]);
    expect(vm.steps.map((s) => s.outputArtifactId)).toEqual([
      "fetch-a",
      "draft-a",
      null,
    ]);
    expect(vm.doneCount).toBe(2);
    expect(vm.progress).toBeCloseTo(2 / 3, 5);
  });

  it("carries output artifact ids onto completed steps", () => {
    reset();
    const vm = buildExecutionViewModel(
      {
        executionId: "exec-1",
        runNumber: 1,
        messages: [
          msg("RUN_STARTED"),
          started("fetch"),
          msg("STEP_BOUND", {
            payload: JSON.stringify({
              step_key: "fetch",
              output_artifact_id: "artifact-news-list",
            }),
          }),
        ],
      },
      STEPS,
    );

    expect(vm.steps[0].status).toBe("done");
    expect(vm.steps[0].outputArtifactId).toBe("artifact-news-list");
  });

  it("RUN_COMPLETED settles any still-running step to done", () => {
    reset();
    const vm = buildExecutionViewModel(
      {
        executionId: "exec-1",
        runNumber: 1,
        messages: [msg("RUN_STARTED"), started("fetch"), msg("RUN_COMPLETED")],
      },
      STEPS,
    );
    expect(vm.state).toBe("completed");
    expect(vm.steps[0].status).toBe("done");
    expect(vm.doneCount).toBe(1);
    expect(vm.progress).toBeCloseTo(1 / 3, 5);
  });

  it("RUN_FAILED marks the running step failed and earlier steps stay done", () => {
    reset();
    const vm = buildExecutionViewModel(
      {
        executionId: "exec-1",
        runNumber: 1,
        messages: [
          msg("RUN_STARTED"),
          started("fetch"),
          bound("fetch"),
          started("draft"),
          msg("RUN_FAILED", { payload: JSON.stringify({ error: "boom" }) }),
        ],
      },
      STEPS,
    );
    expect(vm.state).toBe("failed");
    expect(vm.steps.map((s) => s.status)).toEqual([
      "done",
      "failed",
      "pending",
    ]);
    expect(vm.failedStep?.key).toBe("draft");
  });

  it("settles the prior step when the next STEP_STARTED arrives before its STEP_BOUND", () => {
    reset();
    // fetch STEP_STARTED, then draft STEP_STARTED arrives WITHOUT a fetch
    // STEP_BOUND (out-of-order / missing) — fetch must be settled to done.
    const vm = buildExecutionViewModel(
      {
        executionId: "exec-1",
        runNumber: 1,
        messages: [msg("RUN_STARTED"), started("fetch"), started("draft")],
      },
      STEPS,
    );
    expect(vm.steps.map((s) => s.status)).toEqual([
      "done",
      "running",
      "pending",
    ]);
  });

  it("isolates executions: two groups fold independently", () => {
    reset();
    // Execution 1 already completed its first step; execution 2 just started.
    const exec1 = buildExecutionViewModel(
      {
        executionId: "exec-1",
        runNumber: 1,
        messages: [msg("RUN_STARTED"), started("fetch"), bound("fetch")],
      },
      STEPS,
    );
    const exec2 = buildExecutionViewModel(
      {
        executionId: "exec-2",
        runNumber: 2,
        messages: [msg("RUN_STARTED", { exec: "exec-2" }), started("draft")],
      },
      STEPS,
    );
    expect(exec1.steps[0].status).toBe("done");
    expect(exec1.steps[1].status).toBe("pending");
    expect(exec2.steps[0].status).toBe("pending");
    expect(exec2.steps[1].status).toBe("running");
    expect(exec1.executionId).toBe("exec-1");
    expect(exec2.runNumber).toBe(2);
  });

  it("ignores unknown step keys without throwing", () => {
    reset();
    const vm = buildExecutionViewModel(
      {
        executionId: "exec-1",
        runNumber: 1,
        messages: [
          msg("RUN_STARTED"),
          started("does-not-exist"),
          msg("RUN_COMPLETED"),
        ],
      },
      STEPS,
    );
    expect(vm.state).toBe("completed");
    expect(vm.steps.map((s) => s.status)).toEqual([
      "pending",
      "pending",
      "pending",
    ]);
    expect(vm.degraded).toBe(false);
  });

  it("degrades to summary-only when no ordered steps are available", () => {
    reset();
    const vm = buildExecutionViewModel(
      {
        executionId: "exec-1",
        runNumber: 1,
        messages: [msg("RUN_STARTED"), started("draft")],
      },
      [],
    );
    expect(vm.degraded).toBe(true);
    expect(vm.total).toBe(0);
    expect(vm.progress).toBe(0);
    expect(vm.state).toBe("running");
  });

  it("parses STEP_STARTED before RUN_STARTED (no run event yet) as running", () => {
    reset();
    const vm = buildExecutionViewModel(
      {
        executionId: "exec-1",
        runNumber: 1,
        messages: [started("fetch")],
      },
      STEPS,
    );
    expect(vm.state).toBe("running");
    expect(vm.steps[0].status).toBe("running");
  });

  it("surfaces pending approval requests with their input artifact", () => {
    reset();
    const approval = msg("APPROVAL_RAISED", {
      payload: JSON.stringify({
        approval_request_id: "approval-1",
        input_artifact_id: "artifact-linkedin-draft",
        plan_step_key: "publish-linkedin",
      }),
    });
    const vm = buildExecutionViewModel(
      {
        executionId: "exec-1",
        runNumber: 1,
        messages: [msg("RUN_STARTED"), approval],
      },
      STEPS,
    );

    expect(vm.pendingApproval?.approvalRequestId).toBe("approval-1");
    expect(vm.pendingApproval?.inputArtifactId).toBe("artifact-linkedin-draft");
    expect(vm.pendingApproval?.message).toBe(approval);
  });

  it("clears pending approval state after a decision", () => {
    reset();
    const vm = buildExecutionViewModel(
      {
        executionId: "exec-1",
        runNumber: 1,
        messages: [
          msg("APPROVAL_RAISED", {
            payload: JSON.stringify({ approval_request_id: "approval-1" }),
          }),
          msg("APPROVAL_DECIDED", {
            payload: JSON.stringify({
              approval_request_id: "approval-1",
              approved: true,
            }),
          }),
        ],
      },
      STEPS,
    );

    expect(vm.pendingApproval).toBeNull();
    expect(vm.approvals[0]?.status).toBe("approved");
  });
});
