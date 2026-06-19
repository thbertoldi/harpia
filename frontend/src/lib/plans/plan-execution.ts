import { getSession, getTenant, requireTenantId } from "$lib/auth";
import { toUserMessage } from "$lib/connect-errors";
import { allowsMockFallback } from "$lib/dev-mocks";
import {
  mockPlanExecution,
  mockPlanExecutionsList,
  nextMockPlanExecution,
  resetMockPlanExecution,
} from "$lib/mocks/plan-executions";
import {
  PlanExecutionStatus,
  StepExecutionStatus,
  type PlanExecution,
  type StepExecution,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import { planClient } from "$lib/rpc";

export type PlanExecutionDataSource = "api" | "mock";

export interface PlanExecutionResult {
  execution: PlanExecution;
  source: PlanExecutionDataSource;
  error?: string;
}

export interface PlanExecutionsResult {
  executions: PlanExecution[];
  source: PlanExecutionDataSource;
  error?: string;
}

export interface PlanExecutionStreamHandlers {
  onExecution?: (execution: PlanExecution) => void;
  onSteps: (steps: StepExecution[]) => void;
  onWarning?: (warning: string) => void;
}

export interface PlanExecutionStreamControl {
  stop: () => void;
}

export const PLAN_EXECUTION_POLL_INTERVAL_MS = 5_000;

function resolveTenantId(): string | undefined {
  return getTenant()?.id ?? getSession()?.tenant?.id;
}

function stepIdentity(step: StepExecution): string {
  return step.id || step.planStepKey;
}

function sortStepsByTimeline(steps: StepExecution[]): StepExecution[] {
  return [...steps].sort((left, right) => {
    const createdDelta =
      Date.parse(left.createdAt || left.updatedAt || "") -
      Date.parse(right.createdAt || right.updatedAt || "");
    if (!Number.isNaN(createdDelta) && createdDelta !== 0) {
      return createdDelta;
    }
    return left.planStepKey.localeCompare(right.planStepKey);
  });
}

export function mergeStepExecutions(
  previous: StepExecution[],
  incoming: StepExecution[],
): StepExecution[] {
  const byId = new Map<string, StepExecution>();
  for (const step of previous) {
    byId.set(stepIdentity(step), step);
  }
  for (const step of incoming) {
    byId.set(stepIdentity(step), step);
  }
  return sortStepsByTimeline([...byId.values()]);
}

export function isPlanExecutionTerminal(status: PlanExecutionStatus): boolean {
  return (
    status === PlanExecutionStatus.COMPLETED ||
    status === PlanExecutionStatus.FAILED ||
    status === PlanExecutionStatus.CANCELLED
  );
}

export function statusKeyForPlanExecution(status: PlanExecutionStatus): string {
  switch (status) {
    case PlanExecutionStatus.PENDING:
      return "executions.planStatus.pending";
    case PlanExecutionStatus.RUNNING:
      return "executions.planStatus.running";
    case PlanExecutionStatus.COMPLETED:
      return "executions.planStatus.completed";
    case PlanExecutionStatus.FAILED:
      return "executions.planStatus.failed";
    case PlanExecutionStatus.CANCELLED:
      return "executions.planStatus.cancelled";
    default:
      return "executions.planStatus.unspecified";
  }
}

export function statusKeyForStepExecution(status: StepExecutionStatus): string {
  switch (status) {
    case StepExecutionStatus.PENDING:
      return "executions.stepStatus.pending";
    case StepExecutionStatus.RUNNING:
      return "executions.stepStatus.running";
    case StepExecutionStatus.AWAITING_ELICITATION:
      return "executions.stepStatus.awaitingElicitation";
    case StepExecutionStatus.AWAITING_APPROVAL:
      return "executions.stepStatus.awaitingApproval";
    case StepExecutionStatus.COMPLETED:
      return "executions.stepStatus.completed";
    case StepExecutionStatus.FAILED:
      return "executions.stepStatus.failed";
    default:
      return "executions.stepStatus.unspecified";
  }
}

export function formatStepDuration(
  step: StepExecution,
  nowMs = Date.now(),
): number | null {
  const start = Date.parse(step.createdAt);
  if (Number.isNaN(start)) return null;
  const end = Date.parse(step.updatedAt);
  const finish = Number.isNaN(end) ? nowMs : end;
  return Math.max(0, finish - start);
}

export async function loadPlanExecution(
  executionId: string,
): Promise<PlanExecutionResult> {
  try {
    const tenantId = requireTenantId();
    const response = await planClient.getPlanExecution({
      tenantId,
      planExecutionId: executionId,
    });
    if (!response.planExecution) {
      throw new Error("Execution not found.");
    }
    return { execution: response.planExecution, source: "api" };
  } catch (error) {
    if (allowsMockFallback()) {
      resetMockPlanExecution(executionId);
      return {
        execution: mockPlanExecution(executionId),
        source: "mock",
        error: toUserMessage(error),
      };
    }
    throw error;
  }
}

export async function loadPlanExecutions(): Promise<PlanExecutionsResult> {
  const tenantId = resolveTenantId();
  if (!tenantId) {
    throw new Error("Select a tenant to load plan executions.");
  }

  try {
    const executions: PlanExecution[] = [];
    for await (const response of planClient.listPlanExecutions({
      tenantId,
      pageSize: 30,
      pageToken: "",
    })) {
      executions.push(...response.planExecutions);
    }
    if (executions.length === 0) {
      return { executions, source: "api" };
    }
    executions.sort((left, right) =>
      right.updatedAt.localeCompare(left.updatedAt),
    );
    return { executions, source: "api" };
  } catch (error) {
    if (allowsMockFallback()) {
      return {
        executions: mockPlanExecutionsList(),
        source: "mock",
        error: toUserMessage(error),
      };
    }
    throw error;
  }
}

export function startPlanExecutionStream(
  executionId: string,
  handlers: PlanExecutionStreamHandlers,
): PlanExecutionStreamControl {
  const controller = new AbortController();
  const tenantId = resolveTenantId();
  let stopped = false;
  let pollTimer: ReturnType<typeof setInterval> | null = null;

  async function refreshExecutionFromApi(): Promise<void> {
    if (!tenantId) return;
    const response = await planClient.getPlanExecution({
      tenantId,
      planExecutionId: executionId,
    });
    if (response.planExecution) {
      handlers.onExecution?.(response.planExecution);
      handlers.onSteps(response.planExecution.stepExecutions);
    }
  }

  function startPollingFallback(): void {
    if (pollTimer || stopped) return;

    pollTimer = setInterval(() => {
      if (stopped) return;

      if (!tenantId) {
        if (allowsMockFallback()) {
          const mockExecution = nextMockPlanExecution(executionId);
          handlers.onExecution?.(mockExecution);
          handlers.onSteps(mockExecution.stepExecutions);
        }
        return;
      }

      void refreshExecutionFromApi().catch((error) => {
        handlers.onWarning?.(toUserMessage(error));
      });
    }, PLAN_EXECUTION_POLL_INTERVAL_MS);
  }

  if (!tenantId) {
    startPollingFallback();
    return {
      stop: () => {
        stopped = true;
        if (pollTimer) {
          clearInterval(pollTimer);
          pollTimer = null;
        }
      },
    };
  }

  void (async () => {
    try {
      for await (const response of planClient.listStepExecutions(
        {
          tenantId,
          planExecutionId: executionId,
          pageSize: 100,
          pageToken: "",
        },
        { signal: controller.signal },
      )) {
        if (stopped || controller.signal.aborted) {
          break;
        }
        handlers.onSteps(response.stepExecutions);
      }

      if (!stopped && !controller.signal.aborted) {
        startPollingFallback();
      }
    } catch (error) {
      if (controller.signal.aborted || stopped) return;
      handlers.onWarning?.(toUserMessage(error));
      startPollingFallback();
    }
  })();

  return {
    stop: () => {
      stopped = true;
      controller.abort();
      if (pollTimer) {
        clearInterval(pollTimer);
        pollTimer = null;
      }
    },
  };
}
