import { create } from "@bufbuild/protobuf";
import { describe, expect, it } from "vitest";
import {
  PlanExecutionSchema,
  StepExecutionSchema,
  StepExecutionStatus,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import {
  filterPlanExecutionsByView,
  formatStepDuration,
  mergeStepExecutions,
  statusKeyForPlanExecution,
  statusKeyForStepExecution,
  type PlanExecutionViewFilter,
} from "$lib/plans/plan-execution";
import { PlanExecutionStatus } from "$lib/rpc";

function makeStep(
  id: string,
  planStepKey: string,
  status: StepExecutionStatus,
  createdAt: string,
  updatedAt: string,
) {
  return create(StepExecutionSchema, {
    id,
    planExecutionId: "exec-1",
    planStepKey,
    status,
    attempt: 1,
    createdAt,
    updatedAt,
  });
}

function makeExecution(id: string, status: PlanExecutionStatus) {
  return create(PlanExecutionSchema, {
    id,
    tenantId: "tenant-1",
    planConfigurationId: "config-1",
    status,
    createdAt: "2026-06-15T15:00:00Z",
    updatedAt: "2026-06-15T15:00:00Z",
  });
}

describe("plan execution helpers", () => {
  it("merges step updates by id and keeps timeline order", () => {
    const existing = [
      makeStep(
        "step-a",
        "fetch-news",
        StepExecutionStatus.RUNNING,
        "2026-06-15T15:00:00Z",
        "2026-06-15T15:00:10Z",
      ),
      makeStep(
        "step-b",
        "write-draft",
        StepExecutionStatus.PENDING,
        "2026-06-15T15:01:00Z",
        "2026-06-15T15:01:00Z",
      ),
    ];
    const updates = [
      makeStep(
        "step-a",
        "fetch-news",
        StepExecutionStatus.COMPLETED,
        "2026-06-15T15:00:00Z",
        "2026-06-15T15:00:40Z",
      ),
      makeStep(
        "step-c",
        "adapt-for-linkedin",
        StepExecutionStatus.RUNNING,
        "2026-06-15T15:02:00Z",
        "2026-06-15T15:02:10Z",
      ),
    ];

    const merged = mergeStepExecutions(existing, updates);

    expect(merged).toHaveLength(3);
    expect(merged[0].id).toBe("step-a");
    expect(merged[0].status).toBe(StepExecutionStatus.COMPLETED);
    expect(merged[2].id).toBe("step-c");
  });

  it("computes step duration from created/updated timestamps", () => {
    const step = makeStep(
      "step-1",
      "fetch-news",
      StepExecutionStatus.COMPLETED,
      "2026-06-15T15:00:00Z",
      "2026-06-15T15:01:30Z",
    );
    expect(formatStepDuration(step)).toBe(90_000);
  });

  it("maps execution and step status keys for i18n", () => {
    expect(statusKeyForPlanExecution(PlanExecutionStatus.RUNNING)).toBe(
      "executions.planStatus.running",
    );
    expect(
      statusKeyForStepExecution(StepExecutionStatus.AWAITING_APPROVAL),
    ).toBe("executions.stepStatus.awaitingApproval");
  });

  it.each<[PlanExecutionViewFilter, string[]]>([
    ["all", ["pending", "running", "completed", "failed", "cancelled"]],
    ["running", ["pending", "running"]],
    ["history", ["completed", "cancelled"]],
    ["failed", ["failed"]],
  ])("filters executions for the %s view", (view, expectedIds) => {
    const executions = [
      makeExecution("pending", PlanExecutionStatus.PENDING),
      makeExecution("running", PlanExecutionStatus.RUNNING),
      makeExecution("completed", PlanExecutionStatus.COMPLETED),
      makeExecution("failed", PlanExecutionStatus.FAILED),
      makeExecution("cancelled", PlanExecutionStatus.CANCELLED),
    ];

    expect(
      filterPlanExecutionsByView(executions, view).map((e) => e.id),
    ).toEqual(expectedIds);
  });
});
