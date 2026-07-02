import { create } from "@bufbuild/protobuf";
import {
  ElicitationTimeoutBehavior,
  ExecutorKind,
  PlanBehaviorPoliciesSchema,
  PublishApprovalMode,
  SeedArtifactBindingSchema,
  SlotBindingSchema,
  type PlanBehaviorPolicies,
  type SeedArtifactBinding,
  type SlotBinding,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import {
  parseParameterValuesJson,
  type DateRangeValue,
  type LinkedInTemplateInputValues,
} from "./template-inputs";

export interface LinkedInMaterialization {
  seedArtifacts: SeedArtifactBinding[];
  slotBindings: SlotBinding[];
  behaviorPolicies: PlanBehaviorPolicies;
}

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

function slotBinding(
  stepKey: string,
  executorKind: ExecutorKind,
  installationId: string | undefined,
): SlotBinding | undefined {
  const executorInstallationId = installationId?.trim();
  if (!executorInstallationId) return undefined;
  return create(SlotBindingSchema, {
    stepKey,
    executorKind,
    executorInstallationId,
  });
}

export function materializeLinkedInInputValues(
  values: LinkedInTemplateInputValues,
  installationIdsByStep: Record<string, string>,
): LinkedInMaterialization {
  const fetchInstallationId =
    values.aggregateSourceGroupInstallationId.trim() ||
    values.sourceGroupInstallationIds.find((id) => id.trim())?.trim() ||
    installationIdsByStep["fetch-news"] ||
    "";
  const slotBindings = [
    slotBinding("fetch-news", ExecutorKind.INTEGRATION, fetchInstallationId),
    slotBinding(
      "write-draft",
      ExecutorKind.AGENT,
      installationIdsByStep["write-draft"],
    ),
    slotBinding(
      "adapt-for-linkedin",
      ExecutorKind.AGENT,
      installationIdsByStep["adapt-for-linkedin"],
    ),
    slotBinding(
      "publish-linkedin",
      ExecutorKind.INTEGRATION,
      installationIdsByStep["publish-linkedin"],
    ),
  ].filter((binding): binding is SlotBinding => binding !== undefined);

  return {
    seedArtifacts: [
      create(SeedArtifactBindingSchema, {
        stepKey: "fetch-news",
        inputName: "date_range",
        literalJson: JSON.stringify(values.dateRange),
      }),
      create(SeedArtifactBindingSchema, {
        stepKey: "write-draft",
        inputName: "harpia.internal.ContentPreferences",
        literalJson: JSON.stringify({
          topic: values.theme.trim() || "sports",
          language: values.language,
          tone: values.tone.trim() || "analytical, concise, and practical",
          audience: values.audience.trim(),
          topics_to_avoid: values.topicsToAvoid.trim(),
        }),
      }),
    ],
    slotBindings,
    behaviorPolicies: create(PlanBehaviorPoliciesSchema, {
      elicitationTimeoutBehavior:
        ElicitationTimeoutBehavior.PAUSE_UNTIL_ANSWERED,
      elicitationTimeoutHours: 0,
      publishApprovalMode:
        values.approvalMode === "auto_publish"
          ? PublishApprovalMode.AUTO_PUBLISH
          : PublishApprovalMode.REQUIRE_APPROVAL,
    }),
  };
}
