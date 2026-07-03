import { createClient } from "@connectrpc/connect";
import { AgentService } from "$lib/gen/harpia/agents/v1/agents_pb";
import { ArtifactService } from "$lib/gen/harpia/artifacts/v1/artifacts_pb";
import { ThreadService } from "$lib/gen/harpia/chat/v1/chat_pb";
import { ExecutorService } from "$lib/gen/harpia/executors/v1/executors_pb";
import { IdentityService } from "$lib/gen/harpia/identity/v1/identity_pb";
import { PlanService } from "$lib/gen/harpia/plans/v1/plans_pb";
import { LLMConfigService } from "$lib/gen/harpia/llm_config/v1/llm_config_pb";
import { transport } from "$lib/transport";

export const identityClient = createClient(IdentityService, transport);
export const agentClient = createClient(AgentService, transport);
export const artifactClient = createClient(ArtifactService, transport);
export const planClient = createClient(PlanService, transport);
export const executorClient = createClient(ExecutorService, transport);
export const llmConfigClient = createClient(LLMConfigService, transport);
export const threadClient = createClient(ThreadService, transport);

export type { Tenant, User } from "$lib/gen/harpia/identity/v1/identity_pb";
export type {
  AgentType,
  ListAgentTypesResponse,
} from "$lib/gen/harpia/agents/v1/agents_pb";
export type {
  ExecutorEntitlement,
  ExecutorInstallation,
  ExecutorSKU,
} from "$lib/gen/harpia/executors/v1/executors_pb";
export type {
  GetPlanTemplateByKeyResponse,
  GetPlanTemplateResponse,
  GetPlanExecutionResponse,
  ListPlanExecutionsResponse,
  ListStepExecutionsResponse,
  PlanExecution,
  ListPlanTemplatesResponse,
  PlanStep,
  PlanStepDependency,
  PlanTemplate,
  StepExecution,
  ElicitationRequest,
  ListElicitationsResponse,
  GetElicitationResponse,
  RespondToElicitationResponse,
  WatchElicitationsResponse,
} from "$lib/gen/harpia/plans/v1/plans_pb";
export {
  PlanExecutionStatus,
  StepExecutionStatus,
  ElicitationStatus,
  ElicitationTimeoutBehavior,
  ThreadMessageRole as ElicitationThreadMessageRole,
} from "$lib/gen/harpia/plans/v1/plans_pb";
export type {
  ThreadMessage,
  Thread,
  CreateThreadResponse,
  GetThreadResponse,
  ListThreadsResponse,
  ListThreadMessagesResponse,
  WatchThreadMessagesResponse,
  AppendThreadMessageResponse,
} from "$lib/gen/harpia/chat/v1/chat_pb";
export {
  ThreadMessageRole as ProtoThreadMessageRole,
  ThreadMessageKind as ProtoThreadMessageKind,
  ThreadStatus,
} from "$lib/gen/harpia/chat/v1/chat_pb";
