import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import type { Page, Route } from "@playwright/test";
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

type StubTaskApi = {
  seedTask: (input: SeedTaskInput) => Task;
  waitForTaskStatus: (
    taskId: string,
    status: TaskStatus,
    opts?: { timeoutMs?: number; pollMs?: number },
  ) => Promise<Task>;
  getWatchEventCount: (taskId: string) => number;
  getCreateCallCount: () => number;
  getListCallCount: () => number;
  getWatchCallCount: () => number;
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

function makeTask(input: Required<SeedTaskInput> & { id: string }): Task {
  const now = new Date().toISOString();
  return {
    $typeName: "harpia.tasks.v1.Task",
    id: input.id,
    tenantId: input.tenantId,
    workspaceId: input.workspaceId,
    title: input.title,
    description: input.description,
    status: TaskStatus.PENDING,
    subtasks: [],
    createdAt: now,
    updatedAt: now,
  };
}

function encodeEnvelope(payload: Uint8Array, flags = 0): Uint8Array {
  const out = new Uint8Array(5 + payload.length);
  out[0] = flags;
  const view = new DataView(out.buffer, out.byteOffset, out.byteLength);
  view.setUint32(1, payload.length, false);
  out.set(payload, 5);
  return out;
}

function endStreamEnvelope(): Uint8Array {
  const payload = new TextEncoder().encode(JSON.stringify({ metadata: {} }));
  return encodeEnvelope(payload, 0x02);
}

function toStreamBody(chunks: Uint8Array[]): Buffer {
  return Buffer.concat(chunks.map((chunk) => Buffer.from(chunk)));
}

export async function installTaskApiStub(page: Page): Promise<StubTaskApi> {
  const tasks = new Map<string, Task>();
  const watchEventCount = new Map<string, number>();
  const scheduled = new Map<string, Timer[]>();
  let seq = 0;
  let createCallCount = 0;
  let listCallCount = 0;
  let watchCallCount = 0;

  const bumpWatchEvent = (taskId: string) => {
    const next = (watchEventCount.get(taskId) ?? 0) + 1;
    watchEventCount.set(taskId, next);
  };

  const updateTask = (taskId: string, status: TaskStatus): Task => {
    const task = tasks.get(taskId);
    if (!task) {
      throw new Error(`Tried to update missing task ${taskId}`);
    }
    const nextTask: Task = {
      ...task,
      status,
      updatedAt: new Date().toISOString(),
    };
    tasks.set(taskId, nextTask);
    return nextTask;
  };

  const scheduleFastCompletion = (taskId: string) => {
    const timers: Timer[] = [];
    const states = [
      TaskStatus.PLANNING,
      TaskStatus.IN_PROGRESS,
      TaskStatus.COMPLETED,
    ];
    states.forEach((status, index) => {
      const timer = setTimeout(
        () => {
          updateTask(taskId, status);
        },
        (index + 1) * 80,
      );
      timers.push(timer);
    });
    scheduled.set(taskId, timers);
  };

  const seedTaskLocal = ({
    tenantId,
    title = "E2E seeded task",
    description = "Created by Playwright E2E fixture",
    workspaceId = "default",
  }: SeedTaskInput): Task => {
    seq += 1;
    const id = `e2e-task-${seq}`;
    const task = makeTask({ id, tenantId, title, description, workspaceId });
    tasks.set(id, task);
    watchEventCount.set(id, 0);
    return task;
  };

  const fulfillUnary = async (
    route: Route,
    jsonBody: object,
  ): Promise<void> => {
    await route.fulfill({
      status: 200,
      headers: { "content-type": "application/json" },
      body: JSON.stringify(jsonBody),
    });
  };

  await page.route("**/harpia.tasks.v1.TaskService/*", async (route) => {
    const url = new URL(route.request().url());
    const method = url.pathname.split("/").at(-1) ?? "";

    if (method === "CreateTask") {
      createCallCount += 1;
      const task = seedTaskLocal({
        tenantId: "dev",
        title: "Leader journey stub task",
        description: "Leader journey stub task",
        workspaceId: "default",
      });
      scheduleFastCompletion(task.id);
      await fulfillUnary(route, { task });
      return;
    }

    if (method === "GetTask") {
      const latest = Array.from(tasks.values()).at(-1);
      await fulfillUnary(route, { task: latest ?? null });
      return;
    }

    if (method === "ListTasks") {
      listCallCount += 1;
      const payload = new TextEncoder().encode(
        JSON.stringify({
          tasks: Array.from(tasks.values()),
          nextPageToken: "",
        }),
      );
      const body = toStreamBody([encodeEnvelope(payload), endStreamEnvelope()]);
      await route.fulfill({
        status: 200,
        headers: { "content-type": "application/connect+json" },
        body,
      });
      return;
    }

    if (method === "WatchTask") {
      watchCallCount += 1;
      const task = Array.from(tasks.values()).at(-1);
      if (!task) {
        await route.fulfill({
          status: 404,
          headers: { "content-type": "application/json" },
          body: JSON.stringify({ message: "No task found for watch" }),
        });
        return;
      }

      const planning = updateTask(task.id, TaskStatus.PLANNING);
      const inProgress = updateTask(task.id, TaskStatus.IN_PROGRESS);
      const completed = updateTask(task.id, TaskStatus.COMPLETED);
      bumpWatchEvent(task.id);
      bumpWatchEvent(task.id);
      bumpWatchEvent(task.id);

      const planningPayload = new TextEncoder().encode(
        JSON.stringify({ task: planning }),
      );
      const inProgressPayload = new TextEncoder().encode(
        JSON.stringify({ task: inProgress }),
      );
      const completedPayload = new TextEncoder().encode(
        JSON.stringify({ task: completed }),
      );

      const body = toStreamBody([
        encodeEnvelope(planningPayload),
        encodeEnvelope(inProgressPayload),
        encodeEnvelope(completedPayload),
        endStreamEnvelope(),
      ]);

      await route.fulfill({
        status: 200,
        headers: { "content-type": "application/connect+json" },
        body,
      });
      return;
    }

    await route.fulfill({
      status: 404,
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        message: `Unhandled TaskService method: ${method}`,
      }),
    });
  });

  return {
    seedTask: seedTaskLocal,
    async waitForTaskStatus(taskId, status, opts = {}) {
      const timeoutMs = opts.timeoutMs ?? 5_000;
      const pollMs = opts.pollMs ?? 50;
      const deadline = Date.now() + timeoutMs;
      while (Date.now() < deadline) {
        const task = tasks.get(taskId);
        if (task?.status === status) {
          return task;
        }
        await new Promise((resolve) => setTimeout(resolve, pollMs));
      }
      throw new Error(
        `Stub task ${taskId} did not reach status ${TaskStatus[status]} in ${timeoutMs}ms`,
      );
    },
    getWatchEventCount(taskId) {
      return watchEventCount.get(taskId) ?? 0;
    },
    getCreateCallCount() {
      return createCallCount;
    },
    getListCallCount() {
      return listCallCount;
    },
    getWatchCallCount() {
      return watchCallCount;
    },
  };
}
