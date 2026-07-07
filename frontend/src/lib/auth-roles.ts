export type HarpiaRole = "Leader" | "Overseer" | "Engineer";

export type HarpiaPermission =
  | "manageIntegrations"
  | "manageAgents"
  | "viewAudit"
  | "configurePlans"
  | "manageTenantSettings";

type RoleBearingUser = (object & { role?: string }) | null | undefined;

const ROLE_PERMISSIONS: Record<HarpiaRole, ReadonlySet<HarpiaPermission>> = {
  Leader: new Set([
    "manageIntegrations",
    "manageAgents",
    "viewAudit",
    "configurePlans",
    "manageTenantSettings",
  ]),
  Engineer: new Set([
    "manageIntegrations",
    "manageAgents",
    "viewAudit",
    "configurePlans",
    "manageTenantSettings",
  ]),
  Overseer: new Set(["viewAudit"]),
};

const LEADER_ALIASES = new Set(["Leader", "admin", "owner"]);

/** Maps an explicit role string to a permission-bearing product role, if trusted. */
export function resolvePermissionRole(
  role: string | undefined,
): HarpiaRole | null {
  if (!role) {
    return null;
  }
  if (LEADER_ALIASES.has(role)) {
    return "Leader";
  }
  if (role === "Engineer" || role === "Overseer") {
    return role;
  }
  return null;
}

/** Display label fallback; does not grant permissions by itself. */
export function getUserRole(user: RoleBearingUser): HarpiaRole {
  return resolvePermissionRole(user?.role) ?? "Leader";
}

export function roleFromBackendRoles(
  roles: string[] | undefined,
): string | undefined {
  if (!roles?.length) {
    return undefined;
  }
  const role = roles[0];
  if (LEADER_ALIASES.has(role)) {
    return "Leader";
  }
  if (role === "Engineer" || role === "Overseer") {
    return role;
  }
  return role;
}

export function hasPermission(
  user: RoleBearingUser,
  permission: HarpiaPermission,
): boolean {
  if (!user) {
    return false;
  }
  const role = resolvePermissionRole(user.role);
  if (!role) {
    return false;
  }
  return ROLE_PERMISSIONS[role].has(permission);
}

export function canManageIntegrations(user: RoleBearingUser): boolean {
  return hasPermission(user, "manageIntegrations");
}

export function canManageAgents(user: RoleBearingUser): boolean {
  return hasPermission(user, "manageAgents");
}

export function canViewAudit(user: RoleBearingUser): boolean {
  return hasPermission(user, "viewAudit");
}

export function canConfigurePlans(user: RoleBearingUser): boolean {
  return hasPermission(user, "configurePlans");
}

export function canManageTenantSettings(user: RoleBearingUser): boolean {
  return hasPermission(user, "manageTenantSettings");
}

/** @deprecated Use canManageIntegrations or canManageAgents instead. */
export function isEngineer(user: RoleBearingUser): boolean {
  return resolvePermissionRole(user?.role) === "Engineer";
}
