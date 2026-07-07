import type { ExecutionViewModel } from "$lib/plans/execution-view";

/**
 * 1-based ordinal of `executionId` among all executions that successfully
 * produced `producingStepKey`'s output, ordered by run number. Returns null
 * when the execution didn't produce it (or isn't known) — nothing to badge.
 */
export function executionProducingOrdinal(
  executionViewModels: Pick<
    ExecutionViewModel,
    "executionId" | "runNumber" | "steps"
  >[],
  producingStepKey: string,
  executionId: string,
): number | null {
  const producers = executionViewModels
    .filter((vm) =>
      vm.steps.some(
        (s) =>
          s.key === producingStepKey &&
          s.status === "done" &&
          s.outputArtifactId,
      ),
    )
    .sort((a, b) => a.runNumber - b.runNumber);
  const index = producers.findIndex((vm) => vm.executionId === executionId);
  return index >= 0 ? index + 1 : null;
}

/**
 * Combines a content-revision count with a repeated-execution ordinal into a
 * single iteration number. Prefers the revision count (a real edit history)
 * over the execution ordinal, and omits entirely (returns null) when neither
 * signals more than one iteration — no "v1" noise on a first, unrevised draft.
 */
export function deriveIterationNumber(
  versionCount: number,
  executionOrdinal: number | null,
): number | null {
  if (versionCount > 1) return versionCount;
  if (executionOrdinal !== null && executionOrdinal > 1)
    return executionOrdinal;
  return null;
}
