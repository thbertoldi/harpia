import { describe, expect, it } from "vitest";
import {
  genericInputInitialValues,
  genericParameterValuesJson,
  requiredInputsSatisfied,
  resolveDateRangePreset,
  selectOptions,
} from "./template-inputs";
import type { TemplateInputParameter } from "$lib/gen/harpia/plans/v1/plans_pb";
import { TemplateInputParameterType } from "$lib/gen/harpia/plans/v1/plans_pb";

function param(
  key: string,
  type: TemplateInputParameterType,
  defaultValueJson = "",
  required = false,
): TemplateInputParameter {
  return {
    $typeName: "harpia.plans.v1.TemplateInputParameter",
    key,
    label: key,
    description: "",
    type,
    required,
    defaultValueJson,
    optionsJson: "",
    runtimeMappings: [],
  } as TemplateInputParameter;
}

describe("genericInputInitialValues", () => {
  it("prefers extracted values, falls back to default_value_json, then empty", () => {
    const params = [
      param("theme", TemplateInputParameterType.TEXT),
      param("language", TemplateInputParameterType.LANGUAGE, '"en-US"'),
      param("tone", TemplateInputParameterType.TEXT),
    ];
    const got = genericInputInitialValues(params, { theme: "retail" });
    expect(got.theme).toBe("retail");
    expect(got.language).toBe("en-US");
    expect(got.tone).toBe("");
  });

  it("preserves date range objects from extracted values", () => {
    const params = [
      param("date_range", TemplateInputParameterType.DATE_RANGE),
    ];
    const dateRange = { startDate: "2026-01-01", endDate: "2026-12-31" };
    const got = genericInputInitialValues(params, { date_range: dateRange });
    expect(got.date_range).toEqual(dateRange);
  });

  it("keeps a date range preset rolling instead of freezing concrete dates", () => {
    const params = [
      param(
        "date_range",
        TemplateInputParameterType.DATE_RANGE,
        '{"preset":"last_7_days"}',
      ),
    ];
    const got = genericInputInitialValues(params, {});
    // The preset is preserved verbatim so a scheduled plan rolls forward each
    // run rather than being pinned to a single week at configuration time.
    expect(got.date_range).toEqual({ preset: "last_7_days" });
  });
});

describe("resolveDateRangePreset", () => {
  it("resolves last_7_days to the previous complete week ending yesterday", () => {
    expect(
      resolveDateRangePreset("last_7_days", new Date("2026-07-01T12:00:00Z")),
    ).toEqual({ startDate: "2026-06-24", endDate: "2026-06-30" });
  });

  it("returns null for unknown presets", () => {
    expect(resolveDateRangePreset("all_time", new Date("2026-07-01T12:00:00Z"))).toBeNull();
  });
});

describe("requiredInputsSatisfied", () => {
  it("returns true when every required param has a non-empty value", () => {
    const params = [
      param("theme", TemplateInputParameterType.TEXT, "", true),
      param("tone", TemplateInputParameterType.TEXT),
    ];
    expect(
      requiredInputsSatisfied(params, { theme: "retail", tone: "" }),
    ).toBe(true);
  });

  it("returns false when a required param is missing or empty", () => {
    const params = [
      param("theme", TemplateInputParameterType.TEXT, "", true),
      param("language", TemplateInputParameterType.LANGUAGE, "", true),
    ];
    expect(requiredInputsSatisfied(params, { theme: "retail" })).toBe(false);
    expect(requiredInputsSatisfied(params, { theme: "", language: "en-US" })).toBe(
      false,
    );
  });

  it("treats a rolling date range preset as satisfied", () => {
    const params = [
      param("date_range", TemplateInputParameterType.DATE_RANGE, "", true),
    ];
    expect(
      requiredInputsSatisfied(params, { date_range: { preset: "last_7_days" } }),
    ).toBe(true);
  });

  it("requires both dates for required date ranges", () => {
    const params = [
      param("date_range", TemplateInputParameterType.DATE_RANGE, "", true),
    ];
    expect(
      requiredInputsSatisfied(params, {
        date_range: { startDate: "2026-01-01", endDate: "" },
      }),
    ).toBe(false);
    expect(
      requiredInputsSatisfied(params, {
        date_range: { startDate: "2026-01-01", endDate: "2026-12-31" },
      }),
    ).toBe(true);
  });
});

describe("genericParameterValuesJson", () => {
  it("serializes a flat values object", () => {
    const json = genericParameterValuesJson({ theme: "retail", tone: "formal" });
    expect(JSON.parse(json)).toEqual({ theme: "retail", tone: "formal" });
  });

  it("serializes values with date range objects", () => {
    const values = {
      theme: "retail",
      date_range: { startDate: "2026-01-01", endDate: "2026-12-31" },
    };
    const json = genericParameterValuesJson(values);
    expect(JSON.parse(json)).toEqual(values);
  });
});

describe("selectOptions", () => {
  it("parses {value,label} object arrays (real template shape)", () => {
    const got = selectOptions(
      '[{"value":"pt-BR","label":"Portuguese"},{"value":"en-US","label":"English"}]',
    );
    expect(got).toEqual([
      { value: "pt-BR", label: "Portuguese" },
      { value: "en-US", label: "English" },
    ]);
  });

  it("parses plain string arrays", () => {
    expect(selectOptions('["a","b"]')).toEqual([
      { value: "a", label: "a" },
      { value: "b", label: "b" },
    ]);
  });

  it("drops empty and duplicate values so keyed each never collides", () => {
    const got = selectOptions('[{"value":"x"},{"value":"x"},{"value":""}]');
    expect(got).toEqual([{ value: "x", label: "x" }]);
  });

  it("returns [] on empty or invalid json", () => {
    expect(selectOptions("")).toEqual([]);
    expect(selectOptions("not json")).toEqual([]);
  });
});
