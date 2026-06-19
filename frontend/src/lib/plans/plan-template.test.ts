import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  loadPlanTemplate,
  mockWeeklyNewsletterLinkedInTemplate,
  WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_ID,
  WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_KEY,
  isPlanTemplateUuid,
} from "$lib/plans/plan-template";

const { getPlanTemplate, getPlanTemplateByKey, allowsMockFallback } =
  vi.hoisted(() => ({
    getPlanTemplate: vi.fn(),
    getPlanTemplateByKey: vi.fn(),
    allowsMockFallback: vi.fn(() => false),
  }));

vi.mock("$lib/rpc", () => ({
  planClient: {
    getPlanTemplate,
    getPlanTemplateByKey,
  },
}));

vi.mock("$lib/dev-mocks", () => ({
  allowsMockFallback,
}));

describe("plan template loader", () => {
  beforeEach(() => {
    getPlanTemplate.mockReset();
    getPlanTemplateByKey.mockReset();
    allowsMockFallback.mockReturnValue(false);
  });

  it("detects UUID template identifiers", () => {
    expect(isPlanTemplateUuid(WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_ID)).toBe(
      true,
    );
    expect(isPlanTemplateUuid(WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_KEY)).toBe(
      false,
    );
  });

  it("returns seeded mock template with four steps", () => {
    const template = mockWeeklyNewsletterLinkedInTemplate();
    expect(template.key).toBe(WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_KEY);
    expect(template.steps).toHaveLength(4);
    expect(template.edges).toHaveLength(3);
  });

  it("localizes seeded template content for pt-BR", () => {
    const template = mockWeeklyNewsletterLinkedInTemplate("pt-BR");
    expect(template.name).toBe("Newsletter Semanal (LinkedIn)");
    expect(template.steps[1]?.title).toBe("Escrever Rascunho");
  });

  it("loads template by key from the API", async () => {
    const apiTemplate = mockWeeklyNewsletterLinkedInTemplate();
    getPlanTemplateByKey.mockResolvedValue({ planTemplate: apiTemplate });

    const result = await loadPlanTemplate(
      WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_KEY,
    );

    expect(getPlanTemplateByKey).toHaveBeenCalledWith({
      key: WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_KEY,
    });
    expect(result.source).toBe("api");
    expect(result.template.key).toBe(WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_KEY);
  });

  it("loads template by id from the API", async () => {
    const apiTemplate = mockWeeklyNewsletterLinkedInTemplate();
    getPlanTemplate.mockResolvedValue({ planTemplate: apiTemplate });

    const result = await loadPlanTemplate(
      WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_ID,
    );

    expect(getPlanTemplate).toHaveBeenCalledWith({
      planTemplateId: WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_ID,
    });
    expect(result.source).toBe("api");
  });

  it("rejects when the API fails without explicit mock fallback", async () => {
    getPlanTemplateByKey.mockRejectedValue(new Error("network error"));

    await expect(
      loadPlanTemplate(WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_KEY),
    ).rejects.toThrow("network error");
  });

  it("falls back to mock data when mock fallback is explicitly enabled", async () => {
    allowsMockFallback.mockReturnValue(true);
    getPlanTemplateByKey.mockRejectedValue(new Error("network error"));

    const result = await loadPlanTemplate(
      WEEKLY_NEWSLETTER_LINKEDIN_TEMPLATE_KEY,
    );

    expect(result.source).toBe("mock");
    expect(result.template.steps).toHaveLength(4);
    expect(result.error).toContain("network error");
  });
});
