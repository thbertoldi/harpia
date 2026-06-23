import { describe, expect, it, vi, afterEach } from "vitest";
import { prefersReducedMotion } from "./reducedMotion";

describe("prefersReducedMotion", () => {
  const originalMatchMedia = globalThis.matchMedia;
  afterEach(() => {
    globalThis.matchMedia = originalMatchMedia;
  });

  it("returns false when matchMedia is unavailable (SSR)", () => {
    // @ts-expect-error simulate SSR
    delete globalThis.matchMedia;
    expect(prefersReducedMotion()).toBe(false);
  });

  it("returns true when prefers-reduced-motion: reduce matches", () => {
    globalThis.matchMedia = vi.fn().mockReturnValue({ matches: true }) as unknown as typeof matchMedia;
    expect(prefersReducedMotion()).toBe(true);
  });

  it("returns false when reduce does not match", () => {
    globalThis.matchMedia = vi.fn().mockReturnValue({ matches: false }) as unknown as typeof matchMedia;
    expect(prefersReducedMotion()).toBe(false);
  });
});
