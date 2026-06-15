import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import type { Page } from "@playwright/test";
import {
  FeedbackDecision,
  FeedbackStatus,
  type FeedbackRequest,
} from "../../src/lib/gen/harpia/feedback/v1/feedback_pb";
import {
  TaskService,
  TaskStatus,
  type Task,
} from "../../src/lib/gen/harpia/tasks/v1/tasks_pb";

type SeedTaskInput = {
  tenantId: string;
  title?: string;
  description?: string;
  workspaceId?: string;
};

const apiBaseURL = process.env.E2E_API_BASE_URL ?? "http://127.0.0.1:19080";

const transport = createConnectTransport({
  baseUrl: apiBaseURL,
  fetch: globalThis.fetch,
});

const taskClient = createClient(TaskService, transport);

function devHeaders(tenantId: string): HeadersInit {
  return {
    authorization: "Bearer dev-token",
    "x-tenant-id": tenantId,
  };
}

export async function seedTask({
  tenantId,
  title = "E2E seeded task",
  description = "Created by Playwright E2E fixture",
  workspaceId = "default",
}: SeedTaskInput): Promise<Task> {
  const response = await taskClient.createTask(
    {
      tenantId,
      workspaceId,
      title,
      description,
    },
    { headers: devHeaders(tenantId) },
  );

  if (!response.task) {
    throw new Error("CreateTask returned no task");
  }

  return response.task;
}

export async function waitForTaskStatus(
  tenantId: string,
  taskId: string,
  status: TaskStatus,
  opts: { timeoutMs?: number; pollMs?: number } = {},
): Promise<Task> {
  const timeoutMs = opts.timeoutMs ?? 10_000;
  const pollMs = opts.pollMs ?? 250;
  const deadline = Date.now() + timeoutMs;

  while (Date.now() < deadline) {
    const response = await taskClient.getTask(
      { tenantId, taskId },
      { headers: devHeaders(tenantId) },
    );

    if (response.task?.status === status) {
      return response.task;
    }

    await new Promise((resolve) => setTimeout(resolve, pollMs));
  }

  throw new Error(`Task ${taskId} did not reach status ${TaskStatus[status]}`);
}

type OverseerJourneyScenario = "gate1_gate2_loop" | "switch_gate1_agent_type";

type OverseerJourneyOptions = {
  tenantId?: string;
  taskId?: string;
  scenario?: OverseerJourneyScenario;
};

type FeedbackRecord = {
  request: FeedbackRequest;
  gate: "gate1" | "gate2";
  attempt: number;
  promptVersion: number;
};

type JourneyState = {
  task: Task;
  feedbacks: FeedbackRecord[];
  dispatches: string[];
};

type E2EOverseerScope = typeof globalThis & {
  __HARPIA_E2E_OVERSEER__?: {
    state: JourneyState;
    feedbackClient: {
      listPendingFeedback: () => AsyncGenerator<{
        feedbackRequests: FeedbackRequest[];
        nextPageToken: string;
      }>;
      getFeedbackStatus: (req: {
        feedbackId: string;
      }) => Promise<{ feedbackRequest?: FeedbackRequest }>;
      submitFeedback: (req: {
        feedbackId: string;
        decision: FeedbackDecision;
        comment: string;
      }) => Promise<{ feedbackRequest?: FeedbackRequest }>;
    };
  };
};

export type OverseerJourneyHarness = {
  tenantId: string;
  taskId: string;
  initialGate1AgentType: string;
  switchedGate1AgentType: string;
  waitForTaskStatus: (status: TaskStatus, timeoutMs?: number) => Promise<Task>;
  getDispatchLog: () => Promise<string[]>;
  dispose: () => Promise<void>;
};

function nowIso(): string {
  return new Date().toISOString();
}

function makeFeedback(params: {
  id: string;
  tenantId: string;
  taskId: string;
  question: string;
  options: string[];
  gate: "gate1" | "gate2";
  attempt: number;
  promptVersion?: number;
}): FeedbackRecord {
  const createdAt = nowIso();
  return {
    gate: params.gate,
    attempt: params.attempt,
    promptVersion: params.promptVersion ?? 1,
    request: {
      id: params.id,
      tenantId: params.tenantId,
      taskId: params.taskId,
      subtaskId: "subtask-1",
      agentInstanceId: `agent-${params.gate}-${params.attempt}`,
      status: FeedbackStatus.PENDING,
      question: params.question,
      options: params.options,
      channel: 1,
      externalRef: "",
      createdAt,
      resolvedAt: "",
    },
  };
}

function seedJourneyState(options: OverseerJourneyOptions): {
  state: JourneyState;
  initialGate1AgentType: string;
  switchedGate1AgentType: string;
} {
  const tenantId = options.tenantId ?? "dev";
  const taskId = options.taskId ?? "task-overseer-e2e";
  const initialGate1AgentType = "research";
  const switchedGate1AgentType = "code";
  const gate1Question =
    "Gate 1 — Approve execution plan for subtask: Draft outreach email sequence.";

  const state: JourneyState = {
    task: {
      id: taskId,
      tenantId,
      workspaceId: "default",
      title: "Overseer journey seeded task",
      description: "Playwright deterministic overseer journey",
      status: TaskStatus.AWAITING_FEEDBACK,
      subtasks: [],
      createdAt: nowIso(),
      updatedAt: nowIso(),
    },
    feedbacks: [
      makeFeedback({
        id: "fb-gate1-attempt1",
        tenantId,
        taskId,
        gate: "gate1",
        attempt: 1,
        question: gate1Question,
        options: [
          "subtask_description=Draft outreach email sequence",
          `suggested_agent_type=${initialGate1AgentType}`,
          "trust_score=0.91",
          "estimated_cost_usd=0.42",
        ],
      }),
    ],
    dispatches: [],
  };

  return { state, initialGate1AgentType, switchedGate1AgentType };
}

export async function installOverseerJourneyMocks(
  page: Page,
  options: OverseerJourneyOptions = {},
): Promise<OverseerJourneyHarness> {
  const scenario = options.scenario ?? "gate1_gate2_loop";
  const seeded = seedJourneyState(options);
  const { state, initialGate1AgentType, switchedGate1AgentType } = seeded;

  await page.addInitScript(
    ({
      seededState,
      seededScenario,
      decisions,
      statuses,
      taskStatuses,
    }: {
      seededState: JourneyState;
      seededScenario: OverseerJourneyScenario;
      decisions: typeof FeedbackDecision;
      statuses: typeof FeedbackStatus;
      taskStatuses: typeof TaskStatus;
    }) => {
      const scope = globalThis as E2EOverseerScope;
      const localState = structuredClone(seededState);

      function buildFeedback(params: {
        id: string;
        tenantId: string;
        taskId: string;
        question: string;
        options: string[];
        gate: "gate1" | "gate2";
        attempt: number;
        promptVersion?: number;
      }): FeedbackRecord {
        return {
          gate: params.gate,
          attempt: params.attempt,
          promptVersion: params.promptVersion ?? 1,
          request: {
            id: params.id,
            tenantId: params.tenantId,
            taskId: params.taskId,
            subtaskId: "subtask-1",
            agentInstanceId: `agent-${params.gate}-${params.attempt}`,
            status: statuses.PENDING,
            question: params.question,
            options: params.options,
            channel: 1,
            externalRef: "",
            createdAt: new Date().toISOString(),
            resolvedAt: "",
          },
        };
      }

      function pendingFeedbacks(): FeedbackRequest[] {
        return localState.feedbacks
          .filter((feedback) => feedback.request.status === statuses.PENDING)
          .map((feedback) => feedback.request);
      }

      function resolveFeedback(
        record: FeedbackRecord,
        status: FeedbackStatus,
      ): void {
        record.request.status = status;
        record.request.resolvedAt = new Date().toISOString();
      }

      function updateTask(status: TaskStatus): void {
        localState.task.status = status;
        localState.task.updatedAt = new Date().toISOString();
      }

      function transition(
        record: FeedbackRecord,
        decision: FeedbackDecision,
        comment: string,
      ): void {
        if (record.gate === "gate1") {
          if (decision === decisions.APPROVE) {
            resolveFeedback(record, statuses.APPROVED);
            localState.dispatches.push(
              `dispatch:${record.request.agentInstanceId}`,
            );
            updateTask(taskStatuses.IN_PROGRESS);
            localState.feedbacks.push(
              buildFeedback({
                id: `fb-gate2-attempt${record.attempt}`,
                tenantId: localState.task.tenantId,
                taskId: localState.task.id,
                gate: "gate2",
                attempt: record.attempt,
                question:
                  "Gate 2 — Agent execution complete. Review output before finalizing.",
                options: [
                  `execution_status=completed_attempt_${record.attempt}`,
                  "output=Drafted personalized outreach sequence",
                  "cost_usd=0.39",
                ],
                promptVersion: record.promptVersion,
              }),
            );
            return;
          }

          if (
            seededScenario === "switch_gate1_agent_type" &&
            decision === decisions.MODIFY
          ) {
            resolveFeedback(record, statuses.MODIFIED);
            const switchedAgentType = "code";
            localState.feedbacks.push(
              buildFeedback({
                id: `fb-gate1-attempt${record.attempt + 1}`,
                tenantId: localState.task.tenantId,
                taskId: localState.task.id,
                gate: "gate1",
                attempt: record.attempt + 1,
                question:
                  "Gate 1 — Updated plan from overseer feedback. Confirm agent switch.",
                options: [
                  "subtask_description=Draft outreach email sequence",
                  `suggested_agent_type=${switchedAgentType}`,
                  "trust_score=0.93",
                  "estimated_cost_usd=0.46",
                  `feedback_comment=${comment || "switch agent type"}`,
                ],
                promptVersion: record.promptVersion + 1,
              }),
            );
            updateTask(taskStatuses.AWAITING_FEEDBACK);
            return;
          }
        }

        if (record.gate === "gate2") {
          if (decision === decisions.REJECT) {
            resolveFeedback(record, statuses.REJECTED);
            updateTask(taskStatuses.AWAITING_FEEDBACK);
            localState.feedbacks.push(
              buildFeedback({
                id: `fb-gate2-attempt${record.attempt + 1}`,
                tenantId: localState.task.tenantId,
                taskId: localState.task.id,
                gate: "gate2",
                attempt: record.attempt + 1,
                question:
                  "Gate 2 — Iteration rerun ready after rejection. Please review regenerated output.",
                options: [
                  "iteration_loop=true",
                  `mutated_prompt_version=${record.promptVersion + 1}`,
                  `feedback_comment=${comment || "no comment"}`,
                  "output=Regenerated outreach sequence with revised tone",
                ],
                promptVersion: record.promptVersion + 1,
              }),
            );
            return;
          }

          if (decision === decisions.APPROVE) {
            resolveFeedback(record, statuses.APPROVED);
            updateTask(taskStatuses.COMPLETED);
            return;
          }
        }

        resolveFeedback(record, statuses.CANCELLED);
        updateTask(taskStatuses.CANCELLED);
      }

      scope.__HARPIA_E2E_OVERSEER__ = {
        state: localState,
        feedbackClient: {
          async *listPendingFeedback() {
            yield {
              feedbackRequests: pendingFeedbacks(),
              nextPageToken: "",
            };
          },
          async getFeedbackStatus(req) {
            const feedback = localState.feedbacks.find(
              (entry) => entry.request.id === req.feedbackId,
            );
            return { feedbackRequest: feedback?.request };
          },
          async submitFeedback(req) {
            const feedback = localState.feedbacks.find(
              (entry) => entry.request.id === req.feedbackId,
            );
            if (!feedback) {
              return { feedbackRequest: undefined };
            }
            transition(feedback, req.decision, req.comment);
            return { feedbackRequest: feedback.request };
          },
        },
      };
    },
    {
      seededState: state,
      seededScenario: scenario,
      decisions: FeedbackDecision,
      statuses: FeedbackStatus,
      taskStatuses: TaskStatus,
    },
  );

  return {
    tenantId: state.task.tenantId,
    taskId: state.task.id,
    initialGate1AgentType,
    switchedGate1AgentType,
    async waitForTaskStatus(
      status: TaskStatus,
      timeoutMs = 10_000,
    ): Promise<Task> {
      await page.waitForFunction(
        ({ expected }) => {
          const scope = globalThis as E2EOverseerScope;
          return scope.__HARPIA_E2E_OVERSEER__?.state.task.status === expected;
        },
        { expected: status },
        { timeout: timeoutMs },
      );
      const task = await page.evaluate(() => {
        const scope = globalThis as E2EOverseerScope;
        return scope.__HARPIA_E2E_OVERSEER__?.state.task;
      });
      if (!task) {
        throw new Error("Mock task state unavailable");
      }
      return task;
    },
    async getDispatchLog(): Promise<string[]> {
      return page.evaluate(() => {
        const scope = globalThis as E2EOverseerScope;
        return scope.__HARPIA_E2E_OVERSEER__?.state.dispatches ?? [];
      });
    },
    async dispose() {
      // No-op for in-browser mock.
    },
  };
}
