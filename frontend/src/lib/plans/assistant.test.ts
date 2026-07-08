import { describe, it, expect, vi, beforeEach } from "vitest";

const {
  appendThreadMessage,
  submitConfigurationSelection,
  updatePlanConfiguration,
} = vi.hoisted(() => ({
  appendThreadMessage: vi.fn(),
  submitConfigurationSelection: vi.fn(),
  updatePlanConfiguration: vi.fn(),
}));

vi.mock("$lib/chat/client", () => ({ appendThreadMessage }));
vi.mock("$lib/rpc", () => ({
  planClient: { submitConfigurationSelection, updatePlanConfiguration },
}));

import {
  selectChip,
  editBinding,
  editOverseerBinding,
  editPolicyParameter,
} from "./assistant";
import {
  PlanConfigurationStatus,
  TemplateInputRuntimeTarget,
  type PlanConfiguration,
  type PlanTemplate,
  type OverseerBinding,
} from "$lib/gen/harpia/plans/v1/plans_pb";

beforeEach(() => {
  appendThreadMessage.mockReset();
  submitConfigurationSelection.mockReset();
  updatePlanConfiguration.mockReset();
});

function templateWithSlotBindingParam(
  stepKey = "fetch-news",
  key = "source_group",
): PlanTemplate {
  return {
    id: "tpl",
    inputParameters: [
      {
        key,
        runtimeMappings: [
          {
            target: TemplateInputRuntimeTarget.SLOT_BINDING,
            stepKey,
          },
        ],
      },
    ],
  } as PlanTemplate;
}

function templateWithPolicyParam(
  key = "approval_mode",
  policyKey = "publish_approval_mode",
): PlanTemplate {
  return {
    id: "tpl",
    inputParameters: [
      {
        key,
        runtimeMappings: [
          {
            target: TemplateInputRuntimeTarget.BEHAVIOR_POLICY,
            policyKey,
          },
        ],
      },
    ],
  } as PlanTemplate;
}

describe("selectChip", () => {
  it("submits the prompt answer through PlanService", async () => {
    submitConfigurationSelection.mockResolvedValueOnce({
      planConfiguration: { id: "c" },
    });
    const next = await selectChip({
      tenantId: "t",
      configurationId: "c",
      promptMessageId: "m1",
      optionId: "junior",
      value: "inst-junior",
    });
    expect(next.id).toBe("c");
    expect(submitConfigurationSelection).toHaveBeenCalledWith({
      tenantId: "t",
      planConfigurationId: "c",
      assistantPromptMessageId: "m1",
      selection: { optionId: "junior", value: "inst-junior" },
    });
    expect(appendThreadMessage).not.toHaveBeenCalled();
    expect(updatePlanConfiguration).not.toHaveBeenCalled();
  });
});

describe("editBinding", () => {
  it("updates the binding without flooding the thread (no STEP_REBOUND, no announce)", async () => {
    updatePlanConfiguration.mockResolvedValueOnce({
      planConfiguration: { id: "c" },
    });
    const template = templateWithSlotBindingParam("a", "source_group");
    const config = {
      id: "c",
      status: PlanConfigurationStatus.RUNNABLE,
      slotBindings: [
        { stepKey: "a", executorInstallationId: "inst-old" },
        { stepKey: "b", executorInstallationId: "inst-b" },
      ],
      overseerBindings: [],
      behaviorPolicies: undefined,
      schedule: undefined,
      seedArtifacts: [],
      parameterValuesJson: '{"theme":"existing"}',
    } as unknown as PlanConfiguration;
    await editBinding({
      tenantId: "t",
      configurationId: "c",
      existingConfiguration: config,
      template,
      stepKey: "a",
      newInstallationId: "inst-new",
    });
    // Incremental edit: no thread message, and the save is silent.
    expect(appendThreadMessage).not.toHaveBeenCalled();
    expect(updatePlanConfiguration).toHaveBeenCalledTimes(1);
    const call = updatePlanConfiguration.mock.calls[0][0];
    expect(call.announceSaved).toBeFalsy();
    expect(call.slotBindings).toBeUndefined();
    expect(JSON.parse(call.parameterValuesJson)).toMatchObject({
      theme: "existing",
      source_group: "inst-new",
    });
  });

  it("adds a binding for a step that was previously unbound", async () => {
    appendThreadMessage.mockResolvedValueOnce({});
    updatePlanConfiguration.mockResolvedValueOnce({
      planConfiguration: { id: "c" },
    });
    const template = templateWithSlotBindingParam("a", "source_group");
    // Fresh DRAFT: no slot bindings yet (the regression case).
    const config = {
      id: "c",
      status: PlanConfigurationStatus.DRAFT,
      slotBindings: [],
      overseerBindings: [],
      behaviorPolicies: undefined,
      schedule: undefined,
      seedArtifacts: [],
    } as unknown as PlanConfiguration;
    await editBinding({
      tenantId: "t",
      configurationId: "c",
      existingConfiguration: config,
      template,
      stepKey: "a",
      newInstallationId: "inst-new",
    });
    expect(updatePlanConfiguration).toHaveBeenCalledTimes(1);
    const call = updatePlanConfiguration.mock.calls[0][0];
    expect(call.slotBindings).toBeUndefined();
    expect(JSON.parse(call.parameterValuesJson)).toMatchObject({
      source_group: "inst-new",
    });
  });
});

describe("editOverseerBinding", () => {
  it("upserts an overseer binding without announcing a save", async () => {
    updatePlanConfiguration.mockResolvedValueOnce({
      planConfiguration: { id: "c" },
    });
    const config = {
      id: "c",
      status: PlanConfigurationStatus.DRAFT,
      slotBindings: [
        { stepKey: "write-draft", executorInstallationId: "writer" },
      ],
      overseerBindings: [],
      behaviorPolicies: undefined,
      schedule: undefined,
      seedArtifacts: [],
      parameterValuesJson: '{"theme":"existing"}',
    } as unknown as PlanConfiguration;

    await editOverseerBinding({
      tenantId: "t",
      configurationId: "c",
      existingConfiguration: config,
      stepKey: "write-draft",
      newOverseerUserId: "user-ana",
    });

    expect(appendThreadMessage).not.toHaveBeenCalled();
    expect(updatePlanConfiguration).toHaveBeenCalledTimes(1);
    const call = updatePlanConfiguration.mock.calls[0][0];
    expect(call.announceSaved).toBeFalsy();
    expect(call.slotBindings).toBeUndefined();
    expect(
      call.overseerBindings.find(
        (binding: OverseerBinding) => binding.stepKey === "write-draft",
      )?.overseerUserId,
    ).toBe("user-ana");
    expect(call.parameterValuesJson).toBe('{"theme":"existing"}');
  });
});

describe("editPolicyParameter", () => {
  it("updates one behavior policy input via parameterValuesJson only", async () => {
    updatePlanConfiguration.mockResolvedValueOnce({
      planConfiguration: { id: "c" },
    });
    const config = {
      id: "c",
      status: PlanConfigurationStatus.DRAFT,
      slotBindings: [
        { stepKey: "fetch-news", executorInstallationId: "rss-tech" },
      ],
      overseerBindings: [
        { stepKey: "write-draft", overseerUserId: "user-ana" },
      ],
      behaviorPolicies: undefined,
      schedule: { cron: "0 9 * * 1", timezone: "America/Sao_Paulo" },
      parameterValuesJson:
        '{"theme":"existing","approval_mode":"auto_publish"}',
    } as unknown as PlanConfiguration;

    await editPolicyParameter({
      tenantId: "t",
      configurationId: "c",
      existingConfiguration: config,
      template: templateWithPolicyParam(),
      policyKey: "publish_approval_mode",
      parameterKey: "approval_mode",
      value: "require_approval",
    });

    expect(updatePlanConfiguration).toHaveBeenCalledTimes(1);
    const call = updatePlanConfiguration.mock.calls[0][0];
    expect(call.slotBindings).toBeUndefined();
    expect(call.behaviorPolicies).toBeUndefined();
    expect(call.overseerBindings).toBe(config.overseerBindings);
    expect(call.schedule).toBe(config.schedule);
    expect(JSON.parse(call.parameterValuesJson)).toEqual({
      theme: "existing",
      approval_mode: "require_approval",
    });
  });
});
