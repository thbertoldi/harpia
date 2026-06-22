import type { PageLoad } from "./$types";
import { planClient } from "$lib/rpc";
import type { PlanTemplate } from "$lib/gen/harpia/plans/v1/plans_pb";

// Auth tokens live in localStorage; SSR cannot attach dev-login headers.
export const ssr = false;

export const load: PageLoad = async () => {
  const templates: PlanTemplate[] = [];
  try {
    for await (const page of planClient.listPlanTemplates({
      pageSize: 100,
      pageToken: "",
    })) {
      templates.push(...page.planTemplates);
    }
  } catch (err) {
    console.warn(
      "[/discover] listPlanTemplates failed, showing empty gallery",
      err,
    );
  }
  return { templates };
};
