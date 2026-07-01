import type { TemplateInputParameter } from "$lib/gen/harpia/plans/v1/plans_pb";

export interface DateRangeValue {
  startDate: string;
  endDate: string;
}

// genericInputInitialValues builds a flat string map for a template's inputs.
// Priority per key: extracted value > default_value_json > "".
export function genericInputInitialValues(
  params: TemplateInputParameter[],
  extracted: Record<string, unknown>,
): Record<string, string> {
  const out: Record<string, string> = {};
  for (const p of params) {
    if (p.key in extracted && extracted[p.key] != null) {
      out[p.key] = String(extracted[p.key]);
      continue;
    }
    let fallback = "";
    if (p.defaultValueJson?.trim()) {
      try {
        const parsed = JSON.parse(p.defaultValueJson);
        fallback = parsed == null ? "" : String(parsed);
      } catch {
        fallback = "";
      }
    }
    out[p.key] = fallback;
  }
  return out;
}

// genericParameterValuesJson serializes the form's flat values to the
// parameter_values_json string CreatePlanConfiguration expects.
export function genericParameterValuesJson(
  values: Record<string, string>,
): string {
  return JSON.stringify(values);
}

// selectOptions parses a template parameter's options_json (a JSON string array)
// into option values; returns [] on any parse failure.
export function selectOptions(optionsJson: string): string[] {
  if (!optionsJson?.trim()) return [];
  try {
    const parsed = JSON.parse(optionsJson);
    return Array.isArray(parsed) ? parsed.map((v) => String(v)) : [];
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
