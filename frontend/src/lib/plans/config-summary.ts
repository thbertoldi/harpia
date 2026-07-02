import { toUserMessage } from "$lib/connect-errors";
import type { ExecutorSKU } from "$lib/gen/harpia/executors/v1/executors_pb";
import {
  ElicitationTimeoutBehavior,
  ExecutorKind,
  PublishApprovalMode,
  type PlanConfiguration,
  type PlanStep,
  type PlanTemplate,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import {
  getMockSkuListPrice,
  mockExecutorContext,
} from "$lib/mocks/plan-catalog";
import { allowsMockFallback } from "$lib/dev-mocks";
import { executorClient } from "$lib/rpc";
import type { PlanConfigDraft } from "$lib/plans/plan-config-draft";

export type ConfigLineItemKind = "step" | "executor" | "overseer" | "policy";

export interface ConfigLineItem {
  kind: ConfigLineItemKind;
  key: string;
  label: string;
  detail?: string;
  priceCents: number;
  currency: string;
}

export interface ConfigCostSummary {
  lineItems: ConfigLineItem[];
  totalCents: number;
  currency: string;
}

export interface SharedPlanSummary {
  id: string;
  templateName: string;
  intent: string;
  status: PlanConfiguration["status"];
  executorBindings: Array<{ stepKey: string; installationId: string }>;
  overseerBindings: Array<{ stepKey: string; overseerUserId: string }>;
  sourceGroups: string[];
  audience: string;
}

export type ExecutorSkuSource = "api" | "mock";

export interface ExecutorSkuCatalog {
  skus: ExecutorSKU[];
  source: ExecutorSkuSource;
  error?: string;
}

const DEFAULT_CURRENCY = "USD";

/** Platform fee for overseer review gates (human time is not metered). */
export const OVERSEER_REVIEW_CENTS = 0;

/** Nominal policy surcharges used until budget-policy service exposes list prices. */
export const POLICY_SURCHARGES = {
  publishApproval: 0,
  elicitationPause: 0,
} as const;

export function resolveListPrice(
  sku: ExecutorSKU | undefined,
  skuKey: string,
): { priceCents: number; currency: string } {
  if (sku?.listPrice) {
    return {
      priceCents: Number(sku.listPrice.priceCents),
      currency: sku.listPrice.currency || DEFAULT_CURRENCY,
    };
  }

  const fallback = getMockSkuListPrice(skuKey);
  if (fallback) {
    return {
      priceCents: Number(fallback.priceCents),
      currency: fallback.currency || DEFAULT_CURRENCY,
    };
  }

  return { priceCents: 0, currency: DEFAULT_CURRENCY };
}

export function formatMoney(
  priceCents: number,
  currency: string,
  locale = "en",
): string {
  return new Intl.NumberFormat(locale, {
    style: "currency",
    currency,
    minimumFractionDigits: 2,
  }).format(priceCents / 100);
}

export function sumLineItemCosts(lineItems: ConfigLineItem[]): {
  totalCents: number;
  currency: string;
} {
  if (lineItems.length === 0) {
    return { totalCents: 0, currency: DEFAULT_CURRENCY };
  }

  const currency = lineItems[0]?.currency ?? DEFAULT_CURRENCY;
  const totalCents = lineItems.reduce((sum, item) => sum + item.priceCents, 0);

  return { totalCents, currency };
}

function resolveSkuForStep(
  step: PlanStep,
  draft: PlanConfigDraft,
  skuById: Map<string, ExecutorSKU>,
  skuByKey: Map<string, ExecutorSKU>,
): { sku?: ExecutorSKU; skuKey: string } {
  const binding = draft.slotBindings.find((slot) => slot.stepKey === step.key);
  if (binding?.executorSkuId) {
    const sku = skuById.get(binding.executorSkuId);
    return { sku, skuKey: sku?.key ?? step.defaultExecutorSkuKey };
  }

  const skuKey = step.defaultExecutorSkuKey.trim();
  return { sku: skuByKey.get(skuKey), skuKey };
}

function buildPolicyLineItems(
  draft: PlanConfigDraft,
  currency: string,
): ConfigLineItem[] {
  const items: ConfigLineItem[] = [];
  const policies = draft.behaviorPolicies;

  if (policies.publishApprovalMode === PublishApprovalMode.REQUIRE_APPROVAL) {
    items.push({
      kind: "policy",
      key: "policy:publish-approval",
      label: "Publish approval gate",
      detail: "require_approval",
      priceCents: POLICY_SURCHARGES.publishApproval,
      currency,
    });
  }

  if (
    policies.elicitationTimeoutBehavior ===
    ElicitationTimeoutBehavior.PAUSE_UNTIL_ANSWERED
  ) {
    items.push({
      kind: "policy",
      key: "policy:elicitation-pause",
      label: "Elicitation pause",
      detail:
        policies.elicitationTimeoutHours > 0
          ? `${policies.elicitationTimeoutHours}h timeout`
          : undefined,
      priceCents: POLICY_SURCHARGES.elicitationPause,
      currency,
    });
  }

  return items;
}

/** Builds à la carte line items for a plan configuration draft. */
export function buildConfigLineItems(
  template: PlanTemplate,
  draft: PlanConfigDraft,
  skus: ExecutorSKU[],
): ConfigLineItem[] {
  const skuById = new Map(skus.map((sku) => [sku.id, sku]));
  const skuByKey = new Map(skus.map((sku) => [sku.key, sku]));
  const items: ConfigLineItem[] = [];
  let currency = DEFAULT_CURRENCY;

  for (const step of template.steps) {
    items.push({
      kind: "step",
      key: `step:${step.key}`,
      label: step.title,
      detail: step.key,
      priceCents: 0,
      currency,
    });

    const { sku, skuKey } = resolveSkuForStep(step, draft, skuById, skuByKey);
    const price = resolveListPrice(sku, skuKey);
    currency = price.currency;

    items.push({
      kind: "executor",
      key: `executor:${step.key}`,
      label: sku?.displayName ?? skuKey,
      detail: step.key,
      priceCents: price.priceCents,
      currency: price.currency,
    });

    const overseer = draft.overseerBindings.find(
      (binding) => binding.stepKey === step.key,
    );
    if (overseer) {
      items.push({
        kind: "overseer",
        key: `overseer:${step.key}`,
        label: overseer.overseerUserId,
        detail: step.key,
        priceCents: OVERSEER_REVIEW_CENTS,
        currency,
      });
    }
  }

  items.push(...buildPolicyLineItems(draft, currency));
  return items;
}

export function buildConfigCostSummary(
  template: PlanTemplate,
  draft: PlanConfigDraft,
  skus: ExecutorSKU[],
): ConfigCostSummary {
  const lineItems = buildConfigLineItems(template, draft, skus);
  const { totalCents, currency } = sumLineItemCosts(lineItems);

  return { lineItems, totalCents, currency };
}

function parseParameterValues(raw: string): Record<string, unknown> {
  if (!raw.trim()) return {};
  try {
    const parsed = JSON.parse(raw);
    return parsed && typeof parsed === "object" && !Array.isArray(parsed)
      ? (parsed as Record<string, unknown>)
      : {};
  } catch {
    return {};
  }
}

function stringList(value: unknown): string[] {
  if (Array.isArray(value)) {
    return value
      .map((entry) => (typeof entry === "string" ? entry.trim() : ""))
      .filter(Boolean);
  }
  if (typeof value === "string" && value.trim()) return [value.trim()];
  return [];
}

export function buildPlanSummary(
  template: PlanTemplate,
  configuration: PlanConfiguration,
): SharedPlanSummary {
  const params = parseParameterValues(configuration.parameterValuesJson);
  const intent =
    typeof params.theme === "string" && params.theme.trim()
      ? params.theme.trim()
      : template.name;
  return {
    id: configuration.id,
    templateName: template.name,
    intent,
    status: configuration.status,
    executorBindings: configuration.slotBindings.map((binding) => ({
      stepKey: binding.stepKey,
      installationId: binding.executorInstallationId,
    })),
    overseerBindings: configuration.overseerBindings.map((binding) => ({
      stepKey: binding.stepKey,
      overseerUserId: binding.overseerUserId,
    })),
    sourceGroups: stringList(params.source_groups ?? params.source_group),
    audience: typeof params.audience === "string" ? params.audience.trim() : "",
  };
}

async function fetchExecutorSkusFromApi(): Promise<ExecutorSKU[]> {
  const skus: ExecutorSKU[] = [];

  for await (const page of executorClient.listExecutorSKUs({
    pageSize: 100,
    pageToken: "",
  })) {
    skus.push(...page.executorSkus);
  }

  return skus;
}

/** Loads executor SKUs for list-price resolution; mock fallback only when explicitly enabled. */
export async function loadExecutorSkuCatalog(): Promise<ExecutorSkuCatalog> {
  try {
    const skus = await fetchExecutorSkusFromApi();
    if (skus.length === 0) {
      if (allowsMockFallback()) {
        return {
          skus: mockExecutorContext().skus,
          source: "mock",
          error: "Empty executor SKU catalog",
        };
      }
      throw new Error("Executor SKU catalog is empty.");
    }

    return { skus, source: "api" };
  } catch (error) {
    if (allowsMockFallback()) {
      return {
        skus: mockExecutorContext().skus,
        source: "mock",
        error: toUserMessage(error),
      };
    }
    throw error;
  }
}

export function isAgentStep(step: PlanStep): boolean {
  return step.executorRequirement?.executorKind === ExecutorKind.AGENT;
}
