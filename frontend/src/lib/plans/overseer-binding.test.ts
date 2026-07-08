import { create } from "@bufbuild/protobuf";
import { describe, expect, it } from "vitest";
import {
  ExecutorKind,
  PlanConfigurationStatus,
  SlotBindingSchema,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import { mockNewsToSocialPostTemplate } from "$lib/plans/plan-template";
import {
  assignSelfAsOverseerForSteps,
  formatOverseerPromotionErrors,
  getAgentBackedSteps,
  getDraftOverseerWarning,
  getOverseerRequiredSteps,
  requiresOverseerValidation,
  validateOverseerBindings,
} from "$lib/plans/overseer-binding";

describe("overseer binding helpers", () => {
  const template = mockNewsToSocialPostTemplate();
  const sessionUser = { sub: "dev-overseer" };

  it("identifies agent-backed steps from the weekly newsletter template", () => {
    const agentSteps = getAgentBackedSteps(template);
    expect(agentSteps.map((step) => step.stepKey)).toEqual([
      "write-draft",
      "adapt-for-linkedin",
    ]);
  });

  it("requires overseer validation only for RUNNABLE and SCHEDULED", () => {
    expect(requiresOverseerValidation(PlanConfigurationStatus.DRAFT)).toBe(
      false,
    );
    expect(requiresOverseerValidation(PlanConfigurationStatus.RUNNABLE)).toBe(
      true,
    );
    expect(requiresOverseerValidation(PlanConfigurationStatus.SCHEDULED)).toBe(
      true,
    );
  });

  it("allows incomplete overseer bindings in DRAFT", () => {
    const result = validateOverseerBindings(
      template,
      [],
      PlanConfigurationStatus.DRAFT,
    );

    expect(result.ok).toBe(true);
    expect(result.issues).toHaveLength(0);
  });

  it("fails promotion when agent steps lack overseer bindings", () => {
    const partialBindings = assignSelfAsOverseerForSteps(
      [{ stepKey: "write-draft", title: "Write Draft" }],
      sessionUser,
    );

    const result = validateOverseerBindings(
      template,
      partialBindings,
      PlanConfigurationStatus.RUNNABLE,
    );

    expect(result.ok).toBe(false);
    expect(result.missingStepKeys).toEqual(["adapt-for-linkedin"]);
    expect(formatOverseerPromotionErrors(result.issues)).toEqual([
      'Agent step "Adapt for LinkedIn" (adapt-for-linkedin) is missing an overseer.',
    ]);
  });

  it("passes promotion when all agent steps have overseer bindings", () => {
    const bindings = assignSelfAsOverseerForSteps(
      getAgentBackedSteps(template),
      sessionUser,
    );

    const result = validateOverseerBindings(
      template,
      bindings,
      PlanConfigurationStatus.SCHEDULED,
    );

    expect(result.ok).toBe(true);
    expect(result.issues).toHaveLength(0);
  });

  it("returns a draft warning when bindings are incomplete", () => {
    const warning = getDraftOverseerWarning(template, []);
    expect(warning).toContain("write-draft");
    expect(warning).toContain("adapt-for-linkedin");
  });

  it("limits required steps to agent slot bindings when provided", () => {
    const slotBindings = [
      create(SlotBindingSchema, {
        stepKey: "write-draft",
        executorKind: ExecutorKind.AGENT,
        executorSkuId: "sku-writer",
        executorInstallationId: "install-writer",
      }),
      create(SlotBindingSchema, {
        stepKey: "publish-linkedin",
        executorKind: ExecutorKind.INTEGRATION,
        executorSkuId: "sku-publisher",
        executorInstallationId: "install-publisher",
      }),
    ];

    const requiredSteps = getOverseerRequiredSteps(template, slotBindings);
    expect(requiredSteps.map((step) => step.stepKey)).toEqual(["write-draft"]);
  });
});
