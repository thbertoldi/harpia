import { hasPermission, type HarpiaPermission } from "$lib/auth-roles";

export type PersonaMode = "operator" | "admin";

export const PERSONA_STORAGE_KEY = "harpia.personaMode";

const ADMIN_PERMISSIONS: HarpiaPermission[] = [
  "manageIntegrations",
  "manageAgents",
  "viewAudit",
  "manageTenantSettings",
];

export function readPersonaMode(
  storage: Pick<Storage, "getItem">,
): PersonaMode {
  return storage.getItem(PERSONA_STORAGE_KEY) === "admin" ? "admin" : "operator";
}

export function writePersonaMode(
  storage: Pick<Storage, "setItem">,
  mode: PersonaMode,
): void {
  storage.setItem(PERSONA_STORAGE_KEY, mode);
}

/**
 * A user can switch into the admin persona when they hold at least one
 * admin-side permission. Operator persona is always available.
 */
export function canSwitchToAdminPersona(role: string | undefined): boolean {
  if (!role) return false;
  return ADMIN_PERMISSIONS.some((p) => hasPermission({ role }, p));
}
