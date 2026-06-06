import { redirect } from "@sveltejs/kit";
import type { LayoutServerLoad } from "./$types";

const PUBLIC_ROUTES = ["/login", "/auth/callback"];

export const load: LayoutServerLoad = async ({ cookies, url }) => {
  const sessionCookie = cookies.get("harpia_session");

  let user: { sub: string; email: string; name: string; role?: string } | null =
    null;

  if (sessionCookie) {
    try {
      user = JSON.parse(decodeURIComponent(sessionCookie));
    } catch {
      user = null;
    }
  }

  const isPublic = PUBLIC_ROUTES.some((r) => url.pathname.startsWith(r));

  if (!isPublic && !user) {
    throw redirect(302, "/login");
  }

  return {
    user,
  };
};
