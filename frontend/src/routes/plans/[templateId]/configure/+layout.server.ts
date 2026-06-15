import { redirect } from "@sveltejs/kit";
import { canConfigurePlans } from "$lib/auth-roles";
import type { LayoutServerLoad } from "./$types";

export const load: LayoutServerLoad = async ({ parent }) => {
  const { user } = await parent();

  if (!canConfigurePlans(user)) {
    throw redirect(302, "/");
  }

  return {};
};
