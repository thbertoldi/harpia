import { error, isHttpError } from "@sveltejs/kit";
import type { PageLoad } from "./$types";
import { planClient, executorClient } from "$lib/rpc";
import { getTenant } from "$lib/auth";

type ExecutorCatalogEntry = {
  displayName: string;
  pricePerRunBrl: number | null;
};

// Auth tokens live in localStorage; SSR cannot attach dev-login headers.
export const ssr = false;

async function loadExecutorCatalog(tenantId: string): Promise<Map<string, ExecutorCatalogEntry>> {
  const executorCatalog = new Map<string, ExecutorCatalogEntry>();
  const skuPrices = new Map<string, number | null>();
  for await (const page of executorClient.listExecutorSKUs({ pageSize: 100, pageToken: "" })) {
    for (const sku of page.executorSkus) {
      skuPrices.set(sku.id, sku.priceCents ? Number(sku.priceCents) / 100 : null);
    }
  }
  for await (const page of executorClient.listExecutorInstallations({ tenantId, pageSize: 100, pageToken: "" })) {
    for (const inst of page.installations) {
      executorCatalog.set(inst.id, {
        displayName: inst.displayName,
        pricePerRunBrl: skuPrices.get(inst.executorSkuId) ?? null,
      });
    }
  }
  return executorCatalog;
}

async function loadExecutorCatalogSafe(tenantId: string): Promise<Map<string, ExecutorCatalogEntry>> {
  try {
    return await loadExecutorCatalog(tenantId);
  } catch (err) {
    console.warn("[loadExecutorCatalog] failed, using empty catalog", err);
    return new Map();
  }
}

export const load: PageLoad = async ({ params, url }) => {
  const tenant = getTenant();
  if (!tenant?.id) {
    throw error(401, "Not authenticated");
  }
  const { configurationId } = params;
  const runId = url.searchParams.get("run") ?? "";

  try {
    const config = await planClient.getPlanConfiguration({
      tenantId: tenant.id,
      planConfigurationId: configurationId,
    });
    if (!config.planConfiguration) {
      throw error(404, "Plan configuration not found");
    }
    const template = await planClient.getPlanTemplate({
      planTemplateId: config.planConfiguration.planTemplateId,
    });
    const executorCatalog = await loadExecutorCatalogSafe(tenant.id);
    return {
      configurationId,
      runId,
      configuration: config.planConfiguration,
      template: template.planTemplate,
      executorCatalog,
    };
  } catch (err) {
    if (isHttpError(err)) {
      throw err;
    }
    throw error(404, "Plan configuration not found");
  }
};
