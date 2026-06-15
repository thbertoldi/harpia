import { toUserMessage } from "$lib/connect-errors";
import { translate } from "$lib/i18n";
import type { Locale } from "$lib/i18n";
import {
  ConnectionStatus,
  type ExecutorEntitlement,
  type ExecutorInstallation,
  type ExecutorSKU,
} from "$lib/gen/harpia/executors/v1/executors_pb";
import type { PlanStep, PlanTemplate } from "$lib/gen/harpia/plans/v1/plans_pb";
import {
  mockExecutorContext,
  mockPlanTemplates,
  type MockExecutorContext,
} from "$lib/mocks/plan-catalog";
import { getSession, getTenant } from "$lib/auth";
import { executorClient, planClient } from "$lib/rpc";

export type PlanCatalogSource = "api" | "mock";

export type SkuLockReason =
  | "available"
  | "missing_entitlement"
  | "missing_installation"
  | "not_connected"
  | "not_configured";

export interface RequiredExecutorSku {
  skuKey: string;
  displayName: string;
  stepKeys: string[];
  reason: SkuLockReason;
  messageKey: string;
  messageParams?: Record<string, string>;
}

export interface PlanCatalogEntry {
  template: PlanTemplate;
  requiredSkus: RequiredExecutorSku[];
  isLocked: boolean;
}

export interface PlanCatalogResult {
  entries: PlanCatalogEntry[];
  source: PlanCatalogSource;
  error?: string;
}

export type ExecutorContext = MockExecutorContext;

function resolveTenantId(): string | undefined {
  return getTenant()?.id ?? getSession()?.tenant?.id;
}

function isBlankJson(value: string | undefined): boolean {
  if (!value) return true;
  const trimmed = value.trim();
  return trimmed === "" || trimmed === "{}" || trimmed === "[]";
}

export function isInstallationReady(
  installation: ExecutorInstallation,
): boolean {
  if (!installation.enabled) {
    return false;
  }

  if (installation.detail.case === "integration") {
    const integration = installation.detail.value;
    return (
      integration.connectionStatus === ConnectionStatus.CONNECTED &&
      !isBlankJson(integration.configJson)
    );
  }

  if (installation.detail.case === "agent") {
    const agent = installation.detail.value;
    return Boolean(agent.manifestId && agent.manifestVersion);
  }

  return false;
}

export function resolveSkuLockState(
  sku: ExecutorSKU,
  entitlements: ExecutorEntitlement[],
  installations: ExecutorInstallation[],
): Pick<RequiredExecutorSku, "reason" | "messageKey" | "messageParams"> {
  const entitled = entitlements.some(
    (entitlement) => entitlement.executorSkuId === sku.id,
  );

  if (!entitled) {
    return {
      reason: "missing_entitlement",
      messageKey: "plans.lock.message.missing_entitlement",
      messageParams: { name: sku.displayName, key: sku.key },
    };
  }

  const skuInstallations = installations.filter(
    (installation) =>
      installation.executorSkuId === sku.id && installation.enabled,
  );

  if (skuInstallations.length === 0) {
    return {
      reason: "missing_installation",
      messageKey: "plans.lock.message.missing_installation",
      messageParams: { name: sku.displayName },
    };
  }

  if (skuInstallations.some(isInstallationReady)) {
    return { reason: "available", messageKey: "plans.lock.message.available" };
  }

  const integration = skuInstallations.find(
    (installation) => installation.detail.case === "integration",
  );
  if (integration?.detail.case === "integration") {
    const detail = integration.detail.value;
    if (detail.connectionStatus !== ConnectionStatus.CONNECTED) {
      return {
        reason: "not_connected",
        messageKey: "plans.lock.message.not_connected",
        messageParams: { name: sku.displayName },
      };
    }
    if (isBlankJson(detail.configJson)) {
      return {
        reason: "not_configured",
        messageKey: "plans.lock.message.not_configured",
        messageParams: { name: sku.displayName },
      };
    }
  }

  const agent = skuInstallations.find(
    (installation) => installation.detail.case === "agent",
  );
  if (agent?.detail.case === "agent") {
    const detail = agent.detail.value;
    if (!detail.manifestId || !detail.manifestVersion) {
      return {
        reason: "not_configured",
        messageKey: "plans.lock.message.agent_missing_manifest",
        messageParams: { name: sku.displayName },
      };
    }
  }

  return {
    reason: "not_configured",
    messageKey: "plans.lock.message.installation_not_ready",
    messageParams: { name: sku.displayName },
  };
}

export function formatSkuLockMessage(
  sku: RequiredExecutorSku,
  locale: Locale,
): string {
  return translate(sku.messageKey, locale, sku.messageParams);
}

export function formatPlanLockSummary(
  entry: PlanCatalogEntry,
  locale: Locale,
): string | undefined {
  const blocking = entry.requiredSkus.filter(
    (sku) => sku.reason !== "available",
  );
  if (blocking.length === 0) {
    return undefined;
  }
  return blocking.map((sku) => formatSkuLockMessage(sku, locale)).join(" ");
}

export function collectRequiredSkuKeys(
  steps: PlanStep[],
): Map<string, string[]> {
  const required = new Map<string, string[]>();

  for (const step of steps) {
    const skuKey = step.defaultExecutorSkuKey.trim();
    if (!skuKey) continue;

    const stepKeys = required.get(skuKey) ?? [];
    stepKeys.push(step.key);
    required.set(skuKey, stepKeys);
  }

  return required;
}

export function buildPlanCatalogEntry(
  template: PlanTemplate,
  context: ExecutorContext,
): PlanCatalogEntry {
  const skuByKey = new Map(context.skus.map((sku) => [sku.key, sku]));
  const requiredSkuKeys = collectRequiredSkuKeys(template.steps);

  const requiredSkus: RequiredExecutorSku[] = [];

  for (const [skuKey, stepKeys] of requiredSkuKeys) {
    const sku = skuByKey.get(skuKey);
    const displayName = sku?.displayName ?? skuKey;

    if (!sku) {
      requiredSkus.push({
        skuKey,
        displayName,
        stepKeys,
        reason: "missing_entitlement",
        messageKey: "plans.lock.message.sku_missing_catalog",
        messageParams: { key: skuKey },
      });
      continue;
    }

    const lock = resolveSkuLockState(
      sku,
      context.entitlements,
      context.installations,
    );

    requiredSkus.push({
      skuKey,
      displayName,
      stepKeys,
      ...lock,
    });
  }

  requiredSkus.sort((a, b) => a.displayName.localeCompare(b.displayName));

  const blockingSkus = requiredSkus.filter((sku) => sku.reason !== "available");
  const isLocked = blockingSkus.length > 0;

  return {
    template,
    requiredSkus,
    isLocked,
  };
}

async function fetchPlanTemplatesFromApi(): Promise<PlanTemplate[]> {
  const templates: PlanTemplate[] = [];

  for await (const page of planClient.listPlanTemplates({
    pageSize: 100,
    pageToken: "",
  })) {
    templates.push(...page.planTemplates);
  }

  return templates;
}

async function fetchExecutorContextFromApi(
  tenantId: string,
): Promise<ExecutorContext> {
  const skus: ExecutorSKU[] = [];
  for await (const page of executorClient.listExecutorSKUs({
    pageSize: 100,
    pageToken: "",
  })) {
    skus.push(...page.executorSkus);
  }

  const entitlements: ExecutorEntitlement[] = [];
  for await (const page of executorClient.listExecutorEntitlements({
    tenantId,
    pageSize: 100,
    pageToken: "",
  })) {
    entitlements.push(...page.entitlements);
  }

  const installations: ExecutorInstallation[] = [];
  for await (const page of executorClient.listExecutorInstallations({
    tenantId,
    pageSize: 100,
    pageToken: "",
  })) {
    installations.push(...page.installations);
  }

  return { skus, entitlements, installations };
}

/** Loads plan templates and computes tenant lock state from executor entitlements/installations. */
export async function loadPlanCatalog(): Promise<PlanCatalogResult> {
  let templates: PlanTemplate[];
  let source: PlanCatalogSource = "api";
  let error: string | undefined;

  try {
    templates = await fetchPlanTemplatesFromApi();
    if (templates.length === 0) {
      templates = mockPlanTemplates();
      source = "mock";
    }
  } catch (err) {
    templates = mockPlanTemplates();
    source = "mock";
    error = toUserMessage(err);
  }

  const tenantId = resolveTenantId();
  let context: ExecutorContext;

  try {
    if (!tenantId) {
      throw new Error("Select a tenant to evaluate plan availability.");
    }
    context = await fetchExecutorContextFromApi(tenantId);
  } catch (err) {
    if (source === "mock") {
      context = mockExecutorContext(tenantId ?? "dev");
    } else {
      context = { skus: [], entitlements: [], installations: [] };
      error = error ?? toUserMessage(err);
    }
  }

  const entries = templates.map((template) =>
    buildPlanCatalogEntry(template, context),
  );

  return { entries, source, error };
}
