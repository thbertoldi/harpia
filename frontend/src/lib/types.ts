export enum TaskStatus {
  UNSPECIFIED = 0,
  PENDING = 1,
  PLANNING = 2,
  IN_PROGRESS = 3,
  AWAITING_FEEDBACK = 4,
  COMPLETED = 5,
  FAILED = 6,
  CANCELLED = 7,
}

export enum SubtaskStatus {
  UNSPECIFIED = 0,
  PENDING = 1,
  IN_PROGRESS = 2,
  AWAITING_FEEDBACK = 3,
  COMPLETED = 4,
  FAILED = 5,
}

export enum FeedbackStatus {
  UNSPECIFIED = 0,
  PENDING = 1,
  APPROVED = 2,
  REJECTED = 3,
  MODIFIED = 4,
  EXPIRED = 5,
  CANCELLED = 6,
}

export enum FeedbackChannel {
  UNSPECIFIED = 0,
  UI = 1,
  SLACK = 2,
  EMAIL = 3,
}

export enum FeedbackDecision {
  UNSPECIFIED = 0,
  APPROVE = 1,
  REJECT = 2,
  MODIFY = 3,
}

export enum AgentInstanceStatus {
  UNSPECIFIED = 0,
  IDLE = 1,
  PLANNING = 2,
  EXECUTING = 3,
  AWAITING_FEEDBACK = 4,
  COMPLETED = 5,
  FAILED = 6,
}

export interface Tenant {
  id: string;
  name: string;
  slug: string;
  createdAt: string;
}

export interface User {
  id: string;
  tenantId: string;
  email: string;
  displayName: string;
  avatarUrl: string;
  roles: string[];
  createdAt: string;
}

export interface Subtask {
  id: string;
  taskId: string;
  description: string;
  status: SubtaskStatus;
  assignedAgentId: string;
  createdAt: string;
  updatedAt: string;
}

export interface Task {
  id: string;
  tenantId: string;
  workspaceId: string;
  title: string;
  description: string;
  status: TaskStatus;
  subtasks: Subtask[];
  createdAt: string;
  updatedAt: string;
}

export interface AgentType {
  id: string;
  name: string;
  description: string;
  capabilitiesText: string;
  createdAt: string;
}

export interface AgentInstance {
  id: string;
  agentTypeId: string;
  tenantId: string;
  taskId: string;
  subtaskId: string;
  status: AgentInstanceStatus;
  createdAt: string;
  updatedAt: string;
}

export interface AgentMatch {
  agentType: AgentType;
  similarityScore: number;
}

export interface FeedbackRequest {
  id: string;
  tenantId: string;
  taskId: string;
  subtaskId: string;
  agentInstanceId: string;
  status: FeedbackStatus;
  question: string;
  options: string[];
  channel: FeedbackChannel;
  externalRef: string;
  createdAt: string;
  resolvedAt: string;
}

export interface CreateTaskRequest {
  tenantId: string;
  workspaceId: string;
  title: string;
  description: string;
}

export interface CreateTaskResponse {
  task: Task;
}

export interface GetTaskRequest {
  tenantId: string;
  taskId: string;
}

export interface GetTaskResponse {
  task: Task;
}

export interface ListTasksRequest {
  tenantId: string;
  workspaceId?: string;
  status?: TaskStatus;
  pageSize: number;
  pageToken: string;
}

export interface ListTasksResponse {
  tasks: Task[];
  nextPageToken: string;
}

export interface WatchTaskRequest {
  tenantId: string;
  taskId: string;
}

export interface WatchTaskResponse {
  task: Task;
}

export interface GetCurrentUserRequest {}

export interface GetCurrentUserResponse {
  user: User;
  tenants: Tenant[];
}

export interface ListTenantsRequest {
  pageSize: number;
  pageToken: string;
}

export interface ListTenantsResponse {
  tenants: Tenant[];
  nextPageToken: string;
}

export interface GetTenantRequest {
  tenantId: string;
}

export interface GetTenantResponse {
  tenant: Tenant;
}

export interface RequestFeedbackRequest {
  tenantId: string;
  taskId: string;
  subtaskId: string;
  agentInstanceId: string;
  question: string;
  options: string[];
  channels: FeedbackChannel[];
}

export interface RequestFeedbackResponse {
  feedbackRequest: FeedbackRequest;
}

export interface SubmitFeedbackRequest {
  tenantId: string;
  feedbackId: string;
  decision: FeedbackDecision;
  comment: string;
}

export interface SubmitFeedbackResponse {
  feedbackRequest: FeedbackRequest;
}

export interface ListPendingFeedbackRequest {
  tenantId: string;
  pageSize: number;
  pageToken: string;
}

export interface ListPendingFeedbackResponse {
  feedbackRequests: FeedbackRequest[];
  nextPageToken: string;
}

export interface GetFeedbackStatusRequest {
  tenantId: string;
  feedbackId: string;
}

export interface GetFeedbackStatusResponse {
  feedbackRequest: FeedbackRequest;
}
