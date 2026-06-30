import {
  Activity,
  Bot,
  Compass,
  Files,
  Inbox,
  Plug,
  Plus,
  ScrollText,
  Settings,
} from "lucide-svelte";
import { hasPermission, type HarpiaPermission } from "$lib/auth-roles";
import type { PersonaMode } from "$lib/personas/storage";
import type { NavIcon, ResolvedNavSection } from "./types";

interface M1Section {
  i18nKey: string;
  href: string;
  icon: NavIcon;
}

const OPERATOR_SECTIONS: M1Section[] = [
  { i18nKey: "nav.needsYou", href: "/inbox", icon: Inbox },
  { i18nKey: "nav.discover", href: "/discover", icon: Compass },
  { i18nKey: "nav.newPlan", href: "/new", icon: Plus },
  { i18nKey: "nav.artifacts", href: "/artifacts", icon: Files },
  { i18nKey: "nav.executions", href: "/plans/executions", icon: Activity },
];

const ADMIN_SECTIONS: Array<
  M1Section & { requiredPermission: HarpiaPermission }
> = [
  {
    i18nKey: "nav.adminIntegrations",
    href: "/admin/integrations",
    icon: Plug,
    requiredPermission: "manageIntegrations",
  },
  {
    i18nKey: "nav.adminAgents",
    href: "/admin/agents",
    icon: Bot,
    requiredPermission: "manageAgents",
  },
  {
    i18nKey: "nav.adminAudit",
    href: "/admin/audit",
    icon: ScrollText,
    requiredPermission: "viewAudit",
  },
  {
    i18nKey: "nav.adminSettings",
    href: "/admin/settings",
    icon: Settings,
    requiredPermission: "manageTenantSettings",
  },
];

export function resolveNavSectionsM1(
  personaMode: PersonaMode,
  role: string | undefined,
  translateKey: (key: string) => string,
): ResolvedNavSection[] {
  if (personaMode === "operator") {
    return OPERATOR_SECTIONS.map(({ i18nKey, href, icon }) => ({
      href,
      icon,
      label: translateKey(i18nKey),
    }));
  }
  return ADMIN_SECTIONS.filter((def) =>
    hasPermission({ role }, def.requiredPermission),
  ).map(({ i18nKey, href, icon }) => ({
    href,
    icon,
    label: translateKey(i18nKey),
  }));
}
