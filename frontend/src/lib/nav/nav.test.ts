import { describe, expect, it } from "vitest";
import { isNavSectionActive } from "./active";
import {
  filterNavSections,
  navSectionDefs,
  resolveNavSections,
} from "./sections";

describe("nav sections registry", () => {
  it("includes base sections for all roles", () => {
    // 4 base ("all") sections + 4 permission-gated /admin/* sections.
    expect(filterNavSections("Leader")).toHaveLength(8);
    expect(filterNavSections(undefined)).toHaveLength(4);
  });

  it("shows integrations to leaders and engineers", () => {
    expect(
      filterNavSections("Leader").some((d) => d.href === "/admin/integrations"),
    ).toBe(true);
    expect(
      filterNavSections("Engineer").some(
        (d) => d.href === "/admin/integrations",
      ),
    ).toBe(true);
    expect(
      filterNavSections("Overseer").some(
        (d) => d.href === "/admin/integrations",
      ),
    ).toBe(false);
  });

  it("shows audit log to leader, overseer, and engineer", () => {
    expect(
      filterNavSections("Leader").some((d) => d.href === "/admin/audit"),
    ).toBe(true);
    expect(
      filterNavSections("Overseer").some((d) => d.href === "/admin/audit"),
    ).toBe(true);
    expect(
      filterNavSections("Engineer").some((d) => d.href === "/admin/audit"),
    ).toBe(true);
  });

  it("shows agents catalog to leaders and engineers", () => {
    expect(
      filterNavSections("Leader").some((d) => d.href === "/admin/agents"),
    ).toBe(true);
    expect(
      filterNavSections("Engineer").some((d) => d.href === "/admin/agents"),
    ).toBe(true);
    expect(
      filterNavSections("Overseer").some((d) => d.href === "/admin/agents"),
    ).toBe(false);
  });

  it("hides permission-gated sections from unknown roles", () => {
    expect(filterNavSections("member")).toHaveLength(4);
    expect(
      filterNavSections("member").some((d) => d.href === "/admin/audit"),
    ).toBe(false);
  });

  it("shows plan catalog to all authenticated roles", () => {
    expect(filterNavSections("Leader").some((d) => d.href === "/plans")).toBe(
      true,
    );
    expect(filterNavSections("Engineer").some((d) => d.href === "/plans")).toBe(
      true,
    );
  });

  it("resolves labels via translate callback", () => {
    const labels = resolveNavSections("Leader", (key) => `t:${key}`).map(
      (s) => s.label,
    );
    expect(labels).toEqual([
      "t:nav.needsYou",
      "t:nav.plans",
      "t:nav.artifacts",
      "t:nav.executions",
      "t:nav.integrations",
      "t:nav.audit",
      "t:nav.agents",
      "t:nav.settings",
    ]);
  });
});

describe("isNavSectionActive", () => {
  it("matches exact paths only (no general nesting)", () => {
    expect(isNavSectionActive("/inbox", "/inbox")).toBe(true);
    expect(isNavSectionActive("/plans", "/plans")).toBe(true);
    expect(isNavSectionActive("/plans", "/plans/abc")).toBe(false);
    expect(isNavSectionActive("/inbox", "/admin/audit")).toBe(false);
  });

  it("keeps the home → /tasks special case", () => {
    expect(isNavSectionActive("/", "/tasks")).toBe(true);
    expect(isNavSectionActive("/", "/tasks/ongoing")).toBe(false);
  });
});

describe("navSectionDefs", () => {
  it("lists role-gated sections after base sections", () => {
    expect(navSectionDefs.map((d) => d.href)).toEqual([
      "/inbox",
      "/plans",
      "/artifacts",
      "/plans/executions",
      "/admin/integrations",
      "/admin/audit",
      "/admin/agents",
      "/admin/settings",
    ]);
  });
});
