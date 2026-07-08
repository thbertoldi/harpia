import { create } from "@bufbuild/protobuf";
import { appendThreadMessage } from "$lib/chat/client";
import { parseParameterValuesJson } from "$lib/plans/template-inputs";
import { planClient } from "$lib/rpc";
import {
  OverseerBindingSchema,
  TemplateInputRuntimeTarget,
  type PlanConfiguration,
  type PlanTemplate,
} from "$lib/gen/harpia/plans/v1/plans_pb";

export async function selectChip(args: {
  tenantId: string;
  configurationId: string;
  promptMessageId: string;
  optionId: string;
  value: string;
}): Promise<PlanConfiguration> {
  const response = await planClient.submitConfigurationSelection({
    tenantId: args.tenantId,
    planConfigurationId: args.configurationId,
    assistantPromptMessageId: args.promptMessageId,
    selection: {
      optionId: args.optionId,
      value: args.value,
    },
  });
  if (!response.planConfiguration) {
    throw new Error(
      "selectChip: SubmitConfigurationSelection returned no configuration",
    );
  }
  return response.planConfiguration;
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
