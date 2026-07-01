import { PlanConfigurationStatus } from "$lib/gen/harpia/plans/v1/plans_pb";

export type ProposalStage = "confirm" | "form" | "created";

export type CreatedActionId = "runNow" | "finishSetup" | "schedule" | "anythingElse";

export function confirmSummaryLabel(
  summary: string,
  templateName: string,
  templateKey: string,
): string {
  const trimmed = summary.trim();
  if (trimmed) return trimmed;
  return templateName || templateKey;
}

export function createdActionIds(
  status: PlanConfigurationStatus,
): CreatedActionId[] {
  const primary =
    status === PlanConfigurationStatus.RUNNABLE ? "runNow" : "finishSetup";
  return [primary, "schedule", "anythingElse"];
}

export function createdActionI18nKey(id: CreatedActionId): string {
  switch (id) {
    case "runNow":
      return "thread.created.runNow";
    case "finishSetup":
      return "thread.created.finishSetup";
    case "schedule":
      return "thread.created.schedule";
    case "anythingElse":
      return "thread.created.anythingElse";
  }
}
