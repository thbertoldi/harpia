import { describe, expect, it } from "vitest";
import {
  mockPlanExecution,
  nextMockPlanExecution,
  resetMockPlanExecution,
} from "$lib/mocks/plan-executions";
import { PlanExecutionStatus, StepExecutionStatus } from "$lib/rpc";

describe("mock plan execution stream", () => {
  it("progresses deterministically from running to failed sequence", () => {
    const executionId = "demo-sequence";
    resetMockPlanExecution(executionId);

    const initial = mockPlanExecution(executionId);
    const firstTick = nextMockPlanExecution(executionId);
    const finalTick = [1, 2, 3, 4].reduce(
      (current) => nextMockPlanExecution(current.id),
      firstTick,
    );

    expect(initial.status).toBe(PlanExecutionStatus.PENDING);
    expect(firstTick.status).toBe(PlanExecutionStatus.RUNNING);
    expect(
      finalTick.stepExecutions.some(
        (step) => step.status === StepExecutionStatus.FAILED,
      ),
    ).toBe(true);
    expect(finalTick.status).toBe(PlanExecutionStatus.FAILED);
  });
});
