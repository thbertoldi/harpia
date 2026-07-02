import { create } from "@bufbuild/protobuf";
import { describe, expect, it } from "vitest";
import {
  ExecutorKind as CatalogExecutorKind,
  ExecutorSKUSchema,
  ListPriceSchema,
} from "$lib/gen/harpia/executors/v1/executors_pb";
import {
  ExecutorKind as PlanExecutorKind,
  ElicitationTimeoutBehavior,
  OverseerBindingSchema,
  PlanBehaviorPoliciesSchema,
  PlanConfigurationSchema,
  PlanConfigurationStatus,
  PublishApprovalMode,
  SlotBindingSchema,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import {
  buildConfigCostSummary,
  buildConfigLineItems,
  buildPlanSummary,
  formatMoney,
  resolveListPrice,
  sumLineItemCosts,
} from "$lib/plans/config-summary";
import { planConfigDraftFromConfiguration } from "$lib/plans/plan-config-draft";
import {
  mockExecutorContext,
  SKU_IDS,
  WEEKLY_NEWSLETTER_TEMPLATE,
} from "$lib/mocks/plan-catalog";

describe("config summary", () => {
  const draft = planConfigDraftFromConfiguration(
    create(PlanConfigurationSchema, {
      id: "config-weekly-newsletter",
      tenantId: "dev",
      planTemplateId: WEEKLY_NEWSLETTER_TEMPLATE.id,
      planTemplateVersion: WEEKLY_NEWSLETTER_TEMPLATE.version,
      slotBindings: [
        create(SlotBindingSchema, {
          stepKey: "fetch-news",
          executorKind: PlanExecutorKind.INTEGRATION,
          executorSkuId: SKU_IDS.rssNewsFeed,
          executorInstallationId: "inst-rss",
        }),
        create(SlotBindingSchema, {
          stepKey: "write-draft",
          executorKind: PlanExecutorKind.AGENT,
          executorSkuId: SKU_IDS.newsletterWriter,
          executorInstallationId: "inst-writer",
        }),
        create(SlotBindingSchema, {
          stepKey: "adapt-for-linkedin",
          executorKind: PlanExecutorKind.AGENT,
          executorSkuId: SKU_IDS.linkedinVoice,
          executorInstallationId: "inst-writer",
        }),
        create(SlotBindingSchema, {
          stepKey: "publish-linkedin",
          executorKind: PlanExecutorKind.INTEGRATION,
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
    }),
  );
  const skus = mockExecutorContext().skus;

  it("resolves list price from SKU metadata with mock fallback", () => {
    const pricedSku = create(ExecutorSKUSchema, {
      id: "sku-priced",
      key: "rss-news-feed",
      displayName: "RSS News Feed",
      description: "",
      kind: CatalogExecutorKind.INTEGRATION,
      listPrice: create(ListPriceSchema, {
        priceCents: 500n,
        currency: "USD",
      }),
      createdAt: "2026-06-01T10:00:00Z",
      updatedAt: "2026-06-01T10:00:00Z",
    });

    expect(resolveListPrice(pricedSku, "rss-news-feed")).toEqual({
      priceCents: 500,
      currency: "USD",
    });
    expect(resolveListPrice(undefined, "linkedin-publish")).toEqual({
      priceCents: 1000,
      currency: "USD",
    });
    expect(resolveListPrice(undefined, "unknown-sku")).toEqual({
      priceCents: 0,
      currency: "USD",
    });
  });

  it("builds step, executor, overseer, and policy line items", () => {
    const items = buildConfigLineItems(WEEKLY_NEWSLETTER_TEMPLATE, draft, skus);

    expect(items.some((item) => item.kind === "step")).toBe(true);
    expect(items.some((item) => item.kind === "executor")).toBe(true);
    expect(items.some((item) => item.kind === "overseer")).toBe(true);
    expect(items.some((item) => item.kind === "policy")).toBe(true);

    const executorItems = items.filter((item) => item.kind === "executor");
    expect(executorItems).toHaveLength(WEEKLY_NEWSLETTER_TEMPLATE.steps.length);
    expect(executorItems.map((item) => item.priceCents)).toEqual([
      500, 200, 150, 1000,
    ]);
  });

  it("sums line item costs into total estimated cost per run", () => {
    const summary = buildConfigCostSummary(
      WEEKLY_NEWSLETTER_TEMPLATE,
      draft,
      skus,
    );

    expect(summary.totalCents).toBe(1850);
    expect(summary.currency).toBe("USD");
    expect(sumLineItemCosts(summary.lineItems).totalCents).toBe(1850);
  });

  it("includes policy lines when behavior policies are configured", () => {
    const policyDraft = {
      ...draft,
      behaviorPolicies: create(PlanBehaviorPoliciesSchema, {
        elicitationTimeoutBehavior:
          ElicitationTimeoutBehavior.PAUSE_UNTIL_ANSWERED,
        elicitationTimeoutHours: 24,
        publishApprovalMode: PublishApprovalMode.REQUIRE_APPROVAL,
      }),
    };

    const policyItems = buildConfigLineItems(
      WEEKLY_NEWSLETTER_TEMPLATE,
      policyDraft,
      skus,
    ).filter((item) => item.kind === "policy");

    expect(policyItems).toHaveLength(2);
    expect(policyItems.map((item) => item.key)).toEqual([
      "policy:publish-approval",
      "policy:elicitation-pause",
    ]);
  });

  it("formats money for display", () => {
    expect(formatMoney(1850, "USD", "en")).toBe("$18.50");
  });

  it("builds shared plan summary vocabulary", () => {
    const configuration = create(PlanConfigurationSchema, {
      id: "config-weekly-newsletter",
      planTemplateId: WEEKLY_NEWSLETTER_TEMPLATE.id,
      status: PlanConfigurationStatus.RUNNABLE,
      parameterValuesJson:
        '{"theme":"AI","audience":"founders","source_groups":["rss-tech","rss-business"]}',
      slotBindings: [
        create(SlotBindingSchema, {
          stepKey: "fetch-news",
          executorInstallationId: "rss-aggregate",
        }),
      ],
      overseerBindings: [
        create(OverseerBindingSchema, {
          stepKey: "write-draft",
          overseerUserId: "overseer-1",
        }),
      ],
    });

    expect(buildPlanSummary(WEEKLY_NEWSLETTER_TEMPLATE, configuration)).toEqual({
      id: "config-weekly-newsletter",
      templateName: WEEKLY_NEWSLETTER_TEMPLATE.name,
      intent: "AI",
      status: PlanConfigurationStatus.RUNNABLE,
      executorBindings: [{ stepKey: "fetch-news", installationId: "rss-aggregate" }],
      overseerBindings: [{ stepKey: "write-draft", overseerUserId: "overseer-1" }],
      sourceGroups: ["rss-tech", "rss-business"],
      audience: "founders",
    });
  });
});
