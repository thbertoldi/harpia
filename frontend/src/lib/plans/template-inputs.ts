import type { TemplateInputParameter } from "$lib/gen/harpia/plans/v1/plans_pb";
import { TemplateInputParameterType } from "$lib/gen/harpia/plans/v1/plans_pb";

export interface DateRangeValue {
  startDate: string;
  endDate: string;
}

// A date range can be pinned to explicit dates or left as a rolling preset that
// is resolved at execution time (e.g. a weekly newsletter that always covers the
// previous seven days). Keeping the preset unresolved through configuration is
// what makes a scheduled plan roll forward on every run instead of freezing a
// single week.
export interface DateRangePreset {
  preset: string;
}

export type DateRangeInput = DateRangeValue | DateRangePreset;

export function isDateRangePreset(value: unknown): value is DateRangePreset {
  return (
    !!value &&
    typeof value === "object" &&
    !Array.isArray(value) &&
    typeof (value as { preset?: unknown }).preset === "string" &&
    (value as { preset: string }).preset !== ""
  );
}

// genericInputInitialValues builds a value map for a template's inputs.
// Priority per key: extracted value > default_value_json > "" (or empty object for DATE_RANGE).
// Preset defaults (like a rolling date-range window) are preserved verbatim so
// they stay rolling; they are only turned into concrete dates on demand.
export function genericInputInitialValues(
  params: TemplateInputParameter[],
  extracted: Record<string, unknown>,
): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const p of params) {
    // Integration-selector values must be real installation UUIDs, chosen via
    // the picker. The proposal's suggested source-group *names* are display
    // hints, not ids — prefilling them as the parameter value produced an
    // invalid (non-UUID) slot binding at create time. Leave these empty so the
    // user picks an actual installation.
    if (p.type === TemplateInputParameterType.INTEGRATION_SELECTOR) {
      out[p.key] = "";
      continue;
    }
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

function isoDate(date: Date): string {
  return date.toISOString().slice(0, 10);
}

function addDays(date: Date, days: number): Date {
  const next = new Date(date);
  next.setUTCDate(next.getUTCDate() + days);
  return next;
}

// resolveDateRangePreset turns a named window into concrete dates relative to
// `now`. "last_7_days" is a rolling window ending yesterday (the last complete
// day), so a weekly newsletter always covers the previous week, never today's
// partial data. Used when a user opts into explicit dates, and mirrors the
// resolution the executor must perform at each scheduled run.
export function resolveDateRangePreset(
  preset: string,
  now: Date,
): DateRangeValue | null {
  if (preset === "last_7_days") {
    const end = addDays(now, -1);
    const start = addDays(now, -7);
    return { startDate: isoDate(start), endDate: isoDate(end) };
  }
  return null;
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
    if (isDateRangePreset(value)) return true;
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
  sourceGroupInstallationIds: string[];
  aggregateSourceGroupInstallationId: string;
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

export function parameterValuesJson(
  values: LinkedInTemplateInputValues,
): string {
  const sourceGroup =
    values.aggregateSourceGroupInstallationId.trim() ||
    values.sourceGroupInstallationIds.find((id) => id.trim())?.trim() ||
    "";
  return JSON.stringify({
    theme: values.theme,
    language: values.language,
    tone: values.tone,
    audience: values.audience,
    topics_to_avoid: values.topicsToAvoid,
    source_group: sourceGroup,
    source_groups: values.sourceGroupInstallationIds,
    aggregate_source_group: values.aggregateSourceGroupInstallationId,
    date_range: values.dateRange,
    approval_mode: values.approvalMode,
  });
}
