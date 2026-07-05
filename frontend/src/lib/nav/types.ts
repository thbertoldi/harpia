import type { LayoutDashboard } from "lucide-svelte";
import type { HarpiaPermission } from "$lib/auth-roles";

/** Lucide icon component used in the shell nav. */
export type NavIcon = typeof LayoutDashboard;

export interface NavSectionDef {
  /** i18n key passed to translate(), e.g. nav.tasks */
  i18nKey: string;
  href: string;
  icon: NavIcon;
  /** Defaults to all authenticated roles when omitted. */
  visibleTo?: "all";
  requiredPermission?: HarpiaPermission;
}

export interface ResolvedNavSection {
  label: string;
  href: string;
  icon: NavIcon;
  /**
   * Optional pending-count pill (e.g. inbox approvals/elicitations).
   * Rendered hidden when undefined or 0. The shell owns the live source.
   */
  badge?: number;
}
