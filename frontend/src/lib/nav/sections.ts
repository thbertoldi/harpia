import {
  Activity,
  BookOpen,
  Bot,
  Inbox,
  LayoutDashboard,
  Plug,
  ScrollText,
  Settings,
} from "lucide-svelte";
import { hasPermission } from "$lib/auth-roles";
import type { NavSectionDef, ResolvedNavSection } from "./types";

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
    i18nKey: "nav.ongoing",
    href: "/tasks/ongoing",
    icon: Activity,
    visibleTo: "all",
  },
  {
    i18nKey: "nav.needsYou",
    href: "/inbox",
    icon: Inbox,
    visibleTo: "all",
  },
  {
    i18nKey: "nav.plans",
    href: "/plans",
    icon: BookOpen,
    visibleTo: "all",
  },
  {
    i18nKey: "nav.integrations",
    href: "/integrations",
    icon: Plug,
    requiredPermission: "manageIntegrations",
  },
  {
    i18nKey: "nav.audit",
    href: "/audit",
    icon: ScrollText,
    requiredPermission: "viewAudit",
  },
  {
    i18nKey: "nav.agents",
    href: "/agents",
    icon: Bot,
    requiredPermission: "manageAgents",
  },
  {
    i18nKey: "nav.settings",
    href: "/settings",
    icon: Settings,
    requiredPermission: "manageTenantSettings",
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
  if (def.visibleTo === "all") {
    return true;
  }
  if (def.requiredPermission) {
    return hasPermission({ role }, def.requiredPermission);
  }
  return true;
}
