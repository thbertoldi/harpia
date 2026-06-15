import { error, redirect } from "@sveltejs/kit";
import { canViewAudit } from "$lib/auth-roles";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({ parent }) => {
  const { user } = await parent();

  if (!user) {
    throw redirect(302, "/login");
  }

  if (!canViewAudit(user)) {
    throw error(
      403,
      "Audit log is available to Leader, Overseer, and Engineer roles only.",
    );
  }

  return {};
};
