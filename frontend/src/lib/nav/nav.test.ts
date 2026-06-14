import { describe, expect, it } from "vitest";
import { isNavSectionActive } from "./active";
import {
  filterNavSections,
  navSectionDefs,
  resolveNavSections,
} from "./sections";

describe("nav sections registry", () => {
  it("includes base sections for all roles", () => {
    expect(filterNavSections("Leader")).toHaveLength(3);
    expect(filterNavSections(undefined)).toHaveLength(3);
  });

  it("shows integrations to engineers only", () => {
    expect(filterNavSections("Engineer")).toHaveLength(5);
    expect(
      filterNavSections("Engineer").some((d) => d.href === "/integrations"),
    ).toBe(true);
    expect(filterNavSections("Leader")).toHaveLength(3);
  });

  it("shows audit log to overseer and engineer", () => {
    expect(filterNavSections("Overseer")).toHaveLength(4);
    expect(filterNavSections("Engineer")).toHaveLength(5);
    expect(filterNavSections("Leader")).toHaveLength(3);
  });

  it("resolves labels via translate callback", () => {
    const labels = resolveNavSections("Leader", (key) => `t:${key}`).map(
      (s) => s.label,
    );
    expect(labels).toEqual(["t:nav.tasks", "t:nav.ongoing", "t:nav.oversee"]);
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
  });
});

describe("navSectionDefs", () => {
  it("lists role-gated sections after base sections", () => {
    expect(navSectionDefs.map((d) => d.href)).toEqual([
      "/",
      "/tasks/ongoing",
      "/oversee",
      "/integrations",
      "/audit",
    ]);
  });
});
