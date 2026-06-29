import { describe, it, expect, vi, beforeEach } from "vitest";

const { appendThreadMessage, updatePlanConfiguration } = vi.hoisted(() => ({
  appendThreadMessage: vi.fn(),
  updatePlanConfiguration: vi.fn(),
}));

vi.mock("$lib/chat/client", () => ({ appendThreadMessage }));
vi.mock("$lib/rpc", () => ({ planClient: { updatePlanConfiguration } }));

import { selectChip, editBinding, applyLinkedInSuggestion } from "./assistant";
import {
  PlanConfigurationStatus,
  PublishApprovalMode,
  type PlanConfiguration,
  type PlanTemplate,
  type SeedArtifactBinding,
  type SlotBinding,
} from "$lib/gen/harpia/plans/v1/plans_pb";

beforeEach(() => {
  appendThreadMessage.mockReset();
  updatePlanConfiguration.mockReset();
});

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
  it("binds the Monday sports LinkedIn plan and stores content preferences", async () => {
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
    expect(
      call.slotBindings.map((binding: SlotBinding) => binding.stepKey),
    ).toEqual([
      "fetch-news",
      "write-draft",
      "adapt-for-linkedin",
      "publish-linkedin",
    ]);
    expect(call.behaviorPolicies.publishApprovalMode).toBe(
      PublishApprovalMode.REQUIRE_APPROVAL,
    );
    const dateSeed = call.seedArtifacts.find(
      (seed: SeedArtifactBinding) => seed.stepKey === "fetch-news",
    );
    expect(JSON.parse(dateSeed.literalJson)).toEqual({
      startDate: "2026-06-20",
      endDate: "2026-06-26",
    });
    const preferences = call.seedArtifacts.find(
      (seed: SeedArtifactBinding) =>
        seed.inputName === "harpia.internal.ContentPreferences",
    );
    expect(JSON.parse(preferences.literalJson)).toMatchObject({
      topic: "sports",
      language: "pt-BR",
      tone: "analytical, concise, and practical",
    });
    expect(JSON.parse(call.parameterValuesJson)).toMatchObject({
      theme: "sports",
      language: "pt-BR",
      approval_mode: "require_approval",
    });
  });
});

describe("editBinding", () => {
  it("updates the binding without flooding the thread (no STEP_REBOUND, no announce)", async () => {
    updatePlanConfiguration.mockResolvedValueOnce({
      planConfiguration: { id: "c" },
    });
    const template = {
      id: "tpl",
      steps: [{ key: "a" }, { key: "b" }],
    } as PlanTemplate;
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
      parameterValuesJson: "{\"theme\":\"existing\"}",
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
    expect(
      call.slotBindings.find((b: SlotBinding) => b.stepKey === "a")
        ?.executorInstallationId,
    ).toBe("inst-new");
    expect(call.parameterValuesJson).toBe("{\"theme\":\"existing\"}");
  });

  it("adds a binding for a step that was previously unbound", async () => {
    appendThreadMessage.mockResolvedValueOnce({});
    updatePlanConfiguration.mockResolvedValueOnce({
      planConfiguration: { id: "c" },
    });
    const template = {
      id: "tpl",
      steps: [{ key: "a" }, { key: "b" }],
    } as PlanTemplate;
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
    expect(call.slotBindings).toHaveLength(1);
    expect(
      call.slotBindings.find((b: SlotBinding) => b.stepKey === "a")
        ?.executorInstallationId,
    ).toBe("inst-new");
  });
});
