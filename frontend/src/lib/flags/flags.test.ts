import { describe, expect, it, vi, beforeEach } from "vitest";

const envMock = vi.hoisted(() => ({ env: {} as Record<string, string> }));
vi.mock("$env/dynamic/public", () => envMock);

describe("hasFeature", () => {
  beforeEach(() => {
    envMock.env = {};
  });

  it("returns false for uxRealignment.m1 when env var unset", async () => {
    const { hasFeature } = await import("./index");
    expect(hasFeature("uxRealignment.m1")).toBe(false);
  });

  it("returns true for uxRealignment.m1 when env var is 'true'", async () => {
    envMock.env.PUBLIC_FEATURE_UX_REALIGNMENT_M1 = "true";
    vi.resetModules();
    const { hasFeature } = await import("./index");
    expect(hasFeature("uxRealignment.m1")).toBe(true);
  });

  it("returns false for uxRealignment.m1 when env var is non-'true' string", async () => {
    envMock.env.PUBLIC_FEATURE_UX_REALIGNMENT_M1 = "1";
    vi.resetModules();
    const { hasFeature } = await import("./index");
    expect(hasFeature("uxRealignment.m1")).toBe(false);
  });
});
