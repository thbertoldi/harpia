import { create } from "@bufbuild/protobuf";
import { describe, expect, it } from "vitest";
import {
  ExecutorKind,
  ExecutorSKUSchema,
  ListPriceSchema,
} from "$lib/gen/harpia/executors/v1/executors_pb";
import {
  ElicitationTimeoutBehavior,
  PlanBehaviorPoliciesSchema,
  PublishApprovalMode,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import {
  buildConfigCostSummary,
  buildConfigLineItems,
  formatMoney,
  resolveListPrice,
  sumLineItemCosts,
} from "$lib/plans/config-summary";
import { getPlanConfigDraft } from "$lib/plans/plan-config-draft";
import {
  mockExecutorContext,
  WEEKLY_NEWSLETTER_TEMPLATE,
} from "$lib/mocks/plan-catalog";

describe("config summary", () => {
  const draft = getPlanConfigDraft(WEEKLY_NEWSLETTER_TEMPLATE.id);
  const skus = mockExecutorContext().skus;

  it("resolves list price from SKU metadata with mock fallback", () => {
    const pricedSku = create(ExecutorSKUSchema, {
      id: "sku-priced",
      key: "rss-news-feed",
      displayName: "RSS News Feed",
      description: "",
      kind: ExecutorKind.INTEGRATION,
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
});
