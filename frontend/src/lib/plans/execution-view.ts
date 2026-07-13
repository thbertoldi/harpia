import {
  ExecutionInteractionKind,
  ExecutionPromptState,
  type ChatMessage,
} from "$lib/chat/types";
import type { ExecutionGroup } from "$lib/plans/thread";
import type {
  PlanExecution,
  PlanTemplate,
} from "$lib/gen/harpia/plans/v1/plans_pb";

export type StepStatus = "pending" | "running" | "done" | "failed";
export type ExecutionState =
  | "idle"
  | "queued"
  | "running"
  | "failing"
  | "completed"
  | "failed"
  | "cancelled"
  | "needs-attention";

export interface OrderedStep {
  key: string;
  title: string;
  detail?: string;
}

export interface ExecutionStepView {
  key: string;
  title: string;
  detail?: string;
  status: StepStatus;
  outputArtifactId: string | null;
  outputArtifactVersionId?: string | null;
  outputArtifactTypeKey?: string | null;
  outputContentHash?: string | null;
}

export interface ArtifactReferenceView {
  artifactId: string;
  artifactVersionId: string;
  artifactTypeKey: string;
  contentHash: string;
}

export interface ExecutionApprovalView {
  approvalRequestId: string;
  inputArtifactId: string;
  subjectArtifactRef: ArtifactReferenceView | null;
  planStepKey: string;
  status: "pending" | "approved" | "rejected";
  message: ChatMessage;
}

export interface ExecutionReviewView {
  reviewRequestId: string;
  planStepKey: string;
  subjectArtifactRef: ArtifactReferenceView | null;
  status: "pending" | "accepted" | "revision_requested";
  message: ChatMessage;
}

export type ExecutionInteractionKindView =
  | "elicitation"
  | "review"
  | "approval";

export interface ExecutionInteractionView {
  kind: ExecutionInteractionKindView;
  requestId: string;
  stepExecutionId: string;
  planStepKey: string;
  subjectArtifactRef: ArtifactReferenceView | null;
}

export interface ExecutionPromptActionView {
  actionId: string;
  labelKey: string;
}

/** Exact assistant turn emitted for this execution, never inferred from another run. */
export interface ExecutionAssistantTurn {
  configurationId: string;
  executionId: string;
  state: ExecutionPromptState;
  stepExecutionId: string;
  planStepKey: string;
  completedStepCount: number;
  activeStepCount: number;
  pendingInteraction: ExecutionInteractionView | null;
  latestArtifactRef: ArtifactReferenceView | null;
  actions: ExecutionPromptActionView[];
  message: ChatMessage;
}

export interface ExecutionViewModel {
  executionId: string;
  runNumber: number;
  state: ExecutionState;
  steps: ExecutionStepView[];
  doneCount: number;
  total: number;
  /** done + (running ? 0.5 : 0), in [0, 1]. 0 when there are no steps. */
  progress: number;
  runningStep: ExecutionStepView | null;
  failedStep: ExecutionStepView | null;
  /** Running step's title/detail, for the collapsed header; null once settled. */
  currentStep: { title: string; detail: string } | null;
  /** True when the ordered step list is empty (summary-only rendering). */
  degraded: boolean;
  messages: ChatMessage[];
  approvals: ExecutionApprovalView[];
  pendingApproval: ExecutionApprovalView | null;
  reviews: ExecutionReviewView[];
  pendingReview: ExecutionReviewView | null;
  assistantTurn: ExecutionAssistantTurn | null;
}

function parsedRecord(payloadJson: string): Record<string, unknown> | null {
  if (!payloadJson) return null;
  try {
    const parsed = JSON.parse(payloadJson) as unknown;
    return parsed && typeof parsed === "object"
      ? (parsed as Record<string, unknown>)
      : null;
  } catch {
    return null;
  }
}

function stringField(
  value: Record<string, unknown>,
  snake: string,
  camel: string,
): string {
  const field = value[snake] ?? value[camel];
  return typeof field === "string" ? field : "";
}

function numberField(
  value: Record<string, unknown>,
  snake: string,
  camel: string,
): number {
  const field = value[snake] ?? value[camel];
  return typeof field === "number" && Number.isFinite(field) ? field : 0;
}

function promptState(value: unknown): ExecutionPromptState | null {
  const states: Record<string, ExecutionPromptState> = {
    EXECUTION_PROMPT_STATE_CONFIGURING: ExecutionPromptState.CONFIGURING,
    EXECUTION_PROMPT_STATE_READY_TO_RUN: ExecutionPromptState.READY_TO_RUN,
    EXECUTION_PROMPT_STATE_WAITING_FOR_SCHEDULE:
      ExecutionPromptState.WAITING_FOR_SCHEDULE,
    EXECUTION_PROMPT_STATE_CONFIGURATION_DISABLED:
      ExecutionPromptState.CONFIGURATION_DISABLED,
    EXECUTION_PROMPT_STATE_CONFIGURATION_ARCHIVED:
      ExecutionPromptState.CONFIGURATION_ARCHIVED,
    EXECUTION_PROMPT_STATE_EXECUTION_QUEUED:
      ExecutionPromptState.EXECUTION_QUEUED,
    EXECUTION_PROMPT_STATE_EXECUTION_RUNNING:
      ExecutionPromptState.EXECUTION_RUNNING,
    EXECUTION_PROMPT_STATE_EXECUTION_AWAITING_ELICITATION:
      ExecutionPromptState.EXECUTION_AWAITING_ELICITATION,
    EXECUTION_PROMPT_STATE_EXECUTION_AWAITING_REVIEW:
      ExecutionPromptState.EXECUTION_AWAITING_REVIEW,
    EXECUTION_PROMPT_STATE_EXECUTION_AWAITING_APPROVAL:
      ExecutionPromptState.EXECUTION_AWAITING_APPROVAL,
    EXECUTION_PROMPT_STATE_EXECUTION_FAILING:
      ExecutionPromptState.EXECUTION_FAILING,
    EXECUTION_PROMPT_STATE_EXECUTION_FAILED:
      ExecutionPromptState.EXECUTION_FAILED,
    EXECUTION_PROMPT_STATE_EXECUTION_COMPLETED:
      ExecutionPromptState.EXECUTION_COMPLETED,
    EXECUTION_PROMPT_STATE_EXECUTION_CANCELLED:
      ExecutionPromptState.EXECUTION_CANCELLED,
    EXECUTION_PROMPT_STATE_EXECUTION_NEEDS_ATTENTION:
      ExecutionPromptState.EXECUTION_NEEDS_ATTENTION,
  };
  if (typeof value === "number" && value in ExecutionPromptState) return value;
  return typeof value === "string" ? (states[value] ?? null) : null;
}

function interactionKind(value: unknown): ExecutionInteractionKindView | null {
  switch (value) {
    case "EXECUTION_INTERACTION_KIND_ELICITATION":
    case ExecutionInteractionKind.ELICITATION:
      return "elicitation";
    case "EXECUTION_INTERACTION_KIND_REVIEW":
    case ExecutionInteractionKind.REVIEW:
      return "review";
    case "EXECUTION_INTERACTION_KIND_APPROVAL":
    case ExecutionInteractionKind.APPROVAL:
      return "approval";
    default:
      return null;
  }
}

/** Parses a server-authored proto JSON prompt, rejecting incomplete identities. */
export function parseExecutionPrompt(
  message: ChatMessage,
): ExecutionAssistantTurn | null {
  if (message.kind !== "EXECUTION_PROMPT") return null;
  const raw = parsedRecord(message.payloadJson);
  if (!raw) return null;
  const executionId = stringField(raw, "plan_execution_id", "planExecutionId");
  const configurationId = stringField(
    raw,
    "configuration_id",
    "configurationId",
  );
  const state = promptState(raw.state);
  if (
    !executionId ||
    !configurationId ||
    !state ||
    executionId !== message.executionId
  )
    return null;
  const pendingRaw = raw.pending_interaction ?? raw.pendingInteraction;
  let pendingInteraction: ExecutionInteractionView | null = null;
  if (pendingRaw && typeof pendingRaw === "object") {
    const pointer = pendingRaw as Record<string, unknown>;
    const kind = interactionKind(pointer.kind);
    const requestId = stringField(pointer, "request_id", "requestId");
    const stepExecutionId = stringField(
      pointer,
      "step_execution_id",
      "stepExecutionId",
    );
    const planStepKey = stringField(pointer, "plan_step_key", "planStepKey");
    if (!kind || !requestId || !stepExecutionId || !planStepKey) return null;
    pendingInteraction = {
      kind,
      requestId,
      stepExecutionId,
      planStepKey,
      subjectArtifactRef: parseArtifactReference(
        pointer.subject_artifact_ref ?? pointer.subjectArtifactRef,
      ),
    };
  }
  const actions = Array.isArray(raw.actions)
    ? raw.actions.flatMap((action): ExecutionPromptActionView[] => {
        if (!action || typeof action !== "object") return [];
        const value = action as Record<string, unknown>;
        const actionId = stringField(value, "action_id", "actionId");
        const labelKey = stringField(value, "label_key", "labelKey");
        return actionId && labelKey ? [{ actionId, labelKey }] : [];
      })
    : [];
  return {
    configurationId,
    executionId,
    state,
    stepExecutionId: stringField(raw, "step_execution_id", "stepExecutionId"),
    planStepKey: stringField(raw, "plan_step_key", "planStepKey"),
    completedStepCount: numberField(
      raw,
      "completed_step_count",
      "completedStepCount",
    ),
    activeStepCount: numberField(raw, "active_step_count", "activeStepCount"),
    pendingInteraction,
    latestArtifactRef: parseArtifactReference(
      raw.latest_artifact_ref ?? raw.latestArtifactRef,
    ),
    actions,
    message,
  };
}

export function orderedStepsFromFrozenExecution(
  execution: PlanExecution | undefined,
): OrderedStep[] {
  const template: PlanTemplate | undefined = execution?.planTemplateSnapshot;
  if (!template) return [];
  const active = new Set(execution?.activeStepKeys ?? []);
  return template.steps
    .filter((step) => active.has(step.key))
    .map((step) => ({
      key: step.key,
      title: step.title || step.key,
      detail: step.description || undefined,
    }));
}

export function parseArtifactReference(
  raw: unknown,
): ArtifactReferenceView | null {
  if (!raw || typeof raw !== "object") return null;
  const value = raw as Record<string, unknown>;
  const field = (snake: string, camel: string) => value[snake] ?? value[camel];
  const artifactId = field("artifact_id", "artifactId");
  const artifactVersionId = field("artifact_version_id", "artifactVersionId");
  const artifactTypeKey = field("artifact_type_key", "artifactTypeKey");
  const contentHash = field("content_hash", "contentHash");
  return typeof artifactId === "string" &&
    typeof artifactVersionId === "string" &&
    typeof artifactTypeKey === "string" &&
    typeof contentHash === "string" &&
    artifactId &&
    artifactVersionId &&
    artifactTypeKey &&
    contentHash
    ? { artifactId, artifactVersionId, artifactTypeKey, contentHash }
    : null;
}

export function parseStepBoundArtifactRef(
  payloadJson: string,
): ArtifactReferenceView | null {
  const raw = parsedRecord(payloadJson);
  if (!raw) return null;
  return parseArtifactReference({
    artifact_id: raw.output_artifact_id,
    artifact_version_id: raw.output_artifact_version_id,
    artifact_type_key: raw.output_artifact_type_key,
    content_hash: raw.output_content_hash,
  });
}

/**
 * Parse `step_key` from a STEP_STARTED / STEP_BOUND payload. Returns null on
 * malformed JSON or missing key so callers can ignore unknown shapes.
 */
export function parseStepKey(payloadJson: string): string | null {
  if (!payloadJson) return null;
  try {
    const parsed = JSON.parse(payloadJson) as unknown;
    if (parsed && typeof parsed === "object" && "step_key" in parsed) {
      const key = (parsed as Record<string, unknown>).step_key;
      return typeof key === "string" && key.length > 0 ? key : null;
    }
  } catch {
    // malformed payload — degrade gracefully
  }
  return null;
}

/**
 * Parse `output_artifact_id` from a STEP_BOUND payload. Returns null on
 * malformed JSON or missing key so callers can degrade to non-artifact
 * rendering.
 */
export function parseOutputArtifactId(payloadJson: string): string | null {
  return (
    parseStepBoundArtifactRef(payloadJson)?.artifactId ??
    (() => {
      const raw = parsedRecord(payloadJson);
      return typeof raw?.output_artifact_id === "string" &&
        raw.output_artifact_id
        ? raw.output_artifact_id
        : null;
    })()
  );
}

export function parseApprovalPayload(payloadJson: string): {
  approvalRequestId: string;
  approved: boolean | null;
  inputArtifactId: string;
  planStepKey: string;
  subjectArtifactRef: ArtifactReferenceView | null;
} | null {
  if (!payloadJson) return null;
  try {
    const parsed = JSON.parse(payloadJson) as unknown;
    if (
      parsed &&
      typeof parsed === "object" &&
      "approval_request_id" in parsed
    ) {
      const raw = parsed as Record<string, unknown>;
      const approvalRequestId = raw.approval_request_id;
      if (typeof approvalRequestId !== "string" || !approvalRequestId) {
        return null;
      }
      return {
        approvalRequestId,
        approved: typeof raw.approved === "boolean" ? raw.approved : null,
        inputArtifactId:
          typeof raw.input_artifact_id === "string"
            ? raw.input_artifact_id
            : "",
        planStepKey:
          typeof raw.plan_step_key === "string" ? raw.plan_step_key : "",
        subjectArtifactRef: parseArtifactReference(raw.subject_artifact_ref),
      };
    }
  } catch {
    // malformed payload — degrade gracefully
  }
  return null;
}

/**
 * Fold an execution group's message stream (already grouped by executionId via
 * buildThreadSections) into a per-step status view model. Events are processed
 * in sequence-number order. Folding rules (ADR/live-execution-chat design §2):
 *   STEP_STARTED{step_key} -> that step running (prior running step settled done)
 *   STEP_BOUND{step_key}   -> that step done   (per chat.proto:162 = step completed)
 *   RUN_FAILED              -> the running step failed, execution failed
 *   RUN_COMPLETED           -> any running step forced done, execution completed
 * Unknown step keys and malformed payloads are ignored so the card never throws.
 */
export function buildExecutionViewModel(
  group: Pick<ExecutionGroup, "executionId" | "runNumber" | "messages">,
  orderedSteps: OrderedStep[],
): ExecutionViewModel {
  const steps: ExecutionStepView[] = orderedSteps.map((s) => ({
    key: s.key,
    title: s.title,
    detail: s.detail,
    status: "pending",
    outputArtifactId: null,
    outputArtifactVersionId: null,
    outputArtifactTypeKey: null,
    outputContentHash: null,
  }));
  const byKey = new Map(steps.map((s) => [s.key, s]));
  const approvalsById = new Map<string, ExecutionApprovalView>();
  const reviewsById = new Map<string, ExecutionReviewView>();
  let state: ExecutionState = "idle";
  let runningKey: string | null = null;
  let assistantTurn: ExecutionAssistantTurn | null = null;

  const ordered = [...group.messages].sort((a, b) =>
    Number(a.sequenceNumber - b.sequenceNumber),
  );

  const settle = (key: string | null) => {
    if (key && byKey.has(key) && byKey.get(key)!.status === "running") {
      byKey.get(key)!.status = "done";
    }
  };

  for (const m of ordered) {
    switch (m.kind) {
      case "EXECUTION_PROMPT": {
        const turn = parseExecutionPrompt(m);
        if (!turn || turn.executionId !== group.executionId) break;
        assistantTurn = turn;
        switch (turn.state) {
          case ExecutionPromptState.EXECUTION_QUEUED:
            state = "queued";
            break;
          case ExecutionPromptState.EXECUTION_RUNNING:
          case ExecutionPromptState.EXECUTION_AWAITING_ELICITATION:
          case ExecutionPromptState.EXECUTION_AWAITING_REVIEW:
          case ExecutionPromptState.EXECUTION_AWAITING_APPROVAL:
            state = "running";
            break;
          case ExecutionPromptState.EXECUTION_FAILING:
            state = "failing";
            break;
          case ExecutionPromptState.EXECUTION_FAILED:
            state = "failed";
            break;
          case ExecutionPromptState.EXECUTION_COMPLETED:
            state = "completed";
            break;
          case ExecutionPromptState.EXECUTION_CANCELLED:
            state = "cancelled";
            break;
          case ExecutionPromptState.EXECUTION_NEEDS_ATTENTION:
            state = "needs-attention";
            break;
        }
        break;
      }
      case "RUN_STARTED":
        if (state === "idle") state = "running";
        break;
      case "STEP_STARTED": {
        const key = parseStepKey(m.payloadJson);
        if (state === "idle") state = "running";
        // Settle any previously running step (tolerates a missing/out-of-order
        // STEP_BOUND for it).
        if (runningKey && runningKey !== key) settle(runningKey);
        if (key && byKey.has(key)) {
          byKey.get(key)!.status = "running";
          runningKey = key;
        }
        break;
      }
      case "STEP_BOUND": {
        const key = parseStepKey(m.payloadJson);
        const outputArtifactId = parseOutputArtifactId(m.payloadJson);
        const outputArtifactRef = parseStepBoundArtifactRef(m.payloadJson);
        if (key && byKey.has(key)) {
          const step = byKey.get(key)!;
          step.status = "done";
          step.outputArtifactId = outputArtifactId;
          step.outputArtifactVersionId =
            outputArtifactRef?.artifactVersionId ?? null;
          step.outputArtifactTypeKey =
            outputArtifactRef?.artifactTypeKey ?? null;
          step.outputContentHash = outputArtifactRef?.contentHash ?? null;
        }
        if (key && runningKey === key) runningKey = null;
        else if (!key && runningKey) {
          // No key on the bound event: assume the running step completed.
          settle(runningKey);
          if (outputArtifactId && byKey.has(runningKey)) {
            byKey.get(runningKey)!.outputArtifactId = outputArtifactId;
            byKey.get(runningKey)!.outputArtifactVersionId =
              outputArtifactRef?.artifactVersionId ?? null;
            byKey.get(runningKey)!.outputArtifactTypeKey =
              outputArtifactRef?.artifactTypeKey ?? null;
            byKey.get(runningKey)!.outputContentHash =
              outputArtifactRef?.contentHash ?? null;
          }
          runningKey = null;
        }
        break;
      }
      case "RUN_FAILED":
        state = "failed";
        if (runningKey && byKey.has(runningKey)) {
          byKey.get(runningKey)!.status = "failed";
        }
        runningKey = null;
        break;
      case "RUN_COMPLETED":
        state = "completed";
        for (const s of steps) if (s.status === "running") s.status = "done";
        runningKey = null;
        break;
      case "APPROVAL_RAISED": {
        const parsed = parseApprovalPayload(m.payloadJson);
        if (!parsed) break;
        approvalsById.set(parsed.approvalRequestId, {
          approvalRequestId: parsed.approvalRequestId,
          inputArtifactId: parsed.inputArtifactId,
          subjectArtifactRef: parsed.subjectArtifactRef,
          planStepKey: parsed.planStepKey,
          status: "pending",
          message: m,
        });
        break;
      }
      case "APPROVAL_DECIDED": {
        const parsed = parseApprovalPayload(m.payloadJson);
        if (!parsed) break;
        const existing = approvalsById.get(parsed.approvalRequestId);
        approvalsById.set(parsed.approvalRequestId, {
          approvalRequestId: parsed.approvalRequestId,
          inputArtifactId:
            parsed.inputArtifactId || existing?.inputArtifactId || "",
          subjectArtifactRef:
            parsed.subjectArtifactRef ?? existing?.subjectArtifactRef ?? null,
          planStepKey: parsed.planStepKey || existing?.planStepKey || "",
          status: parsed.approved === false ? "rejected" : "approved",
          message: existing?.message ?? m,
        });
        break;
      }
      case "REVIEW_RAISED": {
        const raw = parsedRecord(m.payloadJson);
        const reviewRequestId =
          typeof raw?.review_request_id === "string"
            ? raw.review_request_id
            : "";
        if (!reviewRequestId) break;
        reviewsById.set(reviewRequestId, {
          reviewRequestId,
          planStepKey:
            typeof raw?.plan_step_key === "string" ? raw.plan_step_key : "",
          subjectArtifactRef: parseArtifactReference(raw?.subject_artifact_ref),
          status: "pending",
          message: m,
        });
        break;
      }
      case "REVIEW_DECIDED": {
        const raw = parsedRecord(m.payloadJson);
        const reviewRequestId =
          typeof raw?.review_request_id === "string"
            ? raw.review_request_id
            : "";
        const existing = reviewsById.get(reviewRequestId);
        if (!reviewRequestId || !existing) break;
        reviewsById.set(reviewRequestId, {
          ...existing,
          status:
            raw?.decision === "revise" ? "revision_requested" : "accepted",
        });
        break;
      }
      default:
        break;
    }
  }

  const doneCount = steps.filter((s) => s.status === "done").length;
  const total = steps.length;
  const runningStep = steps.find((s) => s.status === "running") ?? null;
  const failedStep = steps.find((s) => s.status === "failed") ?? null;
  const approvals = [...approvalsById.values()];
  const pendingApproval =
    approvals.find((approval) => approval.status === "pending") ?? null;
  const reviews = [...reviewsById.values()];
  const pendingReview =
    reviews.find((review) => review.status === "pending") ?? null;
  const runningFrac = runningStep ? 0.5 : 0;
  const progress = total > 0 ? (doneCount + runningFrac) / total : 0;
  const currentStep =
    state === "running" && runningStep
      ? { title: runningStep.title, detail: runningStep.detail ?? "" }
      : null;

  return {
    executionId: group.executionId,
    runNumber: group.runNumber,
    state,
    steps,
    doneCount,
    total,
    progress,
    runningStep,
    failedStep,
    currentStep,
    degraded: total === 0,
    messages: ordered,
    approvals,
    pendingApproval,
    reviews,
    pendingReview,
    assistantTurn,
  };
}

/**
 * Whether an execution is currently in flight — used by the header status pill.
 */
export function isExecutionActive(vm: ExecutionViewModel): boolean {
  return vm.state === "running";
}
