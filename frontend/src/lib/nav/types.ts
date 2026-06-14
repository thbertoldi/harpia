import type { LayoutDashboard } from "lucide-svelte";

export type HarpiaRole = "Leader" | "Overseer" | "Engineer";

/** Lucide icon component used in the shell nav. */
export type NavIcon = typeof LayoutDashboard;

export interface NavSectionDef {
  /** i18n key passed to translate(), e.g. nav.tasks */
  i18nKey: string;
  href: string;
  icon: NavIcon;
  /** Defaults to all authenticated roles when omitted. */
  visibleTo?: HarpiaRole[] | "all";
}

export interface ResolvedNavSection {
  label: string;
  href: string;
  icon: NavIcon;
}
