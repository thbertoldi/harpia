import { describe, expect, it } from "vitest";
import { PublishApprovalMode } from "$lib/gen/harpia/plans/v1/plans_pb";
import {
  buildDefaultLinkedInInputValues,
  materializeLinkedInInputValues,
} from "./linkedin-template-inputs";
import { parameterValuesJson, parseParameterValuesJson } from "./template-inputs";

describe("LinkedIn template input materialization", () => {
  it("turns theme, language, tone, and date range into seed artifacts", () => {
    const values = buildDefaultLinkedInInputValues(
      new Date("2026-06-29T12:00:00Z"),
    );
    values.theme = "retail growth";
    values.language = "en-US";
    values.tone = "executive and direct";
    values.audience = "founders";
    values.topicsToAvoid = "rumors";

    const result = materializeLinkedInInputValues(values, {
      "fetch-news": "rss-installation",
      "write-draft": "writer-agent",
      "adapt-for-linkedin": "linkedin-agent",
      "publish-linkedin": "publisher",
    });

    const dateRange = result.seedArtifacts.find(
      (seed) => seed.inputName === "date_range",
    );
    expect(dateRange).toBeDefined();
    expect(JSON.parse(dateRange!.literalJson)).toEqual({
      startDate: "2026-06-23",
      endDate: "2026-06-29",
    });

    const preferences = result.seedArtifacts.find(
      (seed) => seed.inputName === "harpia.internal.ContentPreferences",
    );
    expect(preferences).toBeDefined();
    expect(JSON.parse(preferences!.literalJson)).toEqual({
      topic: "retail growth",
      language: "en-US",
      tone: "executive and direct",
      audience: "founders",
      topics_to_avoid: "rumors",
    });
    expect(result.behaviorPolicies.publishApprovalMode).toBe(
      PublishApprovalMode.REQUIRE_APPROVAL,
    );
  });

  it("uses explicit source group for fetch-news when selected", () => {
    const values = buildDefaultLinkedInInputValues(
      new Date("2026-06-29T12:00:00Z"),
    );
    values.sourceGroupInstallationId = "rss-selected";

    const result = materializeLinkedInInputValues(values, {
      "fetch-news": "rss-fallback",
      "write-draft": "writer-agent",
      "adapt-for-linkedin": "linkedin-agent",
      "publish-linkedin": "publisher",
    });

    expect(
      result.slotBindings.find((binding) => binding.stepKey === "fetch-news")
        ?.executorInstallationId,
    ).toBe("rss-selected");
  });

  it("serializes and parses parameter values for persistence", () => {
    const values = buildDefaultLinkedInInputValues(
      new Date("2026-06-29T12:00:00Z"),
    );
    values.theme = "retail growth";
    values.language = "es";
    values.audience = "operators";
    values.sourceGroupInstallationId = "rss-selected";
    values.approvalMode = "auto_publish";

    const json = parameterValuesJson(values);

    expect(JSON.parse(json)).toEqual({
      theme: "retail growth",
      language: "es",
      tone: "analytical, concise, and practical",
      audience: "operators",
      topics_to_avoid: "",
      source_group: "rss-selected",
      date_range: {
        startDate: "2026-06-23",
        endDate: "2026-06-29",
      },
      approval_mode: "auto_publish",
    });
    expect(parseParameterValuesJson(json)).toEqual(JSON.parse(json));
    expect(parseParameterValuesJson("not-json")).toEqual({});
  });
});
