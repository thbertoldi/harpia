import { planClient } from "$lib/rpc";
import {
  ElicitationStatus,
  ElicitationTimeoutBehavior,
  ThreadMessageRole,
  type ElicitationRequest,
} from "$lib/gen/harpia/plans/v1/plans_pb";

export { ElicitationStatus, ElicitationTimeoutBehavior, ThreadMessageRole };
export type { ElicitationRequest };

/** A minimal form field derived from an elicitation response JSON Schema. */
export type ElicitationFormField = {
  name: string;
  label: string;
  type: "string" | "number" | "textarea" | "select";
  options?: string[];
  required: boolean;
};

export type ElicitationResponseInput = {
  payloadJson?: string;
  responseText?: string;
};

export type WatchElicitationsOptions = {
  stepExecutionId?: string;
  addressedToMe?: boolean;
};

export function isTerminalStatus(status: ElicitationStatus): boolean {
  return (
    status === ElicitationStatus.ANSWERED ||
    status === ElicitationStatus.TIMED_OUT ||
    status === ElicitationStatus.CANCELLED
  );
}

/** The response form is only editable while the elicitation is pending. */
export function isFormDisabled(status: ElicitationStatus): boolean {
  return status !== ElicitationStatus.PENDING;
}

export function statusLabelKey(status: ElicitationStatus): string {
  switch (status) {
    case ElicitationStatus.PENDING:
      return "elicitations.status.pending";
    case ElicitationStatus.ANSWERED:
      return "elicitations.status.answered";
    case ElicitationStatus.TIMED_OUT:
      return "elicitations.status.timedOut";
    case ElicitationStatus.CANCELLED:
      return "elicitations.status.cancelled";
    default:
      return "elicitations.status.unknown";
  }
}

export function timeoutPolicyKey(behavior: ElicitationTimeoutBehavior): string {
  switch (behavior) {
    case ElicitationTimeoutBehavior.PAUSE_UNTIL_ANSWERED:
      return "elicitations.timeout.policyPauseUntilAnswered";
    case ElicitationTimeoutBehavior.FAIL_STEP:
      return "elicitations.timeout.policyFailStep";
    case ElicitationTimeoutBehavior.FAIL_PLAN:
      return "elicitations.timeout.policyFailPlan";
    default:
      return "elicitations.timeout.policyPauseUntilAnswered";
  }
}

export function roleLabelKey(role: ThreadMessageRole): string {
  switch (role) {
    case ThreadMessageRole.AGENT:
      return "elicitations.thread.agent";
    case ThreadMessageRole.OVERSEER:
      return "elicitations.thread.overseer";
    default:
      return "elicitations.thread.system";
  }
}

export async function loadInboxElicitations(
  tenantId: string,
): Promise<ElicitationRequest[]> {
  const response = await planClient.listElicitations({
    tenantId,
    addressedToMe: true,
    status: ElicitationStatus.PENDING,
    pageSize: 100,
    pageToken: "",
  });
  return response.elicitations;
}

export async function countPendingElicitations(
  tenantId: string,
): Promise<number> {
  const elicitations = await loadInboxElicitations(tenantId);
  return elicitations.length;
}

export async function loadElicitation(
  tenantId: string,
  elicitationId: string,
): Promise<ElicitationRequest> {
  const response = await planClient.getElicitation({ tenantId, elicitationId });
  if (!response.elicitation) {
    throw new Error("Elicitation not found");
  }
  return response.elicitation;
}

export async function respondToElicitation(
  tenantId: string,
  elicitationId: string,
  input: ElicitationResponseInput,
): Promise<ElicitationRequest> {
  const response = await planClient.respondToElicitation({
    tenantId,
    elicitationId,
    payloadJson: input.payloadJson ?? "",
    responseText: input.responseText ?? "",
  });
  if (!response.elicitation) {
    throw new Error("Missing elicitation in response");
  }
  return response.elicitation;
}

export async function* watchElicitations(
  tenantId: string,
  options: WatchElicitationsOptions = {},
): AsyncIterable<ElicitationRequest[]> {
  for await (const event of planClient.watchElicitations({
    tenantId,
    addressedToMe: options.addressedToMe ?? false,
    stepExecutionId: options.stepExecutionId,
  })) {
    yield event.elicitations;
  }
}

type JsonSchema = {
  properties?: Record<string, Record<string, unknown>>;
  required?: string[];
};

/**
 * Parses a JSON Schema into a minimal set of renderable form fields. Only basic
 * types are supported in this slice (string/number/select/textarea); anything
 * else falls back to a raw JSON textarea handled by the caller.
 */
export function parseSchemaFields(schemaJson: string): ElicitationFormField[] {
  const trimmed = (schemaJson ?? "").trim();
  if (trimmed === "" || trimmed === "{}" || trimmed === "null") {
    return [];
  }
  let parsed: JsonSchema;
  try {
    parsed = JSON.parse(trimmed) as JsonSchema;
  } catch {
    return [];
  }
  const properties = parsed.properties;
  if (!properties || typeof properties !== "object") {
    return [];
  }
  const required = Array.isArray(parsed.required) ? parsed.required : [];
  const fields: ElicitationFormField[] = [];
  for (const [name, definition] of Object.entries(properties)) {
    const def = (definition ?? {}) as Record<string, unknown>;
    fields.push({
      name,
      label: typeof def.title === "string" ? def.title : name,
      type: fieldType(def),
      options: Array.isArray(def.enum) ? def.enum.map(String) : undefined,
      required: required.includes(name),
    });
  }
  return fields;
}

function fieldType(def: Record<string, unknown>): ElicitationFormField["type"] {
  if (Array.isArray(def.enum)) {
    return "select";
  }
  if (def.type === "number" || def.type === "integer") {
    return "number";
  }
  const maxLength = typeof def.maxLength === "number" ? def.maxLength : 0;
  if (def.format === "textarea" || maxLength > 200) {
    return "textarea";
  }
  return "string";
}

export function buildPayloadJson(values: Record<string, string>): string {
  const filtered: Record<string, string> = {};
  for (const [key, value] of Object.entries(values)) {
    if (value !== "") {
      filtered[key] = value;
    }
  }
  return JSON.stringify(filtered);
}

export function timeRemainingMs(
  expiresAt: string,
  now: number = Date.now(),
): number | null {
  if (!expiresAt) {
    return null;
  }
  const parsed = Date.parse(expiresAt);
  if (Number.isNaN(parsed)) {
    return null;
  }
  return parsed - now;
}

export function isExpired(
  expiresAt: string,
  now: number = Date.now(),
): boolean {
  const remaining = timeRemainingMs(expiresAt, now);
  return remaining !== null && remaining <= 0;
}

/** Human-friendly countdown like "2h 5m" / "8m" / "<1m"; "" when no deadline. */
export function formatCountdown(
  expiresAt: string,
  now: number = Date.now(),
): string {
  const remaining = timeRemainingMs(expiresAt, now);
  if (remaining === null) {
    return "";
  }
  if (remaining <= 0) {
    return "0m";
  }
  const totalMinutes = Math.floor(remaining / 60000);
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;
  if (hours > 0) {
    return `${hours}h ${minutes}m`;
  }
  if (totalMinutes === 0) {
    return "<1m";
  }
  return `${minutes}m`;
}
