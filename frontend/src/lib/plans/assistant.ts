import { create } from "@bufbuild/protobuf";
import { appendThreadMessage } from "$lib/chat/client";
import { buildDefaultLinkedInInputValues } from "$lib/plans/linkedin-template-inputs";
import {
  parseParameterValuesJson,
  parameterValuesJson,
  type LinkedInTemplateInputValues,
} from "$lib/plans/template-inputs";
import { planClient } from "$lib/rpc";
import {
  OverseerBindingSchema,
  TemplateInputRuntimeTarget,
  type PlanConfiguration,
  type PlanTemplate,
} from "$lib/gen/harpia/plans/v1/plans_pb";

/**
 * Advance the configuration assistant so it derives and appends its next
 * prompt (binding/overseer/policies/matrix). Best-effort: a conversational
 * selection has already persisted by the time this runs, so a failure here
 * must not undo it — the next interaction or a manual refresh re-derives.
 */
async function advanceAssistant(
  tenantId: string,
  configurationId: string,
): Promise<void> {
  try {
    await planClient.nextTurn({
      tenantId,
      planConfigurationId: configurationId,
    });
  } catch {
    /* best-effort: selection already persisted */
  }
}

export async function selectChip(args: {
  tenantId: string;
  configurationId: string;
  threadId: string;
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
    args.threadId,
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
  const nextParameterValuesJson = parameterValuesWithSlotBinding(
    args.template,
    args.existingConfiguration.parameterValuesJson,
    args.stepKey,
    args.newInstallationId,
  );
  const response = await planClient.updatePlanConfiguration({
    tenantId: args.tenantId,
    planConfigurationId: args.configurationId,
    status: args.existingConfiguration.status,
    overseerBindings: args.existingConfiguration.overseerBindings,
    schedule: args.existingConfiguration.schedule,
    parameterValuesJson: nextParameterValuesJson,
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
    overseerBindings: nextOverseerBindings,
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

export async function editPolicyParameter(args: {
  tenantId: string;
  configurationId: string;
  existingConfiguration: PlanConfiguration;
  template: PlanTemplate;
  policyKey: string;
  parameterKey: string;
  value: string;
}): Promise<PlanConfiguration> {
  const nextParameterValuesJson = parameterValuesWithPolicyParameter(
    args.template,
    args.existingConfiguration.parameterValuesJson,
    args.policyKey,
    args.parameterKey,
    args.value,
  );
  const response = await planClient.updatePlanConfiguration({
    tenantId: args.tenantId,
    planConfigurationId: args.configurationId,
    status: args.existingConfiguration.status,
    overseerBindings: args.existingConfiguration.overseerBindings,
    schedule: args.existingConfiguration.schedule,
    parameterValuesJson: nextParameterValuesJson,
  });
  if (!response.planConfiguration) {
    throw new Error(
      "editPolicyParameter: UpdatePlanConfiguration returned no configuration",
    );
  }
  return response.planConfiguration;
}

export async function selectBindingOption(args: {
  tenantId: string;
  configurationId: string;
  threadId: string;
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
    threadId: args.threadId,
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
    threadId: args.threadId,
    stepKey: args.stepKey,
    previousInstallationId,
    newInstallationId: args.value,
    label: args.label,
  });

  await advanceAssistant(args.tenantId, args.configurationId);
  return next;
}

export async function selectOverseerOption(args: {
  tenantId: string;
  configurationId: string;
  threadId: string;
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
    threadId: args.threadId,
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
    threadId: args.threadId,
    stepKey: args.stepKey,
    previousOverseerUserId,
    newOverseerUserId: args.value,
    label: args.label,
  });

  await advanceAssistant(args.tenantId, args.configurationId);
  return next;
}

export async function selectPolicyOption(args: {
  tenantId: string;
  configurationId: string;
  threadId: string;
  promptMessageId: string;
  existingConfiguration: PlanConfiguration;
  template: PlanTemplate;
  policyKey: string;
  parameterKey: string;
  optionId: string;
  value: string;
  label: string;
}): Promise<PlanConfiguration> {
  const previousPolicyValue = String(
    parseParameterValuesJson(args.existingConfiguration.parameterValuesJson)[
      args.parameterKey
    ] ?? "",
  );

  await selectChip({
    tenantId: args.tenantId,
    configurationId: args.configurationId,
    threadId: args.threadId,
    promptMessageId: args.promptMessageId,
    optionId: args.optionId,
    value: args.value,
    label: args.label,
  });

  const next = await editPolicyParameter({
    tenantId: args.tenantId,
    configurationId: args.configurationId,
    existingConfiguration: args.existingConfiguration,
    template: args.template,
    policyKey: args.policyKey,
    parameterKey: args.parameterKey,
    value: args.value,
  });

  await appendStepRebound({
    tenantId: args.tenantId,
    configurationId: args.configurationId,
    threadId: args.threadId,
    stepKey: "",
    policyKey: args.policyKey,
    previousPolicyValue,
    newPolicyValue: args.value,
    label: args.label,
  });

  await advanceAssistant(args.tenantId, args.configurationId);
  return next;
}

export async function appendStepRebound(args: {
  tenantId: string;
  configurationId: string;
  threadId: string;
  stepKey: string;
  previousInstallationId?: string;
  newInstallationId?: string;
  previousOverseerUserId?: string;
  newOverseerUserId?: string;
  policyKey?: string;
  previousPolicyValue?: string;
  newPolicyValue?: string;
  label: string;
}): Promise<void> {
  await appendThreadMessage(
    args.tenantId,
    args.threadId,
    "SYSTEM",
    "STEP_REBOUND",
    args.label,
    JSON.stringify({
      step_key: args.stepKey,
      previous_executor_installation_id: args.previousInstallationId,
      new_executor_installation_id: args.newInstallationId,
      previous_overseer_user_id: args.previousOverseerUserId,
      new_overseer_user_id: args.newOverseerUserId,
      policy_key: args.policyKey,
      previous_policy_value: args.previousPolicyValue,
      new_policy_value: args.newPolicyValue,
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
  values.aggregateSourceGroupInstallationId =
    values.aggregateSourceGroupInstallationId.trim() ||
    values.sourceGroupInstallationIds.find((id) => id.trim())?.trim() ||
    args.installationIdsByStep["fetch-news"] ||
    "";
  // Suggest is an incremental edit (announce_saved=false, no thread event):
  // the user reviews the populated matrix and then explicitly saves.
  const response = await planClient.updatePlanConfiguration({
    tenantId: args.tenantId,
    planConfigurationId: args.configurationId,
    status: args.existingConfiguration.status,
    overseerBindings: args.existingConfiguration.overseerBindings,
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

function parameterValuesWithSlotBinding(
  template: PlanTemplate,
  raw: string | undefined,
  stepKey: string,
  installationId: string,
): string {
  const parameter = template.inputParameters.find((input) =>
    input.runtimeMappings.some(
      (mapping) =>
        mapping.target === TemplateInputRuntimeTarget.SLOT_BINDING &&
        mapping.stepKey === stepKey,
    ),
  );
  if (!parameter?.key) {
    return raw ?? "";
  }
  return JSON.stringify({
    ...parseParameterValuesJson(raw),
    [parameter.key]: installationId,
  });
}

function parameterValuesWithPolicyParameter(
  template: PlanTemplate,
  raw: string | undefined,
  policyKey: string,
  parameterKey: string,
  value: string,
): string {
  const key =
    parameterKey ||
    template.inputParameters.find((input) =>
      input.runtimeMappings.some(
        (mapping) =>
          mapping.target === TemplateInputRuntimeTarget.BEHAVIOR_POLICY &&
          mapping.policyKey === policyKey,
      ),
    )?.key ||
    "";
  if (!key) {
    return raw ?? "";
  }
  return JSON.stringify({
    ...parseParameterValuesJson(raw),
    [key]: value,
  });
}
