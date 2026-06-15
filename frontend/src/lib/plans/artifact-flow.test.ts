import { describe, expect, it } from "vitest";
import {
  buildLinearDagEdges,
  extractArtifactTypeKey,
  formatArtifactTypeLabel,
  orderPlanStepsLinear,
} from "$lib/plans/artifact-flow";
import { mockWeeklyNewsletterLinkedInTemplate } from "$lib/plans/plan-template";

describe("artifact flow helpers", () => {
  const template = mockWeeklyNewsletterLinkedInTemplate();

  it("extracts short artifact type keys from proto ids", () => {
    expect(extractArtifactTypeKey("harpia.artifacts.v1.DateRange")).toBe(
      "DateRange",
    );
    expect(extractArtifactTypeKey("harpia.artifacts.v1.NewsList")).toBe(
      "NewsList",
    );
  });

  it("formats known artifact types to human labels", () => {
    expect(formatArtifactTypeLabel("harpia.artifacts.v1.DateRange", "en")).toBe(
      "Date Range",
    );
    expect(formatArtifactTypeLabel("harpia.artifacts.v1.NewsList", "en")).toBe(
      "News List",
    );
    expect(formatArtifactTypeLabel("harpia.artifacts.v1.TextDraft", "en")).toBe(
      "Text Draft",
    );
    expect(
      formatArtifactTypeLabel("harpia.artifacts.v1.LinkedInPostDraft", "en"),
    ).toBe("LinkedIn Post Draft");
    expect(
      formatArtifactTypeLabel("harpia.artifacts.v1.PublishConfirmation", "en"),
    ).toBe("Publish Confirmation");
  });

  it("orders weekly newsletter steps into a linear chain", () => {
    const ordered = orderPlanStepsLinear(template.steps, template.edges);
    expect(ordered.map((step) => step.key)).toEqual([
      "fetch-news",
      "write-draft",
      "adapt-for-linkedin",
      "publish-linkedin",
    ]);
  });

  it("builds labeled edges for the artifact flow diagram", () => {
    const dagEdges = buildLinearDagEdges(template.steps, template.edges, "en");
    expect(dagEdges).toHaveLength(3);
    expect(dagEdges.map((edge) => edge.artifactLabel)).toEqual([
      "News List",
      "Text Draft",
      "LinkedIn Post Draft",
    ]);
  });
});
