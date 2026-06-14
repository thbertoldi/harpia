import { error, redirect } from "@sveltejs/kit";
import type { PageServerLoad } from "./$types";

const AUDIT_ROLES = new Set(["Overseer", "Engineer"]);

export const load: PageServerLoad = async ({ parent }) => {
  const { user } = await parent();

  if (!user) {
    throw redirect(302, "/login");
  }

  if (!user.role || !AUDIT_ROLES.has(user.role)) {
    throw error(403, "Audit log is available to Overseer and Engineer roles only.");
  }

  return {};
};
