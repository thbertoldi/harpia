import { beforeEach, describe, expect, it, vi } from "vitest";
import { loadAgentCatalog } from "$lib/agent-catalog";

const { listAgentTypes, allowsMockFallback } = vi.hoisted(() => ({
  listAgentTypes: vi.fn(),
  allowsMockFallback: vi.fn(() => false),
}));

vi.mock("$lib/rpc", () => ({
  agentClient: {
    listAgentTypes,
  },
}));

vi.mock("$lib/dev-mocks", () => ({
  allowsMockFallback,
}));

describe("loadAgentCatalog", () => {
  beforeEach(() => {
    listAgentTypes.mockReset();
    allowsMockFallback.mockReturnValue(false);
  });

  it("rejects when the API fails without explicit mock fallback", async () => {
    listAgentTypes.mockImplementation(() => {
      throw new Error("network error");
    });

    await expect(loadAgentCatalog()).rejects.toThrow("network error");
  });

  it("falls back to mock catalog when mock fallback is explicitly enabled", async () => {
    allowsMockFallback.mockReturnValue(true);
    listAgentTypes.mockImplementation(() => {
      throw new Error("network error");
    });

    const result = await loadAgentCatalog();

    expect(result.source).toBe("mock");
    expect(result.entries.length).toBeGreaterThan(0);
    expect(result.error).toContain("network error");
  });
});
