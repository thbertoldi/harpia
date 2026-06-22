import type { ChatMessage } from "$lib/chat/types";
import type { PlanStep } from "$lib/gen/harpia/plans/v1/plans_pb";

export type CanvasStepStatus =
  | "pending"
  | "running"
  | "awaiting_elicitation"
  | "awaiting_approval"
  | "done"
  | "failed";

export interface CanvasStepState {
  status: CanvasStepStatus;
  stepExecutionId?: string;
  pendingElicitationId?: string;
  pendingApprovalId?: string;
  startedAt?: string;
  completedAt?: string;
  outputArtifactId?: string;
  errorMessage?: string;
}

/**
 * Project a stream of M3 chat messages onto a canvas state map keyed by
 * step.key. Pure function — replays messages in sequence order.
 *
 * Pointer messages (ELICITATION_RAISED, APPROVAL_RAISED) attach to the step
 * that most recently STEP_STARTED in the same execution. Their corresponding
 * *_ANSWERED / *_DECIDED clear the pointer and revert status to "running"
 * until STEP_BOUND completes the step.
 *
 * RUN_FAILED at the run level marks the currently-running step as "failed"
 * and stores the error message from the payload.
 */
export function buildCanvasState(
  messages: ChatMessage[],
  steps: PlanStep[],
): Record<string, CanvasStepState> {
  const state: Record<string, CanvasStepState> = {};
  for (const step of steps) {
    state[step.key] = { status: "pending" };
  }

  // Track the "currently active" step key per execution so pointer
  // messages can attach. M4 assumes a single execution context per call.
  let activeStepKey: string | null = null;

  const sorted = [...messages].sort((a, b) =>
    a.sequenceNumber < b.sequenceNumber ? -1 : 1,
  );

  for (const m of sorted) {
    let payload: Record<string, unknown> = {};
    try {
      payload = JSON.parse(m.payloadJson) as Record<string, unknown>;
    } catch {
      payload = {};
    }

    switch (m.kind) {
      case "STEP_STARTED": {
        const stepKey = String(payload.step_key ?? "");
        if (!stepKey || !state[stepKey]) break;
        state[stepKey] = {
          ...state[stepKey],
          status: "running",
          stepExecutionId: String(payload.step_execution_id ?? ""),
          startedAt: m.createdAt,
          pendingElicitationId: undefined,
          pendingApprovalId: undefined,
        };
        activeStepKey = stepKey;
        break;
      }
      case "STEP_BOUND": {
        const stepKey = String(payload.step_key ?? "");
        if (!stepKey || !state[stepKey]) break;
        state[stepKey] = {
          ...state[stepKey],
          status: "done",
          completedAt: m.createdAt,
          outputArtifactId: String(payload.output_artifact_id ?? ""),
          pendingElicitationId: undefined,
          pendingApprovalId: undefined,
        };
        if (activeStepKey === stepKey) {
          activeStepKey = null;
        }
        break;
      }
      case "ELICITATION_RAISED": {
        if (!activeStepKey || !state[activeStepKey]) break;
        state[activeStepKey] = {
          ...state[activeStepKey],
          status: "awaiting_elicitation",
          pendingElicitationId: String(payload.elicitation_id ?? ""),
        };
        break;
      }
      case "ELICITATION_ANSWERED": {
        if (!activeStepKey || !state[activeStepKey]) break;
        if (state[activeStepKey].status === "awaiting_elicitation") {
          state[activeStepKey] = {
            ...state[activeStepKey],
            status: "running",
            pendingElicitationId: undefined,
          };
        }
        break;
      }
      case "APPROVAL_RAISED": {
        if (!activeStepKey || !state[activeStepKey]) break;
        state[activeStepKey] = {
          ...state[activeStepKey],
          status: "awaiting_approval",
          pendingApprovalId: String(payload.approval_request_id ?? ""),
        };
        break;
      }
      case "APPROVAL_DECIDED": {
        if (!activeStepKey || !state[activeStepKey]) break;
        if (state[activeStepKey].status === "awaiting_approval") {
          state[activeStepKey] = {
            ...state[activeStepKey],
            status: "running",
            pendingApprovalId: undefined,
          };
        }
        break;
      }
      case "RUN_FAILED": {
        if (!activeStepKey || !state[activeStepKey]) break;
        state[activeStepKey] = {
          ...state[activeStepKey],
          status: "failed",
          errorMessage: String(payload.error ?? ""),
        };
        activeStepKey = null;
        break;
      }
      // RUN_STARTED, RUN_COMPLETED, CONFIGURATION_SAVED, USER_TEXT,
      // ASSISTANT_TEXT — no step-level effect.
      default:
        break;
    }
  }

  return state;
}
