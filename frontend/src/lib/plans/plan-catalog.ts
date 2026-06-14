import { toUserMessage } from "$lib/connect-errors";
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
  message: string;
}

export interface PlanCatalogEntry {
  template: PlanTemplate;
  requiredSkus: RequiredExecutorSku[];
  isLocked: boolean;
  lockSummary?: string;
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
): Pick<RequiredExecutorSku, "reason" | "message"> {
  const entitled = entitlements.some(
    (entitlement) => entitlement.executorSkuId === sku.id,
  );

  if (!entitled) {
    return {
      reason: "missing_entitlement",
      message: `Missing SKU entitlement for ${sku.displayName} (${sku.key}). Ask a platform engineer to provision access.`,
    };
  }

  const skuInstallations = installations.filter(
    (installation) =>
      installation.executorSkuId === sku.id && installation.enabled,
  );

  if (skuInstallations.length === 0) {
    return {
      reason: "missing_installation",
      message: `No installation configured for ${sku.displayName}. Create one under Integrations.`,
    };
  }

  if (skuInstallations.some(isInstallationReady)) {
    return { reason: "available", message: "" };
  }

  const integration = skuInstallations.find(
    (installation) => installation.detail.case === "integration",
  );
  if (integration?.detail.case === "integration") {
    const detail = integration.detail.value;
    if (detail.connectionStatus !== ConnectionStatus.CONNECTED) {
      return {
        reason: "not_connected",
        message: `${sku.displayName} is not connected. Finish OAuth or connection setup in Integrations.`,
      };
    }
    if (isBlankJson(detail.configJson)) {
      return {
        reason: "not_configured",
        message: `${sku.displayName} needs configuration (for example feed URLs or OAuth settings).`,
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
        message: `${sku.displayName} agent installation is missing manifest metadata.`,
      };
    }
  }

  return {
    reason: "not_configured",
    message: `${sku.displayName} installation is not ready to run plan steps.`,
  };
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
        message: `Executor SKU "${skuKey}" is not in the catalog. Provision the SKU before using this plan.`,
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
  const lockSummary = isLocked
    ? blockingSkus.map((sku) => sku.message).join(" ")
    : undefined;

  return {
    template,
    requiredSkus,
    isLocked,
    lockSummary,
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
