import { toUserMessage } from "$lib/connect-errors";
import {
  MOCK_PLAN_EXECUTION_ID,
  mockPlanExecutionDetail,
} from "$lib/mocks/plan-executions";
import { planClient } from "$lib/rpc";
import {
  StepExecutionStatus,
  type PlanConfiguration,
  type PlanExecution,
  type StepExecution,
} from "$lib/gen/harpia/plans/v1/plans_pb";

type SnapshotStep = {
  key: string;
  title?: string;
};

type PlanConfigurationWithTemplateSnapshot = PlanConfiguration & {
  planTemplateSnapshot?: {
    steps?: SnapshotStep[];
  };
};

export type PlanExecutionDetailSource = "api" | "mock";

export type StepExecutionDisplayRow = {
  id: string;
  index: number;
  stepKey: string;
  stepLabel: string;
  status: StepExecutionStatus;
  inputArtifactId: string;
  outputArtifactId: string;
  approvalRequestId: string;
  createdAt: string;
  updatedAt: string;
  attempt: number;
  canRetry: boolean;
};

export type PlanExecutionDetailResult = {
  execution: PlanExecution;
  stepExecutions: StepExecution[];
  rows: StepExecutionDisplayRow[];
  source: PlanExecutionDetailSource;
  error?: string;
};

function titleFromStepKey(stepKey: string): string {
  const words = stepKey.split("-").filter(Boolean);
  if (words.length === 0) return stepKey;
  return words
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(" ");
}

function stepLabelLookup(execution: PlanExecution): Map<string, string> {
  const lookup = new Map<string, string>();
  const snapshot =
    execution.planConfigurationSnapshot as PlanConfigurationWithTemplateSnapshot;

  for (const step of snapshot.planTemplateSnapshot?.steps ?? []) {
    if (step.key && step.title) {
      lookup.set(step.key, step.title);
    }
  }

  return lookup;
}

function sortSteps(stepExecutions: StepExecution[]): StepExecution[] {
  return [...stepExecutions].sort((a, b) => {
    const aTime = Date.parse(a.createdAt);
    const bTime = Date.parse(b.createdAt);
    if (!Number.isNaN(aTime) && !Number.isNaN(bTime) && aTime !== bTime) {
      return aTime - bTime;
    }
    if (a.attempt !== b.attempt) {
      return a.attempt - b.attempt;
    }
    return a.id.localeCompare(b.id);
  });
}

export function isRetryVisible(status: StepExecutionStatus): boolean {
  return status === StepExecutionStatus.FAILED;
}

export function mapStepExecutionsToDisplayRows(
  execution: PlanExecution,
  stepExecutions: StepExecution[],
): StepExecutionDisplayRow[] {
  const labels = stepLabelLookup(execution);
  const ordered = sortSteps(stepExecutions);
  return ordered.map((stepExecution, index) => ({
    id: stepExecution.id,
    index: index + 1,
    stepKey: stepExecution.planStepKey,
    stepLabel:
      labels.get(stepExecution.planStepKey) ??
      titleFromStepKey(stepExecution.planStepKey),
    status: stepExecution.status,
    inputArtifactId: stepExecution.inputArtifactId,
    outputArtifactId: stepExecution.outputArtifactId,
    approvalRequestId: stepExecution.approvalRequestId,
    createdAt: stepExecution.createdAt,
    updatedAt: stepExecution.updatedAt,
    attempt: stepExecution.attempt,
    canRetry: isRetryVisible(stepExecution.status),
  }));
}

async function listStepExecutions(
  tenantId: string,
  executionId: string,
): Promise<StepExecution[]> {
  const stepExecutions: StepExecution[] = [];
  for await (const page of planClient.listStepExecutions({
    tenantId,
    planExecutionId: executionId,
    pageSize: 100,
    pageToken: "",
  })) {
    stepExecutions.push(...page.stepExecutions);
  }
  return stepExecutions;
}

function normalizeStepExecutions(
  execution: PlanExecution,
  listedSteps: StepExecution[],
): StepExecution[] {
  if (listedSteps.length > 0) {
    return listedSteps;
  }
  return execution.stepExecutions ?? [];
}

export async function loadPlanExecutionDetail(
  tenantId: string,
  executionId: string,
): Promise<PlanExecutionDetailResult> {
  try {
    const executionResponse = await planClient.getPlanExecution({
      tenantId,
      planExecutionId: executionId,
    });
    const execution = executionResponse.planExecution;
    if (!execution) {
      throw new Error("Plan execution not found");
    }
    const listedSteps = await listStepExecutions(tenantId, executionId);
    const stepExecutions = normalizeStepExecutions(execution, listedSteps);
    return {
      execution,
      stepExecutions,
      rows: mapStepExecutionsToDisplayRows(execution, stepExecutions),
      source: "api",
    };
  } catch (error) {
    if (executionId === MOCK_PLAN_EXECUTION_ID) {
      const mock = mockPlanExecutionDetail();
      return {
        execution: mock.execution,
        stepExecutions: mock.stepExecutions,
        rows: mapStepExecutionsToDisplayRows(
          mock.execution,
          mock.stepExecutions,
        ),
        source: "mock",
        error: toUserMessage(error),
      };
    }
    throw error;
  }
}
