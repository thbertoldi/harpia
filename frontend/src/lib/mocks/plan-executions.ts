import { create } from "@bufbuild/protobuf";
import {
  PlanExecutionSchema,
  PlanExecutionStatus,
  StepExecutionSchema,
  StepExecutionStatus,
  type PlanExecution,
  type StepExecution,
} from "$lib/gen/harpia/plans/v1/plans_pb";

const DEMO_TENANT_ID = "dev";
const DEMO_PLAN_CONFIGURATION_ID = "demo-plan-configuration";
const DEMO_EXECUTION_ID = "demo-plan-execution";
const DEMO_BASE_TIME_MS = Date.parse("2026-06-15T15:00:00Z");

type StepProgress = {
  stepKey: string;
  status: StepExecutionStatus;
  createdOffsetSec: number;
  updatedOffsetSec: number;
};

const MOCK_STEP_SEQUENCE: StepProgress[][] = [
  [
    {
      stepKey: "fetch-news",
      status: StepExecutionStatus.PENDING,
      createdOffsetSec: 0,
      updatedOffsetSec: 0,
    },
    {
      stepKey: "write-draft",
      status: StepExecutionStatus.PENDING,
      createdOffsetSec: 30,
      updatedOffsetSec: 30,
    },
    {
      stepKey: "adapt-for-linkedin",
      status: StepExecutionStatus.PENDING,
      createdOffsetSec: 60,
      updatedOffsetSec: 60,
    },
  ],
  [
    {
      stepKey: "fetch-news",
      status: StepExecutionStatus.RUNNING,
      createdOffsetSec: 0,
      updatedOffsetSec: 20,
    },
    {
      stepKey: "write-draft",
      status: StepExecutionStatus.PENDING,
      createdOffsetSec: 30,
      updatedOffsetSec: 30,
    },
    {
      stepKey: "adapt-for-linkedin",
      status: StepExecutionStatus.PENDING,
      createdOffsetSec: 60,
      updatedOffsetSec: 60,
    },
  ],
  [
    {
      stepKey: "fetch-news",
      status: StepExecutionStatus.COMPLETED,
      createdOffsetSec: 0,
      updatedOffsetSec: 70,
    },
    {
      stepKey: "write-draft",
      status: StepExecutionStatus.RUNNING,
      createdOffsetSec: 30,
      updatedOffsetSec: 90,
    },
    {
      stepKey: "adapt-for-linkedin",
      status: StepExecutionStatus.PENDING,
      createdOffsetSec: 60,
      updatedOffsetSec: 60,
    },
  ],
  [
    {
      stepKey: "fetch-news",
      status: StepExecutionStatus.COMPLETED,
      createdOffsetSec: 0,
      updatedOffsetSec: 70,
    },
    {
      stepKey: "write-draft",
      status: StepExecutionStatus.COMPLETED,
      createdOffsetSec: 30,
      updatedOffsetSec: 140,
    },
    {
      stepKey: "adapt-for-linkedin",
      status: StepExecutionStatus.RUNNING,
      createdOffsetSec: 60,
      updatedOffsetSec: 160,
    },
  ],
  [
    {
      stepKey: "fetch-news",
      status: StepExecutionStatus.COMPLETED,
      createdOffsetSec: 0,
      updatedOffsetSec: 70,
    },
    {
      stepKey: "write-draft",
      status: StepExecutionStatus.COMPLETED,
      createdOffsetSec: 30,
      updatedOffsetSec: 140,
    },
    {
      stepKey: "adapt-for-linkedin",
      status: StepExecutionStatus.FAILED,
      createdOffsetSec: 60,
      updatedOffsetSec: 210,
    },
  ],
];

const demoCursorByExecution = new Map<string, number>();

function toIso(offsetSec: number): string {
  return new Date(DEMO_BASE_TIME_MS + offsetSec * 1000).toISOString();
}

function resolveExecutionStatus(steps: StepExecution[]): PlanExecutionStatus {
  if (steps.some((step) => step.status === StepExecutionStatus.FAILED)) {
    return PlanExecutionStatus.FAILED;
  }
  if (steps.every((step) => step.status === StepExecutionStatus.COMPLETED)) {
    return PlanExecutionStatus.COMPLETED;
  }
  if (steps.some((step) => step.status === StepExecutionStatus.RUNNING)) {
    return PlanExecutionStatus.RUNNING;
  }
  return PlanExecutionStatus.PENDING;
}

function buildStepExecution(
  executionId: string,
  sequenceIndex: number,
  progress: StepProgress,
): StepExecution {
  return create(StepExecutionSchema, {
    id: `${executionId}-${progress.stepKey}`,
    planExecutionId: executionId,
    planStepKey: progress.stepKey,
    status: progress.status,
    attempt: 1,
    createdAt: toIso(progress.createdOffsetSec),
    updatedAt: toIso(progress.updatedOffsetSec + sequenceIndex * 5),
  });
}

function buildExecutionFromSequence(
  executionId: string,
  sequenceIndex: number,
): PlanExecution {
  const boundedIndex = Math.max(
    0,
    Math.min(sequenceIndex, MOCK_STEP_SEQUENCE.length - 1),
  );
  const steps = MOCK_STEP_SEQUENCE[boundedIndex].map((progress) =>
    buildStepExecution(executionId, boundedIndex, progress),
  );
  const status = resolveExecutionStatus(steps);

  return create(PlanExecutionSchema, {
    id: executionId,
    tenantId: DEMO_TENANT_ID,
    planConfigurationId: DEMO_PLAN_CONFIGURATION_ID,
    status,
    stepExecutions: steps,
    triggeredAt: toIso(0),
    completedAt:
      status === PlanExecutionStatus.COMPLETED ||
      status === PlanExecutionStatus.FAILED
        ? toIso(240)
        : "",
    createdAt: toIso(0),
    updatedAt: steps.reduce(
      (latest, step) => (step.updatedAt > latest ? step.updatedAt : latest),
      toIso(0),
    ),
  });
}

export function mockPlanExecution(
  executionId = DEMO_EXECUTION_ID,
): PlanExecution {
  return buildExecutionFromSequence(executionId, 0);
}

export function nextMockPlanExecution(
  executionId = DEMO_EXECUTION_ID,
): PlanExecution {
  const previousIndex = demoCursorByExecution.get(executionId) ?? 0;
  const nextIndex = Math.min(previousIndex + 1, MOCK_STEP_SEQUENCE.length - 1);
  demoCursorByExecution.set(executionId, nextIndex);
  return buildExecutionFromSequence(executionId, nextIndex);
}

export function resetMockPlanExecution(executionId = DEMO_EXECUTION_ID): void {
  demoCursorByExecution.delete(executionId);
}

export function mockPlanExecutionsList(): PlanExecution[] {
  return [
    buildExecutionFromSequence("exec-live-demo", 1),
    buildExecutionFromSequence("exec-recent-failed", 4),
    create(PlanExecutionSchema, {
      ...buildExecutionFromSequence("exec-recent-success", 3),
      id: "exec-recent-success",
      status: PlanExecutionStatus.COMPLETED,
      completedAt: toIso(180),
      updatedAt: toIso(180),
      stepExecutions: buildExecutionFromSequence(
        "exec-recent-success",
        3,
      ).stepExecutions.map((step) =>
        create(StepExecutionSchema, {
          ...step,
          status: StepExecutionStatus.COMPLETED,
          updatedAt: toIso(180),
        }),
      ),
    }),
  ];
}
