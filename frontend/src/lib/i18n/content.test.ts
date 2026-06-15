import { describe, expect, it } from "vitest";
import {
  resolveLocalizedContent,
  resolveLocalizedText,
} from "$lib/i18n/content";

describe("content i18n", () => {
  it("resolves locale-specific content keys", () => {
    expect(
      resolveLocalizedContent("catalog.sku.rss-news-feed.name", "pt-BR"),
    ).toBe("Feed de Noticias RSS");
  });

  it("falls back to english when pt-BR key is missing", () => {
    expect(
      resolveLocalizedContent(
        "catalog.agent.tool.calendar-write.description",
        "pt-BR",
      ),
    ).toBe("Propose and confirm calendar events.");
  });

  it("resolves localized text objects with fallback", () => {
    expect(resolveLocalizedText({ en: "Hello", "pt-BR": "Ola" }, "pt-BR")).toBe(
      "Ola",
    );
    expect(resolveLocalizedText({ en: "Hello" }, "pt-BR")).toBe("Hello");
  });
});
