import { create } from "@bufbuild/protobuf";
import { describe, expect, it } from "vitest";
import {
  ExecutorRequirementSchema,
  PlanStepSchema,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import { stepWillRun } from "./step-participation";

const optionalStep = create(PlanStepSchema, {
  executorRequirement: create(ExecutorRequirementSchema, {
    optionalCapabilities: ["image-generation", "carousel-authoring"],
  }),
});

describe("stepWillRun", () => {
  it("runs steps without optional capabilities", () => {
    expect(stepWillRun(create(PlanStepSchema), [])).toBe(true);
  });

  it("does not run optional steps when a capability is unspecified", () => {
    expect(stepWillRun(optionalStep, [])).toBe(false);
  });

  it("does not run optional steps when only some capabilities are included", () => {
    expect(stepWillRun(optionalStep, ["image-generation"])).toBe(false);
  });

  it("runs optional steps when all capabilities are included", () => {
    expect(
      stepWillRun(optionalStep, ["image-generation", "carousel-authoring"]),
    ).toBe(true);
  });
});
