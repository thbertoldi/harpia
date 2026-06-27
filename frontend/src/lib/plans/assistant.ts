import { create } from "@bufbuild/protobuf";
import { appendThreadMessage } from "$lib/chat/client";
import { buildLinkedInSuggestion } from "$lib/plans/linkedin-suggestions";
import { planClient } from "$lib/rpc";
import {
  SlotBindingSchema,
  type PlanConfiguration,
  type PlanTemplate,
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
  // Upsert: a step bound for the first time has no existing entry, so .map
  // alone would silently drop the pick (the bug where a selected executor
  // never reached the canvas). Append when absent; the server resolves the
  // SKU + kind from the installation id on validation, so installation id is
  // the only field the client must supply.
  const existing = args.existingConfiguration.slotBindings;
  const alreadyBound = existing.some((b) => b.stepKey === args.stepKey);
  const nextSlotBindings = alreadyBound
    ? existing.map((b) =>
        b.stepKey === args.stepKey
          ? { ...b, executorInstallationId: args.newInstallationId }
          : b,
      )
    : [
        ...existing,
        create(SlotBindingSchema, {
          stepKey: args.stepKey,
          executorInstallationId: args.newInstallationId,
        }),
      ];
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

export async function applyLinkedInSuggestion(args: {
  tenantId: string;
  configurationId: string;
  existingConfiguration: PlanConfiguration;
  template: PlanTemplate;
  topic: string;
  installationIdsByStep: Record<string, string>;
  today?: Date;
}): Promise<PlanConfiguration> {
  const suggestion = buildLinkedInSuggestion({
    topic: args.topic,
    installationIdsByStep: args.installationIdsByStep,
    today: args.today ?? new Date(),
  });
  await appendThreadMessage(
    args.tenantId,
    args.configurationId,
    "SYSTEM",
    "STEP_REBOUND",
    "",
    JSON.stringify({
      reason: "linkedin_suggestion",
      topic: args.topic,
      bound_steps: suggestion.slotBindings.map((binding) => binding.stepKey),
    }),
  );
  const response = await planClient.updatePlanConfiguration({
    tenantId: args.tenantId,
    planConfigurationId: args.configurationId,
    status: args.existingConfiguration.status,
    seedArtifacts: suggestion.seedArtifacts,
    slotBindings: suggestion.slotBindings,
    overseerBindings: args.existingConfiguration.overseerBindings,
    behaviorPolicies: suggestion.behaviorPolicies,
    schedule: args.existingConfiguration.schedule,
  });
  if (!response.planConfiguration) {
    throw new Error(
      "applyLinkedInSuggestion: UpdatePlanConfiguration returned no configuration",
    );
  }
  return response.planConfiguration;
}
