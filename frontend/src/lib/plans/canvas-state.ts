import type { ChatMessage } from "$lib/chat/types";
import type { PlanStep, SlotBinding } from "$lib/gen/harpia/plans/v1/plans_pb";

export type CanvasStepStatus =
  | "unbound"
  | "bound"
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

/** Most recently started step that is still running (linear DAG primary case). */
function mostRecentRunningStepKey(
  state: Record<string, CanvasStepState>,
): string | null {
  let bestKey: string | null = null;
  let bestStarted = "";
  for (const [key, step] of Object.entries(state)) {
    if (step.status !== "running" || !step.startedAt) continue;
    if (!bestKey || step.startedAt > bestStarted) {
      bestKey = key;
      bestStarted = step.startedAt;
    }
  }
  return bestKey;
}

function findStepByPendingElicitation(
  state: Record<string, CanvasStepState>,
  elicitationId: string,
): string | null {
  for (const [key, step] of Object.entries(state)) {
    if (step.pendingElicitationId === elicitationId) return key;
  }
  return null;
}

function findStepByPendingApproval(
  state: Record<string, CanvasStepState>,
  approvalId: string,
): string | null {
  for (const [key, step] of Object.entries(state)) {
    if (step.pendingApprovalId === approvalId) return key;
  }
  return null;
}

/**
 * Project a stream of M3 chat messages onto a canvas state map keyed by
 * step.key. Pure function — replays messages in sequence order.
 *
 * Pointer messages (ELICITATION_RAISED, APPROVAL_RAISED) attach to the
 * running step with the latest startedAt (M4 assumes mostly linear DAGs;
 * parallel fan-out may mis-route if multiple steps are running).
 *
 * RUN_FAILED marks the most recently running step as failed.
 */
export function buildCanvasState(
  messages: ChatMessage[],
  steps: PlanStep[],
  slotBindings: SlotBinding[] = [],
): Record<string, CanvasStepState> {
  // Seed config-time status from slot bindings. Once an execution event
  // (STEP_STARTED / STEP_BOUND / RUN_FAILED / ELICITATION_RAISED /
  // APPROVAL_RAISED) fires for a step, the projection below overrides
  // "bound" / "unbound" with the runtime status. Without a run, the
  // canvas shows the static binding state per spec §5.3.
  const boundByStep = new Set<string>();
  for (const sb of slotBindings) {
    if (sb.executorInstallationId) boundByStep.add(sb.stepKey);
  }
  const state: Record<string, CanvasStepState> = {};
  for (const step of steps) {
    state[step.key] = { status: boundByStep.has(step.key) ? "bound" : "unbound" };
  }

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
        const prior = state[stepKey];
        // Idempotent replay: do not wipe in-flight pointers on duplicate STEP_STARTED.
        if (
          prior.status === "awaiting_elicitation" ||
          prior.status === "awaiting_approval"
        ) {
          break;
        }
        state[stepKey] = {
          ...prior,
          status: "running",
          stepExecutionId: String(payload.step_execution_id ?? ""),
          startedAt: m.createdAt,
          pendingElicitationId: undefined,
          pendingApprovalId: undefined,
        };
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
        break;
      }
      case "ELICITATION_RAISED": {
        const targetKey = mostRecentRunningStepKey(state);
        if (!targetKey || !state[targetKey]) break;
        state[targetKey] = {
          ...state[targetKey],
          status: "awaiting_elicitation",
          pendingElicitationId: String(payload.elicitation_id ?? ""),
        };
        break;
      }
      case "ELICITATION_ANSWERED": {
        const elicitId = String(payload.elicitation_id ?? "");
        const targetKey =
          findStepByPendingElicitation(state, elicitId) ??
          mostRecentRunningStepKey(state);
        if (!targetKey || !state[targetKey]) break;
        if (state[targetKey].status === "awaiting_elicitation") {
          state[targetKey] = {
            ...state[targetKey],
            status: "running",
            pendingElicitationId: undefined,
          };
        }
        break;
      }
      case "APPROVAL_RAISED": {
        const targetKey = mostRecentRunningStepKey(state);
        if (!targetKey || !state[targetKey]) break;
        state[targetKey] = {
          ...state[targetKey],
          status: "awaiting_approval",
          pendingApprovalId: String(payload.approval_request_id ?? ""),
        };
        break;
      }
      case "APPROVAL_DECIDED": {
        const approvalId = String(payload.approval_request_id ?? "");
        const targetKey =
          findStepByPendingApproval(state, approvalId) ??
          mostRecentRunningStepKey(state);
        if (!targetKey || !state[targetKey]) break;
        if (state[targetKey].status === "awaiting_approval") {
          state[targetKey] = {
            ...state[targetKey],
            status: "running",
            pendingApprovalId: undefined,
          };
        }
        break;
      }
      case "RUN_FAILED": {
        const targetKey = mostRecentRunningStepKey(state);
        if (!targetKey || !state[targetKey]) break;
        state[targetKey] = {
          ...state[targetKey],
          status: "failed",
          errorMessage: String(payload.error ?? ""),
        };
        break;
      }
      default:
        break;
    }
  }

  return state;
}
