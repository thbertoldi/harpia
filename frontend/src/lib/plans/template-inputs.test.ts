import { describe, expect, it } from "vitest";
import {
  genericInputInitialValues,
  genericParameterValuesJson,
  selectOptions,
} from "./template-inputs";
import type { TemplateInputParameter } from "$lib/gen/harpia/plans/v1/plans_pb";
import { TemplateInputParameterType } from "$lib/gen/harpia/plans/v1/plans_pb";

function param(
  key: string,
  type: TemplateInputParameterType,
  defaultValueJson = "",
): TemplateInputParameter {
  return {
    $typeName: "harpia.plans.v1.TemplateInputParameter",
    key,
    label: key,
    description: "",
    type,
    required: false,
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
});

describe("genericParameterValuesJson", () => {
  it("serializes a flat values object", () => {
    const json = genericParameterValuesJson({ theme: "retail", tone: "formal" });
    expect(JSON.parse(json)).toEqual({ theme: "retail", tone: "formal" });
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
