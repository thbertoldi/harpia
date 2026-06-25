import { redirect } from "@sveltejs/kit";
import { canManageAgents } from "$lib/auth-roles";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({ parent }) => {
  const { user } = await parent();

  if (!canManageAgents(user)) {
    throw redirect(302, "/");
  }

  return {};
};
