import { error } from "@sveltejs/kit";
import type { PageLoad } from "./$types";
import { planClient, executorClient } from "$lib/rpc";
import { getTenant } from "$lib/auth";

async function loadExecutorCatalog(tenantId: string) {
  const executorCatalog = new Map<string, { displayName: string; pricePerRunBrl: number | null }>();
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

export const load: PageLoad = async ({ params }) => {
  const tenant = getTenant();
  if (!tenant?.id) {
    throw error(401, "Not authenticated");
  }
  const { configurationId } = params;
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
    const executorCatalog = await loadExecutorCatalog(tenant.id);
    return {
      configurationId,
      configuration: config.planConfiguration,
      template: template.planTemplate,
      executorCatalog,
    };
  } catch {
    throw error(404, "Plan configuration not found");
  }
};
