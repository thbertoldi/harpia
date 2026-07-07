import { describe, expect, it } from "vitest";
import { deriveIterationNumber, executionProducingOrdinal } from "./iteration";

function vm(
  executionId: string,
  runNumber: number,
  stepStatus: "pending" | "running" | "done" | "failed",
  outputArtifactId: string | null,
) {
  return {
    executionId,
    runNumber,
    steps: [
      { key: "draft", title: "Draft", status: stepStatus, outputArtifactId },
    ],
  };
}

describe("executionProducingOrdinal", () => {
  it("returns null when only one execution has produced the step's output", () => {
    const vms = [vm("exec-1", 1, "done", "artifact-1")];
    expect(executionProducingOrdinal(vms, "draft", "exec-1")).toBe(1);
  });

  it("orders producers by run number, not array order", () => {
    const vms = [
      vm("exec-2", 2, "done", "artifact-2"),
      vm("exec-1", 1, "done", "artifact-1"),
    ];
    expect(executionProducingOrdinal(vms, "draft", "exec-1")).toBe(1);
    expect(executionProducingOrdinal(vms, "draft", "exec-2")).toBe(2);
  });

  it("skips executions where the step never completed", () => {
    const vms = [
      vm("exec-1", 1, "done", "artifact-1"),
      vm("exec-2", 2, "running", null),
    ];
    expect(executionProducingOrdinal(vms, "draft", "exec-2")).toBeNull();
  });

  it("returns null for an unknown execution id", () => {
    const vms = [vm("exec-1", 1, "done", "artifact-1")];
    expect(executionProducingOrdinal(vms, "draft", "exec-9")).toBeNull();
  });
});

describe("deriveIterationNumber", () => {
  it("omits the badge when neither signal indicates more than one iteration", () => {
    expect(deriveIterationNumber(1, null)).toBeNull();
    expect(deriveIterationNumber(1, 1)).toBeNull();
  });

  it("prefers the content-revision count when it exceeds one", () => {
    expect(deriveIterationNumber(3, 1)).toBe(3);
  });

  it("falls back to the execution ordinal when there's no revision history", () => {
    expect(deriveIterationNumber(1, 2)).toBe(2);
    expect(deriveIterationNumber(0, 2)).toBe(2);
  });
});
