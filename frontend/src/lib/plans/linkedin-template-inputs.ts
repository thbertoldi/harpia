import {
  parseParameterValuesJson,
  type DateRangeValue,
  type LinkedInTemplateInputValues,
} from "./template-inputs";

function isoDate(date: Date): string {
  return date.toISOString().slice(0, 10);
}

function addDays(date: Date, days: number): Date {
  const next = new Date(date);
  next.setUTCDate(next.getUTCDate() + days);
  return next;
}

export function buildDefaultLinkedInInputValues(
  today = new Date(),
): LinkedInTemplateInputValues {
  const end = today;
  const start = addDays(end, -6);
  return {
    theme: "sports",
    language: "pt-BR",
    tone: "analytical, concise, and practical",
    audience: "",
    topicsToAvoid: "",
    sourceGroupInstallationIds: [],
    aggregateSourceGroupInstallationId: "",
    dateRange: {
      startDate: isoDate(start),
      endDate: isoDate(end),
    },
    approvalMode: "require_approval",
  };
}

function isLanguage(value: unknown): value is LinkedInTemplateInputValues["language"] {
  return value === "pt-BR" || value === "en-US" || value === "es";
}

function isApprovalMode(
  value: unknown,
): value is LinkedInTemplateInputValues["approvalMode"] {
  return value === "require_approval" || value === "auto_publish";
}

function parseDateRange(value: unknown): DateRangeValue | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return null;
  }
  const candidate = value as Record<string, unknown>;
  return typeof candidate.startDate === "string" &&
    typeof candidate.endDate === "string"
    ? {
        startDate: candidate.startDate,
        endDate: candidate.endDate,
      }
    : null;
}

function parseStringList(value: unknown): string[] {
  if (Array.isArray(value)) {
    return value
      .map((entry) => (typeof entry === "string" ? entry.trim() : ""))
      .filter(Boolean);
  }
  if (typeof value === "string" && value.trim()) return [value.trim()];
  return [];
}

export function linkedInInputValuesFromParameterValuesJson(
  raw: string | undefined,
  today = new Date(),
): LinkedInTemplateInputValues {
  const defaults = buildDefaultLinkedInInputValues(today);
  const parsed = parseParameterValuesJson(raw);
  const dateRange = parseDateRange(parsed.date_range);
  const sourceGroupInstallationIds = parseStringList(
    parsed.source_groups ?? parsed.source_group,
  );

  return {
    ...defaults,
    theme: typeof parsed.theme === "string" ? parsed.theme : defaults.theme,
    language: isLanguage(parsed.language) ? parsed.language : defaults.language,
    tone: typeof parsed.tone === "string" ? parsed.tone : defaults.tone,
    audience:
      typeof parsed.audience === "string" ? parsed.audience : defaults.audience,
    topicsToAvoid:
      typeof parsed.topics_to_avoid === "string"
        ? parsed.topics_to_avoid
        : defaults.topicsToAvoid,
    sourceGroupInstallationIds,
    aggregateSourceGroupInstallationId:
      typeof parsed.aggregate_source_group === "string"
        ? parsed.aggregate_source_group
        : defaults.aggregateSourceGroupInstallationId,
    dateRange: dateRange ?? defaults.dateRange,
    approvalMode: isApprovalMode(parsed.approval_mode)
      ? parsed.approval_mode
      : defaults.approvalMode,
  };
}
