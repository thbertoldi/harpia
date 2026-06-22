import type { PageLoad } from "./$types";
import { planClient } from "$lib/rpc";
import { requireTenantId } from "$lib/auth";
import type { PlanTemplate } from "$lib/gen/harpia/plans/v1/plans_pb";

export const load: PageLoad = async ({ url }) => {
  const tenantId = requireTenantId();
  const templates: PlanTemplate[] = [];
  for await (const page of planClient.listPlanTemplates({ tenantId, pageSize: 50, pageToken: "" })) {
    templates.push(...page.planTemplates);
  }
  return { templates, autoTemplateId: url.searchParams.get("template") ?? "" };
};
