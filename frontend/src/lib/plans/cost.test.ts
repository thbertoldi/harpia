import { describe, it, expect } from "vitest";
import { computeRunCost, type ExecutorPriceLookup } from "./cost";
import type {
  PlanConfiguration,
  PlanTemplate,
  PlanStep,
} from "$lib/gen/harpia/plans/v1/plans_pb";

function tpl(...stepKeys: string[]): PlanTemplate {
  return {
    id: "tpl",
    steps: stepKeys.map((k) => ({ key: k, title: k }) as PlanStep),
  } as PlanTemplate;
}

function cfg(...bindings: Array<[string, string]>): PlanConfiguration {
  return {
    id: "cfg",
    slotBindings: bindings.map(([stepKey, installationId]) => ({
      stepKey,
      executorInstallationId: installationId,
    })),
  } as unknown as PlanConfiguration;
}

const pricing: ExecutorPriceLookup = (id) => {
  if (id === "inst-a")
    return { displayName: "Junior Writer", pricePerRunBrl: 0.05 };
  if (id === "inst-b")
    return { displayName: "Senior Reviewer", pricePerRunBrl: 0.2 };
  return null;
};

describe("computeRunCost", () => {
  it("returns zero with all steps unbound", () => {
    const cost = computeRunCost(tpl("a", "b"), cfg(), pricing);
    expect(cost.totalPerRunBrl).toBe(0);
    expect(cost.unboundStepCount).toBe(2);
    expect(cost.breakdown).toHaveLength(2);
    expect(cost.breakdown[0].pricePerRunBrl).toBeNull();
  });

  it("sums prices when fully bound", () => {
    const cost = computeRunCost(
      tpl("a", "b"),
      cfg(["a", "inst-a"], ["b", "inst-b"]),
      pricing,
    );
    expect(cost.totalPerRunBrl).toBeCloseTo(0.25, 5);
    expect(cost.unboundStepCount).toBe(0);
    expect(cost.breakdown[0].executorName).toBe("Junior Writer");
  });

  it("handles partial bindings", () => {
    const cost = computeRunCost(tpl("a", "b"), cfg(["a", "inst-a"]), pricing);
    expect(cost.totalPerRunBrl).toBeCloseTo(0.05, 5);
    expect(cost.unboundStepCount).toBe(1);
  });

  it("counts bound but unpriced executors as unboundStepCount=0 and adds null to breakdown", () => {
    const cost = computeRunCost(tpl("a"), cfg(["a", "unknown"]), pricing);
    expect(cost.unboundStepCount).toBe(0);
    expect(cost.totalPerRunBrl).toBe(0);
    expect(cost.breakdown[0].executorName).toBeNull();
    expect(cost.breakdown[0].pricePerRunBrl).toBeNull();
  });
});
