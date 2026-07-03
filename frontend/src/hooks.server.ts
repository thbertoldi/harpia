import { redirect, type Handle } from "@sveltejs/kit";

/**
 * Stabilization (stabilize-user-journey, ADR-017): the standalone plan
 * configuration, plan execution, and artifact surfaces were removed in favor
 * of the conversational model + the Runs panel. Any lingering deep link to
 * those paths is redirected Home rather than surfacing a 404.
 */
const DELETED_ROUTE_PREFIXES = [
  "/plans/configurations",
  "/plans/executions",
  "/artifacts",
];

export const handle: Handle = async ({ event, resolve }) => {
  const { pathname } = event.url;
  for (const prefix of DELETED_ROUTE_PREFIXES) {
    if (pathname === prefix || pathname.startsWith(`${prefix}/`)) {
      throw redirect(302, "/");
    }
  }
  return resolve(event);
};
