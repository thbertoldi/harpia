import { describe, expect, it } from "vitest";
import { create } from "@bufbuild/protobuf";
import { PlanTemplateSchema } from "$lib/gen/harpia/plans/v1/plans_pb";
import {
  DEFAULT_ICON_KEY,
  MAX_SUGGESTIONS,
  suggestionForTemplate,
  verticalIconKey,
} from "$lib/plans/suggestions";

function template(opts: { key?: string; name?: string; vertical?: string }) {
  return create(PlanTemplateSchema, {
    key: opts.key ?? "t",
    name: opts.name ?? "T",
    vertical: opts.vertical ?? "",
  });
}

describe("verticalIconKey", () => {
  it("maps known verticals deterministically", () => {
    expect(verticalIconKey("social-media")).toBe("share");
    expect(verticalIconKey("LinkedIn")).toBe("share");
    expect(verticalIconKey("news")).toBe("newspaper");
    expect(verticalIconKey("rss")).toBe("newspaper");
    expect(verticalIconKey("content-marketing")).toBe("edit");
    expect(verticalIconKey("Writing")).toBe("edit");
    expect(verticalIconKey("analytics")).toBe("chart");
    expect(verticalIconKey("data")).toBe("chart");
    expect(verticalIconKey("marketing")).toBe("megaphone");
  });

  it("falls back to the default for unknown or empty verticals", () => {
    expect(verticalIconKey("something-new")).toBe(DEFAULT_ICON_KEY);
    expect(verticalIconKey("")).toBe(DEFAULT_ICON_KEY);
    expect(verticalIconKey(undefined)).toBe(DEFAULT_ICON_KEY);
    expect(verticalIconKey("   ")).toBe(DEFAULT_ICON_KEY);
  });
});

describe("suggestionForTemplate", () => {
  it("uses the catalog suggestion prompt when the content key exists", () => {
    const s = suggestionForTemplate(
      template({
        key: "news-to-social-post",
        name: "Weekly Newsletter (LinkedIn)",
        vertical: "social-media",
      }),
      "en",
    );
    expect(s.templateKey).toBe("news-to-social-post");
    expect(s.iconKey).toBe("share");
    expect(s.label).toBe("Weekly Newsletter (LinkedIn)");
    // Resolves to the localized suggestion content key (not the composed fallback)
    expect(s.prompt).toBe(
      "Set up a weekly LinkedIn newsletter from the latest news",
    );
  });

  it("falls back to a composed prompt when no suggestion key exists", () => {
    const s = suggestionForTemplate(
      template({ key: "brand-new-template", name: "Brand New", vertical: "" }),
      "en",
    );
    expect(s.prompt).toBe("Create a Brand New");
    expect(s.iconKey).toBe(DEFAULT_ICON_KEY);
  });

  it("localizes the composed fallback per locale", () => {
    const s = suggestionForTemplate(
      template({ key: "brand-new-template", name: "Brand New", vertical: "" }),
      "pt-BR",
    );
    expect(s.prompt).toBe("Criar um Brand New");
  });

  it("uses the key as the label when the name is blank", () => {
    const s = suggestionForTemplate(
      template({ key: "k", name: "   ", vertical: "" }),
      "en",
    );
    expect(s.label).toBe("k");
  });

  it("caps nothing here — MAX_SUGGESTIONS is exported for callers", () => {
    expect(MAX_SUGGESTIONS).toBe(4);
  });
});
