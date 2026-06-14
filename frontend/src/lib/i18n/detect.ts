import type { Locale } from "./types";

const SUPPORTED_LOCALES: Locale[] = ["en", "pt-BR"];

export function normalizeLocaleTag(tag: string): Locale | null {
  const trimmed = tag.trim();
  if (!trimmed) return null;

  const lower = trimmed.toLowerCase();
  if (lower === "en" || lower.startsWith("en-")) return "en";
  if (lower === "pt-br" || lower.startsWith("pt-br")) return "pt-BR";
  if (lower === "pt" || lower.startsWith("pt-")) return "pt-BR";

  return null;
}

export function detectLocale(navigatorLanguage?: string): Locale {
  if (navigatorLanguage) {
    const normalized = normalizeLocaleTag(navigatorLanguage);
    if (normalized) return normalized;
  }

  return "en";
}

export function isSupportedLocale(value: string): value is Locale {
  return (SUPPORTED_LOCALES as readonly string[]).includes(value);
}
