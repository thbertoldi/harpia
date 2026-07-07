import type { ChatMessage } from "$lib/chat/types";
import type { ExecutionGroup } from "$lib/plans/thread";

export type StepStatus = "pending" | "running" | "done" | "failed";
export type ExecutionState = "idle" | "running" | "completed" | "failed";

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
}

export interface ExecutionApprovalView {
  approvalRequestId: string;
  inputArtifactId: string;
  planStepKey: string;
  status: "pending" | "approved" | "rejected";
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
  if (!payloadJson) return null;
  try {
    const parsed = JSON.parse(payloadJson) as unknown;
    if (
      parsed &&
      typeof parsed === "object" &&
      "output_artifact_id" in parsed
    ) {
      const id = (parsed as Record<string, unknown>).output_artifact_id;
      return typeof id === "string" && id.length > 0 ? id : null;
    }
  } catch {
    // malformed payload — degrade gracefully
  }
  return null;
}

export function parseApprovalPayload(payloadJson: string): {
  approvalRequestId: string;
  approved: boolean | null;
  inputArtifactId: string;
  planStepKey: string;
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
  }));
  const byKey = new Map(steps.map((s) => [s.key, s]));
  const approvalsById = new Map<string, ExecutionApprovalView>();
  let state: ExecutionState = "idle";
  let runningKey: string | null = null;

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
        if (key && byKey.has(key)) {
          const step = byKey.get(key)!;
          step.status = "done";
          step.outputArtifactId = outputArtifactId;
        }
        if (key && runningKey === key) runningKey = null;
        else if (!key && runningKey) {
          // No key on the bound event: assume the running step completed.
          settle(runningKey);
          if (outputArtifactId && byKey.has(runningKey)) {
            byKey.get(runningKey)!.outputArtifactId = outputArtifactId;
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
          planStepKey: parsed.planStepKey || existing?.planStepKey || "",
          status: parsed.approved === false ? "rejected" : "approved",
          message: existing?.message ?? m,
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
  };
}

/**
 * Whether an execution is currently in flight — used by the header status pill.
 */
export function isExecutionActive(vm: ExecutionViewModel): boolean {
  return vm.state === "running";
}
