import { error } from "@sveltejs/kit";
import { canManageTenantSettings } from "$lib/auth-roles";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({ parent }) => {
  const { user } = await parent();

  if (!canManageTenantSettings(user)) {
    throw error(
      403,
      "Access denied. Tenant settings require Engineer or Leader role.",
    );
  }

  return {};
};
