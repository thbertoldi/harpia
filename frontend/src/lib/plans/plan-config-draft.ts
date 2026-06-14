import { create } from "@bufbuild/protobuf";
import {
  ExecutorKind,
  ElicitationTimeoutBehavior,
  OverseerBindingSchema,
  PlanBehaviorPoliciesSchema,
  PublishApprovalMode,
  SlotBindingSchema,
  type OverseerBinding,
  type PlanBehaviorPolicies,
  type SlotBinding,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import { SKU_IDS, WEEKLY_NEWSLETTER_TEMPLATE } from "$lib/mocks/plan-catalog";

export interface PlanConfigDraft {
  templateId: string;
  slotBindings: SlotBinding[];
  overseerBindings: OverseerBinding[];
  behaviorPolicies: PlanBehaviorPolicies;
}

const draftStore = new Map<string, PlanConfigDraft>();

function mockWeeklyNewsletterDraft(): PlanConfigDraft {
  return {
    templateId: WEEKLY_NEWSLETTER_TEMPLATE.id,
    slotBindings: [
      create(SlotBindingSchema, {
        stepKey: "fetch-news",
        executorKind: ExecutorKind.INTEGRATION,
        executorSkuId: SKU_IDS.rssNewsFeed,
        executorInstallationId: "inst-rss",
      }),
      create(SlotBindingSchema, {
        stepKey: "write-draft",
        executorKind: ExecutorKind.AGENT,
        executorSkuId: SKU_IDS.newsletterWriter,
        executorInstallationId: "inst-writer",
      }),
      create(SlotBindingSchema, {
        stepKey: "adapt-for-linkedin",
        executorKind: ExecutorKind.AGENT,
        executorSkuId: SKU_IDS.linkedinVoice,
        executorInstallationId: "inst-writer",
      }),
      create(SlotBindingSchema, {
        stepKey: "publish-linkedin",
        executorKind: ExecutorKind.INTEGRATION,
        executorSkuId: SKU_IDS.linkedinPublish,
        executorInstallationId: "inst-linkedin",
      }),
    ],
    overseerBindings: [
      create(OverseerBindingSchema, {
        stepKey: "write-draft",
        overseerUserId: "dev-overseer",
      }),
      create(OverseerBindingSchema, {
        stepKey: "adapt-for-linkedin",
        overseerUserId: "dev-overseer",
      }),
    ],
    behaviorPolicies: create(PlanBehaviorPoliciesSchema, {
      elicitationTimeoutBehavior:
        ElicitationTimeoutBehavior.PAUSE_UNTIL_ANSWERED,
      elicitationTimeoutHours: 48,
      publishApprovalMode: PublishApprovalMode.REQUIRE_APPROVAL,
    }),
  };
}

function matchesWeeklyNewsletterTemplate(templateId: string): boolean {
  return (
    templateId === WEEKLY_NEWSLETTER_TEMPLATE.id ||
    templateId === WEEKLY_NEWSLETTER_TEMPLATE.key
  );
}

/** Returns a draft configuration for MVP configure/summary flows. */
export function getPlanConfigDraft(templateId: string): PlanConfigDraft {
  const cached = draftStore.get(templateId);
  if (cached) return cached;

  if (matchesWeeklyNewsletterTemplate(templateId)) {
    const draft = mockWeeklyNewsletterDraft();
    draftStore.set(templateId, draft);
    return draft;
  }

  const draft: PlanConfigDraft = {
    templateId,
    slotBindings: [],
    overseerBindings: [],
    behaviorPolicies: create(PlanBehaviorPoliciesSchema, {
      elicitationTimeoutBehavior: ElicitationTimeoutBehavior.UNSPECIFIED,
      elicitationTimeoutHours: 0,
      publishApprovalMode: PublishApprovalMode.UNSPECIFIED,
    }),
  };
  draftStore.set(templateId, draft);
  return draft;
}

/** Persists draft state for in-memory MVP flows (future configure wizard). */
export function setPlanConfigDraft(draft: PlanConfigDraft): void {
  draftStore.set(draft.templateId, draft);
}
