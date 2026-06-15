import type { Locale } from "./types";

function toIntlLocale(locale: Locale): string {
  return locale === "pt-BR" ? "pt-BR" : "en-US";
}

export function formatLocaleDate(
  value: string | Date,
  locale: Locale,
  options: Intl.DateTimeFormatOptions = {
    month: "short",
    day: "numeric",
    year: "numeric",
  },
): string {
  const date = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(date.getTime())) {
    return typeof value === "string" ? value : "";
  }
  return date.toLocaleDateString(toIntlLocale(locale), options);
}

export function formatLocaleDateTime(
  value: string | Date,
  locale: Locale,
  options: Intl.DateTimeFormatOptions = {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  },
): string {
  const date = value instanceof Date ? value : new Date(value);
  if (Number.isNaN(date.getTime())) {
    return typeof value === "string" ? value : "";
  }
  return date.toLocaleString(toIntlLocale(locale), options);
}
