import type {
  Task,
  CreateTaskRequest,
  CreateTaskResponse,
  GetTaskRequest,
  GetTaskResponse,
  ListTasksRequest,
  ListTasksResponse,
  WatchTaskRequest,
  WatchTaskResponse,
  GetCurrentUserRequest,
  GetCurrentUserResponse,
  ListTenantsRequest,
  ListTenantsResponse,
  GetTenantRequest,
  GetTenantResponse,
  RequestFeedbackRequest,
  RequestFeedbackResponse,
  SubmitFeedbackRequest,
  SubmitFeedbackResponse,
  ListPendingFeedbackRequest,
  ListPendingFeedbackResponse,
  GetFeedbackStatusRequest,
  GetFeedbackStatusResponse,
} from "./types";

const baseUrl = "http://localhost:8080";

async function connectRPC<TReq, TRes>(
  service: string,
  method: string,
  request: TReq,
): Promise<TRes> {
  const url = `${baseUrl}/${service}/${method}`;
  const res = await fetch(url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "Connect-Protocol-Version": "1",
    },
    body: JSON.stringify(request),
  });

  if (!res.ok) {
    const text = await res.text();
    throw new Error(`ConnectRPC error ${res.status}: ${text}`);
  }

  return res.json() as Promise<TRes>;
}

export async function createTask(
  req: CreateTaskRequest,
): Promise<CreateTaskResponse> {
  return connectRPC<CreateTaskRequest, CreateTaskResponse>(
    "harpia.tasks.v1.TaskService",
    "CreateTask",
    req,
  );
}

export async function getTask(req: GetTaskRequest): Promise<GetTaskResponse> {
  return connectRPC<GetTaskRequest, GetTaskResponse>(
    "harpia.tasks.v1.TaskService",
    "GetTask",
    req,
  );
}

export async function listTasks(
  req: ListTasksRequest,
): Promise<ListTasksResponse> {
  return connectRPC<ListTasksRequest, ListTasksResponse>(
    "harpia.tasks.v1.TaskService",
    "ListTasks",
    req,
  );
}

export function watchTask(
  tenantId: string,
  taskId: string,
  onUpdate: (task: Task) => void,
  onError?: (err: Error) => void,
): AbortController {
  const controller = new AbortController();
  const url = `${baseUrl}/harpia.tasks.v1.TaskService/WatchTask`;

  const body = JSON.stringify({ tenantId, taskId } satisfies WatchTaskRequest);

  fetch(url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "Connect-Protocol-Version": "1",
    },
    body,
    signal: controller.signal,
  })
    .then(async (res) => {
      if (!res.ok) {
        throw new Error(`ConnectRPC stream error ${res.status}`);
      }

      const reader = res.body?.getReader();
      if (!reader) return;

      const decoder = new TextDecoder();
      let buffer = "";

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split("\n");
        buffer = lines.pop() || "";

        for (const line of lines) {
          if (!line.trim()) continue;
          try {
            const envelope: WatchTaskResponse = JSON.parse(line);
            if (envelope.task) {
              onUpdate(envelope.task);
            }
          } catch {
            // skip unparseable lines
          }
        }
      }
    })
    .catch((err) => {
      if (err.name !== "AbortError") {
        onError?.(err instanceof Error ? err : new Error(String(err)));
      }
    });

  return controller;
}

export async function getCurrentUser(
  req?: GetCurrentUserRequest,
): Promise<GetCurrentUserResponse> {
  return connectRPC<GetCurrentUserRequest, GetCurrentUserResponse>(
    "harpia.identity.v1.IdentityService",
    "GetCurrentUser",
    req ?? {},
  );
}

export async function listTenants(
  req: ListTenantsRequest,
): Promise<ListTenantsResponse> {
  return connectRPC<ListTenantsRequest, ListTenantsResponse>(
    "harpia.identity.v1.IdentityService",
    "ListTenants",
    req,
  );
}

export async function getTenant(
  req: GetTenantRequest,
): Promise<GetTenantResponse> {
  return connectRPC<GetTenantRequest, GetTenantResponse>(
    "harpia.identity.v1.IdentityService",
    "GetTenant",
    req,
  );
}

export async function requestFeedback(
  req: RequestFeedbackRequest,
): Promise<RequestFeedbackResponse> {
  return connectRPC<RequestFeedbackRequest, RequestFeedbackResponse>(
    "harpia.feedback.v1.FeedbackService",
    "RequestFeedback",
    req,
  );
}

export async function submitFeedback(
  req: SubmitFeedbackRequest,
): Promise<SubmitFeedbackResponse> {
  return connectRPC<SubmitFeedbackRequest, SubmitFeedbackResponse>(
    "harpia.feedback.v1.FeedbackService",
    "SubmitFeedback",
    req,
  );
}

export async function listPendingFeedback(
  req: ListPendingFeedbackRequest,
): Promise<ListPendingFeedbackResponse> {
  return connectRPC<ListPendingFeedbackRequest, ListPendingFeedbackResponse>(
    "harpia.feedback.v1.FeedbackService",
    "ListPendingFeedback",
    req,
  );
}

export async function getFeedbackStatus(
  req: GetFeedbackStatusRequest,
): Promise<GetFeedbackStatusResponse> {
  return connectRPC<GetFeedbackStatusRequest, GetFeedbackStatusResponse>(
    "harpia.feedback.v1.FeedbackService",
    "GetFeedbackStatus",
    req,
  );
}
