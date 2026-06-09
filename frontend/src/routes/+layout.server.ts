import { dev } from "$app/environment";
import { redirect } from "@sveltejs/kit";
import type { LayoutServerLoad } from "./$types";

const PUBLIC_ROUTES = ["/login", "/auth/callback"];
const SESSION_COOKIE = "harpia_session";

function isDevSession(user: { sub?: unknown } | null): boolean {
  return typeof user?.sub === "string" && user.sub.startsWith("dev-");
}

export const load: LayoutServerLoad = async ({ cookies, url }) => {
  const sessionCookie = cookies.get(SESSION_COOKIE);

  let user: { sub: string; email: string; name: string; role?: string } | null =
    null;

  if (sessionCookie) {
    try {
      user = JSON.parse(decodeURIComponent(sessionCookie));
    } catch {
      user = null;
    }
  }

  if (!dev && isDevSession(user)) {
    cookies.delete(SESSION_COOKIE, { path: "/" });
    user = null;
  }

  const isPublic = PUBLIC_ROUTES.some((r) => url.pathname.startsWith(r));

  if (!isPublic && !user) {
    throw redirect(302, "/login");
  }

  return {
    user,
  };
};
