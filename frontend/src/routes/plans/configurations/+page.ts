import type { PageLoad } from "./$types";
import { planClient } from "$lib/rpc";
import { requireTenantId } from "$lib/auth";
import type {
  PlanConfiguration,
  PlanTemplate,
} from "$lib/gen/harpia/plans/v1/plans_pb";

// Auth tokens live in localStorage; SSR cannot attach dev-login headers.
export const ssr = false;

export const load: PageLoad = async () => {
  const tenantId = requireTenantId();
  const configurations: PlanConfiguration[] = [];
  const templatesById = new Map<string, PlanTemplate>();
  try {
    for await (const page of planClient.listPlanConfigurations({
      tenantId,
      pageSize: 200,
      pageToken: "",
    })) {
      configurations.push(...page.planConfigurations);
    }
    // Load the templates referenced by these configurations so the row can
    // display the template name. A small set in practice (templates are
    // few) — one GET per unique template id.
    const uniqueTemplateIds = new Set(configurations.map((c) => c.planTemplateId));
    await Promise.all(
      Array.from(uniqueTemplateIds).map(async (id) => {
        try {
          const res = await planClient.getPlanTemplate({ planTemplateId: id });
          if (res.planTemplate) templatesById.set(id, res.planTemplate);
        } catch {
          // Skip — row will show the template id instead of name.
        }
      }),
    );
  } catch (err) {
    console.warn("[/plans/configurations] list failed", err);
  }
  return { configurations, templatesById };
};
