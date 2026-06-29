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
import type { LinkedInTemplateInputValues } from "./template-inputs";

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
    sourceGroupInstallationId: "",
    dateRange: {
      startDate: isoDate(start),
      endDate: isoDate(end),
    },
    approvalMode: "require_approval",
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
    values.sourceGroupInstallationId.trim() ||
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
