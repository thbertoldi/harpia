import { get, writable } from "svelte/store";
import en from "./en.json";
import ptBR from "./pt-BR.json";
import { detectLocale, isSupportedLocale } from "./detect";
import { LOCALE_STORAGE_KEY, type Locale } from "./types";

export { detectLocale, isSupportedLocale, normalizeLocaleTag } from "./detect";
export { LOCALE_STORAGE_KEY, SUPPORTED_LOCALES, type Locale } from "./types";

const catalogs: Record<Locale, Record<string, string>> = {
  en,
  "pt-BR": ptBR,
};

export const locale = writable<Locale>("en");

export function initLocale(): void {
  if (typeof localStorage !== "undefined") {
    const stored = localStorage.getItem(LOCALE_STORAGE_KEY);
    if (stored && isSupportedLocale(stored)) {
      locale.set(stored);
      return;
    }
  }

  const detected = detectLocale(
    typeof navigator !== "undefined" ? navigator.language : undefined,
  );
  locale.set(detected);
}

export function setLocale(next: Locale): void {
  locale.set(next);
  if (typeof localStorage !== "undefined") {
    localStorage.setItem(LOCALE_STORAGE_KEY, next);
  }
}

export function translate(
  key: string,
  loc: Locale,
  params?: Record<string, string | number>,
): string {
  const dict = catalogs[loc] ?? catalogs.en;
  let text = dict[key] ?? catalogs.en[key] ?? key;

  if (params) {
    for (const [name, value] of Object.entries(params)) {
      text = text.replace(new RegExp(`\\{${name}\\}`, "g"), String(value));
    }
  }

  return text;
}

/** Non-reactive shorthand; prefer translate(key, $locale) in Svelte templates. */
export function t(
  key: string,
  params?: Record<string, string | number>,
): string {
  return translate(key, get(locale), params);
}

export function translateRole(role: string, loc: Locale): string {
  const key = `role.${role}`;
  const translated = translate(key, loc);
  return translated !== key ? translated : role;
}
