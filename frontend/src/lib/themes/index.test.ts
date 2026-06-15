import { describe, expect, it } from "vitest";
import { PLATFORM_DEFAULT_THEME, isThemeName, resolveTheme } from "./index";

describe("themes", () => {
  it("recognizes registered theme names", () => {
    expect(isThemeName("aiuna")).toBe(true);
    expect(isThemeName("default")).toBe(true);
    expect(isThemeName("tenant-base")).toBe(true);
    expect(isThemeName("unknown")).toBe(false);
  });

  it("uses tenant theme key when assigned", () => {
    expect(resolveTheme("default")).toBe("default");
    expect(resolveTheme("aiuna")).toBe("aiuna");
    expect(resolveTheme("tenant-base")).toBe("tenant-base");
  });

  it("falls back to platform default when tenant theme is missing or invalid", () => {
    expect(resolveTheme(undefined)).toBe(PLATFORM_DEFAULT_THEME);
    expect(resolveTheme("")).toBe(PLATFORM_DEFAULT_THEME);
    expect(resolveTheme("not-a-theme")).toBe(PLATFORM_DEFAULT_THEME);
  });
});
