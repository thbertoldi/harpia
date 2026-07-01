import { error } from "@sveltejs/kit";
import type { PageLoad } from "./$types";
import { getTenant } from "$lib/auth";
import { executorClient, planClient, threadClient } from "$lib/rpc";

export const ssr = false;

export const load: PageLoad = async ({ params }) => {
  const tenant = getTenant();
  if (!tenant?.id) throw error(401, "Not authenticated");

  const threadResponse = await threadClient.getThread({
    tenantId: tenant.id,
    threadId: params.threadId,
  });
  const thread = threadResponse.thread;
  if (!thread) throw error(404, "Thread not found");
  if (!thread.activePlanConfigurationId) {
    return {
      thread,
      threadId: params.threadId,
      configurationId: "",
      configuration: undefined,
      template: undefined,
      executorCatalog: new Map(),
    };
  }

  const configResponse = await planClient.getPlanConfiguration({
    tenantId: tenant.id,
    planConfigurationId: thread.activePlanConfigurationId,
  });
  const configuration = configResponse.planConfiguration;
  if (!configuration) throw error(404, "Plan configuration not found");

  const templateResponse = await planClient.getPlanTemplate({
    planTemplateId: configuration.planTemplateId,
  });

  const executorCatalog = new Map<
    string,
    { displayName: string; pricePerRunBrl: number | null }
  >();
  try {
    const skuPrices = new Map<string, number | null>();
    for await (const page of executorClient.listExecutorSKUs({
      pageSize: 100,
      pageToken: "",
    })) {
      for (const sku of page.executorSkus) {
        const cents = sku.listPrice?.priceCents;
        skuPrices.set(sku.id, cents ? Number(cents) / 100 : null);
      }
    }
    for await (const page of executorClient.listExecutorInstallations({
      tenantId: tenant.id,
      pageSize: 100,
      pageToken: "",
    })) {
      for (const inst of page.installations) {
        executorCatalog.set(inst.id, {
          displayName: inst.displayName,
          pricePerRunBrl: skuPrices.get(inst.executorSkuId) ?? null,
        });
      }
    }
  } catch {
    // The chat route can still render without pricing.
  }

  return {
    thread,
    threadId: params.threadId,
    configurationId: configuration.id,
    configuration,
    template: templateResponse.planTemplate,
    executorCatalog,
  };
};
