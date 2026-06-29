export interface DateRangeValue {
  startDate: string;
  endDate: string;
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
