import { describe, expect, it } from "vitest";
import {
  linkedInInputValuesFromParameterValuesJson,
  materializeLinkedInInputValues,
} from "./linkedin-template-inputs";
import { parameterValuesJson, type LinkedInTemplateInputValues } from "./template-inputs";

function values(): LinkedInTemplateInputValues {
  return {
    theme: "AI",
    language: "pt-BR",
    tone: "practical",
    audience: "founders",
    topicsToAvoid: "rumors",
    sourceGroupInstallationIds: ["rss-tech", "rss-business"],
    aggregateSourceGroupInstallationId: "rss-aggregate",
    dateRange: { startDate: "2026-06-01", endDate: "2026-07-01" },
    approvalMode: "require_approval",
  };
}

describe("LinkedIn template source groups", () => {
  it("serializes selected source groups as a list", () => {
    expect(JSON.parse(parameterValuesJson(values()))).toMatchObject({
      source_groups: ["rss-tech", "rss-business"],
    });
  });

  it("parses source_groups with singular source_group fallback", () => {
    expect(
      linkedInInputValuesFromParameterValuesJson(
        '{"source_groups":["rss-tech","rss-business"]}',
      ).sourceGroupInstallationIds,
    ).toEqual(["rss-tech", "rss-business"]);
    expect(
      linkedInInputValuesFromParameterValuesJson(
        '{"source_group":"rss-legacy"}',
      ).sourceGroupInstallationIds,
    ).toEqual(["rss-legacy"]);
  });

  it("materializes one fetch-news SlotBinding using the aggregate installation", () => {
    const materialized = materializeLinkedInInputValues(values(), {
      "write-draft": "writer",
    });
    const fetchBindings = materialized.slotBindings.filter(
      (binding) => binding.stepKey === "fetch-news",
    );
    expect(fetchBindings).toHaveLength(1);
    expect(fetchBindings[0].executorInstallationId).toBe("rss-aggregate");
  });
});
