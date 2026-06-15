import { describe, expect, it } from "vitest";
import { DEV_TENANT, selectDefaultTenant } from "./auth";

describe("selectDefaultTenant", () => {
  it("returns the tenant when exactly one is available", () => {
    const tenant = { id: "tenant-1", name: "Dev" };
    expect(selectDefaultTenant([tenant])).toEqual(tenant);
  });

  it("returns null when no tenants are available", () => {
    expect(selectDefaultTenant([])).toBeNull();
  });

  it("returns null when multiple tenants require an explicit choice", () => {
    expect(
      selectDefaultTenant([
        { id: "tenant-1", name: "Dev" },
        { id: "tenant-2", name: "Other" },
      ]),
    ).toBeNull();
  });

  it("maps the dev tenant constant for dev login flows", () => {
    expect(DEV_TENANT).toEqual({
      id: "dev",
      name: "Dev Workspace",
      themeKey: "default",
    });
  });
});
