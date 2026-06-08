import { createClient } from "@connectrpc/connect";
import { FeedbackService } from "$lib/gen/harpia/feedback/v1/feedback_pb";
import { IdentityService } from "$lib/gen/harpia/identity/v1/identity_pb";
import { TaskService } from "$lib/gen/harpia/tasks/v1/tasks_pb";
import { transport } from "$lib/transport";

export const taskClient = createClient(TaskService, transport);
export const identityClient = createClient(IdentityService, transport);
export const feedbackClient = createClient(FeedbackService, transport);

export type {
  FeedbackRequest,
  GetFeedbackStatusResponse,
  ListPendingFeedbackResponse,
  RequestFeedbackResponse,
  SubmitFeedbackResponse,
} from "$lib/gen/harpia/feedback/v1/feedback_pb";
export {
  FeedbackChannel,
  FeedbackDecision,
  FeedbackStatus,
} from "$lib/gen/harpia/feedback/v1/feedback_pb";
export type { Tenant, User } from "$lib/gen/harpia/identity/v1/identity_pb";
export type {
  CreateTaskResponse,
  GetTaskResponse,
  ListTasksResponse,
  Subtask,
  Task,
  WatchTaskResponse,
} from "$lib/gen/harpia/tasks/v1/tasks_pb";
export { SubtaskStatus, TaskStatus } from "$lib/gen/harpia/tasks/v1/tasks_pb";
