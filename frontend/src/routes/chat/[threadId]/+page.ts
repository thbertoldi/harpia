import { error } from "@sveltejs/kit";
import type { PageLoad } from "./$types";
import { getTenant } from "$lib/auth";
import { executorClient, planClient, threadClient } from "$lib/rpc";
import type {
  PlanConfiguration,
  PlanTemplate,
} from "$lib/gen/harpia/plans/v1/plans_pb";

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

  const configurations: PlanConfiguration[] = [];
  for await (const page of planClient.listPlanConfigurations({
    tenantId: tenant.id,
    originThreadId: params.threadId,
    pageSize: 50,
    pageToken: "",
  })) {
    configurations.push(...page.planConfigurations);
  }

  if (configurations.length === 0) {
    return {
      thread,
      threadId: params.threadId,
      configurationId: "",
      configuration: undefined,
      configurations,
      template: undefined,
      templateByConfigurationId: new Map<string, PlanTemplate>(),
      executorCatalog: new Map(),
    };
  }

  const configuration =
    configurations.find(
      (config) => config.id === thread.activePlanConfigurationId,
    ) ?? configurations[0];
  if (!configuration) throw error(404, "Plan configuration not found");

  const templateById = new Map<string, PlanTemplate>();
  for (const templateId of new Set(
    configurations.map((config) => config.planTemplateId),
  )) {
    const templateResponse = await planClient.getPlanTemplate({
      planTemplateId: templateId,
    });
    if (templateResponse.planTemplate) {
      templateById.set(templateId, templateResponse.planTemplate);
    }
  }
  const templateByConfigurationId = new Map<string, PlanTemplate>();
  for (const config of configurations) {
    const template = templateById.get(config.planTemplateId);
    if (template) templateByConfigurationId.set(config.id, template);
  }

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
    configurations,
    template: templateByConfigurationId.get(configuration.id),
    templateByConfigurationId,
    executorCatalog,
  };
};
