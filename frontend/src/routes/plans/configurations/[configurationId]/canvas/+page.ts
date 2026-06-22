import { error } from "@sveltejs/kit";
import type { PageLoad } from "./$types";
import { planClient } from "$lib/rpc";
import { getTenant } from "$lib/auth";

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
    return {
      configurationId,
      runId,
      configuration: config.planConfiguration,
      template: template.planTemplate,
    };
  } catch {
    throw error(404, "Plan configuration not found");
  }
};
