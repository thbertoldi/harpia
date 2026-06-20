import { describe, expect, it } from "vitest";
import { resolveNavSectionsM1 } from "./sections-m1";

const translate = (key: string) => `t:${key}`;

describe("resolveNavSectionsM1 — operator persona", () => {
  it("returns the operator sections regardless of role", () => {
    const hrefs = resolveNavSectionsM1("operator", "Leader", translate).map(
      (s) => s.href,
    );
    expect(hrefs).toEqual(["/inbox", "/discover", "/new"]);
  });

  it("returns operator sections even for an unknown role", () => {
    const hrefs = resolveNavSectionsM1("operator", "member", translate).map(
      (s) => s.href,
    );
    expect(hrefs).toEqual(["/inbox", "/discover", "/new"]);
  });

  it("translates labels via the callback", () => {
    const labels = resolveNavSectionsM1("operator", "Leader", translate).map(
      (s) => s.label,
    );
    expect(labels).toEqual(["t:nav.needsYou", "t:nav.discover", "t:nav.newPlan"]);
  });
});

describe("resolveNavSectionsM1 — admin persona", () => {
  it("returns all admin sections for Leader", () => {
    const hrefs = resolveNavSectionsM1("admin", "Leader", translate).map(
      (s) => s.href,
    );
    expect(hrefs).toEqual([
      "/admin/integrations",
      "/admin/agents",
      "/admin/audit",
      "/admin/settings",
    ]);
  });

  it("returns all admin sections for Engineer", () => {
    const hrefs = resolveNavSectionsM1("admin", "Engineer", translate).map(
      (s) => s.href,
    );
    expect(hrefs).toEqual([
      "/admin/integrations",
      "/admin/agents",
      "/admin/audit",
      "/admin/settings",
    ]);
  });

  it("returns only audit for Overseer (viewAudit only)", () => {
    const hrefs = resolveNavSectionsM1("admin", "Overseer", translate).map(
      (s) => s.href,
    );
    expect(hrefs).toEqual(["/admin/audit"]);
  });

  it("returns empty array for unknown role", () => {
    expect(resolveNavSectionsM1("admin", "member", translate)).toEqual([]);
  });
});
