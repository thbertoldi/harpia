import type { TemplateInputParameter } from "$lib/gen/harpia/plans/v1/plans_pb";
import { TemplateInputParameterType } from "$lib/gen/harpia/plans/v1/plans_pb";

export interface DateRangeValue {
  startDate: string;
  endDate: string;
}

// genericInputInitialValues builds a value map for a template's inputs.
// Priority per key: extracted value > default_value_json > "" (or empty object for DATE_RANGE).
export function genericInputInitialValues(
  params: TemplateInputParameter[],
  extracted: Record<string, unknown>,
): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const p of params) {
    if (p.key in extracted && extracted[p.key] != null) {
      out[p.key] = extracted[p.key];
      continue;
    }
    let fallback: unknown = "";
    if (p.defaultValueJson?.trim()) {
      try {
        const parsed = JSON.parse(p.defaultValueJson);
        fallback = parsed ?? "";
      } catch {
        fallback = "";
      }
    }
    out[p.key] = fallback;
  }
  return out;
}

// genericParameterValuesJson serializes the form's values to the
// parameter_values_json string CreatePlanConfiguration expects.
export function genericParameterValuesJson(
  values: Record<string, unknown>,
): string {
  return JSON.stringify(values);
}

function isNonEmptyRequiredValue(
  value: unknown,
  type: TemplateInputParameterType,
): boolean {
  if (value == null) return false;
  if (typeof value === "string") return value.trim() !== "";
  if (type === TemplateInputParameterType.DATE_RANGE) {
    const range = value as DateRangeValue;
    return Boolean(range.startDate?.trim() && range.endDate?.trim());
  }
  if (typeof value === "object" && !Array.isArray(value)) {
    return Object.keys(value as object).length > 0;
  }
  return true;
}

/** True when every required parameter has a non-empty value. */
export function requiredInputsSatisfied(
  params: TemplateInputParameter[],
  values: Record<string, unknown>,
): boolean {
  for (const param of params) {
    if (!param.required) continue;
    if (!isNonEmptyRequiredValue(values[param.key], param.type)) {
      return false;
    }
  }
  return true;
}

export interface SelectOption {
  value: string;
  label: string;
}

// selectOptions parses a template parameter's options_json into {value,label}
// pairs. Supports both shapes seen in templates: a plain string array
// (["a","b"]) and an array of objects ([{"value":"a","label":"A"}]). Returns []
// on any parse failure. Always returns distinct-valued options so keyed {#each}
// blocks never collide.
export function selectOptions(optionsJson: string): SelectOption[] {
  if (!optionsJson?.trim()) return [];
  try {
    const parsed = JSON.parse(optionsJson);
    if (!Array.isArray(parsed)) return [];
    const seen = new Set<string>();
    const out: SelectOption[] = [];
    for (const entry of parsed) {
      let value: string;
      let label: string;
      if (entry && typeof entry === "object") {
        value = String((entry as { value?: unknown }).value ?? "");
        label = String((entry as { label?: unknown }).label ?? value);
      } else {
        value = String(entry);
        label = value;
      }
      if (value === "" || seen.has(value)) continue;
      seen.add(value);
      out.push({ value, label });
    }
    return out;
  } catch {
    return [];
  }
}

export interface LinkedInTemplateInputValues {
  theme: string;
  language: "pt-BR" | "en-US" | "es";
  tone: string;
  audience: string;
  topicsToAvoid: string;
  sourceGroupInstallationId: string;
  dateRange: DateRangeValue;
  approvalMode: "require_approval" | "auto_publish";
}

export function parseParameterValuesJson(
  raw: string | undefined,
): Record<string, unknown> {
  if (!raw?.trim()) return {};
  try {
    const parsed = JSON.parse(raw);
    return parsed && typeof parsed === "object" && !Array.isArray(parsed)
      ? (parsed as Record<string, unknown>)
      : {};
  } catch {
    return {};
  }
}

export function parameterValuesJson(values: LinkedInTemplateInputValues): string {
  return JSON.stringify({
    theme: values.theme,
    language: values.language,
    tone: values.tone,
    audience: values.audience,
    topics_to_avoid: values.topicsToAvoid,
    source_group: values.sourceGroupInstallationId,
    date_range: values.dateRange,
    approval_mode: values.approvalMode,
  });
}
