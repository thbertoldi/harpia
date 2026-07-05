import { describe, expect, it } from "vitest";
import { create } from "@bufbuild/protobuf";
import {
  PlanStepSchema,
  PlanTemplateSchema,
  type PlanTemplate,
} from "$lib/gen/harpia/plans/v1/plans_pb";
import { localizedPlanName, localizedStepTitle } from "$lib/plans/catalog-i18n";

function step(key: string, title: string) {
  return create(PlanStepSchema, { key, title });
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

describe("localizedPlanName", () => {
  it("returns the localized name when the catalog key exists", () => {
    const tpl = template(
      "weekly-newsletter-linkedin",
      "Weekly Newsletter (LinkedIn)",
    );
    expect(localizedPlanName(tpl, "pt-BR")).toBe(
      "Newsletter Semanal (LinkedIn)",
    );
    expect(localizedPlanName(tpl, "en")).toBe("Weekly Newsletter (LinkedIn)");
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
    const tpl = template("weekly-newsletter-linkedin", "Weekly Newsletter", [
      { key: "fetch-news", title: "Fetch News" },
      { key: "write-draft", title: "Write Draft" },
    ]);

    expect(localizedStepTitle(tpl, "write-draft", "pt-BR")).toBe(
      "Escrever Rascunho",
    );
    expect(localizedStepTitle(tpl, "fetch-news", "pt-BR")).toBe(
      "Buscar Noticias",
    );
  });

  it("falls back to template step title when the catalog key is absent", () => {
    const tpl = template("unknown-template", "Custom Plan", [
      { key: "do-thing", title: "Do The Thing" },
    ]);

    expect(localizedStepTitle(tpl, "do-thing", "pt-BR")).toBe("Do The Thing");
  });

  it("falls back to the stepKey for an unknown step", () => {
    const tpl = template("weekly-newsletter-linkedin", "Weekly Newsletter", [
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
