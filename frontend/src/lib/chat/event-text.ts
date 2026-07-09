import type { ChatMessage } from "$lib/chat/types";

/**
 * Resolved i18n key + interpolation params for a system-event chat message.
 * When `eventTextKey` returns null, callers fall back to `message.text`
 * (the backend's English dev-fallback). Unknown kinds and STEP_REBOUND
 * messages that carry a dynamic, user-authored label intentionally resolve
 * null.
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
export type TranslationKeyResolver = (key: string) => string;

interface StepKeyPayload {
  step_key?: unknown;
}

interface RunFailedPayload {
  error?: unknown;
}

interface StepReboundPayload {
  policy_key?: unknown;
  new_policy_value?: unknown;
}

const policyOptionKeys: Record<string, Record<string, string>> = {
  publish_approval_mode: {
    require_approval:
      "assistant.policiesStep.option.publish_approval_mode.require_approval",
    auto_publish:
      "assistant.policiesStep.option.publish_approval_mode.auto_publish",
  },
  elicitation_timeout_behavior: {
    pause_until_answered:
      "assistant.policiesStep.option.elicitation_timeout_behavior.pause_until_answered",
    fail_step:
      "assistant.policiesStep.option.elicitation_timeout_behavior.fail_step",
    fail_plan:
      "assistant.policiesStep.option.elicitation_timeout_behavior.fail_plan",
  },
};

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

function parsePolicyRebound(
  payloadJson: string,
): { policyKey: string; value: string } | null {
  if (!payloadJson) return null;
  try {
    const parsed = JSON.parse(payloadJson) as StepReboundPayload;
    const policyKey = parsed?.policy_key;
    const value = parsed?.new_policy_value;
    if (
      typeof policyKey !== "string" ||
      policyKey.length === 0 ||
      typeof value !== "string" ||
      value.length === 0
    ) {
      return null;
    }
    return { policyKey, value };
  } catch {
    return null;
  }
}

function policyOptionKey(policyKey: string, value: string): string | null {
  return policyOptionKeys[policyKey]?.[value] ?? null;
}

/**
 * Resolve the canonical i18n key for a system-event message kind. Returns null
 * for kinds without a fixed template so the caller can fall back to the
 * backend's English `text` (dev safety). STEP_REBOUND carries a dynamic,
 * user-selection echo rather than a template, except for behavior-policy
 * rebounds whose option labels are canonical frontend i18n keys.
 *
 * @param stepTitleFor translates a step_key to its template title; defaults to
 *   the identity function (raw key) when no resolver is supplied.
 */
export function eventTextKey(
  message: Pick<ChatMessage, "kind" | "payloadJson">,
  stepTitleFor: StepTitleResolver = (key) => key,
  translateKey: TranslationKeyResolver = (key) => key,
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
    case "STEP_REBOUND": {
      const rebound = parsePolicyRebound(message.payloadJson);
      if (!rebound) return null;
      const optionKey = policyOptionKey(rebound.policyKey, rebound.value);
      return {
        key: "thread.event.policy_rebound",
        params: { value: optionKey ? translateKey(optionKey) : rebound.value },
      };
    }
    default:
      return null;
  }
}
