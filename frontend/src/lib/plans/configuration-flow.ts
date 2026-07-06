import {
  ElicitationTimeoutBehavior,
  PublishApprovalMode,
  type PlanConfiguration,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import { parseParameterValuesJson } from "$lib/plans/template-inputs";
import {
  policyChipsShown,
  type BindingStepPayload,
  type MatrixRow,
  type OverseerStepPayload,
  type PoliciesStepPayload,
  type PolicyField,
} from "$lib/plans/matrix";

export type ConfigurationAssistantState =
  | "PLAN_PROPOSED"
  | "AWAITING_TEMPLATE"
  | "BINDING_STEP"
  | "OVERSEER_STEP"
  | "POLICIES_STEP"
  | "BINDING_MATRIX"
  | "RUNNABLE"
  | "landing";

export interface AssistantPromptOption {
  id: string;
  label: string;
  sublabel?: string;
  value: string;
  price_brl?: number;
}

export interface GenericAssistantPromptPayload {
  state: string;
  step_key: string;
  options: AssistantPromptOption[];
}

export interface PromptVisibility {
  isLive: boolean;
  editingAnswered: boolean;
  submitted: boolean;
}

export interface BindingStepView {
  rows: MatrixRow[];
  focusedRow: MatrixRow | null;
  boundCount: number;
  totalCount: number;
  showChips: boolean;
}

export interface OverseerStepView {
  rows: MatrixRow[];
  focusedRow: MatrixRow | null;
  requiredRows: MatrixRow[];
  overseerCount: number;
  totalRequired: number;
  showChips: boolean;
}

export interface PoliciesStepView {
  fields: PolicyField[];
  selectedCount: number;
  totalCount: number;
  complete: boolean;
  showChips: boolean;
}

type ParameterValues = Record<string, unknown>;

export function parseAssistantPromptState(
  payloadJson: string,
): ConfigurationAssistantState | string | null {
  try {
    const raw = JSON.parse(payloadJson) as { state?: unknown };
    return typeof raw.state === "string" ? raw.state : null;
  } catch {
    return null;
  }
}

export function parseGenericAssistantPromptPayload(
  payloadJson: string,
): GenericAssistantPromptPayload {
  try {
    const raw = JSON.parse(payloadJson) as Record<string, unknown>;
    return {
      state: String(raw.state ?? ""),
      step_key: String(raw.step_key ?? ""),
      options: Array.isArray(raw.options)
        ? raw.options.map(normalizeAssistantPromptOption)
        : [],
    };
  } catch {
    return { state: "", step_key: "", options: [] };
  }
}

export function promptChipsShown(visibility: PromptVisibility): boolean {
  return (
    (visibility.isLive || visibility.editingAnswered) && !visibility.submitted
  );
}

export function buildBindingStepView(
  payload: BindingStepPayload,
  visibility: PromptVisibility,
): BindingStepView {
  const focusedRow = focusedMatrixRow(payload.rows, payload.step_key);
  return {
    rows: payload.rows,
    focusedRow,
    boundCount: countBoundExecutors(payload.rows),
    totalCount: payload.rows.length,
    showChips: promptChipsShown(visibility) && focusedRow !== null,
  };
}

export function buildOverseerStepView(
  payload: OverseerStepPayload,
  visibility: PromptVisibility,
): OverseerStepView {
  const requiredRows = payload.rows.filter((row) =>
    payload.required_step_keys.includes(row.step_key),
  );
  const focusedRow = focusedMatrixRow(payload.rows, payload.step_key);
  return {
    rows: payload.rows,
    focusedRow,
    requiredRows,
    overseerCount: requiredRows.filter((row) => row.current_overseer_id !== "")
      .length,
    totalRequired: requiredRows.length,
    showChips: promptChipsShown(visibility) && focusedRow !== null,
  };
}

export function buildPoliciesStepView(
  payload: PoliciesStepPayload,
  visibility: PromptVisibility,
): PoliciesStepView {
  const selectedCount = payload.fields.filter(
    (field) => field.current_value !== "",
  ).length;
  return {
    fields: payload.fields,
    selectedCount,
    totalCount: payload.fields.length,
    complete:
      payload.fields.length > 0 &&
      payload.fields.every((field) => field.current_value !== ""),
    showChips: payload.fields.length > 0 && policyChipsShown(visibility),
  };
}

export function policiesComplete(config: PlanConfiguration): boolean {
  const policies = config.behaviorPolicies;
  return (
    policies?.publishApprovalMode !== undefined &&
    policies.publishApprovalMode !== PublishApprovalMode.UNSPECIFIED &&
    policies?.elicitationTimeoutBehavior !== undefined &&
    policies.elicitationTimeoutBehavior !==
      ElicitationTimeoutBehavior.UNSPECIFIED
  );
}

export function reflectBindingPayload(
  payload: BindingStepPayload,
  stepKey: string,
  installationId: string,
): BindingStepPayload {
  return {
    ...payload,
    rows: payload.rows.map((row) =>
      row.step_key === stepKey
        ? { ...row, current_executor_id: installationId }
        : row,
    ),
  };
}

export function reflectOverseerPayload(
  payload: OverseerStepPayload,
  stepKey: string,
  overseerUserId: string,
  label: string,
): OverseerStepPayload {
  return {
    ...payload,
    rows: payload.rows.map((row) =>
      row.step_key === stepKey
        ? {
            ...row,
            current_overseer_id: overseerUserId,
            current_overseer_label: label || overseerUserId,
          }
        : row,
    ),
  };
}

export function reflectPolicyPayload(
  payload: PoliciesStepPayload,
  fieldKey: string,
  value: string,
): PoliciesStepPayload {
  return {
    ...payload,
    fields: payload.fields.map((field) =>
      field.key === fieldKey ? { ...field, current_value: value } : field,
    ),
  };
}

export function hydratePoliciesPayloadFromConfiguration(
  payload: PoliciesStepPayload,
  config: PlanConfiguration,
): PoliciesStepPayload {
  const values = parseParameterValuesJson(config.parameterValuesJson);
  return hydratePoliciesPayloadFromValues(
    payload,
    values,
    policiesComplete(config),
  );
}

export function hydratePoliciesPayloadFromValues(
  payload: PoliciesStepPayload,
  values: ParameterValues,
  policiesSet: boolean,
): PoliciesStepPayload {
  return {
    ...payload,
    policies_set: policiesSet,
    fields: payload.fields.map((field) => ({
      ...field,
      current_value: String(
        values[field.parameter_key] ?? field.current_value ?? "",
      ),
    })),
  };
}

function focusedMatrixRow(
  rows: MatrixRow[],
  stepKey: string,
): MatrixRow | null {
  return rows.find((row) => row.step_key === stepKey) ?? null;
}

function countBoundExecutors(rows: MatrixRow[]): number {
  return rows.filter((row) => row.current_executor_id !== "").length;
}

function normalizeAssistantPromptOption(raw: unknown): AssistantPromptOption {
  if (typeof raw !== "object" || raw === null) {
    return { id: "", label: "", value: "" };
  }
  const option = raw as Record<string, unknown>;
  const normalized: AssistantPromptOption = {
    id: String(option.id ?? ""),
    label: String(option.label ?? ""),
    value: String(option.value ?? ""),
  };
  if (typeof option.sublabel === "string")
    normalized.sublabel = option.sublabel;
  if (
    typeof option.price_brl === "number" &&
    Number.isFinite(option.price_brl)
  ) {
    normalized.price_brl = option.price_brl;
  }
  return normalized;
}
