import type { ChatMessage } from "$lib/chat/types";

/**
 * Resolved i18n key + interpolation params for a system-event chat message.
 * When `eventTextKey` returns null, callers fall back to `message.text`
 * (the backend's English dev-fallback). Unknown kinds and kinds that carry
 * a dynamic, user-authored label (STEP_REBOUND) intentionally resolve null.
 */
export interface EventTextKey {
  key: string;
  params?: Record<string, string>;
}

/**
 * Maps a step_key to its human title (the template step's `title`). Defaulting
 * to the raw key keeps the resolver dependency-free and unit-testable; the
 * page wires in the real template lookup via SystemEventCard's optional prop.
 */
export type StepTitleResolver = (stepKey: string) => string;

interface StepKeyPayload {
  step_key?: unknown;
}

interface RunFailedPayload {
  error?: unknown;
}

function parseStepKey(payloadJson: string): string | null {
  if (!payloadJson) return null;
  try {
    const parsed = JSON.parse(payloadJson) as StepKeyPayload;
    const key = parsed?.step_key;
    return typeof key === "string" && key.length > 0 ? key : null;
  } catch {
    return null;
  }
}

function parseRunFailureReason(payloadJson: string): string | null {
  if (!payloadJson) return null;
  try {
    const parsed = JSON.parse(payloadJson) as RunFailedPayload;
    const error = parsed?.error;
    return typeof error === "string" && error.trim().length > 0 ? error : null;
  } catch {
    return null;
  }
}

/**
 * Resolve the canonical i18n key for a system-event message kind. Returns null
 * for kinds without a fixed template so the caller can fall back to the
 * backend's English `text` (dev safety). STEP_REBOUND carries a dynamic,
 * user-selection echo rather than a template, so it also resolves null.
 *
 * @param stepTitleFor translates a step_key to its template title; defaults to
 *   the identity function (raw key) when no resolver is supplied.
 */
export function eventTextKey(
  message: Pick<ChatMessage, "kind" | "payloadJson">,
  stepTitleFor: StepTitleResolver = (key) => key,
): EventTextKey | null {
  switch (message.kind) {
    case "PLAN_ATTACHED":
      return { key: "thread.event.plan_attached" };
    case "CONFIGURATION_STARTED":
      return { key: "thread.event.configuration_started" };
    case "CONFIGURATION_SAVED":
      return { key: "thread.event.configuration_saved" };
    case "SCHEDULE_SET":
      return { key: "thread.event.schedule_set" };
    case "RUN_STARTED":
      return { key: "thread.event.run_started" };
    case "RUN_COMPLETED":
      return { key: "thread.event.run_completed" };
    case "RUN_FAILED": {
      const reason = parseRunFailureReason(message.payloadJson);
      return reason
        ? { key: "thread.event.run_failed_reason", params: { reason } }
        : { key: "thread.event.run_failed" };
    }
    case "STEP_STARTED": {
      const stepKey = parseStepKey(message.payloadJson);
      return {
        key: "thread.event.step_started",
        params: { step: stepKey ? stepTitleFor(stepKey) : "" },
      };
    }
    case "STEP_BOUND": {
      const stepKey = parseStepKey(message.payloadJson);
      return {
        key: "thread.event.step_bound",
        params: { step: stepKey ? stepTitleFor(stepKey) : "" },
      };
    }
    default:
      return null;
  }
}
