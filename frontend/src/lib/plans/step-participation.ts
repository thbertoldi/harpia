import type { PlanStep } from "$lib/gen/harpia/plans/v1/plans_pb";

/**
 * Reports whether a step participates in a configuration's run. A step with
 * optional capabilities participates only when every optional capability is
 * included by the server-derived configuration projection.
 */
export function stepWillRun(
  step: PlanStep,
  includedOptionalCapabilities: readonly string[],
): boolean {
  const optionalCapabilities =
    step.executorRequirement?.optionalCapabilities ?? [];
  if (optionalCapabilities.length === 0) return true;

  const included = new Set(includedOptionalCapabilities);
  return optionalCapabilities.every((capability) => included.has(capability));
}
