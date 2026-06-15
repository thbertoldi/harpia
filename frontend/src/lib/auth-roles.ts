export type HarpiaRole = "Leader" | "Overseer" | "Engineer";

export type HarpiaPermission =
  | "manageIntegrations"
  | "manageAgents"
  | "viewAudit"
  | "configurePlans";

const ROLE_PERMISSIONS: Record<HarpiaRole, ReadonlySet<HarpiaPermission>> = {
  Leader: new Set([
    "manageIntegrations",
    "manageAgents",
    "viewAudit",
    "configurePlans",
  ]),
  Engineer: new Set([
    "manageIntegrations",
    "manageAgents",
    "viewAudit",
    "configurePlans",
  ]),
  Overseer: new Set(["viewAudit"]),
};

const LEADER_ALIASES = new Set(["Leader", "admin", "owner"]);

export function normalizeRole(
  role: string | undefined,
): HarpiaRole | null {
  if (!role) {
    return "Leader";
  }
  if (LEADER_ALIASES.has(role)) {
    return "Leader";
  }
  if (role === "Engineer" || role === "Overseer") {
    return role;
  }
  return null;
}

export function getUserRole(
  user: { role?: string } | null | undefined,
): HarpiaRole {
  return normalizeRole(user?.role) ?? "Leader";
}

export function hasPermission(
  user: { role?: string } | null | undefined,
  permission: HarpiaPermission,
): boolean {
  if (!user) {
    return false;
  }
  const role = normalizeRole(user.role);
  if (!role) {
    return false;
  }
  return ROLE_PERMISSIONS[role].has(permission);
}

export function canManageIntegrations(
  user: { role?: string } | null | undefined,
): boolean {
  return hasPermission(user, "manageIntegrations");
}

export function canManageAgents(
  user: { role?: string } | null | undefined,
): boolean {
  return hasPermission(user, "manageAgents");
}

export function canViewAudit(
  user: { role?: string } | null | undefined,
): boolean {
  return hasPermission(user, "viewAudit");
}

export function canConfigurePlans(
  user: { role?: string } | null | undefined,
): boolean {
  return hasPermission(user, "configurePlans");
}

/** @deprecated Use canManageIntegrations or canManageAgents instead. */
export function isEngineer(
  user: { role?: string } | null | undefined,
): boolean {
  return normalizeRole(user?.role) === "Engineer";
}
