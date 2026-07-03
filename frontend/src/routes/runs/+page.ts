import type { PageLoad } from "./$types";
import { getTenant } from "$lib/auth";
import { planClient } from "$lib/rpc";
import { groupExecutionsByConfiguration } from "$lib/plans/runs-panel";
import type {
  PlanConfiguration,
  PlanExecution,
  PlanTemplate,
} from "$lib/gen/harpia/plans/v1/plans_pb";

export const ssr = false;

export const load: PageLoad = async () => {
  const tenant = getTenant();
  if (!tenant?.id) {
    return {
      groups: [],
      templateByConfigurationId: new Map<string, PlanTemplate>(),
    };
  }

  // Runs joins every PlanConfiguration onto its executions, so both are fetched
  // in parallel — the grouping helper tolerates either side being empty.
  const [configurations, executions] = await Promise.all([
    listConfigurations(tenant.id),
    listExecutions(tenant.id),
  ]);

  const groups = groupExecutionsByConfiguration(configurations, executions);

  // Resolve the catalog name for each configuration's template so the Runs
  // panel can render a friendly title without an extra round-trip per group.
  const templateByConfigurationId = new Map<string, PlanTemplate>();
  const templateById = new Map<string, PlanTemplate>();
  for (const templateId of new Set(
    configurations.map((config) => config.planTemplateId),
  )) {
    if (!templateId) continue;
    try {
      const templateResponse = await planClient.getPlanTemplate({
        planTemplateId: templateId,
      });
      if (templateResponse.planTemplate) {
        templateById.set(templateId, templateResponse.planTemplate);
      }
    } catch {
      // A missing template should not blank the whole panel.
    }
  }
  for (const config of configurations) {
    const template = templateById.get(config.planTemplateId);
    if (template) templateByConfigurationId.set(config.id, template);
  }

  return { groups, templateByConfigurationId };
};

async function listConfigurations(
  tenantId: string,
): Promise<PlanConfiguration[]> {
  const out: PlanConfiguration[] = [];
  for await (const page of planClient.listPlanConfigurations({
    tenantId,
    pageSize: 100,
    pageToken: "",
  })) {
    out.push(...page.planConfigurations);
  }
  return out;
}

async function listExecutions(tenantId: string): Promise<PlanExecution[]> {
  const out: PlanExecution[] = [];
  for await (const page of planClient.listPlanExecutions({
    tenantId,
    pageSize: 100,
    pageToken: "",
  })) {
    out.push(...page.planExecutions);
  }
  return out;
}
