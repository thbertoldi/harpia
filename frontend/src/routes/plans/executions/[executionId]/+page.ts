import { error, redirect } from "@sveltejs/kit";
import type { PageLoad } from "./$types";
import { planClient } from "$lib/rpc";
import { getTenant } from "$lib/auth";

export const load: PageLoad = async ({ params }) => {
  const tenant = getTenant();
  if (!tenant?.id) {
    throw error(401, "Not authenticated");
  }
  const { executionId } = params;
  try {
    const response = await planClient.getPlanExecution({
      tenantId: tenant.id,
      planExecutionId: executionId,
    });
    const configId = response.planExecution?.planConfigurationId;
    if (!configId) {
      throw error(404, "Plan execution not found");
    }
    throw redirect(
      302,
      `/plans/configurations/${configId}/canvas?run=${executionId}`,
    );
  } catch (e) {
    if (e && typeof e === "object" && "status" in e && "location" in e) {
      throw e;
    }
    throw error(404, "Plan execution not found");
  }
};
