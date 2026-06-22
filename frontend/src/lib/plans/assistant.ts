import { appendThreadMessage } from "$lib/chat/client";
import { planClient } from "$lib/rpc";
import type {
  PlanConfiguration,
  PlanTemplate,
} from "$lib/gen/harpia/plans/v1/plans_pb";

export async function selectChip(args: {
  tenantId: string;
  configurationId: string;
  promptMessageId: string;
  optionId: string;
  value: string;
  /** Human-readable chip label, surfaced in the right-aligned bubble. */
  label?: string;
}): Promise<void> {
  const payload = JSON.stringify({
    in_response_to_message_id: args.promptMessageId,
    option_id: args.optionId,
    value: args.value,
  });
  await appendThreadMessage(
    args.tenantId,
    args.configurationId,
    "OVERSEER",
    "USER_SELECTION",
    args.label ?? "",
    payload,
  );
}

export async function editBinding(args: {
  tenantId: string;
  configurationId: string;
  existingConfiguration: PlanConfiguration;
  template: PlanTemplate;
  stepKey: string;
  newInstallationId: string;
}): Promise<PlanConfiguration> {
  const previous =
    args.existingConfiguration.slotBindings.find(
      (b) => b.stepKey === args.stepKey,
    )?.executorInstallationId ?? "";
  const reboundPayload = JSON.stringify({
    step_key: args.stepKey,
    previous_executor_installation_id: previous,
    new_executor_installation_id: args.newInstallationId,
  });
  await appendThreadMessage(
    args.tenantId,
    args.configurationId,
    "SYSTEM",
    "STEP_REBOUND",
    "",
    reboundPayload,
  );
  const nextSlotBindings = args.existingConfiguration.slotBindings.map((b) =>
    b.stepKey === args.stepKey
      ? { ...b, executorInstallationId: args.newInstallationId }
      : b,
  );
  const response = await planClient.updatePlanConfiguration({
    tenantId: args.tenantId,
    planConfigurationId: args.configurationId,
    status: args.existingConfiguration.status,
    seedArtifacts: args.existingConfiguration.seedArtifacts,
    slotBindings: nextSlotBindings,
    overseerBindings: args.existingConfiguration.overseerBindings,
    behaviorPolicies: args.existingConfiguration.behaviorPolicies,
    schedule: args.existingConfiguration.schedule,
  });
  if (!response.planConfiguration) {
    throw new Error(
      "editBinding: UpdatePlanConfiguration returned no configuration",
    );
  }
  return response.planConfiguration;
}
