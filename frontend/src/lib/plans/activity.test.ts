import { describe, expect, it } from "vitest";
import { mapStepRowsToActivity } from "./activity";
import { StepExecutionStatus } from "$lib/gen/harpia/plans/v1/plans_pb";

describe("mapStepRowsToActivity", () => {
  it("uses human labels for completed artifact-producing steps", () => {
    const items = mapStepRowsToActivity([
      {
        id: "step-1",
        index: 1,
        stepKey: "fetch-news",
        stepLabel: "Fetch News",
        status: StepExecutionStatus.COMPLETED,
        inputArtifactId: "",
        outputArtifactId: "artifact-news",
        approvalRequestId: "",
        createdAt: "2026-06-29T10:00:00Z",
        updatedAt: "2026-06-29T10:01:00Z",
        attempt: 1,
        canRetry: false,
      },
    ]);

    expect(items).toEqual([
      {
        id: "step-1",
        kind: "artifact_created",
        title: "Sources ready",
        body: "I found source material for the plan.",
        artifactId: "artifact-news",
        status: "completed",
        timestamp: "2026-06-29T10:01:00Z",
      },
    ]);
  });
});
