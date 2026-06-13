import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
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
