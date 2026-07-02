import { describe, expect, it } from "vitest";
import { PlanConfigurationStatus } from "$lib/gen/harpia/plans/v1/plans_pb";
import { confirmSummaryLabel, createdActionIds } from "./plan-proposal-logic";
import { requiredInputsSatisfied } from "$lib/plans/template-inputs";
import { TemplateInputParameterType } from "$lib/gen/harpia/plans/v1/plans_pb";
import type { TemplateInputParameter } from "$lib/gen/harpia/plans/v1/plans_pb";

function param(key: string, required = false): TemplateInputParameter {
  return {
    $typeName: "harpia.plans.v1.TemplateInputParameter",
    key,
    label: key,
    description: "",
    type: TemplateInputParameterType.TEXT,
    required,
    defaultValueJson: "",
    optionsJson: "",
    runtimeMappings: [],
  } as TemplateInputParameter;
}

describe("PlanProposalCard logic", () => {
  it("confirm summary uses payload summary with template fallback", () => {
    expect(confirmSummaryLabel("weekly digest", "", "digest")).toBe(
      "weekly digest",
    );
    expect(confirmSummaryLabel("", "Weekly Digest", "digest")).toBe(
      "Weekly Digest",
    );
  });

  it("requiredInputsSatisfied gates immediate create vs form reveal", () => {
    const params = [param("theme", true), param("tone")];
    expect(requiredInputsSatisfied(params, { theme: "retail" })).toBe(true);
    expect(requiredInputsSatisfied(params, { theme: "" })).toBe(false);
  });

  it("contextual actions branch on authoritative status", () => {
    expect(createdActionIds(PlanConfigurationStatus.RUNNABLE)[0]).toBe(
      "runNow",
    );
    expect(createdActionIds(PlanConfigurationStatus.DRAFT)[0]).toBe(
      "finishSetup",
    );
  });
});
