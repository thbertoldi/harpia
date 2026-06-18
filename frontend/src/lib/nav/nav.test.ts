import { describe, expect, it } from "vitest";
import { isNavSectionActive } from "./active";
import {
  filterNavSections,
  navSectionDefs,
  resolveNavSections,
} from "./sections";

describe("nav sections registry", () => {
  it("includes base sections for all roles", () => {
    expect(filterNavSections("Leader")).toHaveLength(9);
    expect(filterNavSections(undefined)).toHaveLength(5);
  });

  it("shows integrations to leaders and engineers", () => {
    expect(
      filterNavSections("Leader").some((d) => d.href === "/integrations"),
    ).toBe(true);
    expect(
      filterNavSections("Engineer").some((d) => d.href === "/integrations"),
    ).toBe(true);
    expect(
      filterNavSections("Overseer").some((d) => d.href === "/integrations"),
    ).toBe(false);
  });

  it("shows audit log to leader, overseer, and engineer", () => {
    expect(filterNavSections("Leader").some((d) => d.href === "/audit")).toBe(
      true,
    );
    expect(filterNavSections("Overseer").some((d) => d.href === "/audit")).toBe(
      true,
    );
    expect(filterNavSections("Engineer").some((d) => d.href === "/audit")).toBe(
      true,
    );
  });

  it("shows agents catalog to leaders and engineers", () => {
    expect(filterNavSections("Leader").some((d) => d.href === "/agents")).toBe(
      true,
    );
    expect(
      filterNavSections("Engineer").some((d) => d.href === "/agents"),
    ).toBe(true);
    expect(
      filterNavSections("Overseer").some((d) => d.href === "/agents"),
    ).toBe(false);
  });

  it("hides permission-gated sections from unknown roles", () => {
    expect(filterNavSections("member")).toHaveLength(5);
    expect(filterNavSections("member").some((d) => d.href === "/audit")).toBe(
      false,
    );
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
      "t:nav.tasks",
      "t:nav.ongoing",
      "t:nav.oversee",
      "t:nav.elicitations",
      "t:nav.plans",
      "t:nav.integrations",
      "t:nav.audit",
      "t:nav.agents",
      "t:nav.settings",
    ]);
  });
});

describe("isNavSectionActive", () => {
  it("highlights home for /tasks but not /tasks/ongoing", () => {
    expect(isNavSectionActive("/", "/tasks")).toBe(true);
    expect(isNavSectionActive("/", "/tasks/ongoing")).toBe(false);
    expect(isNavSectionActive("/tasks/ongoing", "/tasks/ongoing")).toBe(true);
  });

  it("matches exact paths", () => {
    expect(isNavSectionActive("/oversee", "/oversee")).toBe(true);
    expect(isNavSectionActive("/oversee", "/audit")).toBe(false);
    expect(isNavSectionActive("/plans", "/plans")).toBe(true);
  });
});

describe("navSectionDefs", () => {
  it("lists role-gated sections after base sections", () => {
    expect(navSectionDefs.map((d) => d.href)).toEqual([
      "/",
      "/tasks/ongoing",
      "/oversee",
      "/elicitations",
      "/plans",
      "/integrations",
      "/audit",
      "/agents",
      "/settings",
    ]);
  });
});
