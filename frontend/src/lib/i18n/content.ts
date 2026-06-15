import type { Locale } from "$lib/i18n/types";
import en from "./content/en.json";
import ptBR from "./content/pt-BR.json";

export type LocalizedText = Partial<Record<Locale, string>>;

type ContentCatalog = Record<string, string>;

const contentCatalogs: Record<Locale, ContentCatalog> = {
  en,
  "pt-BR": ptBR,
};

export function resolveLocalizedText(
  value: LocalizedText,
  locale: Locale,
  fallbackLocale: Locale = "en",
): string {
  return value[locale] ?? value[fallbackLocale] ?? "";
}

export function resolveLocalizedContent(
  key: string,
  locale: Locale,
  fallbackLocale: Locale = "en",
): string {
  const primaryCatalog = contentCatalogs[locale] ?? contentCatalogs.en;
  const fallbackCatalog = contentCatalogs[fallbackLocale] ?? contentCatalogs.en;
  return primaryCatalog[key] ?? fallbackCatalog[key] ?? key;
}
