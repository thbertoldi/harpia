import type { PlanStep } from "$lib/gen/harpia/plans/v1/plans_pb";
import { stepWillRun } from "$lib/plans/step-participation";

/**
 * Pure helpers for the binding-matrix card (M6 §2.1).
 *
 * The matrix card is driven by a single `ASSISTANT_PROMPT` whose
 * `payload_json` carries the whole step list. These helpers parse that
 * payload, total the per-run cost from the rows' own option pricing, and
 * decide whether the configuration is complete enough to be made runnable.
 *
 * The backend protocol is locked; the shapes below mirror it exactly. See
 * the spec at docs/superpowers/specs/2026-06-22-harpia-m6-lapidacao-design.md.
 */

export interface MatrixOption {
  id: string;
  label: string;
  sublabel?: string;
  value: string;
  /** Per-run price in BRL. May be absent (e.g. free integrations). */
  price_brl?: number;
}

export interface MatrixRow {
  step_key: string;
  step_title: string;
  contracts: {
    input: string;
    output: string;
  };
  options: MatrixOption[];
  /** Executor option id currently bound to this step, or "" when unbound. */
  current_executor_id: string;
  current_overseer_id: string;
  current_overseer_label: string;
}

export interface MatrixPayload {
  state: "BINDING_MATRIX";
  /** True once behavior policies have been set on the configuration. */
  policies_set: boolean;
  rows: MatrixRow[];
}

export interface BindingStepPayload {
  state: "BINDING_STEP";
  step_key: string;
  options: MatrixOption[];
  /** True once behavior policies have been set on the configuration. */
  policies_set: boolean;
  rows: MatrixRow[];
}

export interface OverseerStepPayload {
  state: "OVERSEER_STEP";
  step_key: string;
  options: MatrixOption[];
  required_step_keys: string[];
  rows: MatrixRow[];
}

export interface PolicyField {
  key: string;
  parameter_key: string;
  current_value: string;
  options: MatrixOption[];
}

export interface PoliciesStepPayload {
  state: "POLICIES_STEP";
  fields: PolicyField[];
  policies_set: boolean;
}

export interface MatrixHydrationInput {
  slotBindings: Array<{
    stepKey: string;
    executorInstallationId: string;
  }>;
  overseerBindings?: Array<{
    stepKey: string;
    overseerUserId: string;
  }>;
  policiesSet: boolean;
  /** Fresh template steps used to reconcile cached prompt rows. */
  steps?: readonly PlanStep[];
  /** Server-derived opt-in projection from the current configuration. */
  includedOptionalCapabilities?: readonly string[];
}

export interface MatrixParticipationInput {
  steps: readonly PlanStep[];
  includedOptionalCapabilities: readonly string[];
}

/**
 * Parse the matrix `ASSISTANT_PROMPT` payload.
 *
 * @throws if the JSON is malformed, not an object, or carries a state other
 *   than `BINDING_MATRIX`.
 */
export function parseMatrixPayload(json: string): MatrixPayload {
  const obj = parsePayloadObject(json, "parseMatrixPayload");
  if (obj.state !== "BINDING_MATRIX") {
    throw new Error(
      `parseMatrixPayload: expected state "BINDING_MATRIX", got ${JSON.stringify(obj.state)}`,
    );
  }
  if (!Array.isArray(obj.rows)) {
    throw new Error("parseMatrixPayload: rows is not an array");
  }
  const rows: MatrixRow[] = obj.rows.map((r, i) => normalizeRow(r, i));
  return {
    state: "BINDING_MATRIX",
    policies_set: obj.policies_set === true,
    rows,
  };
}

export function parseBindingStepPayload(json: string): BindingStepPayload {
  const obj = parsePayloadObject(json, "parseBindingStepPayload");
  if (obj.state !== "BINDING_STEP") {
    throw new Error(
      `parseBindingStepPayload: expected state "BINDING_STEP", got ${JSON.stringify(obj.state)}`,
    );
  }
  if (!Array.isArray(obj.rows)) {
    throw new Error("parseBindingStepPayload: rows is not an array");
  }
  const rawOptions = Array.isArray(obj.options) ? obj.options : [];
  return {
    state: "BINDING_STEP",
    step_key: String(obj.step_key ?? ""),
    options: rawOptions.map((o, i) => normalizeOption(o, -1, i)),
    policies_set: obj.policies_set === true,
    rows: obj.rows.map((r, i) => normalizeRow(r, i)),
  };
}

export function parseOverseerStepPayload(json: string): OverseerStepPayload {
  const obj = parsePayloadObject(json, "parseOverseerStepPayload");
  if (obj.state !== "OVERSEER_STEP") {
    throw new Error(
      `parseOverseerStepPayload: expected state "OVERSEER_STEP", got ${JSON.stringify(obj.state)}`,
    );
  }
  if (!Array.isArray(obj.rows)) {
    throw new Error("parseOverseerStepPayload: rows is not an array");
  }
  const rawOptions = Array.isArray(obj.options) ? obj.options : [];
  const rawRequired = Array.isArray(obj.required_step_keys)
    ? obj.required_step_keys
    : [];
  return {
    state: "OVERSEER_STEP",
    step_key: String(obj.step_key ?? ""),
    options: rawOptions.map((o, i) => normalizeOption(o, -1, i)),
    required_step_keys: rawRequired.map((key) => String(key)),
    rows: obj.rows.map((r, i) => normalizeRow(r, i)),
  };
}

export function parsePoliciesStepPayload(json: string): PoliciesStepPayload {
  const obj = parsePayloadObject(json, "parsePoliciesStepPayload");
  if (obj.state !== "POLICIES_STEP") {
    throw new Error(
      `parsePoliciesStepPayload: expected state "POLICIES_STEP", got ${JSON.stringify(obj.state)}`,
    );
  }
  if (!Array.isArray(obj.fields)) {
    throw new Error("parsePoliciesStepPayload: fields is not an array");
  }
  return {
    state: "POLICIES_STEP",
    fields: obj.fields.map((field, i) => normalizePolicyField(field, i)),
    policies_set: obj.policies_set === true,
  };
}

export interface PolicyCardVisibility {
  isLive: boolean;
  editingAnswered: boolean;
  submitted: boolean;
}

/**
 * Whether the policy card should render any interactive chips at all. Chips
 * appear while the prompt is live or while the user is revisiting an answered
 * card, and only until the card is submitted.
 */
export function policyChipsShown(v: PolicyCardVisibility): boolean {
  return (v.isLive || v.editingAnswered) && !v.submitted;
}

/**
 * Whether one field's chips are visible.
 *
 * On the live/forward flow an already-answered field hides its chips and
 * collapses to a caption. When the user reopens an answered card to edit,
 * chips must reappear even for fields that already hold a value — otherwise
 * the edit affordance is a dead end (nothing renders to re-select).
 */
export function policyFieldChipsShown(
  field: PolicyField,
  v: PolicyCardVisibility,
): boolean {
  return policyChipsShown(v) && (!field.current_value || v.editingAnswered);
}

/** True when an option matches the field's currently selected value. */
export function isPolicyOptionSelected(
  field: PolicyField,
  option: MatrixOption,
): boolean {
  return (
    option.value === field.current_value || option.id === field.current_value
  );
}

function parsePayloadObject(
  json: string,
  caller:
    | "parseMatrixPayload"
    | "parseBindingStepPayload"
    | "parseOverseerStepPayload"
    | "parsePoliciesStepPayload",
): Record<string, unknown> {
  let raw: unknown;
  try {
    raw = JSON.parse(json);
  } catch {
    throw new Error(`${caller}: payload is not valid JSON`);
  }
  if (typeof raw !== "object" || raw === null) {
    throw new Error(`${caller}: payload is not an object`);
  }
  return raw as Record<string, unknown>;
}

function normalizePolicyField(raw: unknown, index: number): PolicyField {
  if (typeof raw !== "object" || raw === null) {
    throw new Error(
      `parsePoliciesStepPayload: field ${index} is not an object`,
    );
  }
  const field = raw as Record<string, unknown>;
  const options = Array.isArray(field.options)
    ? field.options.map((option, i) => normalizeOption(option, index, i))
    : [];
  return {
    key: String(field.key ?? ""),
    parameter_key: String(field.parameter_key ?? ""),
    current_value: String(field.current_value ?? ""),
    options,
  };
}

function normalizeRow(raw: unknown, index: number): MatrixRow {
  if (typeof raw !== "object" || raw === null) {
    throw new Error(`parseMatrixPayload: row ${index} is not an object`);
  }
  const r = raw as Record<string, unknown>;
  const contracts =
    typeof r.contracts === "object" && r.contracts !== null
      ? (r.contracts as Record<string, unknown>)
      : {};
  const options = Array.isArray(r.options)
    ? r.options.map((o, j) => normalizeOption(o, index, j))
    : [];
  return {
    step_key: String(r.step_key ?? ""),
    step_title: String(r.step_title ?? ""),
    contracts: {
      input: String(contracts.input ?? ""),
      output: String(contracts.output ?? ""),
    },
    options,
    current_executor_id: String(r.current_executor_id ?? ""),
    current_overseer_id: String(r.current_overseer_id ?? ""),
    current_overseer_label: String(r.current_overseer_label ?? ""),
  };
}

function normalizeOption(
  raw: unknown,
  rowIndex: number,
  optIndex: number,
): MatrixOption {
  if (typeof raw !== "object" || raw === null) {
    throw new Error(
      `parseMatrixPayload: row ${rowIndex} option ${optIndex} is not an object`,
    );
  }
  const o = raw as Record<string, unknown>;
  const option: MatrixOption = {
    id: String(o.id ?? ""),
    label: String(o.label ?? ""),
    value: String(o.value ?? ""),
  };
  if (typeof o.sublabel === "string") option.sublabel = o.sublabel;
  if (typeof o.price_brl === "number" && Number.isFinite(o.price_brl)) {
    option.price_brl = o.price_brl;
  }
  return option;
}

export interface RunCostSummary {
  totalBrl: number;
  unboundCount: number;
}

/**
 * Sum the per-run cost across rows.
 *
 * For each row, the bound executor is the option whose id matches
 * `current_executor_id`; its `price_brl` (defaulting to 0) is added to the
 * total. Rows with no binding — or a binding that doesn't resolve to a known
 * option — add 0 and bump `unboundCount`.
 *
 * Note: `cost.ts` (M5) computes the equivalent total from the proto
 * configuration + executor catalog. The matrix payload already carries
 * per-option prices inline, so this avoids the extra catalog lookup at the
 * card layer.
 */
export function computeRunCostBRL(
  rows: MatrixRow[],
  participation?: MatrixParticipationInput,
): RunCostSummary {
  let totalBrl = 0;
  let unboundCount = 0;
  for (const row of matrixRowsThatWillRun(rows, participation)) {
    if (!row.current_executor_id) {
      unboundCount += 1;
      continue;
    }
    const bound = row.options.find((o) => o.id === row.current_executor_id);
    if (!bound) {
      unboundCount += 1;
      continue;
    }
    totalBrl += bound.price_brl ?? 0;
  }
  return { totalBrl, unboundCount };
}

/**
 * The configuration is complete (and the primary "Save & make runnable"
 * action may enable) when every row has an executor bound and the behavior
 * policies have been set.
 */
export function isMatrixComplete(
  payload: MatrixPayload,
  participation?: MatrixParticipationInput,
): boolean {
  if (!payload.policies_set) return false;
  const rows = matrixRowsThatWillRun(payload.rows, participation);
  if (rows.length === 0) return false;
  return rows.every((row) => row.current_executor_id !== "");
}

/**
 * Filters rows to the steps that participate in the current configuration.
 * Missing template metadata is deliberately conservative for older callers:
 * unknown rows remain visible until fresh template data is available.
 */
export function matrixRowsThatWillRun(
  rows: MatrixRow[],
  participation?: MatrixParticipationInput,
): MatrixRow[] {
  if (!participation) return rows;
  const stepsByKey = new Map(
    participation.steps.map((step) => [step.key, step]),
  );
  return rows.filter((row) => {
    const step = stepsByKey.get(row.step_key);
    return (
      step === undefined ||
      stepWillRun(step, participation.includedOptionalCapabilities)
    );
  });
}

export function hydrateMatrixPayload<
  T extends MatrixPayload | BindingStepPayload | OverseerStepPayload,
>(payload: T, input: MatrixHydrationInput): T {
  const bindings = new Map(
    input.slotBindings.map((binding) => [
      binding.stepKey,
      binding.executorInstallationId,
    ]),
  );
  const overseerBindings = new Map(
    (input.overseerBindings ?? []).map((binding) => [
      binding.stepKey,
      binding.overseerUserId,
    ]),
  );
  const participation =
    input.steps === undefined
      ? undefined
      : {
          steps: input.steps,
          includedOptionalCapabilities:
            input.includedOptionalCapabilities ?? [],
        };
  return {
    ...payload,
    policies_set: input.policiesSet,
    rows: matrixRowsThatWillRun(payload.rows, participation).map((row) => ({
      ...row,
      current_executor_id:
        bindings.get(row.step_key) ?? row.current_executor_id,
      current_overseer_id:
        overseerBindings.get(row.step_key) ?? row.current_overseer_id,
      current_overseer_label:
        overseerBindings.get(row.step_key) ?? row.current_overseer_label,
    })),
  } as T;
}
