import { describe, expect, it } from "vitest";
import { StepExecutionStatus } from "$lib/gen/harpia/plans/v1/plans_pb";
import { mockPlanExecutionDetail } from "$lib/mocks/plan-executions";
import {
  isRetryVisible,
  mapStepExecutionsToDisplayRows,
} from "./plan-execution-detail";

describe("plan execution detail mapping", () => {
  it("maps step executions with artifact ids", () => {
    const { execution, stepExecutions } = mockPlanExecutionDetail();
    const rows = mapStepExecutionsToDisplayRows(execution, stepExecutions);

    expect(rows).toHaveLength(3);
    expect(rows[0]?.inputArtifactId).toBe("artifact-date-range");
    expect(rows[0]?.outputArtifactId).toBe("artifact-news-list");
    expect(rows[1]?.inputArtifactId).toBe("artifact-news-list");
    expect(rows[1]?.outputArtifactId).toBe("artifact-text-draft");
  });

  it("shows retry only for failed steps", () => {
    expect(isRetryVisible(StepExecutionStatus.FAILED)).toBe(true);
    expect(isRetryVisible(StepExecutionStatus.COMPLETED)).toBe(false);
    expect(isRetryVisible(StepExecutionStatus.RUNNING)).toBe(false);
    expect(isRetryVisible(StepExecutionStatus.PENDING)).toBe(false);
  });
});
