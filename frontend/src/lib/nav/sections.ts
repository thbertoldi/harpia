import { Bot, Eye, LayoutDashboard } from "lucide-svelte";
import type { HarpiaRole, NavSectionDef, ResolvedNavSection } from "./types";

/**
 * Shell navigation registry. Feature PRs append entries here instead of
 * editing +layout.svelte — reduces merge conflicts on the app shell.
 */
export const navSectionDefs: NavSectionDef[] = [
  {
    i18nKey: "nav.tasks",
    href: "/",
    icon: LayoutDashboard,
    visibleTo: "all",
  },
  {
    i18nKey: "nav.oversee",
    href: "/oversee",
    icon: Eye,
    visibleTo: "all",
  },
  {
    i18nKey: "nav.agents",
    href: "/agents",
    icon: Bot,
    visibleTo: ["Engineer"],
  },
];

export function filterNavSections(
  role: string | undefined,
  defs: NavSectionDef[] = navSectionDefs,
): NavSectionDef[] {
  return defs.filter((def) => isVisibleToRole(def, role));
}

export function resolveNavSections(
  role: string | undefined,
  translateKey: (key: string) => string,
  defs: NavSectionDef[] = navSectionDefs,
): ResolvedNavSection[] {
  return filterNavSections(role, defs).map((def) => ({
    href: def.href,
    icon: def.icon,
    label: translateKey(def.i18nKey),
  }));
}

function isVisibleToRole(
  def: NavSectionDef,
  role: string | undefined,
): boolean {
  const visibility = def.visibleTo ?? "all";
  if (visibility === "all") {
    return true;
  }
  return role !== undefined && visibility.includes(role as HarpiaRole);
}
