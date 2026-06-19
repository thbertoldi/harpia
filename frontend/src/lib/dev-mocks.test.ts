import { describe, expect, it } from "vitest";
import { allowsMockFallback } from "$lib/dev-mocks";

describe("allowsMockFallback", () => {
  it("is disabled unless explicitly enabled in dev", () => {
    expect(allowsMockFallback()).toBe(false);
  });
});
