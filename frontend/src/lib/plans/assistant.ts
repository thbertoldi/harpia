import { create } from "@bufbuild/protobuf";
import { appendThreadMessage } from "$lib/chat/client";
import {
  buildDefaultLinkedInInputValues,
  materializeLinkedInInputValues,
} from "$lib/plans/linkedin-template-inputs";
import {
  parameterValuesJson,
  type LinkedInTemplateInputValues,
} from "$lib/plans/template-inputs";
import { planClient } from "$lib/rpc";
import {
  OverseerBindingSchema,
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
  // Incremental binding picks are not announced in the thread (no STEP_REBOUND
  // event, announce_saved=false): the thread would otherwise flood with a
  // message on every executor change. The explicit Save announces instead.
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
    parameterValuesJson: args.existingConfiguration.parameterValuesJson,
  });
  if (!response.planConfiguration) {
    throw new Error(
      "editBinding: UpdatePlanConfiguration returned no configuration",
    );
  }
  return response.planConfiguration;
}

export async function editOverseerBinding(args: {
  tenantId: string;
  configurationId: string;
  existingConfiguration: PlanConfiguration;
  stepKey: string;
  newOverseerUserId: string;
}): Promise<PlanConfiguration> {
  const existing = args.existingConfiguration.overseerBindings;
  const alreadyBound = existing.some((b) => b.stepKey === args.stepKey);
  const nextOverseerBindings = alreadyBound
    ? existing.map((b) =>
        b.stepKey === args.stepKey
          ? { ...b, overseerUserId: args.newOverseerUserId }
          : b,
      )
    : [
        ...existing,
        create(OverseerBindingSchema, {
          stepKey: args.stepKey,
          overseerUserId: args.newOverseerUserId,
        }),
      ];
  const response = await planClient.updatePlanConfiguration({
    tenantId: args.tenantId,
    planConfigurationId: args.configurationId,
    status: args.existingConfiguration.status,
    seedArtifacts: args.existingConfiguration.seedArtifacts,
    slotBindings: args.existingConfiguration.slotBindings,
    overseerBindings: nextOverseerBindings,
    behaviorPolicies: args.existingConfiguration.behaviorPolicies,
    schedule: args.existingConfiguration.schedule,
    parameterValuesJson: args.existingConfiguration.parameterValuesJson,
  });
  if (!response.planConfiguration) {
    throw new Error(
      "editOverseerBinding: UpdatePlanConfiguration returned no configuration",
    );
  }
  return response.planConfiguration;
}

export async function selectBindingOption(args: {
  tenantId: string;
  configurationId: string;
  promptMessageId: string;
  existingConfiguration: PlanConfiguration;
  template: PlanTemplate;
  stepKey: string;
  optionId: string;
  value: string;
  label: string;
}): Promise<PlanConfiguration> {
  const previousInstallationId =
    args.existingConfiguration.slotBindings.find(
      (binding) => binding.stepKey === args.stepKey,
    )?.executorInstallationId ?? "";

  await selectChip({
    tenantId: args.tenantId,
    configurationId: args.configurationId,
    promptMessageId: args.promptMessageId,
    optionId: args.optionId,
    value: args.value,
    label: args.label,
  });

  const next = await editBinding({
    tenantId: args.tenantId,
    configurationId: args.configurationId,
    existingConfiguration: args.existingConfiguration,
    template: args.template,
    stepKey: args.stepKey,
    newInstallationId: args.value,
  });

  await appendStepRebound({
    tenantId: args.tenantId,
    configurationId: args.configurationId,
    stepKey: args.stepKey,
    previousInstallationId,
    newInstallationId: args.value,
    label: args.label,
  });

  return next;
}

export async function selectOverseerOption(args: {
  tenantId: string;
  configurationId: string;
  promptMessageId: string;
  existingConfiguration: PlanConfiguration;
  stepKey: string;
  optionId: string;
  value: string;
  label: string;
}): Promise<PlanConfiguration> {
  const previousOverseerUserId =
    args.existingConfiguration.overseerBindings.find(
      (binding) => binding.stepKey === args.stepKey,
    )?.overseerUserId ?? "";

  await selectChip({
    tenantId: args.tenantId,
    configurationId: args.configurationId,
    promptMessageId: args.promptMessageId,
    optionId: args.optionId,
    value: args.value,
    label: args.label,
  });

  const next = await editOverseerBinding({
    tenantId: args.tenantId,
    configurationId: args.configurationId,
    existingConfiguration: args.existingConfiguration,
    stepKey: args.stepKey,
    newOverseerUserId: args.value,
  });

  await appendStepRebound({
    tenantId: args.tenantId,
    configurationId: args.configurationId,
    stepKey: args.stepKey,
    previousOverseerUserId,
    newOverseerUserId: args.value,
    label: args.label,
  });

  return next;
}

export async function appendStepRebound(args: {
  tenantId: string;
  configurationId: string;
  stepKey: string;
  previousInstallationId?: string;
  newInstallationId?: string;
  previousOverseerUserId?: string;
  newOverseerUserId?: string;
  label: string;
}): Promise<void> {
  await appendThreadMessage(
    args.tenantId,
    args.configurationId,
    "SYSTEM",
    "STEP_REBOUND",
    args.label,
    JSON.stringify({
      step_key: args.stepKey,
      previous_executor_installation_id: args.previousInstallationId,
      new_executor_installation_id: args.newInstallationId,
      previous_overseer_user_id: args.previousOverseerUserId,
      new_overseer_user_id: args.newOverseerUserId,
    }),
  );
}

export async function applyLinkedInSuggestion(args: {
  tenantId: string;
  configurationId: string;
  existingConfiguration: PlanConfiguration;
  template: PlanTemplate;
  topic: string;
  values?: LinkedInTemplateInputValues;
  installationIdsByStep: Record<string, string>;
  today?: Date;
}): Promise<PlanConfiguration> {
  const values = args.values
    ? { ...args.values, dateRange: { ...args.values.dateRange } }
    : buildDefaultLinkedInInputValues(args.today ?? new Date());
  values.theme = args.topic.trim() || values.theme;
  const suggestion = materializeLinkedInInputValues(
    values,
    args.installationIdsByStep,
  );
  // Suggest is an incremental edit (announce_saved=false, no thread event):
  // the user reviews the populated matrix and then explicitly saves.
  const response = await planClient.updatePlanConfiguration({
    tenantId: args.tenantId,
    planConfigurationId: args.configurationId,
    status: args.existingConfiguration.status,
    seedArtifacts: suggestion.seedArtifacts,
    slotBindings: suggestion.slotBindings,
    overseerBindings: args.existingConfiguration.overseerBindings,
    behaviorPolicies: suggestion.behaviorPolicies,
    schedule: args.existingConfiguration.schedule,
    parameterValuesJson: parameterValuesJson(values),
  });
  if (!response.planConfiguration) {
    throw new Error(
      "applyLinkedInSuggestion: UpdatePlanConfiguration returned no configuration",
    );
  }
  return response.planConfiguration;
}
