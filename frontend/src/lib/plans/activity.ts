import { StepExecutionStatus } from "$lib/gen/harpia/plans/v1/plans_pb";
import type { StepExecutionDisplayRow } from "$lib/plans/plan-execution-detail";

export type PlanActivityKind =
  | "step_running"
  | "artifact_created"
  | "approval_waiting"
  | "step_failed";

export interface PlanActivityItem {
  id: string;
  kind: PlanActivityKind;
  title: string;
  body: string;
  artifactId: string;
  status: "running" | "completed" | "waiting" | "failed";
  timestamp: string;
}

function completedTitle(stepKey: string): string {
  switch (stepKey) {
    case "fetch-news":
      return "Sources ready";
    case "write-draft":
      return "Draft ready";
    case "adapt-for-linkedin":
      return "LinkedIn post ready";
    case "publish-linkedin":
      return "Publish confirmation ready";
    default:
      return "Step completed";
  }
}

function completedBody(stepKey: string): string {
  switch (stepKey) {
    case "fetch-news":
      return "I found source material for the plan.";
    case "write-draft":
      return "I drafted the long-form content.";
    case "adapt-for-linkedin":
      return "I adapted the draft for LinkedIn.";
    case "publish-linkedin":
      return "The publish step produced a confirmation.";
    default:
      return "The step finished.";
  }
}

export function mapStepRowsToActivity(
  rows: StepExecutionDisplayRow[],
): PlanActivityItem[] {
  return rows.map((row) => {
    if (row.status === StepExecutionStatus.FAILED) {
      return {
        id: row.id,
        kind: "step_failed",
        title: `${row.stepLabel} failed`,
        body: "Open details to inspect the error and retry when available.",
        artifactId: row.outputArtifactId,
        status: "failed",
        timestamp: row.updatedAt || row.createdAt,
      };
    }
    if (row.status === StepExecutionStatus.AWAITING_APPROVAL) {
      return {
        id: row.id,
        kind: "approval_waiting",
        title: "Ready for approval",
        body: "Review the generated asset before continuing.",
        artifactId: row.inputArtifactId || row.outputArtifactId,
        status: "waiting",
        timestamp: row.updatedAt || row.createdAt,
      };
    }
    if (row.status === StepExecutionStatus.COMPLETED) {
      return {
        id: row.id,
        kind: "artifact_created",
        title: completedTitle(row.stepKey),
        body: completedBody(row.stepKey),
        artifactId: row.outputArtifactId,
        status: "completed",
        timestamp: row.updatedAt || row.createdAt,
      };
    }
    return {
      id: row.id,
      kind: "step_running",
      title: `${row.stepLabel} is running`,
      body: "Aiuna is working on this step.",
      artifactId: row.outputArtifactId,
      status: "running",
      timestamp: row.updatedAt || row.createdAt,
    };
  });
}
