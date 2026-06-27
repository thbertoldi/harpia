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

export interface LinkedInSuggestionInput {
  topic: string;
  installationIdsByStep: Record<string, string>;
  today: Date;
}

export interface LinkedInSuggestion {
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

function slotBinding(
  stepKey: string,
  executorKind: ExecutorKind,
  installationIdsByStep: Record<string, string>,
): SlotBinding | undefined {
  const executorInstallationId = installationIdsByStep[stepKey]?.trim();
  if (!executorInstallationId) return undefined;
  return create(SlotBindingSchema, {
    stepKey,
    executorKind,
    executorInstallationId,
  });
}

export function buildLinkedInSuggestion(
  input: LinkedInSuggestionInput,
): LinkedInSuggestion {
  const topic = input.topic.trim() || "sports";
  const end = input.today;
  const start = addDays(end, -6);
  const slotBindings = [
    slotBinding("fetch-news", ExecutorKind.INTEGRATION, input.installationIdsByStep),
    slotBinding("write-draft", ExecutorKind.AGENT, input.installationIdsByStep),
    slotBinding(
      "adapt-for-linkedin",
      ExecutorKind.AGENT,
      input.installationIdsByStep,
    ),
    slotBinding(
      "publish-linkedin",
      ExecutorKind.INTEGRATION,
      input.installationIdsByStep,
    ),
  ].filter((binding): binding is SlotBinding => binding !== undefined);

  return {
    seedArtifacts: [
      create(SeedArtifactBindingSchema, {
        stepKey: "fetch-news",
        inputName: "date_range",
        literalJson: JSON.stringify({
          startDate: isoDate(start),
          endDate: isoDate(end),
        }),
      }),
      create(SeedArtifactBindingSchema, {
        stepKey: "write-draft",
        inputName: "harpia.internal.ContentPreferences",
        literalJson: JSON.stringify({
          topic,
          tone: "analytical, concise, and practical",
          topics_to_avoid: "",
        }),
      }),
    ],
    slotBindings,
    behaviorPolicies: create(PlanBehaviorPoliciesSchema, {
      elicitationTimeoutBehavior:
        ElicitationTimeoutBehavior.PAUSE_UNTIL_ANSWERED,
      elicitationTimeoutHours: 0,
      publishApprovalMode: PublishApprovalMode.REQUIRE_APPROVAL,
    }),
  };
}
