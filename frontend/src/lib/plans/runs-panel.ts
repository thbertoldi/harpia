import {
  PlanConfigurationStatus,
  type PlanConfiguration,
  type PlanExecution,
} from "$lib/gen/harpia/plans/v1/plans_pb";

/**
 * A plan configuration paired with the executions that belong to it.
 *
 * Executions with no matching configuration (e.g. whose configuration was
 * archived) still surface, grouped under a synthetic orphan bucket so the user
 * can see and follow them — see {@link groupExecutionsByConfiguration}.
 */
export interface PlanRunGroup {
  configuration?: PlanConfiguration;
  /** Set when an execution's configuration could not be resolved. */
  orphan?: boolean;
  executions: PlanExecution[];
}

/**
 * Join executions onto their PlanConfiguration via plan_configuration_id, and
 * include configurations that have no executions yet (the Runs panel is the
 * single place to view and manage every plan). Groups are ordered newest
 * activity first: the most recent updatedAt across configuration + executions.
 */
export function groupExecutionsByConfiguration(
  configurations: PlanConfiguration[],
  executions: PlanExecution[],
): PlanRunGroup[] {
  const executionsByConfig = new Map<string, PlanExecution[]>();
  for (const execution of executions) {
    const key = execution.planConfigurationId;
    if (!key) continue;
    const bucket = executionsByConfig.get(key);
    if (bucket) {
      bucket.push(execution);
    } else {
      executionsByConfig.set(key, [execution]);
    }
  }

  const groups: PlanRunGroup[] = [];

  for (const configuration of configurations) {
    groups.push({
      configuration,
      executions: sortExecutions(
        executionsByConfig.get(configuration.id) ?? [],
      ),
    });
    executionsByConfig.delete(configuration.id);
  }

  // Executions whose configuration was not in the fetched set (archived,
  // deleted, or out of page range) surface as orphan buckets so the user can
  // still navigate back to the origin thread.
  for (const [configurationId, orphanExecutions] of executionsByConfig) {
    if (orphanExecutions.length === 0) continue;
    groups.push({
      orphan: true,
      configuration: {
        id: configurationId,
      } as PlanConfiguration,
      executions: sortExecutions(orphanExecutions),
    });
  }

  return groups.sort((a, b) => groupSortKey(b) - groupSortKey(a));
}

function sortExecutions(items: PlanExecution[]): PlanExecution[] {
  return [...items].sort((left, right) =>
    (right.updatedAt || right.createdAt || "").localeCompare(
      left.updatedAt || left.createdAt || "",
    ),
  );
}

function groupSortKey(group: PlanRunGroup): number {
  const latestExecution = group.executions[0];
  const executionTime = latestExecution
    ? Date.parse(latestExecution.updatedAt || latestExecution.createdAt || "")
    : 0;
  const configurationTime = group.configuration
    ? Date.parse(
        group.configuration.updatedAt || group.configuration.createdAt || "",
      )
    : 0;
  return Math.max(executionTime, configurationTime);
}

/**
 * Whether a plan accepts light inline edits (sources / schedule / tone) without
 * a structural change. Only recurring plans that are live (RUNNABLE or
 * SCHEDULED) qualify — drafts and disabled plans are edited structurally in the
 * conversation.
 */
export function canInlineEdit(
  configuration: PlanConfiguration | undefined,
): boolean {
  if (!configuration) return false;
  return (
    configuration.status === PlanConfigurationStatus.RUNNABLE ||
    configuration.status === PlanConfigurationStatus.SCHEDULED
  );
}

/**
 * Count of executions currently in flight for a group, used for the live
 * "running" badge.
 */
export function countActiveExecutions(group: PlanRunGroup): number {
  let count = 0;
  for (const execution of group.executions) {
    if (
      execution.status === 1 /* PENDING */ ||
      execution.status === 2 /* RUNNING */
    ) {
      count += 1;
    }
  }
  return count;
}
