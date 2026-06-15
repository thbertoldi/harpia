import { describe, expect, it } from "vitest";
import {
  canConfigurePlans,
  canManageAgents,
  canManageIntegrations,
  canViewAudit,
  getUserRole,
  hasPermission,
  isEngineer,
  normalizeRole,
} from "./auth-roles";

describe("normalizeRole", () => {
  it("maps unset and backend admin aliases to Leader", () => {
    expect(normalizeRole(undefined)).toBe("Leader");
    expect(normalizeRole("")).toBe("Leader");
    expect(normalizeRole("Leader")).toBe("Leader");
    expect(normalizeRole("admin")).toBe("Leader");
    expect(normalizeRole("owner")).toBe("Leader");
  });

  it("preserves explicit product roles", () => {
    expect(normalizeRole("Engineer")).toBe("Engineer");
    expect(normalizeRole("Overseer")).toBe("Overseer");
  });

  it("returns null for unknown explicit roles", () => {
    expect(normalizeRole("member")).toBeNull();
    expect(normalizeRole("guest")).toBeNull();
  });
});

describe("permission matrix", () => {
  it("grants Leader full tenant-scoped configuration access", () => {
    const leader = { role: "Leader" };
    expect(canManageIntegrations(leader)).toBe(true);
    expect(canManageAgents(leader)).toBe(true);
    expect(canViewAudit(leader)).toBe(true);
    expect(canConfigurePlans(leader)).toBe(true);
  });

  it("grants Engineer the same configuration access as before", () => {
    const engineer = { role: "Engineer" };
    expect(canManageIntegrations(engineer)).toBe(true);
    expect(canManageAgents(engineer)).toBe(true);
    expect(canViewAudit(engineer)).toBe(true);
    expect(canConfigurePlans(engineer)).toBe(true);
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
});

describe("getUserRole", () => {
  it("defaults unset roles to Leader for display", () => {
    expect(getUserRole(undefined)).toBe("Leader");
    expect(getUserRole({})).toBe("Leader");
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
