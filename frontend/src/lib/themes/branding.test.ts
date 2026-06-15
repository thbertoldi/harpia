import { describe, expect, it } from "vitest";
import { brandName } from "./branding";

describe("branding", () => {
  it("maps theme packages to product names", () => {
    expect(brandName("default", "en")).toBe("Harpia");
    expect(brandName("aiuna", "en")).toBe("AIUNA");
    expect(brandName("default", "pt-BR")).toBe("Harpia");
    expect(brandName("aiuna", "pt-BR")).toBe("AIUNA");
  });
});
