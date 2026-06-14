import { redirect } from "@sveltejs/kit";
import type { PageServerLoad } from "./$types";

export const load: PageServerLoad = async ({ parent }) => {
  const { user } = await parent();

  if (user?.role !== "Engineer") {
    throw redirect(302, "/");
  }

  return {};
};
