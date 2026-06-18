import { describe, expect, it } from "vitest";
import {
  canConfigurePlans,
  canManageAgents,
  canManageIntegrations,
  canManageTenantSettings,
  canViewAudit,
  getUserRole,
  hasPermission,
  isEngineer,
  resolvePermissionRole,
  roleFromBackendRoles,
} from "./auth-roles";

describe("resolvePermissionRole", () => {
  it("returns null when role is missing", () => {
    expect(resolvePermissionRole(undefined)).toBeNull();
    expect(resolvePermissionRole("")).toBeNull();
  });

  it("maps trusted Leader aliases", () => {
    expect(resolvePermissionRole("Leader")).toBe("Leader");
    expect(resolvePermissionRole("admin")).toBe("Leader");
    expect(resolvePermissionRole("owner")).toBe("Leader");
  });

  it("preserves explicit product roles", () => {
    expect(resolvePermissionRole("Engineer")).toBe("Engineer");
    expect(resolvePermissionRole("Overseer")).toBe("Overseer");
  });

  it("returns null for unknown explicit roles", () => {
    expect(resolvePermissionRole("member")).toBeNull();
    expect(resolvePermissionRole("guest")).toBeNull();
  });
});

describe("roleFromBackendRoles", () => {
  it("maps backend admin to Leader", () => {
    expect(roleFromBackendRoles(["admin"])).toBe("Leader");
  });

  it("returns undefined when backend sends no roles", () => {
    expect(roleFromBackendRoles(undefined)).toBeUndefined();
    expect(roleFromBackendRoles([])).toBeUndefined();
  });
});

describe("permission matrix", () => {
  it("grants Leader full tenant-scoped configuration access", () => {
    const leader = { role: "Leader" };
    expect(canManageIntegrations(leader)).toBe(true);
    expect(canManageAgents(leader)).toBe(true);
    expect(canViewAudit(leader)).toBe(true);
    expect(canConfigurePlans(leader)).toBe(true);
    expect(canManageTenantSettings(leader)).toBe(true);
  });

  it("grants Engineer tenant settings access", () => {
    const engineer = { role: "Engineer" };
    expect(canManageIntegrations(engineer)).toBe(true);
    expect(canManageAgents(engineer)).toBe(true);
    expect(canViewAudit(engineer)).toBe(true);
    expect(canConfigurePlans(engineer)).toBe(true);
    expect(canManageTenantSettings(engineer)).toBe(true);
  });

  it("keeps Overseer limited to audit visibility", () => {
    const overseer = { role: "Overseer" };
    expect(canViewAudit(overseer)).toBe(true);
    expect(canManageIntegrations(overseer)).toBe(false);
    expect(canManageAgents(overseer)).toBe(false);
    expect(canConfigurePlans(overseer)).toBe(false);
  });

  it("denies unknown explicit roles gated permissions", () => {
    const member = { role: "member" };
    expect(hasPermission(member, "manageIntegrations")).toBe(false);
    expect(hasPermission(member, "manageAgents")).toBe(false);
    expect(hasPermission(member, "viewAudit")).toBe(false);
    expect(hasPermission(member, "configurePlans")).toBe(false);
  });

  it("denies permissions when user is missing", () => {
    expect(canConfigurePlans(null)).toBe(false);
    expect(canConfigurePlans(undefined)).toBe(false);
  });

  it("denies permissions when role is missing from authenticated user", () => {
    const cookieUser = {
      sub: "user-1",
      email: "user@harpia.local",
      name: "User",
    };
    expect(canConfigurePlans(cookieUser)).toBe(false);
    expect(canConfigurePlans({})).toBe(false);
    expect(canManageIntegrations(cookieUser)).toBe(false);
    expect(canManageAgents(cookieUser)).toBe(false);
    expect(canViewAudit(cookieUser)).toBe(false);
  });
});

describe("route guard expectations", () => {
  it("blocks Overseer from integrations, agents, and plan configuration", () => {
    const overseer = { role: "Overseer" };
    expect(canManageIntegrations(overseer)).toBe(false);
    expect(canManageAgents(overseer)).toBe(false);
    expect(canConfigurePlans(overseer)).toBe(false);
    expect(canViewAudit(overseer)).toBe(true);
  });

  it("blocks cookie users without role from gated routes", () => {
    const userWithoutRole = {
      sub: "zitadel-subject",
      email: "admin@harpia.local",
      name: "Admin",
    };
    expect(canConfigurePlans(userWithoutRole)).toBe(false);
    expect(canManageAgents(userWithoutRole)).toBe(false);
  });
});

describe("getUserRole", () => {
  it("defaults missing roles to Leader for display only", () => {
    expect(getUserRole(undefined)).toBe("Leader");
    expect(getUserRole({})).toBe("Leader");
    expect(canConfigurePlans({})).toBe(false);
  });

  it("labels unknown explicit roles as Leader without granting access", () => {
    expect(getUserRole({ role: "Admin" })).toBe("Leader");
    expect(canConfigurePlans({ role: "Admin" })).toBe(false);
  });
});

describe("isEngineer", () => {
  it("matches only the Engineer role", () => {
    expect(isEngineer({ role: "Engineer" })).toBe(true);
    expect(isEngineer({ role: "Leader" })).toBe(false);
    expect(isEngineer({ role: "Overseer" })).toBe(false);
    expect(isEngineer(null)).toBe(false);
  });
});
