import { create } from "@bufbuild/protobuf";
import {
  PlanConfigurationSchema,
  PlanExecutionSchema,
  PlanExecutionStatus,
  StepExecutionSchema,
  StepExecutionStatus,
  type PlanConfiguration,
  type PlanExecution,
  type StepExecution,
} from "$lib/gen/harpia/plans/v1/plans_pb";

export const MOCK_PLAN_EXECUTION_ID = "e2000000-0000-4000-8000-000000000132";

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

type SnapshotStep = {
  key: string;
  title: string;
};

type PlanConfigurationWithTemplateSnapshot = PlanConfiguration & {
  planTemplateSnapshot?: {
    steps?: SnapshotStep[];
  };
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

function withTemplateSnapshot(
  configuration: PlanConfiguration,
  steps: SnapshotStep[],
): PlanConfiguration {
  const snapshot = configuration as PlanConfigurationWithTemplateSnapshot;
  snapshot.planTemplateSnapshot = { steps };
  return snapshot;
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

export function mockPlanExecutionDetail(): {
  execution: PlanExecution;
  stepExecutions: StepExecution[];
} {
  const configuration = withTemplateSnapshot(
    create(PlanConfigurationSchema, {
      id: "c2000000-0000-4000-8000-000000000001",
      tenantId: "dev",
      workspaceId: "workspace-dev",
      planTemplateId: "a1000000-0000-4000-8000-000000000001",
      planTemplateVersion: 1,
      createdAt: "2026-06-14T15:00:00Z",
      updatedAt: "2026-06-14T15:00:00Z",
    }),
    [
      { key: "fetch-news", title: "Fetch News" },
      { key: "write-draft", title: "Write Draft" },
      { key: "adapt-for-linkedin", title: "Adapt for LinkedIn" },
      { key: "publish-linkedin", title: "Publish LinkedIn" },
    ],
  );

  const stepExecutions = [
    create(StepExecutionSchema, {
      id: "s2000000-0000-4000-8000-000000000001",
      planExecutionId: MOCK_PLAN_EXECUTION_ID,
      planStepKey: "fetch-news",
      status: StepExecutionStatus.COMPLETED,
      inputArtifactId: "artifact-date-range",
      outputArtifactId: "artifact-news-list",
      attempt: 1,
      createdAt: "2026-06-14T15:01:00Z",
      updatedAt: "2026-06-14T15:01:45Z",
    }),
    create(StepExecutionSchema, {
      id: "s2000000-0000-4000-8000-000000000002",
      planExecutionId: MOCK_PLAN_EXECUTION_ID,
      planStepKey: "write-draft",
      status: StepExecutionStatus.COMPLETED,
      inputArtifactId: "artifact-news-list",
      outputArtifactId: "artifact-text-draft",
      attempt: 1,
      createdAt: "2026-06-14T15:02:00Z",
      updatedAt: "2026-06-14T15:03:10Z",
    }),
    create(StepExecutionSchema, {
      id: "s2000000-0000-4000-8000-000000000003",
      planExecutionId: MOCK_PLAN_EXECUTION_ID,
      planStepKey: "adapt-for-linkedin",
      status: StepExecutionStatus.FAILED,
      inputArtifactId: "artifact-text-draft",
      outputArtifactId: "",
      attempt: 1,
      createdAt: "2026-06-14T15:03:20Z",
      updatedAt: "2026-06-14T15:04:05Z",
    }),
  ];

  return {
    execution: create(PlanExecutionSchema, {
      id: MOCK_PLAN_EXECUTION_ID,
      tenantId: "dev",
      planConfigurationId: configuration.id,
      planConfigurationSnapshot: configuration,
      status: PlanExecutionStatus.FAILED,
      stepExecutions,
      triggeredAt: "2026-06-14T15:01:00Z",
      completedAt: "2026-06-14T15:04:05Z",
      createdAt: "2026-06-14T15:01:00Z",
      updatedAt: "2026-06-14T15:04:05Z",
    }),
    stepExecutions,
  };
}
