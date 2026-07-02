import { describe, it, expect, vi, beforeEach } from "vitest";

const { appendThreadMessage, updatePlanConfiguration } = vi.hoisted(() => ({
  appendThreadMessage: vi.fn(),
  updatePlanConfiguration: vi.fn(),
}));

vi.mock("$lib/chat/client", () => ({ appendThreadMessage }));
vi.mock("$lib/rpc", () => ({ planClient: { updatePlanConfiguration } }));

import {
  selectChip,
  selectBindingOption,
  selectOverseerOption,
  editBinding,
  editOverseerBinding,
  editPolicyParameter,
  selectPolicyOption,
  applyLinkedInSuggestion,
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
  it("appends a USER_SELECTION message", async () => {
    appendThreadMessage.mockResolvedValueOnce({});
    await selectChip({
      tenantId: "t",
      configurationId: "c",
      promptMessageId: "m1",
      optionId: "junior",
      value: "inst-junior",
    });
    expect(appendThreadMessage).toHaveBeenCalledWith(
      "t",
      "c",
      "OVERSEER",
      "USER_SELECTION",
      "",
      expect.stringContaining("junior"),
    );
  });
});

describe("applyLinkedInSuggestion", () => {
  it("stores LinkedIn parameter values and leaves materialization to the server", async () => {
    appendThreadMessage.mockResolvedValueOnce({});
    updatePlanConfiguration.mockResolvedValueOnce({
      planConfiguration: { id: "c" },
    });
    const config = {
      id: "c",
      status: PlanConfigurationStatus.DRAFT,
      seedArtifacts: [],
      slotBindings: [],
      overseerBindings: [],
      behaviorPolicies: undefined,
      schedule: undefined,
    } as unknown as PlanConfiguration;
    const template = {
      id: "tpl",
      steps: [
        { key: "fetch-news", defaultExecutorSkuKey: "rss-news-feed" },
        {
          key: "write-draft",
          defaultExecutorSkuKey: "newsletter-writer-senior",
        },
        {
          key: "adapt-for-linkedin",
          defaultExecutorSkuKey: "linkedin-voice-senior",
        },
        { key: "publish-linkedin", defaultExecutorSkuKey: "linkedin-publish" },
      ],
    } as PlanTemplate;

    await applyLinkedInSuggestion({
      tenantId: "t",
      configurationId: "c",
      existingConfiguration: config,
      template,
      topic: "sports",
      installationIdsByStep: {
        "fetch-news": "inst-rss-sports",
        "write-draft": "inst-newsletter",
        "adapt-for-linkedin": "inst-linkedin-voice",
        "publish-linkedin": "inst-linkedin-approval",
      },
      today: new Date("2026-06-26T12:00:00Z"),
    });

    const call = updatePlanConfiguration.mock.calls[0][0];
    expect(call.seedArtifacts).toBeUndefined();
    expect(call.slotBindings).toBeUndefined();
    expect(call.behaviorPolicies).toBeUndefined();
    expect(JSON.parse(call.parameterValuesJson)).toMatchObject({
      theme: "sports",
      language: "pt-BR",
      source_group: "inst-rss-sports",
      approval_mode: "require_approval",
    });
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

describe("selectBindingOption", () => {
  it("appends selection, persists SlotBinding silently, and emits STEP_REBOUND", async () => {
    appendThreadMessage.mockResolvedValue({});
    updatePlanConfiguration.mockResolvedValueOnce({
      planConfiguration: {
        id: "c",
        status: PlanConfigurationStatus.DRAFT,
        slotBindings: [
          { stepKey: "fetch-news", executorInstallationId: "rss-tech" },
        ],
        overseerBindings: [],
        seedArtifacts: [],
      },
    });
    const config = {
      id: "c",
      status: PlanConfigurationStatus.DRAFT,
      slotBindings: [],
      overseerBindings: [],
      behaviorPolicies: undefined,
      schedule: undefined,
      seedArtifacts: [],
    } as unknown as PlanConfiguration;
    const template = templateWithSlotBindingParam();

    const next = await selectBindingOption({
      tenantId: "t",
      configurationId: "c",
      promptMessageId: "prompt-1",
      existingConfiguration: config,
      template,
      stepKey: "fetch-news",
      optionId: "rss-tech",
      value: "rss-tech",
      label: "Tech RSS",
    });

    expect(next.id).toBe("c");
    expect(appendThreadMessage).toHaveBeenNthCalledWith(
      1,
      "t",
      "c",
      "OVERSEER",
      "USER_SELECTION",
      "Tech RSS",
      expect.stringContaining("prompt-1"),
    );
    expect(updatePlanConfiguration).toHaveBeenCalledTimes(1);
    expect(updatePlanConfiguration.mock.calls[0][0].announceSaved).toBeFalsy();
    expect(
      updatePlanConfiguration.mock.calls[0][0].slotBindings,
    ).toBeUndefined();
    expect(
      JSON.parse(updatePlanConfiguration.mock.calls[0][0].parameterValuesJson),
    ).toMatchObject({ source_group: "rss-tech" });
    expect(appendThreadMessage).toHaveBeenNthCalledWith(
      2,
      "t",
      "c",
      "SYSTEM",
      "STEP_REBOUND",
      "Tech RSS",
      expect.stringContaining('"new_executor_installation_id":"rss-tech"'),
    );
  });

  it("records previous and new installation ids when rebinding", async () => {
    appendThreadMessage.mockResolvedValue({});
    updatePlanConfiguration.mockResolvedValueOnce({
      planConfiguration: { id: "c" },
    });
    const config = {
      id: "c",
      status: PlanConfigurationStatus.DRAFT,
      slotBindings: [
        { stepKey: "fetch-news", executorInstallationId: "rss-old" },
      ],
      overseerBindings: [],
      behaviorPolicies: undefined,
      schedule: undefined,
      seedArtifacts: [],
    } as unknown as PlanConfiguration;
    const template = templateWithSlotBindingParam();

    await selectBindingOption({
      tenantId: "t",
      configurationId: "c",
      promptMessageId: "prompt-1",
      existingConfiguration: config,
      template,
      stepKey: "fetch-news",
      optionId: "rss-new",
      value: "rss-new",
      label: "Brazil RSS",
    });

    const reboundPayload = appendThreadMessage.mock.calls[1][5];
    expect(reboundPayload).toContain(
      '"previous_executor_installation_id":"rss-old"',
    );
    expect(reboundPayload).toContain(
      '"new_executor_installation_id":"rss-new"',
    );
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

describe("selectPolicyOption", () => {
  it("appends selection, persists policy parameter, and emits policy STEP_REBOUND", async () => {
    appendThreadMessage.mockResolvedValue({});
    updatePlanConfiguration.mockResolvedValueOnce({
      planConfiguration: { id: "c" },
    });
    const config = {
      id: "c",
      status: PlanConfigurationStatus.DRAFT,
      slotBindings: [],
      overseerBindings: [],
      behaviorPolicies: undefined,
      schedule: undefined,
      parameterValuesJson: '{"approval_mode":"auto_publish"}',
    } as unknown as PlanConfiguration;

    await selectPolicyOption({
      tenantId: "t",
      configurationId: "c",
      promptMessageId: "prompt-1",
      existingConfiguration: config,
      template: templateWithPolicyParam(),
      policyKey: "publish_approval_mode",
      parameterKey: "approval_mode",
      optionId: "require_approval",
      value: "require_approval",
      label: "Require approval",
    });

    expect(appendThreadMessage).toHaveBeenNthCalledWith(
      1,
      "t",
      "c",
      "OVERSEER",
      "USER_SELECTION",
      "Require approval",
      expect.stringContaining("prompt-1"),
    );
    expect(updatePlanConfiguration).toHaveBeenCalledTimes(1);
    expect(
      JSON.parse(updatePlanConfiguration.mock.calls[0][0].parameterValuesJson),
    ).toMatchObject({ approval_mode: "require_approval" });
    expect(appendThreadMessage).toHaveBeenNthCalledWith(
      2,
      "t",
      "c",
      "SYSTEM",
      "STEP_REBOUND",
      "Require approval",
      expect.stringContaining('"policy_key":"publish_approval_mode"'),
    );
    expect(appendThreadMessage.mock.calls[1][5]).toContain(
      '"previous_policy_value":"auto_publish"',
    );
    expect(appendThreadMessage.mock.calls[1][5]).toContain(
      '"new_policy_value":"require_approval"',
    );
  });
});

describe("selectOverseerOption", () => {
  it("appends selection, persists OverseerBinding silently, and emits overseer STEP_REBOUND", async () => {
    appendThreadMessage.mockResolvedValue({});
    updatePlanConfiguration.mockResolvedValueOnce({
      planConfiguration: {
        id: "c",
        status: PlanConfigurationStatus.DRAFT,
        slotBindings: [
          { stepKey: "write-draft", executorInstallationId: "writer" },
        ],
        overseerBindings: [
          { stepKey: "write-draft", overseerUserId: "user-ana" },
        ],
        seedArtifacts: [],
      },
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
    } as unknown as PlanConfiguration;

    const next = await selectOverseerOption({
      tenantId: "t",
      configurationId: "c",
      promptMessageId: "prompt-1",
      existingConfiguration: config,
      stepKey: "write-draft",
      optionId: "user-ana",
      value: "user-ana",
      label: "Ana",
    });

    expect(next.id).toBe("c");
    expect(appendThreadMessage).toHaveBeenNthCalledWith(
      1,
      "t",
      "c",
      "OVERSEER",
      "USER_SELECTION",
      "Ana",
      expect.stringContaining("prompt-1"),
    );
    expect(updatePlanConfiguration).toHaveBeenCalledTimes(1);
    expect(updatePlanConfiguration.mock.calls[0][0].announceSaved).toBeFalsy();
    expect(
      updatePlanConfiguration.mock.calls[0][0].overseerBindings.find(
        (binding: OverseerBinding) => binding.stepKey === "write-draft",
      )?.overseerUserId,
    ).toBe("user-ana");
    expect(appendThreadMessage).toHaveBeenNthCalledWith(
      2,
      "t",
      "c",
      "SYSTEM",
      "STEP_REBOUND",
      "Ana",
      expect.stringContaining('"new_overseer_user_id":"user-ana"'),
    );
  });

  it("records previous and new overseer user ids when rebinding", async () => {
    appendThreadMessage.mockResolvedValue({});
    updatePlanConfiguration.mockResolvedValueOnce({
      planConfiguration: { id: "c" },
    });
    const config = {
      id: "c",
      status: PlanConfigurationStatus.DRAFT,
      slotBindings: [],
      overseerBindings: [
        { stepKey: "write-draft", overseerUserId: "user-old" },
      ],
      behaviorPolicies: undefined,
      schedule: undefined,
      seedArtifacts: [],
    } as unknown as PlanConfiguration;

    await selectOverseerOption({
      tenantId: "t",
      configurationId: "c",
      promptMessageId: "prompt-1",
      existingConfiguration: config,
      stepKey: "write-draft",
      optionId: "user-new",
      value: "user-new",
      label: "Paula",
    });

    const reboundPayload = appendThreadMessage.mock.calls[1][5];
    expect(reboundPayload).toContain('"previous_overseer_user_id":"user-old"');
    expect(reboundPayload).toContain('"new_overseer_user_id":"user-new"');
  });
});
