import { describe, expect, it } from "vitest";
import { create } from "@bufbuild/protobuf";
import {
  PlanStepSchema,
  PlanTemplateSchema,
  TemplateInputParameterType,
  type PlanTemplate,
  type TemplateInputParameter,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import {
  localizedInputDescription,
  localizedInputLabel,
  localizedPlanDescription,
  localizedPlanDescriptionByKey,
  localizedPlanName,
  localizedPlanNameByKey,
  localizedStepDescription,
  localizedStepTitle,
  localizedStepTitleByKey,
} from "$lib/plans/catalog-i18n";

function step(key: string, title: string, description = "") {
  return create(PlanStepSchema, { key, title, description });
}

function template(
  key: string,
  name: string,
  steps: Array<{ key: string; title: string }> = [],
): PlanTemplate {
  return create(PlanTemplateSchema, {
    id: "tpl-1",
    key,
    name,
    steps: steps.map((s) => step(s.key, s.title)),
  });
}

function input(
  key: string,
  label: string,
  description = "Backend description",
): TemplateInputParameter {
  return {
    $typeName: "harpia.plans.v1.TemplateInputParameter",
    key,
    label,
    description,
    type: TemplateInputParameterType.TEXT,
    required: false,
    defaultValueJson: "",
    optionsJson: "",
    runtimeMappings: [],
  } as TemplateInputParameter;
}

describe("localizedPlanName", () => {
  it("returns the localized name when the catalog key exists", () => {
    const tpl = template("news-to-social-post", "Weekly Newsletter (LinkedIn)");
    expect(localizedPlanName(tpl, "pt-BR")).toBe(
      "Newsletter Semanal (LinkedIn)",
    );
    expect(localizedPlanName(tpl, "en")).toBe("Weekly Newsletter (LinkedIn)");
  });

  it("resolves candidate names by key with backend-string fallback", () => {
    expect(
      localizedPlanNameByKey(
        "news-to-social-post",
        "pt-BR",
        "Weekly Newsletter",
      ),
    ).toBe("Newsletter Semanal (LinkedIn)");
    expect(localizedPlanNameByKey("custom-plan", "pt-BR", "Custom")).toBe(
      "Custom",
    );
  });

  it("resolves plan descriptions with fallback", () => {
    const tpl = create(PlanTemplateSchema, {
      key: "news-to-social-post",
      name: "Weekly Newsletter",
      description: "Backend description",
    });

    expect(localizedPlanDescription(tpl, "en")).toBe(
      "Fetch news, write a draft, adapt for LinkedIn, and publish.",
    );
    expect(localizedPlanDescriptionByKey("unknown", "pt-BR", "Backend")).toBe(
      "Backend",
    );
  });

  it("falls back to template.name when the key is absent", () => {
    const tpl = template("unknown-template", "Custom Plan");
    expect(localizedPlanName(tpl, "pt-BR")).toBe("Custom Plan");
    expect(localizedPlanName(tpl, "en")).toBe("Custom Plan");
  });

  it("falls back to an empty template.key", () => {
    const tpl = template("", "Plan With No Key");
    expect(localizedPlanName(tpl, "pt-BR")).toBe("Plan With No Key");
  });
});

describe("localizedStepTitle", () => {
  it("returns the localized title when the catalog key exists", () => {
    const tpl = template("news-to-social-post", "News to Social Post", [
      { key: "fetch-news", title: "Fetch News" },
      { key: "write-draft", title: "Write Draft" },
    ]);

    expect(localizedStepTitle(tpl, "write-draft", "pt-BR")).toBe(
      "Escrever Rascunho",
    );
    expect(localizedStepTitle(tpl, "fetch-news", "pt-BR")).toBe(
      "Buscar Noticias",
    );
    expect(
      localizedStepTitleByKey(
        "news-to-social-post",
        "publish-linkedin",
        "en",
        "Backend Publish",
      ),
    ).toBe("Publish LinkedIn");
  });

  it("resolves localized step descriptions with backend fallback", () => {
    const tpl = create(PlanTemplateSchema, {
      key: "news-to-social-post",
      name: "Weekly Newsletter",
      steps: [step("fetch-news", "Fetch News", "Backend fetch")],
    });

    expect(localizedStepDescription(tpl, "fetch-news", "en")).toBe(
      "Collect curated articles for the configured date range.",
    );
    expect(localizedStepDescription(tpl, "missing", "pt-BR")).toBe("");
  });

  it("falls back to template step title when the catalog key is absent", () => {
    const tpl = template("unknown-template", "Custom Plan", [
      { key: "do-thing", title: "Do The Thing" },
    ]);

    expect(localizedStepTitle(tpl, "do-thing", "pt-BR")).toBe("Do The Thing");
  });

  it("falls back to the stepKey for an unknown step", () => {
    const tpl = template("news-to-social-post", "News to Social Post", [
      { key: "fetch-news", title: "Fetch News" },
    ]);

    expect(localizedStepTitle(tpl, "no-such-step", "pt-BR")).toBe(
      "no-such-step",
    );
  });

  it("falls back to the stepKey when both key and step are unknown", () => {
    const tpl = template("unknown-template", "Custom Plan");
    expect(localizedStepTitle(tpl, "mystery-step", "pt-BR")).toBe(
      "mystery-step",
    );
  });

  it("falls back to the template step title when template.key is empty", () => {
    const tpl = template("", "No Key Plan", [
      { key: "step-one", title: "Step One" },
    ]);
    expect(localizedStepTitle(tpl, "step-one", "pt-BR")).toBe("Step One");
  });

  it("localizes the news-digest-draft template steps", () => {
    const tpl = template("news-digest-draft", "News Digest Draft", [
      { key: "fetch-news", title: "Fetch News" },
      { key: "write-draft", title: "Write Draft" },
    ]);

    expect(localizedPlanName(tpl, "pt-BR")).toBe(
      "Rascunho de Resumo de Noticias",
    );
    expect(localizedStepTitle(tpl, "write-draft", "pt-BR")).toBe(
      "Escrever Rascunho",
    );
  });
});

describe("localized input metadata", () => {
  it("resolves flat template input labels and descriptions", () => {
    expect(localizedInputLabel(input("source_group", "Source"), "pt-BR")).toBe(
      "Grupo de fontes",
    );
    expect(
      localizedInputDescription(input("approval_mode", "Approval"), "en"),
    ).toBe("Whether publishing requires your approval first.");
  });

  it("falls back to backend input metadata when no key exists", () => {
    const parameter = input("custom_field", "Custom label", "Custom help");

    expect(localizedInputLabel(parameter, "pt-BR")).toBe("Custom label");
    expect(localizedInputDescription(parameter, "pt-BR")).toBe("Custom help");
  });
});
