import type {
  PlanConfiguration,
  PlanTemplate,
} from "$lib/gen/harpia/plans/v1/plans_pb";

export type CostBreakdownEntry = {
  stepKey: string;
  stepTitle: string;
  executorInstallationId: string | null;
  executorName: string | null;
  pricePerRunBrl: number | null;
};

export type RunCost = {
  totalPerRunBrl: number;
  currency: "BRL";
  unboundStepCount: number;
  breakdown: CostBreakdownEntry[];
};

export type ExecutorPriceLookup = (
  installationId: string,
) => { displayName: string; pricePerRunBrl: number | null } | null;

export function computeRunCost(
  template: PlanTemplate,
  configuration: PlanConfiguration,
  pricing: ExecutorPriceLookup,
): RunCost {
  const bindingByStep = new Map<string, string>();
  for (const sb of configuration.slotBindings ?? []) {
    if (sb.executorInstallationId) {
      bindingByStep.set(sb.stepKey, sb.executorInstallationId);
    }
  }
  let total = 0;
  let unbound = 0;
  const breakdown: CostBreakdownEntry[] = [];
  for (const step of template.steps ?? []) {
    const installationId = bindingByStep.get(step.key) ?? null;
    if (!installationId) {
      unbound += 1;
      breakdown.push({
        stepKey: step.key,
        stepTitle: step.title || step.key,
        executorInstallationId: null,
        executorName: null,
        pricePerRunBrl: null,
      });
      continue;
    }
    const info = pricing(installationId);
    const price = info?.pricePerRunBrl ?? null;
    if (price != null) total += price;
    breakdown.push({
      stepKey: step.key,
      stepTitle: step.title || step.key,
      executorInstallationId: installationId,
      executorName: info?.displayName ?? null,
      pricePerRunBrl: price,
    });
  }
  return {
    totalPerRunBrl: total,
    currency: "BRL",
    unboundStepCount: unbound,
    breakdown,
  };
}
