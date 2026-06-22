import { describe, it, expect, vi, beforeEach } from "vitest";

const { appendThreadMessage, updatePlanConfiguration } = vi.hoisted(() => ({
  appendThreadMessage: vi.fn(),
  updatePlanConfiguration: vi.fn(),
}));

vi.mock("$lib/chat/client", () => ({ appendThreadMessage }));
vi.mock("$lib/rpc", () => ({ planClient: { updatePlanConfiguration } }));

import { selectChip, editBinding } from "./assistant";
import {
  PlanConfigurationStatus,
  type PlanConfiguration,
  type PlanTemplate,
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

describe("editBinding", () => {
  it("writes STEP_REBOUND then UpdatePlanConfiguration", async () => {
    appendThreadMessage.mockResolvedValueOnce({});
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
    } as unknown as PlanConfiguration;
    await editBinding({
      tenantId: "t",
      configurationId: "c",
      existingConfiguration: config,
      template,
      stepKey: "a",
      newInstallationId: "inst-new",
    });
    expect(appendThreadMessage).toHaveBeenCalledWith(
      "t",
      "c",
      "SYSTEM",
      "STEP_REBOUND",
      "",
      expect.stringContaining("inst-old"),
    );
    expect(updatePlanConfiguration).toHaveBeenCalledTimes(1);
    const call = updatePlanConfiguration.mock.calls[0][0];
    expect(
      call.slotBindings.find((b: SlotBinding) => b.stepKey === "a")
        ?.executorInstallationId,
    ).toBe("inst-new");
  });
});
