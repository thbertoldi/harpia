import { beforeEach, describe, expect, it, vi } from "vitest";
import { loadPlanExecutions } from "$lib/plans/plan-execution";

const { listPlanExecutions, getTenant, getSession, allowsMockFallback } =
  vi.hoisted(() => ({
    listPlanExecutions: vi.fn(),
    getTenant: vi.fn(() => ({ id: "tenant-1" })),
    getSession: vi.fn(() => null),
    allowsMockFallback: vi.fn(() => false),
  }));

vi.mock("$lib/rpc", () => ({
  planClient: {
    listPlanExecutions,
  },
}));

vi.mock("$lib/auth", () => ({
  getTenant,
  getSession,
  requireTenantId: () => "tenant-1",
}));

vi.mock("$lib/dev-mocks", () => ({
  allowsMockFallback,
}));

describe("loadPlanExecutions", () => {
  beforeEach(() => {
    listPlanExecutions.mockReset();
    allowsMockFallback.mockReturnValue(false);
    getTenant.mockReturnValue({ id: "tenant-1" });
  });

  it("rejects when the API fails without explicit mock fallback", async () => {
    listPlanExecutions.mockImplementation(() => {
      throw new Error("network error");
    });

    await expect(loadPlanExecutions()).rejects.toThrow("network error");
  });

  it("returns an empty list from the API without substituting mock data", async () => {
    listPlanExecutions.mockImplementation(async function* () {
      yield { planExecutions: [] };
    });

    const result = await loadPlanExecutions();

    expect(result.source).toBe("api");
    expect(result.executions).toEqual([]);
  });

  it("falls back to mock executions when mock fallback is explicitly enabled", async () => {
    allowsMockFallback.mockReturnValue(true);
    listPlanExecutions.mockImplementation(() => {
      throw new Error("network error");
    });

    const result = await loadPlanExecutions();

    expect(result.source).toBe("mock");
    expect(result.executions.length).toBeGreaterThan(0);
    expect(result.error).toContain("network error");
  });
});
