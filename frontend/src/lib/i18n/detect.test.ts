import { get } from "svelte/store";
import { describe, expect, it } from "vitest";
import { detectLocale, normalizeLocaleTag } from "./detect";
import { locale, t, translate } from "./index";

describe("normalizeLocaleTag", () => {
  it("maps English tags to en", () => {
    expect(normalizeLocaleTag("en")).toBe("en");
    expect(normalizeLocaleTag("en-US")).toBe("en");
  });

  it("maps Portuguese tags to pt-BR", () => {
    expect(normalizeLocaleTag("pt")).toBe("pt-BR");
    expect(normalizeLocaleTag("pt-BR")).toBe("pt-BR");
    expect(normalizeLocaleTag("pt-PT")).toBe("pt-BR");
  });

  it("returns null for unsupported tags", () => {
    expect(normalizeLocaleTag("fr-FR")).toBeNull();
    expect(normalizeLocaleTag("")).toBeNull();
  });
});

describe("detectLocale", () => {
  it("prefers a supported navigator language", () => {
    expect(detectLocale("pt-BR")).toBe("pt-BR");
    expect(detectLocale("en-GB")).toBe("en");
  });

  it("falls back to en when language is unsupported", () => {
    expect(detectLocale("ja-JP")).toBe("en");
    expect(detectLocale(undefined)).toBe("en");
  });
});

describe("t", () => {
  it("reads from the active locale store", () => {
    locale.set("pt-BR");
    expect(t("nav.logout")).toBe("Sair");
    locale.set("en");
    expect(t("nav.logout")).toBe("Log out");
    expect(get(locale)).toBe("en");
  });
});

describe("translate", () => {
  it("returns localized strings", () => {
    expect(translate("nav.tasks", "en")).toBe("Tasks");
    expect(translate("nav.tasks", "pt-BR")).toBe("Tarefas");
  });

  it("interpolates params and falls back to en then key", () => {
    expect(translate("tasks.subtask.other", "en", { count: 3 })).toBe(
      "3 subtasks",
    );
    expect(translate("missing.key", "pt-BR")).toBe("missing.key");
  });
});
